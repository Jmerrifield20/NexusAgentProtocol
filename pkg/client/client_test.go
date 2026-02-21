package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nexus-protocol/nexus/pkg/client"
)

// ── Stub server ─────────────────────────────────────────────────────────

func stubRegistryServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/resolve", func(w http.ResponseWriter, r *http.Request) {
		tr := r.URL.Query().Get("trust_root")
		cn := r.URL.Query().Get("capability_node")
		aid := r.URL.Query().Get("agent_id")
		if tr == "" || cn == "" || aid == "" {
			http.Error(w, `{"error":"missing params"}`, http.StatusBadRequest)
			return
		}
		if aid == "not_found" {
			http.Error(w, `{"error":"agent not found"}`, http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"id":          "550e8400-e29b-41d4-a716-446655440000",
			"uri":         "agent://" + tr + "/" + cn + "/" + aid,
			"endpoint":    "https://agent.example.com",
			"status":      "active",
			"cert_serial": "abc123",
		})
	})

	mux.HandleFunc("/api/v1/agents", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{
				"id":              "550e8400-e29b-41d4-a716-446655440000",
				"trust_root":      "nexus.io",
				"capability_node": "finance/taxes",
				"agent_id":        "agent_test123",
				"status":          "pending",
			})
		case http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{
				"agents": []map[string]any{
					{"id": "550e8400-e29b-41d4-a716-446655440000", "trust_root": "nexus.io", "status": "active"},
				},
			})
		}
	})

	mux.HandleFunc("/api/v1/agents/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if strings.HasSuffix(path, "/activate") {
			json.NewEncoder(w).Encode(map[string]any{
				"agent": map[string]any{
					"trust_root":      "nexus.io",
					"capability_node": "finance/taxes",
					"agent_id":        "agent_test123",
				},
				"certificate": map[string]any{
					"pem": "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----",
				},
				"private_key_pem": "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----",
				"ca_pem":          "-----BEGIN CERTIFICATE-----\nca\n-----END CERTIFICATE-----",
			})
			return
		}

		if strings.HasSuffix(path, "/revoke") {
			auth := r.Header.Get("Authorization")
			if auth == "" {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"status": "revoked"})
			return
		}

		// GET /api/v1/agents/:id
		parts := strings.Split(strings.TrimPrefix(path, "/api/v1/agents/"), "/")
		id := parts[0]
		if id == "not-found-id" {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"id":              id,
			"trust_root":      "nexus.io",
			"capability_node": "finance/taxes",
			"agent_id":        "agent_test123",
			"display_name":    "Test Agent",
			"endpoint":        "https://agent.example.com",
			"owner_domain":    "example.com",
			"status":          "active",
		})
	})

	mux.HandleFunc("/api/v1/token", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "test-jwt-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	})

	mux.HandleFunc("/v1/resolve", func(w http.ResponseWriter, r *http.Request) {
		tr := r.URL.Query().Get("trust_root")
		cn := r.URL.Query().Get("capability_node")
		aid := r.URL.Query().Get("agent_id")
		json.NewEncoder(w).Encode(map[string]any{
			"uri":      "agent://" + tr + "/" + cn + "/" + aid,
			"endpoint": "https://resolver.example.com",
			"status":   "active",
		})
	})

	return httptest.NewServer(mux)
}

// ── Tests ────────────────────────────────────────────────────────────────

func TestResolve_success(t *testing.T) {
	srv := stubRegistryServer(t)
	defer srv.Close()

	c, err := client.New(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	result, err := c.Resolve(context.Background(), "agent://nexus.io/finance/taxes/agent_abc")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if result.Endpoint != "https://agent.example.com" {
		t.Errorf("unexpected endpoint: %s", result.Endpoint)
	}
	if result.Status != "active" {
		t.Errorf("unexpected status: %s", result.Status)
	}
}

func TestResolve_notFound(t *testing.T) {
	srv := stubRegistryServer(t)
	defer srv.Close()

	c, _ := client.New(srv.URL)
	_, err := c.Resolve(context.Background(), "agent://nexus.io/finance/taxes/not_found")
	if err == nil {
		t.Error("expected error for not-found agent")
	}
}

func TestResolve_cache(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode(map[string]any{
			"uri":      "agent://nexus.io/x/agent_1",
			"endpoint": "https://cached.example.com",
			"status":   "active",
		})
	}))
	defer srv.Close()

	c, _ := client.New(srv.URL, client.WithCacheTTL(5*time.Minute))

	c.Resolve(context.Background(), "agent://nexus.io/x/agent_1")
	c.Resolve(context.Background(), "agent://nexus.io/x/agent_1")

	if callCount != 1 {
		t.Errorf("expected 1 HTTP call (cached), got %d", callCount)
	}
}

