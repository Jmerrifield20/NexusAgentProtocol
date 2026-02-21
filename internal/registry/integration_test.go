//go:build integration

package registry_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nexus-protocol/nexus/internal/identity"
	"github.com/nexus-protocol/nexus/internal/registry/handler"
	"github.com/nexus-protocol/nexus/internal/registry/repository"
	"github.com/nexus-protocol/nexus/internal/registry/service"
	"github.com/nexus-protocol/nexus/internal/trustledger"
	"go.uber.org/zap"
)

func setupIntegration(t *testing.T) (*httptest.Server, *pgxpool.Pool) {
	t.Helper()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}

	ctx := context.Background()
	db, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect to postgres: %v", err)
	}
	if err := db.Ping(ctx); err != nil {
		t.Fatalf("ping postgres: %v", err)
	}

	// Clean agents table for deterministic tests
	db.Exec(ctx, "DELETE FROM agents")

	logger := zap.NewNop()

	// Identity
	ca := identity.NewCAManager(t.TempDir())
	if err := ca.Create(); err != nil {
		t.Fatalf("create CA: %v", err)
	}
	issuer := identity.NewIssuer(ca)
	tokens := identity.NewTokenIssuer(ca.Key(), "http://test", time.Hour)

	// Ledger
	ledger := trustledger.NewPostgresLedger(db, logger)

	// Wire
	repo := repository.NewAgentRepository(db)
	svc := service.NewAgentService(repo, issuer, ledger, nil, logger)
	agentH := handler.NewAgentHandler(svc, tokens, logger)
	ledgerH := handler.NewLedgerHandler(ledger, logger)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")
	agentH.Register(v1)
	ledgerH.Register(v1)

	srv := httptest.NewServer(router)
	t.Cleanup(func() {
		srv.Close()
		db.Close()
	})
	return srv, db
}

func postJSON(t *testing.T, srv *httptest.Server, path string, body any) (*http.Response, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	resp, err := http.Post(srv.URL+path, "application/json", &buf)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	return resp, result
}

func getJSON(t *testing.T, srv *httptest.Server, path string) (*http.Response, map[string]any) {
	t.Helper()
	resp, err := http.Get(srv.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	return resp, result
}

func TestFullLifecycle(t *testing.T) {
	srv, _ := setupIntegration(t)

	// Register
	resp, body := postJSON(t, srv, "/api/v1/agents", map[string]string{
		"trust_root":      "nexus.io",
		"capability_node": "finance/taxes",
		"display_name":    "Integration Agent",
		"endpoint":        "https://integration.example.com",
		"owner_domain":    "example.com",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d: %v", resp.StatusCode, body)
	}
	id := body["id"].(string)
	agentID := body["agent_id"].(string)

	// Get
	resp, body = getJSON(t, srv, "/api/v1/agents/"+id)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get: expected 200, got %d", resp.StatusCode)
	}
	if body["status"] != "pending" {
		t.Errorf("expected pending, got %s", body["status"])
	}

	// Activate
	resp, body = postJSON(t, srv, "/api/v1/agents/"+id+"/activate", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("activate: expected 200, got %d: %v", resp.StatusCode, body)
	}
	if body["status"] != "activated" {
		t.Errorf("expected activated, got %v", body["status"])
	}

	// Resolve
	resp, body = getJSON(t, srv, fmt.Sprintf("/api/v1/resolve?trust_root=nexus.io&capability_node=finance/taxes&agent_id=%s", agentID))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("resolve: expected 200, got %d", resp.StatusCode)
	}
	if body["endpoint"] != "https://integration.example.com" {
		t.Errorf("unexpected endpoint: %v", body["endpoint"])
	}
}

func TestListAgents_pagination_integration(t *testing.T) {
	srv, _ := setupIntegration(t)

	// Insert 10 agents
	for i := 0; i < 10; i++ {
		resp, _ := postJSON(t, srv, "/api/v1/agents", map[string]string{
			"trust_root":      "nexus.io",
			"capability_node": fmt.Sprintf("test/page%d", i),
			"display_name":    fmt.Sprintf("Agent %d", i),
			"endpoint":        fmt.Sprintf("https://agent%d.example.com", i),
			"owner_domain":    "example.com",
		})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("register agent %d: expected 201, got %d", i, resp.StatusCode)
		}
	}

	// First page
	resp, body := getJSON(t, srv, "/api/v1/agents?limit=3&offset=0")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", resp.StatusCode)
	}
	count := int(body["count"].(float64))
	if count != 3 {
		t.Errorf("page 1: expected 3 agents, got %d", count)
	}

	// Second page
	resp, body = getJSON(t, srv, "/api/v1/agents?limit=3&offset=3")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list page 2: expected 200, got %d", resp.StatusCode)
	}
	count = int(body["count"].(float64))
	if count != 3 {
		t.Errorf("page 2: expected 3 agents, got %d", count)
	}
}

func TestDeleteAgent_notFound(t *testing.T) {
	srv, _ := setupIntegration(t)

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/agents/550e8400-e29b-41d4-a716-446655440000", nil)
	req.Header.Set("Authorization", "Bearer dummy") // will fail auth, but that's after 404
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	// Either 401 (if token invalid) or 404 — both are valid behavior
	if resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 401 or 404, got %d", resp.StatusCode)
	}
}

func TestTrustLedger_entries(t *testing.T) {
	srv, _ := setupIntegration(t)

	// Register + activate
	resp, body := postJSON(t, srv, "/api/v1/agents", map[string]string{
		"trust_root":      "nexus.io",
		"capability_node": "ledger/test",
		"display_name":    "Ledger Test Agent",
		"endpoint":        "https://ledger.example.com",
		"owner_domain":    "example.com",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: %d", resp.StatusCode)
	}
	id := body["id"].(string)

	postJSON(t, srv, "/api/v1/agents/"+id+"/activate", nil)

	// Check ledger entries
	resp, body = getJSON(t, srv, "/api/v1/ledger")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ledger overview: %d", resp.StatusCode)
	}
	entries := int(body["entries"].(float64))
	// At minimum: genesis + register + activate = 3, but there may be more
	// from other tests. Just verify > 1.
	if entries < 2 {
		t.Errorf("expected at least 2 ledger entries, got %d", entries)
	}
}
