package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nexus-protocol/nexus/internal/identity"
	"github.com/nexus-protocol/nexus/internal/registry/handler"
	"github.com/nexus-protocol/nexus/internal/registry/model"
	"github.com/nexus-protocol/nexus/internal/registry/repository"
	"github.com/nexus-protocol/nexus/internal/registry/service"
	"go.uber.org/zap"
)

// ── Stub repo ────────────────────────────────────────────────────────────

type stubAgentRepo struct {
	mu    sync.RWMutex
	rows  map[uuid.UUID]*model.Agent
	byKey map[string]uuid.UUID
}

func newStubAgentRepo() *stubAgentRepo {
	return &stubAgentRepo{
		rows:  make(map[uuid.UUID]*model.Agent),
		byKey: make(map[string]uuid.UUID),
	}
}

func agentKey(a *model.Agent) string {
	return a.TrustRoot + "/" + a.CapabilityNode + "/" + a.AgentID
}

func (s *stubAgentRepo) Create(_ context.Context, agent *model.Agent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	agent.ID = uuid.New()
	now := time.Now().UTC()
	agent.CreatedAt = now
	agent.UpdatedAt = now
	cp := *agent
	s.rows[agent.ID] = &cp
	s.byKey[agentKey(agent)] = agent.ID
	return nil
}

func (s *stubAgentRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.rows[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *a
	return &cp, nil
}

func (s *stubAgentRepo) GetByAgentID(_ context.Context, trustRoot, capNode, agentID string) (*model.Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := trustRoot + "/" + capNode + "/" + agentID
	id, ok := s.byKey[key]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *s.rows[id]
	return &cp, nil
}

func (s *stubAgentRepo) List(_ context.Context, trustRoot, capNode string, limit, offset int) ([]*model.Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*model.Agent
	for _, a := range s.rows {
		if trustRoot != "" && a.TrustRoot != trustRoot {
			continue
		}
		if capNode != "" && a.CapabilityNode != capNode {
			continue
		}
		cp := *a
		result = append(result, &cp)
	}
	if offset > len(result) {
		return nil, nil
	}
	result = result[offset:]
	if limit > 0 && limit < len(result) {
		result = result[:limit]
	}
	return result, nil
}

func (s *stubAgentRepo) ListByOwnerDomain(_ context.Context, domain string, limit, offset int) ([]*model.Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*model.Agent
	for _, a := range s.rows {
		if a.OwnerDomain == domain && a.Status == model.AgentStatusActive {
			cp := *a
			result = append(result, &cp)
		}
	}
	if offset > len(result) {
		return nil, nil
	}
	result = result[offset:]
	if limit > 0 && limit < len(result) {
		result = result[:limit]
	}
	return result, nil
}

func (s *stubAgentRepo) Update(_ context.Context, agent *model.Agent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.rows[agent.ID]; !ok {
		return repository.ErrNotFound
	}
	cp := *agent
	s.rows[agent.ID] = &cp
	return nil
}

func (s *stubAgentRepo) UpdateStatus(_ context.Context, id uuid.UUID, status model.AgentStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.rows[id]
	if !ok {
		return repository.ErrNotFound
	}
	a.Status = status
	return nil
}

func (s *stubAgentRepo) ActivateWithCert(_ context.Context, id uuid.UUID, serial, certPEM string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.rows[id]
	if !ok {
		return repository.ErrNotFound
	}
	a.Status = model.AgentStatusActive
	a.CertSerial = serial
	a.PublicKeyPEM = certPEM
	return nil
}

func (s *stubAgentRepo) Delete(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.rows[id]
	if !ok {
		return repository.ErrNotFound
	}
	delete(s.byKey, agentKey(a))
	delete(s.rows, id)
	return nil
}

// ── Helpers ──────────────────────────────────────────────────────────────

func testCA(t *testing.T) *identity.CAManager {
	t.Helper()
	dir := t.TempDir()
	ca := identity.NewCAManager(dir)
	if err := ca.Create(); err != nil {
		t.Fatalf("create test CA: %v", err)
	}
	return ca
}

func setupTestRouter(t *testing.T, repo *stubAgentRepo, withAuth bool) (*gin.Engine, *service.AgentService, *identity.TokenIssuer) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	svc := service.NewAgentService(repo, nil, nil, nil, zap.NewNop())

	var tokens *identity.TokenIssuer
	if withAuth {
		ca := testCA(t)
		tokens = identity.NewTokenIssuer(ca.Key(), "http://test", time.Hour)
	}

	h := handler.NewAgentHandler(svc, tokens, zap.NewNop())
	v1 := r.Group("/api/v1")
	h.Register(v1)
	return r, svc, tokens
}

