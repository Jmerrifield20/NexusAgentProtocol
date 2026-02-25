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
	"github.com/jmerrifield20/NexusAgentProtocol/internal/identity"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/registry/handler"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/registry/model"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/registry/repository"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/registry/service"
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

func (s *stubAgentRepo) ListByOwnerUserID(_ context.Context, ownerUserID uuid.UUID, limit, offset int) ([]*model.Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*model.Agent
	for _, a := range s.rows {
		if a.OwnerUserID != nil && *a.OwnerUserID == ownerUserID {
			cp := *a
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (s *stubAgentRepo) SearchByOrg(_ context.Context, orgName string, limit, offset int) ([]*model.Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*model.Agent
	for _, a := range s.rows {
		if a.TrustRoot == orgName && a.Status == model.AgentStatusActive {
			cp := *a
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (s *stubAgentRepo) SearchByCapability(_ context.Context, capability, domain string, limit, offset int) ([]*model.Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*model.Agent
	for _, a := range s.rows {
		if a.Status != model.AgentStatusActive {
			continue
		}
		if a.CapabilityNode != capability && !strings.HasPrefix(a.CapabilityNode, capability+"/") {
			continue
		}
		if domain != "" && a.TrustRoot != domain {
			continue
		}
		cp := *a
		out = append(out, &cp)
	}
	return out, nil
}

func (s *stubAgentRepo) CountByOwner(_ context.Context, ownerUserID uuid.UUID) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, a := range s.rows {
		if a.OwnerUserID != nil && *a.OwnerUserID == ownerUserID && a.Status != "revoked" {
			count++
		}
	}
	return count, nil
}

func (s *stubAgentRepo) ListActiveByOwnerUserID(_ context.Context, ownerUserID uuid.UUID, limit, offset int) ([]*model.Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*model.Agent
	for _, a := range s.rows {
		if a.OwnerUserID != nil && *a.OwnerUserID == ownerUserID && a.Status == model.AgentStatusActive {
			cp := *a
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (s *stubAgentRepo) ListActiveByUsername(_ context.Context, username string, limit, offset int) ([]*model.Agent, error) {
	return nil, nil
}

func (s *stubAgentRepo) CountActiveByOwnerUserID(_ context.Context, ownerUserID uuid.UUID) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, a := range s.rows {
		if a.OwnerUserID != nil && *a.OwnerUserID == ownerUserID && a.Status == model.AgentStatusActive {
			count++
		}
	}
	return count, nil
}

func (s *stubAgentRepo) ListVerifiedDomainsByUserID(_ context.Context, ownerUserID uuid.UUID) ([]string, error) {
	return nil, nil
}

func (s *stubAgentRepo) UpdateStatusWithReason(_ context.Context, id uuid.UUID, status model.AgentStatus, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.rows[id]
	if !ok {
		return repository.ErrNotFound
	}
	a.Status = status
	a.RevocationReason = reason
	return nil
}

func (s *stubAgentRepo) Suspend(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.rows[id]
	if !ok || a.Status != model.AgentStatusActive {
		return repository.ErrNotFound
	}
	a.Status = model.AgentStatusSuspended
	now := time.Now()
	a.SuspendedAt = &now
	return nil
}

func (s *stubAgentRepo) Restore(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.rows[id]
	if !ok || a.Status != model.AgentStatusSuspended {
		return repository.ErrNotFound
	}
	a.Status = model.AgentStatusActive
	a.SuspendedAt = nil
	return nil
}

func (s *stubAgentRepo) ListRevokedCerts(_ context.Context) ([]*model.Agent, error) {
	return nil, nil
}

func (s *stubAgentRepo) Deprecate(_ context.Context, id uuid.UUID, sunsetDate *time.Time, replacementURI string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.rows[id]
	if !ok || a.Status != model.AgentStatusActive {
		return repository.ErrNotFound
	}
	a.Status = model.AgentStatusDeprecated
	now := time.Now()
	a.DeprecatedAt = &now
	a.SunsetDate = sunsetDate
	a.ReplacementURI = replacementURI
	return nil
}

func (s *stubAgentRepo) ListActiveEndpoints(_ context.Context) ([]*model.Agent, error) {
	return nil, nil
}

func (s *stubAgentRepo) UpdateHealthStatus(_ context.Context, id uuid.UUID, status string, lastSeenAt time.Time) error {
	return nil
}

func (s *stubAgentRepo) SearchBySkill(_ context.Context, skillID string, limit, offset int) ([]*model.Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*model.Agent
	for _, a := range s.rows {
		if a.Status != model.AgentStatusActive {
			continue
		}
		for _, id := range a.SkillIDs {
			if id == skillID {
				cp := *a
				out = append(out, &cp)
				break
			}
		}
	}
	return out, nil
}

func (s *stubAgentRepo) SearchByTool(_ context.Context, toolName string, limit, offset int) ([]*model.Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*model.Agent
	for _, a := range s.rows {
		if a.Status != model.AgentStatusActive {
			continue
		}
		for _, name := range a.ToolNames {
			if name == toolName {
				cp := *a
				out = append(out, &cp)
				break
			}
		}
	}
	return out, nil
}

func (s *stubAgentRepo) Search(_ context.Context, q string, limit, offset int) ([]*model.Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*model.Agent
	for _, a := range s.rows {
		if strings.Contains(a.DisplayName, q) || strings.Contains(a.CapabilityNode, q) {
			cp := *a
			out = append(out, &cp)
		}
	}
	return out, nil
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
		"category":"finance",
		"org_name":"acme",
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
	// Register response is now {"agent": {...}, "agent_uri": "..."}. Unwrap the
	// inner agent map so callers can access fields directly (backward compat).
	if inner, ok := result["agent"]; ok {
		if agentMap, ok := inner.(map[string]any); ok {
			return agentMap
		}
	}
	return result
}

// ── Tests ────────────────────────────────────────────────────────────────

func TestCreateAgent_201(t *testing.T) {
	repo := newStubAgentRepo()
	router, _, _ := setupTestRouter(t, repo, false)

	body := `{
		"category":"finance",
		"org_name":"acme",
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

	// Register response shape: {"agent": {...}, "agent_uri": "..."}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	agent := resp["agent"].(map[string]any)
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
	router, _, _ := setupTestRouter(t, repo, false) // no auth = dev mode

	created := registerAgent(t, router)
	id := created["id"].(string)

	w := patchAgent(t, router, id, "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["display_name"] != "Patched Name" {
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

	url := "/api/v1/resolve?trust_root=example.com&capability_node=finance&agent_id=" + agent.AgentID
	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestResolveAgent_400_missingParams(t *testing.T) {
	router, _, _ := setupTestRouter(t, newStubAgentRepo(), false)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/resolve?trust_root=acme", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// ── UpdateAgent auth tests ────────────────────────────────────────────────

// setupTestRouterFull wires both agent task tokens and user JWTs.
func setupTestRouterFull(t *testing.T, repo *stubAgentRepo) (
	router *gin.Engine,
	svc *service.AgentService,
	tokens *identity.TokenIssuer,
	userTokens *identity.UserTokenIssuer,
) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	svc = service.NewAgentService(repo, nil, nil, nil, zap.NewNop())
	ca := testCA(t)
	tokens = identity.NewTokenIssuer(ca.Key(), "http://test", time.Hour)
	userTokens = identity.NewUserTokenIssuer(ca.Key(), "http://test", time.Hour)
	h := handler.NewAgentHandler(svc, tokens, zap.NewNop())
	h.SetUserTokenIssuer(userTokens)
	v1 := r.Group("/api/v1")
	h.Register(v1)
	router = r
	return
}

// registerHostedAgent creates a nap_hosted agent owned by the given user UUID.
func registerHostedAgent(t *testing.T, repo *stubAgentRepo, ownerID uuid.UUID) *model.Agent {
	t.Helper()
	uid := ownerID
	a := &model.Agent{
		TrustRoot:        "testuser",  // org namespace = username for hosted agents
		CapabilityNode:   "finance",   // category
		AgentID:          "agent_" + ownerID.String()[:8],
		DisplayName:      "Test Hosted Agent",
		Endpoint:         "https://agent.example.com",
		Status:           model.AgentStatusActive,
		OwnerUserID:      &uid,
		RegistrationType: model.RegistrationTypeNAPHosted,
	}
	if err := repo.Create(context.Background(), a); err != nil {
		t.Fatalf("registerHostedAgent: %v", err)
	}
	return a
}

func patchAgent(t *testing.T, router *gin.Engine, id, token string) *httptest.ResponseRecorder {
	t.Helper()
	body := `{"display_name":"Patched Name"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/agents/"+id, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// TestUpdateAgent_200_noAuth verifies the existing dev-mode behaviour is preserved:
// when no token issuer is configured, unauthenticated updates still succeed.
func TestUpdateAgent_200_noAuth(t *testing.T) {
	repo := newStubAgentRepo()
	router, _, _ := setupTestRouter(t, repo, false) // no auth configured

	created := registerAgent(t, router)
	w := patchAgent(t, router, created["id"].(string), "")
	if w.Code != http.StatusOK {
		t.Fatalf("dev-mode: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUpdateAgent_401_noToken confirms anonymous requests are rejected once auth is on.
func TestUpdateAgent_401_noToken(t *testing.T) {
	repo := newStubAgentRepo()
	router, _, _ := setupTestRouter(t, repo, true) // auth enabled

	created := registerAgent(t, router)
	w := patchAgent(t, router, created["id"].(string), "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUpdateAgent_200_agentToken confirms the owning agent's task token is accepted.
func TestUpdateAgent_200_agentToken(t *testing.T) {
	repo := newStubAgentRepo()
	router, svc, tokens, _ := setupTestRouterFull(t, repo)

	created := registerAgent(t, router)
	id := created["id"].(string)
	uid, _ := uuid.Parse(id)
	svc.Activate(context.Background(), uid)
	agent, _ := svc.Get(context.Background(), uid)

	tok, _ := tokens.Issue(agent.URI(), nil)
	w := patchAgent(t, router, id, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with own token, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUpdateAgent_403_wrongAgentToken confirms a token for a different agent is rejected.
func TestUpdateAgent_403_wrongAgentToken(t *testing.T) {
	repo := newStubAgentRepo()
	router, _, tokens, _ := setupTestRouterFull(t, repo)

	created := registerAgent(t, router)
	tok, _ := tokens.Issue("agent://other.io/cap/agent_other", nil)
	w := patchAgent(t, router, created["id"].(string), tok)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUpdateAgent_200_adminScope confirms a nexus:admin scoped token can update any agent.
func TestUpdateAgent_200_adminScope(t *testing.T) {
	repo := newStubAgentRepo()
	router, _, tokens, _ := setupTestRouterFull(t, repo)

	created := registerAgent(t, router)
	tok, _ := tokens.Issue("agent://admin.io/system/admin_1", []string{"nexus:admin"})
	w := patchAgent(t, router, created["id"].(string), tok)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with admin scope, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUpdateAgent_200_userOwnsAgent confirms a user JWT is accepted for their own hosted agent.
func TestUpdateAgent_200_userOwnsAgent(t *testing.T) {
	repo := newStubAgentRepo()
	router, _, _, userTokens := setupTestRouterFull(t, repo)

	ownerID := uuid.New()
	agent := registerHostedAgent(t, repo, ownerID)

	tok, _ := userTokens.Issue(ownerID.String(), "user@example.com", "testuser")
	w := patchAgent(t, router, agent.ID.String(), tok)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for owner, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUpdateAgent_403_userJWT_wrongOwner confirms a user JWT for a different user is rejected.
func TestUpdateAgent_403_userJWT_wrongOwner(t *testing.T) {
	repo := newStubAgentRepo()
	router, _, _, userTokens := setupTestRouterFull(t, repo)

	ownerID := uuid.New()
	agent := registerHostedAgent(t, repo, ownerID)

	// Token for a completely different user
	differentUserID := uuid.New()
	tok, _ := userTokens.Issue(differentUserID.String(), "other@example.com", "other")
	w := patchAgent(t, router, agent.ID.String(), tok)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for wrong owner, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUpdateAgent_403_userJWT_domainAgent confirms a user JWT cannot update a domain-verified
// agent (which has no owner_user_id) — the agent's own task token is required for those.
func TestUpdateAgent_403_userJWT_domainAgent(t *testing.T) {
	repo := newStubAgentRepo()
	router, _, _, userTokens := setupTestRouterFull(t, repo)

	// Domain agent: registered without an owner_user_id (the normal domain path)
	created := registerAgent(t, router)
	tok, _ := userTokens.Issue(uuid.New().String(), "anyone@example.com", "anyone")
	w := patchAgent(t, router, created["id"].(string), tok)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for user JWT on domain agent, got %d: %s", w.Code, w.Body.String())
	}
}

// Ensure unused import is consumed.
var _ = bytes.NewReader
