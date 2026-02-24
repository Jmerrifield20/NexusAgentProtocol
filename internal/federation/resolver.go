package federation

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/jmerrifield20/NexusAgentProtocol/internal/registry/model"
	"go.uber.org/zap"
)

// RemoteResolver implements service.RemoteResolver by querying federated registries.
// Discovery priority:
//  1. Federation table lookup (fedSvc.GetByTrustRoot)
//  2. DNS TXT record: _nap-registry.{trustRoot} → "v=nap1 url=https://..."
//  3. Configured rootRegistryURL fallback
type RemoteResolver struct {
	fedSvc          *FederationService
	rootRegistryURL string
	dnsEnabled      bool
	timeout         time.Duration
	logger          *zap.Logger

	// dnsDiscoverFn is the DNS discovery function. Defaults to dnsDiscover.
	// Tests can override this to avoid real DNS lookups.
	dnsDiscoverFn func(trustRoot string) (string, bool)
}

// NewRemoteResolver creates a RemoteResolver.
// fedSvc and rootRegistryURL may be zero-valued to skip those discovery paths.
func NewRemoteResolver(
	fedSvc *FederationService,
	rootRegistryURL string,
	dnsEnabled bool,
	timeout time.Duration,
	logger *zap.Logger,
) *RemoteResolver {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	rr := &RemoteResolver{
		fedSvc:          fedSvc,
		rootRegistryURL: rootRegistryURL,
		dnsEnabled:      dnsEnabled,
		timeout:         timeout,
		logger:          logger,
	}
	rr.dnsDiscoverFn = rr.dnsDiscover
	return rr
}

// Resolve attempts to find an agent on a remote registry.
// It returns *model.Agent populated from the remote response, or an error if
// the agent could not be found through any discovery path.
func (r *RemoteResolver) Resolve(ctx context.Context, trustRoot, capNode, agentID string) (*model.Agent, error) {
	endpointURL, err := r.discoverEndpoint(ctx, trustRoot)
	if err != nil {
		return nil, fmt.Errorf("discover registry for %q: %w", trustRoot, err)
	}

	client := NewRegistryClient(endpointURL, r.timeout)
	remote, err := client.Resolve(ctx, trustRoot, capNode, agentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("agent not found on remote registry %s", endpointURL)
		}
		return nil, fmt.Errorf("remote resolve from %s: %w", endpointURL, err)
	}

	// Map the remote response to a local model.Agent for uniform handling.
	agent := &model.Agent{
		AgentID:        remote.AgentID,
		TrustRoot:      remote.TrustRoot,
		CapabilityNode: remote.CapabilityNode,
		DisplayName:    remote.DisplayName,
		Description:    remote.Description,
		Endpoint:       remote.Endpoint,
		Status:         model.AgentStatus(remote.Status),
		Version:        remote.Version,
	}

	r.logger.Info("remote resolve succeeded",
		zap.String("trust_root", trustRoot),
		zap.String("agent_id", agentID),
		zap.String("registry", endpointURL),
	)
	return agent, nil
}

// discoverEndpoint finds the HTTP base URL of the registry responsible for trustRoot.
func (r *RemoteResolver) discoverEndpoint(ctx context.Context, trustRoot string) (string, error) {
	// 1. Federation table lookup.
	if r.fedSvc != nil {
		reg, err := r.fedSvc.GetByTrustRoot(ctx, trustRoot)
		if err == nil && reg.Status == StatusActive {
			return reg.EndpointURL, nil
		}
	}

	// 2. DNS TXT discovery.
	if r.dnsEnabled {
		if url, ok := r.dnsDiscoverFn(trustRoot); ok {
			// When fedSvc is available (root mode), require the trust root
			// to be approved in the federation table before accepting DNS results.
			if r.fedSvc != nil {
				reg, err := r.fedSvc.GetByTrustRoot(ctx, trustRoot)
				if err != nil || reg.Status != StatusActive {
					r.logger.Warn("DNS discovery rejected: trust root not approved",
						zap.String("trust_root", trustRoot),
						zap.String("dns_url", url),
					)
					// fall through to step 3
				} else {
					return url, nil
				}
			} else {
				return url, nil
			}
		}
	}

	// 3. Root registry fallback.
	if r.rootRegistryURL != "" {
		return r.rootRegistryURL, nil
	}

	return "", fmt.Errorf("no registry endpoint found for trust_root %q", trustRoot)
}

// dnsDiscover looks up _nap-registry.{trustRoot} TXT records.
// Expected format: "v=nap1 url=https://registry.example.com"
func (r *RemoteResolver) dnsDiscover(trustRoot string) (string, bool) {
	host := "_nap-registry." + trustRoot
	txts, err := net.LookupTXT(host)
	if err != nil {
		return "", false
	}
	for _, txt := range txts {
		if !strings.HasPrefix(txt, "v=nap1 ") {
			continue
		}
		for _, part := range strings.Fields(txt) {
			if strings.HasPrefix(part, "url=") {
				return strings.TrimPrefix(part, "url="), true
			}
		}
	}
	return "", false
}
