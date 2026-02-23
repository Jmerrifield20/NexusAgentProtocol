// Package resolver implements the Nexus gRPC resolver service.
//
// The resolver translates agent:// URIs into live transport endpoints by
// querying the registry over HTTP. Results are cached in-memory with a
// configurable TTL to minimise latency and registry load.
package resolver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	resolverv1 "github.com/jmerrifield20/NexusAgentProtocol/api/proto/resolver/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Config holds resolver service configuration.
type Config struct {
	RegistryAddr string        // e.g. "localhost:8080" or "https://registry.nexusagentprotocol.com"
	CacheTTL     time.Duration // 0 disables caching
	HTTPTimeout  time.Duration // default 5s
}

// Service implements resolverv1.ResolverServiceServer.
type Service struct {
	resolverv1.UnimplementedResolverServiceServer

	cfg        Config
	httpClient *http.Client
	cache      *resolverCache
	logger     *zap.Logger
}

// New creates a resolver Service.
func New(cfg Config, logger *zap.Logger) *Service {
	timeout := cfg.HTTPTimeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	svc := &Service{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: timeout},
		logger:     logger,
	}

	if cfg.CacheTTL > 0 {
		svc.cache = newResolverCache(cfg.CacheTTL)
	}

	return svc
}

// Resolve implements ResolverServiceServer.Resolve.
//
// It translates a single agent:// URI into its transport endpoint by:
//  1. Checking the in-memory cache
//  2. Querying the registry HTTP API on a cache miss
//  3. Caching the result and returning it
func (s *Service) Resolve(ctx context.Context, req *resolverv1.ResolveRequest) (*resolverv1.ResolveResponse, error) {
	if err := validateRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	cacheKey := buildCacheKey(req.TrustRoot, req.CapabilityNode, req.AgentId)

	// Cache hit
	if s.cache != nil {
		if entry, ok := s.cache.get(cacheKey); ok {
			s.logger.Debug("cache hit", zap.String("key", cacheKey))
			return &resolverv1.ResolveResponse{
				Uri:        buildURI(req),
				Endpoint:   entry.endpoint,
				Status:     entry.status,
				CertSerial: entry.certSerial,
			}, nil
		}
	}

	// Registry lookup
	regResult, err := s.queryRegistry(ctx, req)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if s.cache != nil {
		s.cache.set(cacheKey, regResult.Endpoint, regResult.Status, regResult.CertSerial)
	}

	s.logger.Info("resolved",
		zap.String("uri", regResult.URI),
		zap.String("endpoint", regResult.Endpoint),
		zap.String("status", regResult.Status),
	)

	return &resolverv1.ResolveResponse{
		Uri:        regResult.URI,
		Endpoint:   regResult.Endpoint,
		Status:     regResult.Status,
		CertSerial: regResult.CertSerial,
	}, nil
}

// ResolveMany implements ResolverServiceServer.ResolveMany.
//
// It fans out concurrent Resolve calls for each request in the batch,
// collecting results and errors independently so a single failure does
// not abort the entire batch.
func (s *Service) ResolveMany(ctx context.Context, req *resolverv1.ResolveManyRequest) (*resolverv1.ResolveManyResponse, error) {
	if len(req.Requests) == 0 {
		return &resolverv1.ResolveManyResponse{}, nil
	}
	if len(req.Requests) > 100 {
		return nil, status.Error(codes.InvalidArgument, "batch size must not exceed 100")
	}

	type indexedResult struct {
		idx    int
		result *resolverv1.ResolveResult
	}

	resultCh := make(chan indexedResult, len(req.Requests))

	for i, r := range req.Requests {
		i, r := i, r
		go func() {
			resp, err := s.Resolve(ctx, r)
			res := &resolverv1.ResolveResult{Request: r}
			if err != nil {
				res.Error = err.Error()
			} else {
				res.Response = resp
			}
			resultCh <- indexedResult{idx: i, result: res}
		}()
	}

	results := make([]*resolverv1.ResolveResult, len(req.Requests))
	for range req.Requests {
		ir := <-resultCh
		results[ir.idx] = ir.result
	}

	return &resolverv1.ResolveManyResponse{Results: results}, nil
}

// Invalidate removes a URI from the cache. Called when an agent is revoked or updated.
func (s *Service) Invalidate(trustRoot, capNode, agentID string) {
	if s.cache != nil {
		s.cache.invalidate(buildCacheKey(trustRoot, capNode, agentID))
	}
}

// CacheStats returns current cache size (for metrics/health).
func (s *Service) CacheStats() int {
	if s.cache == nil {
		return 0
	}
	return s.cache.len()
}

// StartCacheEviction starts a background goroutine that periodically evicts
// expired cache entries. Call cancel to stop it.
func (s *Service) StartCacheEviction(ctx context.Context, interval time.Duration) {
	if s.cache == nil {
		return
	}
	if interval == 0 {
		interval = time.Minute
	}
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				n := s.cache.evict()
				if n > 0 {
					s.logger.Debug("cache eviction", zap.Int("evicted", n))
				}
			}
		}
	}()
}

// registryResolveResult mirrors the JSON response from GET /api/v1/resolve.
type registryResolveResult struct {
	URI        string `json:"uri"`
	Endpoint   string `json:"endpoint"`
	Status     string `json:"status"`
	CertSerial string `json:"cert_serial,omitempty"`
	Error      string `json:"error,omitempty"`
}

// queryRegistry calls the registry HTTP API to resolve an agent URI.
func (s *Service) queryRegistry(ctx context.Context, req *resolverv1.ResolveRequest) (*registryResolveResult, error) {
	url := fmt.Sprintf(
		"http://%s/api/v1/resolve?trust_root=%s&capability_node=%s&agent_id=%s",
		s.cfg.RegistryAddr,
		req.TrustRoot,
		req.CapabilityNode,
		req.AgentId,
	)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "build registry request: %v", err)
	}
	httpReq.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		s.logger.Error("registry request failed",
			zap.String("url", url),
			zap.Error(err),
		)
		return nil, status.Errorf(codes.Unavailable, "registry unreachable: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read registry response: %v", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		// handled below
	case http.StatusNotFound:
		return nil, status.Errorf(codes.NotFound, "agent not found: %s/%s/%s",
			req.TrustRoot, req.CapabilityNode, req.AgentId)
	case http.StatusBadRequest:
		return nil, status.Errorf(codes.InvalidArgument, "registry rejected request: %s", string(body))
	default:
		return nil, status.Errorf(codes.Internal, "registry error %d: %s", resp.StatusCode, string(body))
	}

	var result registryResolveResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, status.Errorf(codes.Internal, "decode registry response: %v", err)
	}
	if result.Error != "" {
		return nil, status.Errorf(codes.FailedPrecondition, "registry: %s", result.Error)
	}
	return &result, nil
}

// validateRequest checks that all required URI components are present.
func validateRequest(req *resolverv1.ResolveRequest) error {
	if req.TrustRoot == "" {
		return fmt.Errorf("trust_root is required")
	}
	if req.CapabilityNode == "" {
		return fmt.Errorf("capability_node is required")
	}
	if req.AgentId == "" {
		return fmt.Errorf("agent_id is required")
	}
	return nil
}

// buildCacheKey constructs a canonical cache key for a URI.
func buildCacheKey(trustRoot, capNode, agentID string) string {
	return "agent://" + trustRoot + "/" + capNode + "/" + agentID
}

// buildURI constructs the agent:// URI string from a request.
func buildURI(req *resolverv1.ResolveRequest) string {
	return buildCacheKey(req.TrustRoot, req.CapabilityNode, req.AgentId)
}