func registerAgent(t *testing.T, router *gin.Engine) map[string]any {
	t.Helper()
	body := `{
		"trust_root":"nexus.io",
		"capability_node":"finance/taxes",
		"display_name":"Tax Agent",
		"endpoint":"https://tax.example.com",
		"owner_domain":"example.com"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("register agent: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	return result
}

// ── Tests ────────────────────────────────────────────────────────────────

func TestCreateAgent_201(t *testing.T) {
	repo := newStubAgentRepo()
	router, _, _ := setupTestRouter(t, repo, false)

	body := `{
		"trust_root":"nexus.io",
		"capability_node":"finance/taxes",
		"display_name":"Tax Agent",
		"endpoint":"https://tax.example.com",
		"owner_domain":"example.com"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var agent map[string]any
	json.Unmarshal(w.Body.Bytes(), &agent)
	if agent["status"] != "pending" {
		t.Errorf("expected pending status, got %v", agent["status"])
	}
}

func TestCreateAgent_400(t *testing.T) {
	router, _, _ := setupTestRouter(t, newStubAgentRepo(), false)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents", strings.NewReader(`{invalid`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListAgents_200(t *testing.T) {
	repo := newStubAgentRepo()
	router, _, _ := setupTestRouter(t, repo, false)

	registerAgent(t, router)
	registerAgent(t, router)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	count := int(resp["count"].(float64))
	if count != 2 {
		t.Errorf("expected 2 agents, got %d", count)
	}
}

func TestListAgents_pagination(t *testing.T) {
	repo := newStubAgentRepo()
	router, _, _ := setupTestRouter(t, repo, false)

	for i := 0; i < 5; i++ {
		registerAgent(t, router)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents?limit=2&offset=0", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	count := int(resp["count"].(float64))
	if count != 2 {
		t.Errorf("expected 2 agents with limit=2, got %d", count)
	}
}

func TestGetAgent_200(t *testing.T) {
	repo := newStubAgentRepo()
	router, _, _ := setupTestRouter(t, repo, false)

	created := registerAgent(t, router)
	id := created["id"].(string)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+id, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetAgent_404(t *testing.T) {
	router, _, _ := setupTestRouter(t, newStubAgentRepo(), false)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetAgent_400_badUUID(t *testing.T) {
	router, _, _ := setupTestRouter(t, newStubAgentRepo(), false)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/not-a-uuid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestUpdateAgent_200(t *testing.T) {
	repo := newStubAgentRepo()
	router, _, _ := setupTestRouter(t, repo, false)

	created := registerAgent(t, router)
	id := created["id"].(string)

	body := `{"display_name":"Updated Name"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/agents/"+id, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["display_name"] != "Updated Name" {
		t.Errorf("name not updated: %v", resp["display_name"])
	}
}

func TestUpdateAgent_404(t *testing.T) {
	router, _, _ := setupTestRouter(t, newStubAgentRepo(), false)

	body := `{"display_name":"x"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/agents/"+uuid.New().String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestActivateAgent_200(t *testing.T) {
	repo := newStubAgentRepo()
	router, _, _ := setupTestRouter(t, repo, false)

	created := registerAgent(t, router)
	id := created["id"].(string)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/"+id+"/activate", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestActivateAgent_404(t *testing.T) {
	router, _, _ := setupTestRouter(t, newStubAgentRepo(), false)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/"+uuid.New().String()+"/activate", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestDeleteAgent_204_withToken(t *testing.T) {
	repo := newStubAgentRepo()
	router, svc, tokens := setupTestRouter(t, repo, true)

	created := registerAgent(t, router)
	id := created["id"].(string)
	uid, _ := uuid.Parse(id)

	// Activate to get URI for token
	svc.Activate(context.Background(), uid)
	agent, _ := svc.Get(context.Background(), uid)

	tok, _ := tokens.Issue(agent.URI(), []string{"nexus:admin"})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/agents/"+id, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteAgent_401_noToken(t *testing.T) {
	repo := newStubAgentRepo()
	router, _, _ := setupTestRouter(t, repo, true)

	created := registerAgent(t, router)
	id := created["id"].(string)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/agents/"+id, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestDeleteAgent_403_wrongAgent(t *testing.T) {
	repo := newStubAgentRepo()
	router, _, tokens := setupTestRouter(t, repo, true)

	created := registerAgent(t, router)
	id := created["id"].(string)

	// Token for a different agent
	tok, _ := tokens.Issue("agent://other.io/something/agent_xyz", nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/agents/"+id, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRevokeAgent_200(t *testing.T) {
	repo := newStubAgentRepo()
	router, svc, tokens := setupTestRouter(t, repo, true)

	created := registerAgent(t, router)
	id := created["id"].(string)
	uid, _ := uuid.Parse(id)

	svc.Activate(context.Background(), uid)
	agent, _ := svc.Get(context.Background(), uid)

	tok, _ := tokens.Issue(agent.URI(), nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/"+id+"/revoke", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRevokeAgent_401_noToken(t *testing.T) {
	repo := newStubAgentRepo()
	router, _, _ := setupTestRouter(t, repo, true)

	created := registerAgent(t, router)
	id := created["id"].(string)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/"+id+"/revoke", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestRevokeAgent_403_wrongAgent(t *testing.T) {
	repo := newStubAgentRepo()
	router, _, tokens := setupTestRouter(t, repo, true)

	created := registerAgent(t, router)
	id := created["id"].(string)

	tok, _ := tokens.Issue("agent://other.io/x/agent_other", nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/"+id+"/revoke", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRevokeAgent_200_adminScope(t *testing.T) {
	repo := newStubAgentRepo()
	router, svc, tokens := setupTestRouter(t, repo, true)

	created := registerAgent(t, router)
	id := created["id"].(string)
	uid, _ := uuid.Parse(id)

	svc.Activate(context.Background(), uid)

	// Admin token from a different agent URI but with nexus:admin scope
	tok, _ := tokens.Issue("agent://admin.io/system/admin_1", []string{"nexus:admin"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/"+id+"/revoke", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with admin scope, got %d: %s", w.Code, w.Body.String())
	}
}

func TestResolveAgent_200(t *testing.T) {
	repo := newStubAgentRepo()
	router, svc, _ := setupTestRouter(t, repo, false)

	created := registerAgent(t, router)
	id := created["id"].(string)
	uid, _ := uuid.Parse(id)
	svc.Activate(context.Background(), uid)

	agent, _ := svc.Get(context.Background(), uid)

	url := "/api/v1/resolve?trust_root=nexus.io&capability_node=finance/taxes&agent_id=" + agent.AgentID
	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestResolveAgent_400_missingParams(t *testing.T) {
	router, _, _ := setupTestRouter(t, newStubAgentRepo(), false)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/resolve?trust_root=nexus.io", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// Ensure unused import is consumed.
var _ = bytes.NewReader
