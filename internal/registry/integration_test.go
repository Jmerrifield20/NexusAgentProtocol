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
	"github.com/jmerrifield20/NexusAgentProtocol/internal/email"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/identity"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/registry/handler"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/registry/repository"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/registry/service"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/trustledger"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/users"
	"go.uber.org/zap"
)

// integrationEnv holds all wired-up components for an integration test.
type integrationEnv struct {
	srv        *httptest.Server
	db         *pgxpool.Pool
	userTokens *identity.UserTokenIssuer
}

func setupIntegration(t *testing.T) (*httptest.Server, *pgxpool.Pool) {
	t.Helper()
	env := setupIntegrationEnv(t)
	return env.srv, env.db
}

func setupIntegrationEnv(t *testing.T) *integrationEnv {
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

	// Clean tables for deterministic tests
	db.Exec(ctx, "DELETE FROM agents")
	db.Exec(ctx, "DELETE FROM email_verifications")
	db.Exec(ctx, "DELETE FROM users")

	logger := zap.NewNop()

	// Identity
	ca := identity.NewCAManager(t.TempDir())
	if err := ca.Create(); err != nil {
		t.Fatalf("create CA: %v", err)
	}
	issuer := identity.NewIssuer(ca)
	tokens := identity.NewTokenIssuer(ca.Key(), "http://test", time.Hour)
	userTokens := identity.NewUserTokenIssuer(ca.Key(), "http://test", time.Hour)

	// Ledger
	ledger := trustledger.NewPostgresLedger(db, logger)

	// User layer
	userRepo := users.NewUserRepository(db)
	mailer := email.NewNoopSender(logger)
	userSvc := users.NewUserService(userRepo, mailer, "http://test", logger)
	userSvc.SetFrontendURL("http://localhost:3000")

	// Agent layer
	repo := repository.NewAgentRepository(db)
	svc := service.NewAgentService(repo, issuer, ledger, nil, logger)
	svc.SetEmailChecker(userSvc)
	svc.SetFreeTierConfig(service.FreeTierConfig{
		TrustRoot: "nexusagentprotocol.com",
		MaxAgents: 3,
	})
	svc.SetTokenIssuer(tokens)
	svc.SetRegistryURL("http://test")

	// Handlers
	agentH := handler.NewAgentHandler(svc, tokens, logger)
	agentH.SetUserTokenIssuer(userTokens)
	ledgerH := handler.NewLedgerHandler(ledger, logger)
	authH := handler.NewAuthHandler(userSvc, userTokens, nil, logger)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")
	agentH.Register(v1)
	ledgerH.Register(v1)
	authH.Register(v1)

	srv := httptest.NewServer(router)
	t.Cleanup(func() {
		srv.Close()
		db.Close()
	})
	return &integrationEnv{srv: srv, db: db, userTokens: userTokens}
}

// ── HTTP helpers ──────────────────────────────────────────────────────────────

func postJSON(t *testing.T, srv *httptest.Server, path string, body any) (*http.Response, map[string]any) {
	t.Helper()
	return postJSONWithToken(t, srv, path, body, "")
}

