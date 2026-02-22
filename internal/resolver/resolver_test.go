package resolver_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nexus-protocol/nexus/internal/resolver"
	resolverv1 "github.com/nexus-protocol/nexus/api/proto/resolver/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// stubRegistry starts an httptest server that behaves like the real registry.
func stubRegistry(t *testing.T, agents map[string]map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		key := fmt.Sprintf("%s/%s/%s",
			q.Get("trust_root"),
			q.Get("capability_node"),
			q.Get("agent_id"),
		)

		agent, ok := agents[key]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "agent not found"}) //nolint:errcheck
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
			"uri":         "agent://" + key,
			"endpoint":    agent["endpoint"],
			"status":      agent["status"],
			"cert_serial": agent["cert_serial"],
		})
	}))
}

// newTestService creates a resolver Service pointed at a stub registry.
func newTestService(t *testing.T, srv *httptest.Server, cacheTTL time.Duration) *resolver.Service {
	t.Helper()
	// Strip "http://" prefix — Config.RegistryAddr is host:port
	addr := srv.Listener.Addr().String()
	cfg := resolver.Config{
		RegistryAddr: addr,
		CacheTTL:     cacheTTL,
		HTTPTimeout:  2 * time.Second,
	}
	return resolver.New(cfg, zap.NewNop())
}

// ── Resolve ───────────────────────────────────────────────────────────────────

func TestResolve_found(t *testing.T) {
	reg := stubRegistry(t, map[string]map[string]string{
		"nexusagentprotocol.com/finance/taxes/agent_abc": {
			"endpoint":    "https://agent.example.com/finance",
			"status":      "active",
			"cert_serial": "deadbeef",
		},
	})
	defer reg.Close()

	svc := newTestService(t, reg, 0) // no caching

	resp, err := svc.Resolve(context.Background(), &resolverv1.ResolveRequest{
		TrustRoot:      "nexusagentprotocol.com",
		CapabilityNode: "finance/taxes",
		AgentId:        "agent_abc",
	})
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}

	if resp.Endpoint != "https://agent.example.com/finance" {
		t.Errorf("Endpoint: got %q", resp.Endpoint)
	}
	if resp.Status != "active" {
		t.Errorf("Status: got %q", resp.Status)
	}
	if resp.Uri != "agent://nexusagentprotocol.com/finance/taxes/agent_abc" {
		t.Errorf("URI: got %q", resp.Uri)
	}
	if resp.CertSerial != "deadbeef" {
		t.Errorf("CertSerial: got %q", resp.CertSerial)
	}
}

func TestResolve_notFound(t *testing.T) {
	reg := stubRegistry(t, map[string]map[string]string{})
	defer reg.Close()

	svc := newTestService(t, reg, 0)

	_, err := svc.Resolve(context.Background(), &resolverv1.ResolveRequest{
		TrustRoot:      "nexusagentprotocol.com",
		CapabilityNode: "finance/taxes",
		AgentId:        "agent_missing",
	})
	if err == nil {
		t.Fatal("expected error for missing agent, got nil")
	}

	st, _ := status.FromError(err)
	if st.Code() != codes.NotFound {
		t.Errorf("expected NotFound, got %v", st.Code())
	}
}

func TestResolve_missingFields(t *testing.T) {
	svc := resolver.New(resolver.Config{RegistryAddr: "localhost:9999"}, zap.NewNop())

	cases := []struct {
		name string
		req  *resolverv1.ResolveRequest
	}{
		{"empty trust_root", &resolverv1.ResolveRequest{CapabilityNode: "a", AgentId: "agent_x"}},
		{"empty capability_node", &resolverv1.ResolveRequest{TrustRoot: "nexusagentprotocol.com", AgentId: "agent_x"}},
		{"empty agent_id", &resolverv1.ResolveRequest{TrustRoot: "nexusagentprotocol.com", CapabilityNode: "a"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Resolve(context.Background(), tc.req)
			if err == nil {
				t.Error("expected InvalidArgument error, got nil")
			}
			st, _ := status.FromError(err)
			if st.Code() != codes.InvalidArgument {
				t.Errorf("expected InvalidArgument, got %v", st.Code())
			}
		})
	}
}

// ── Cache ─────────────────────────────────────────────────────────────────────

func TestResolve_cacheHit(t *testing.T) {
	callCount := 0
	reg := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
			"uri":      "agent://nexusagentprotocol.com/a/agent_x",
			"endpoint": "https://cached.example.com",
			"status":   "active",
		})
	}))
	defer reg.Close()

	svc := newTestService(t, reg, time.Minute) // cache enabled
	req := &resolverv1.ResolveRequest{TrustRoot: "nexusagentprotocol.com", CapabilityNode: "a", AgentId: "agent_x"}

	// First call: cache miss → registry query
	if _, err := svc.Resolve(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	// Second call: cache hit → no registry query
	if _, err := svc.Resolve(context.Background(), req); err != nil {
		t.Fatal(err)
	}

	if callCount != 1 {
		t.Errorf("registry called %d times, expected 1 (second call should be a cache hit)", callCount)
	}
}