func TestResolveViaService_success(t *testing.T) {
	srv := stubRegistryServer(t)
	defer srv.Close()

	c, _ := client.New(srv.URL)

	result, err := c.ResolveViaService(context.Background(), srv.URL, "agent://nexus.io/finance/taxes/agent_abc")
	if err != nil {
		t.Fatalf("ResolveViaService: %v", err)
	}
	if result.Endpoint != "https://resolver.example.com" {
		t.Errorf("unexpected endpoint: %s", result.Endpoint)
	}
}

func TestGetAgent_success(t *testing.T) {
	srv := stubRegistryServer(t)
	defer srv.Close()

	c, _ := client.New(srv.URL)

	agent, err := c.GetAgent(context.Background(), "550e8400-e29b-41d4-a716-446655440000")
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if agent.DisplayName != "Test Agent" {
		t.Errorf("unexpected display name: %s", agent.DisplayName)
	}
}

func TestGetAgent_notFound(t *testing.T) {
	srv := stubRegistryServer(t)
	defer srv.Close()

	c, _ := client.New(srv.URL)

	_, err := c.GetAgent(context.Background(), "not-found-id")
	if err == nil {
		t.Error("expected error for not-found agent")
	}
}

func TestRegisterAgent_success(t *testing.T) {
	srv := stubRegistryServer(t)
	defer srv.Close()

	c, _ := client.New(srv.URL)

	result, err := c.RegisterAgent(context.Background(), client.RegisterAgentRequest{
		TrustRoot:      "nexus.io",
		CapabilityNode: "finance/taxes",
		DisplayName:    "Tax Agent",
		Endpoint:       "https://tax.example.com",
		OwnerDomain:    "example.com",
	})
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	if result.ID == "" {
		t.Error("expected non-empty ID")
	}
	if !strings.Contains(result.URI, "agent://nexus.io/finance/taxes/") {
		t.Errorf("unexpected URI: %s", result.URI)
	}
}

func TestActivateAgent_success(t *testing.T) {
	srv := stubRegistryServer(t)
	defer srv.Close()

	c, _ := client.New(srv.URL)

	result, err := c.ActivateAgent(context.Background(), "550e8400-e29b-41d4-a716-446655440000")
	if err != nil {
		t.Fatalf("ActivateAgent: %v", err)
	}
	if result.CertPEM == "" {
		t.Error("expected cert PEM")
	}
	if result.PrivateKeyPEM == "" {
		t.Error("expected private key PEM")
	}
}

func TestRevokeAgent_success(t *testing.T) {
	srv := stubRegistryServer(t)
	defer srv.Close()

	c, _ := client.New(srv.URL, client.WithBearerToken("test-token"))

	err := c.RevokeAgent(context.Background(), "550e8400-e29b-41d4-a716-446655440000")
	if err != nil {
		t.Fatalf("RevokeAgent: %v", err)
	}
}

func TestRevokeAgent_401(t *testing.T) {
	srv := stubRegistryServer(t)
	defer srv.Close()

	c, _ := client.New(srv.URL) // no bearer token

	err := c.RevokeAgent(context.Background(), "550e8400-e29b-41d4-a716-446655440000")
	if err == nil {
		t.Error("expected error for unauthorized revoke")
	}
}

func TestListAgents_success(t *testing.T) {
	srv := stubRegistryServer(t)
	defer srv.Close()

	c, _ := client.New(srv.URL)

	agents, err := c.ListAgents(context.Background(), "nexus.io", "")
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if len(agents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(agents))
	}
}

func TestFetchToken_success(t *testing.T) {
	srv := stubRegistryServer(t)
	defer srv.Close()

	c, _ := client.New(srv.URL)

	token, err := c.FetchToken(context.Background())
	if err != nil {
		t.Fatalf("FetchToken: %v", err)
	}
	if token != "test-jwt-token" {
		t.Errorf("unexpected token: %s", token)
	}
}

func TestCallAgent_resolveAndCall(t *testing.T) {
	// Agent endpoint mock
	agentSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"reply": "ok"})
	}))
	defer agentSrv.Close()

	// Registry that resolves to agentSrv
	regSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/resolve") {
			json.NewEncoder(w).Encode(map[string]any{
				"uri":      "agent://nexus.io/x/agent_1",
				"endpoint": agentSrv.URL,
				"status":   "active",
			})
			return
		}
		if r.URL.Path == "/api/v1/token" {
			json.NewEncoder(w).Encode(map[string]any{
				"access_token": "test-jwt",
				"expires_in":   3600,
			})
			return
		}
	}))
	defer regSrv.Close()

	c, _ := client.New(regSrv.URL)

	var reply map[string]string
	err := c.CallAgent(context.Background(), "agent://nexus.io/x/agent_1", http.MethodPost, "/invoke", nil, &reply)
	if err != nil {
		t.Fatalf("CallAgent: %v", err)
	}
	if reply["reply"] != "ok" {
		t.Errorf("unexpected reply: %v", reply)
	}
}