func postJSONWithToken(t *testing.T, srv *httptest.Server, path string, body any, token string) (*http.Response, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req, err := http.NewRequest(http.MethodPost, srv.URL+path, &buf)
	if err != nil {
		t.Fatalf("build request POST %s: %v", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
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
	return getJSONWithToken(t, srv, path, "")
}

func getJSONWithToken(t *testing.T, srv *httptest.Server, path, token string) (*http.Response, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, srv.URL+path, nil)
	if err != nil {
		t.Fatalf("build request GET %s: %v", path, err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	return resp, result
}

// ── Domain-verified lifecycle ─────────────────────────────────────────────────

func TestFullLifecycle(t *testing.T) {
	srv, _ := setupIntegration(t)

	// Register
	resp, body := postJSON(t, srv, "/api/v1/agents", map[string]string{
		"trust_root":      "nexusagentprotocol.com",
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

	// Activate (dnsVerifier=nil means any domain passes)
	resp, body = postJSON(t, srv, "/api/v1/agents/"+id+"/activate", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("activate: expected 200, got %d: %v", resp.StatusCode, body)
	}
	if body["status"] != "activated" {
		t.Errorf("expected activated, got %v", body["status"])
	}

	// Resolve
	resp, body = getJSON(t, srv, fmt.Sprintf("/api/v1/resolve?trust_root=nexusagentprotocol.com&capability_node=finance/taxes&agent_id=%s", agentID))
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
			"trust_root":      "nexusagentprotocol.com",
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
	req.Header.Set("Authorization", "Bearer dummy")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 401 or 404, got %d", resp.StatusCode)
	}
}

func TestTrustLedger_entries(t *testing.T) {
	srv, _ := setupIntegration(t)

	resp, body := postJSON(t, srv, "/api/v1/agents", map[string]string{
		"trust_root":      "nexusagentprotocol.com",
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

	resp, body = getJSON(t, srv, "/api/v1/ledger")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ledger overview: %d", resp.StatusCode)
	}
	entries := int(body["entries"].(float64))
	if entries < 2 {
		t.Errorf("expected at least 2 ledger entries, got %d", entries)
	}
}

// ── Auth flow ─────────────────────────────────────────────────────────────────

func TestAuthFlow_SignupLoginVerifyEmail(t *testing.T) {
	env := setupIntegrationEnv(t)

	// Signup
	resp, body := postJSON(t, env.srv, "/api/v1/auth/signup", map[string]string{
		"email":    "alice@integration.test",
		"password": "securepassword123",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("signup: expected 201, got %d: %v", resp.StatusCode, body)
	}
	if body["token"] == nil {
		t.Fatal("signup: expected token in response")
	}
	if body["note"] == nil {
		t.Error("signup: expected note about email verification")
	}

	// Login
	resp, body = postJSON(t, env.srv, "/api/v1/auth/login", map[string]string{
		"email":    "alice@integration.test",
		"password": "securepassword123",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login: expected 200, got %d: %v", resp.StatusCode, body)
	}
	if body["token"] == nil {
		t.Fatal("login: expected token in response")
	}

	// Wrong password should fail
	resp, _ = postJSON(t, env.srv, "/api/v1/auth/login", map[string]string{
		"email":    "alice@integration.test",
		"password": "wrongpassword",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("wrong password: expected 401, got %d", resp.StatusCode)
	}

	// Duplicate signup should fail
	resp, _ = postJSON(t, env.srv, "/api/v1/auth/signup", map[string]string{
		"email":    "alice@integration.test",
		"password": "anotherpassword1",
	})
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("duplicate signup: expected 409, got %d", resp.StatusCode)
	}
}

// ── NAP-Hosted agent registration ─────────────────────────────────────────────

func TestNAPHosted_RegisterAndListMyAgents(t *testing.T) {
	env := setupIntegrationEnv(t)

	// Create and verify a user manually so we can test the hosted flow.
	resp, body := postJSON(t, env.srv, "/api/v1/auth/signup", map[string]string{
		"email":    "bob@integration.test",
		"password": "securepassword123",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("signup: expected 201, got %d: %v", resp.StatusCode, body)
	}
	token := body["token"].(string)

	// Mark email verified directly in DB so activation can proceed.
	ctx := context.Background()
	env.db.Exec(ctx,
		"UPDATE users SET email_verified = true WHERE email = 'bob@integration.test'",
	)

	// Register a hosted agent.
	resp, body = postJSONWithToken(t, env.srv, "/api/v1/agents", map[string]string{
		"display_name":      "Bob's Hosted Agent",
		"description":       "Integration test agent",
		"endpoint":          "https://bob-agent.example.com",
		"registration_type": "nap_hosted",
	}, token)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register hosted: expected 201, got %d: %v", resp.StatusCode, body)
	}
	agentID := body["id"].(string)
	uri, _ := body["agent_uri"].(string)
	if uri == "" {
		t.Error("register hosted: expected non-empty agent_uri")
	}

	// Activate the hosted agent.
	resp, body = postJSONWithToken(t, env.srv, "/api/v1/agents/"+agentID+"/activate", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("activate hosted: expected 200, got %d: %v", resp.StatusCode, body)
	}

	// List my agents via the authenticated endpoint.
	resp, body = getJSONWithToken(t, env.srv, "/api/v1/users/me/agents", token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list my agents: expected 200, got %d: %v", resp.StatusCode, body)
	}
	agentList, ok := body["agents"].([]any)
	if !ok || len(agentList) == 0 {
		t.Fatalf("list my agents: expected at least 1 agent, got %v", body)
	}

	first := agentList[0].(map[string]any)
	if first["registration_type"] != "nap_hosted" {
		t.Errorf("expected nap_hosted, got %v", first["registration_type"])
	}
}

func TestNAPHosted_QuotaEnforced(t *testing.T) {
	env := setupIntegrationEnv(t)

	resp, body := postJSON(t, env.srv, "/api/v1/auth/signup", map[string]string{
		"email":    "quota@integration.test",
		"password": "securepassword123",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("signup: %d", resp.StatusCode)
	}
	token := body["token"].(string)

	register := func() int {
		resp, _ := postJSONWithToken(t, env.srv, "/api/v1/agents", map[string]string{
			"display_name":      "Quota Agent",
			"endpoint":          "https://quota.example.com",
			"registration_type": "nap_hosted",
		}, token)
		return resp.StatusCode
	}

	// First 3 should succeed (MaxAgents=3 in test setup)
	for i := 0; i < 3; i++ {
		if status := register(); status != http.StatusCreated {
			t.Fatalf("agent %d: expected 201, got %d", i+1, status)
		}
	}

	// 4th should be rejected
	if status := register(); status != http.StatusForbidden && status != http.StatusUnprocessableEntity {
		t.Errorf("quota exceeded: expected 403 or 422, got %d", status)
	}
}