func TestResolve_cacheInvalidate(t *testing.T) {
	callCount := 0
	reg := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
			"uri": "agent://nexusagentprotocol.com/a/agent_x", "endpoint": "https://e.example.com", "status": "active",
		})
	}))
	defer reg.Close()

	svc := newTestService(t, reg, time.Minute)
	req := &resolverv1.ResolveRequest{TrustRoot: "nexusagentprotocol.com", CapabilityNode: "a", AgentId: "agent_x"}

	if _, err := svc.Resolve(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	svc.Invalidate("nexusagentprotocol.com", "a", "agent_x")

	if _, err := svc.Resolve(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if callCount != 2 {
		t.Errorf("expected 2 registry calls after invalidation, got %d", callCount)
	}
}

// ── ResolveMany ───────────────────────────────────────────────────────────────

func TestResolveMany(t *testing.T) {
	reg := stubRegistry(t, map[string]map[string]string{
		"nexusagentprotocol.com/a/agent_1": {"endpoint": "https://a1.example.com", "status": "active"},
		"nexusagentprotocol.com/b/agent_2": {"endpoint": "https://b2.example.com", "status": "active"},
	})
	defer reg.Close()

	svc := newTestService(t, reg, 0)

	resp, err := svc.ResolveMany(context.Background(), &resolverv1.ResolveManyRequest{
		Requests: []*resolverv1.ResolveRequest{
			{TrustRoot: "nexusagentprotocol.com", CapabilityNode: "a", AgentId: "agent_1"},
			{TrustRoot: "nexusagentprotocol.com", CapabilityNode: "b", AgentId: "agent_2"},
			{TrustRoot: "nexusagentprotocol.com", CapabilityNode: "x", AgentId: "agent_missing"}, // will error
		},
	})
	if err != nil {
		t.Fatalf("ResolveMany() error: %v", err)
	}

	if len(resp.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(resp.Results))
	}
	if resp.Results[0].Error != "" {
		t.Errorf("result[0] unexpected error: %s", resp.Results[0].Error)
	}
	if resp.Results[1].Error != "" {
		t.Errorf("result[1] unexpected error: %s", resp.Results[1].Error)
	}
	if resp.Results[2].Error == "" {
		t.Error("result[2] expected error for missing agent, got none")
	}
}

func TestResolveMany_empty(t *testing.T) {
	svc := resolver.New(resolver.Config{RegistryAddr: "localhost:9999"}, zap.NewNop())
	resp, err := svc.ResolveMany(context.Background(), &resolverv1.ResolveManyRequest{})
	if err != nil {
		t.Fatalf("empty batch should succeed: %v", err)
	}
	if len(resp.Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(resp.Results))
	}
}

func TestResolveMany_batchTooLarge(t *testing.T) {
	svc := resolver.New(resolver.Config{RegistryAddr: "localhost:9999"}, zap.NewNop())
	reqs := make([]*resolverv1.ResolveRequest, 101)
	for i := range reqs {
		reqs[i] = &resolverv1.ResolveRequest{TrustRoot: "t", CapabilityNode: "c", AgentId: fmt.Sprintf("agent_%d", i)}
	}
	_, err := svc.ResolveMany(context.Background(), &resolverv1.ResolveManyRequest{Requests: reqs})
	if err == nil {
		t.Error("expected error for batch > 100, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", st.Code())
	}
}

// ── End-to-end gRPC client/server ─────────────────────────────────────────────

func TestGRPCEndToEnd(t *testing.T) {
	reg := stubRegistry(t, map[string]map[string]string{
		"nexusagentprotocol.com/finance/agent_e2e": {
			"endpoint": "https://e2e.example.com",
			"status":   "active",
		},
	})
	defer reg.Close()

	// Start a real gRPC server on a random port
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	addr := lis.Addr().String()
	svc := newTestService(t, reg, time.Minute)

	grpcSrv := grpc.NewServer()
	resolverv1.RegisterResolverServiceServer(grpcSrv, svc)

	go grpcSrv.Serve(lis) //nolint:errcheck
	t.Cleanup(func() { grpcSrv.GracefulStop() })

	// Connect a gRPC client
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	client := resolverv1.NewResolverServiceClient(conn)

	resp, err := client.Resolve(context.Background(), &resolverv1.ResolveRequest{
		TrustRoot:      "nexusagentprotocol.com",
		CapabilityNode: "finance",
		AgentId:        "agent_e2e",
	})
	if err != nil {
		t.Fatalf("gRPC Resolve() error: %v", err)
	}
	if resp.Endpoint != "https://e2e.example.com" {
		t.Errorf("Endpoint: got %q", resp.Endpoint)
	}
}
