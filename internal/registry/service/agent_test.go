package service_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/identity"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/registry/model"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/registry/repository"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/registry/service"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/trustledger"
	"go.uber.org/zap"
)

// ── In-memory stub for agentRepo ──────────────────────────────────────────

type stubAgentRepo struct {
	mu    sync.RWMutex
	rows  map[uuid.UUID]*model.Agent
	byKey map[string]uuid.UUID // "trust_root/cap_node/agent_id" → UUID
}

func newStubAgentRepo() *stubAgentRepo {
	return &stubAgentRepo{
		rows:  make(map[uuid.UUID]*model.Agent),
		byKey: make(map[string]uuid.UUID),
	}
}

func agentKey(trustRoot, capNode, agentID string) string {
	return trustRoot + "/" + capNode + "/" + agentID
}

func (s *stubAgentRepo) Create(_ context.Context, agent *model.Agent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	agent.ID = uuid.New()
	cp := *agent
	s.rows[agent.ID] = &cp
	s.byKey[agentKey(agent.TrustRoot, agent.CapabilityNode, agent.AgentID)] = agent.ID
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
	// Mirror the real repo: capNode is the top-level category from the URI.
	// Match agents whose capability_node equals capNode OR starts with capNode+">".
	for _, a := range s.rows {
		if a.TrustRoot != trustRoot || a.AgentID != agentID {
			continue
		}
		if a.CapabilityNode == capNode || strings.HasPrefix(a.CapabilityNode, capNode+">") {
			cp := *a
			return &cp, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (s *stubAgentRepo) List(_ context.Context, trustRoot, capNode string, limit, offset int) ([]*model.Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*model.Agent
	for _, a := range s.rows {
		if trustRoot != "" && a.TrustRoot != trustRoot {
			continue
		}
		if capNode != "" && a.CapabilityNode != capNode && !strings.HasPrefix(a.CapabilityNode, capNode+">") {
			continue
		}
		cp := *a
		result = append(result, &cp)
	}
	// Apply offset/limit
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
	delete(s.byKey, agentKey(a.TrustRoot, a.CapabilityNode, a.AgentID))
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
		if a.CapabilityNode != capability && !strings.HasPrefix(a.CapabilityNode, capability+">") {
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
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*model.Agent
	for _, a := range s.rows {
		if a.Status == model.AgentStatusRevoked && a.CertSerial != "" {
			cp := *a
			out = append(out, &cp)
		}
	}
	return out, nil
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
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*model.Agent
	for _, a := range s.rows {
		if a.Status == model.AgentStatusActive && a.Endpoint != "" {
			cp := *a
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (s *stubAgentRepo) UpdateHealthStatus(_ context.Context, id uuid.UUID, status string, lastSeenAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.rows[id]
	if !ok {
		return repository.ErrNotFound
	}
	a.HealthStatus = status
	a.LastSeenAt = &lastSeenAt
	return nil
}

// ── Helpers ──────────────────────────────────────────────────────────────

func newTestAgentService(repo *stubAgentRepo, issuer *identity.Issuer, ledger trustledger.Ledger, verifier service.DomainVerifier) *service.AgentService {
	return service.NewAgentService(repo, issuer, ledger, verifier, zap.NewNop())
}

func testRegisterRequest() *model.RegisterRequest {
	return &model.RegisterRequest{
		Capability:  "finance>accounting",
		OrgName:     "acme",
		DisplayName: "Tax Agent",
		Description: "Handles tax computations",
		Endpoint:    "https://tax.example.com",
		OwnerDomain: "example.com",
	}
}

func testCA(t *testing.T) *identity.CAManager {
	t.Helper()
	dir := t.TempDir()
	ca := identity.NewCAManager(dir)
	if err := ca.Create(); err != nil {
		t.Fatalf("create test CA: %v", err)
	}
	return ca
}

// stubDomainVerifier implements service.DomainVerifier.
type stubDomainVerifier struct {
	verified bool
}

func (s *stubDomainVerifier) IsDomainVerified(_ context.Context, _ string) (bool, error) {
	return s.verified, nil
}

// ── Tests ────────────────────────────────────────────────────────────────

func TestRegister_success(t *testing.T) {
	repo := newStubAgentRepo()
	svc := newTestAgentService(repo, nil, nil, nil)

	agent, err := svc.Register(context.Background(), testRegisterRequest())
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if agent.ID == uuid.Nil {
		t.Error("expected non-nil ID")
	}
	if agent.Status != model.AgentStatusPending {
		t.Errorf("expected pending, got %s", agent.Status)
	}
	if !strings.HasPrefix(agent.AgentID, "agent_") {
		t.Errorf("AgentID should start with agent_, got %q", agent.AgentID)
	}
}

func TestRegister_setsURI(t *testing.T) {
	repo := newStubAgentRepo()
	svc := newTestAgentService(repo, nil, nil, nil)

	agent, err := svc.Register(context.Background(), testRegisterRequest())
	if err != nil {
		t.Fatal(err)
	}
	uri := agent.URI()
	// Format: agent://{owner_domain}/{category}/{agent_id}
	if !strings.HasPrefix(uri, "agent://example.com/finance/agent_") {
		t.Errorf("unexpected URI: %s", uri)
	}
}

func TestRegister_normalizesCapability(t *testing.T) {
	repo := newStubAgentRepo()
	svc := newTestAgentService(repo, nil, nil, nil)

	req := testRegisterRequest()
	req.Capability = ""        // clear primary field
	req.Category = ""          // clear secondary field
	req.CapabilityNode = "Finance" // legacy fallback: uppercased, should normalize to "finance"

	agent, err := svc.Register(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if agent.CapabilityNode != "finance" {
		t.Errorf("expected normalized capability, got %q", agent.CapabilityNode)
	}
}

func TestRegister_writesLedger(t *testing.T) {
	repo := newStubAgentRepo()
	ledger := trustledger.New()
	svc := newTestAgentService(repo, nil, ledger, nil)

	_, err := svc.Register(context.Background(), testRegisterRequest())
	if err != nil {
		t.Fatal(err)
	}

	n, _ := ledger.Len(context.Background())
	if n != 2 { // genesis + register
		t.Errorf("expected 2 ledger entries, got %d", n)
	}
}

func TestGet_found(t *testing.T) {
	repo := newStubAgentRepo()
	svc := newTestAgentService(repo, nil, nil, nil)

	created, _ := svc.Register(context.Background(), testRegisterRequest())

	got, err := svc.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID mismatch")
	}
}

func TestGet_notFound(t *testing.T) {
	svc := newTestAgentService(newStubAgentRepo(), nil, nil, nil)
	_, err := svc.Get(context.Background(), uuid.New())
	if err == nil {
		t.Error("expected error for missing agent")
	}
}

func TestList_filterByOrgName(t *testing.T) {
	repo := newStubAgentRepo()
	svc := newTestAgentService(repo, nil, nil, nil)

	req1 := testRegisterRequest()
	req1.OwnerDomain = "acme.com"
	svc.Register(context.Background(), req1)

	req2 := testRegisterRequest()
	req2.OwnerDomain = "other-corp.com"
	svc.Register(context.Background(), req2)

	// trust_root = owner_domain for domain-verified agents
	agents, err := svc.List(context.Background(), "acme.com", "", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(agents))
	}
}

func TestList_filterByCategory(t *testing.T) {
	repo := newStubAgentRepo()
	svc := newTestAgentService(repo, nil, nil, nil)

	req1 := testRegisterRequest()
	req1.Capability = "finance>accounting"
	req1.OrgName = "acme"
	svc.Register(context.Background(), req1)

	req2 := testRegisterRequest()
	req2.Capability = "healthcare>clinical"
	req2.OrgName = "mayo"
	svc.Register(context.Background(), req2)

	// Prefix filter: "finance" should match "finance>accounting"
	agents, err := svc.List(context.Background(), "", "finance", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(agents))
	}
}

func TestActivate_pendingToActive(t *testing.T) {
	repo := newStubAgentRepo()
	svc := newTestAgentService(repo, nil, nil, nil)

	agent, _ := svc.Register(context.Background(), testRegisterRequest())
	result, err := svc.Activate(context.Background(), agent.ID)
	if err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if result.Agent.Status != model.AgentStatusActive {
		t.Errorf("expected active, got %s", result.Agent.Status)
	}
}

func TestActivate_issuesCert(t *testing.T) {
	repo := newStubAgentRepo()
	ca := testCA(t)
	issuer := identity.NewIssuer(ca)
	svc := newTestAgentService(repo, issuer, nil, nil)

	agent, _ := svc.Register(context.Background(), testRegisterRequest())
	result, err := svc.Activate(context.Background(), agent.ID)
	if err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if result.CertPEM == "" {
		t.Error("expected cert to be issued")
	}
	if result.KeyPEM == "" {
		t.Error("expected key to be returned")
	}
	if result.Serial == "" {
		t.Error("expected serial to be set")
	}
	if result.CAPEM == "" {
		t.Error("expected CA PEM")
	}
}

func TestActivate_requiresDomainVerification(t *testing.T) {
	repo := newStubAgentRepo()
	verifier := &stubDomainVerifier{verified: false}
	svc := newTestAgentService(repo, nil, nil, verifier)

	agent, _ := svc.Register(context.Background(), testRegisterRequest())
	_, err := svc.Activate(context.Background(), agent.ID)
	if err == nil {
		t.Error("expected activation to fail when domain not verified")
	}
	if !strings.Contains(err.Error(), "not verified") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestActivate_skipsDomainCheck_whenVerifierNil(t *testing.T) {
	repo := newStubAgentRepo()
	svc := newTestAgentService(repo, nil, nil, nil) // nil verifier

	agent, _ := svc.Register(context.Background(), testRegisterRequest())
	result, err := svc.Activate(context.Background(), agent.ID)
	if err != nil {
		t.Fatalf("expected activation to succeed: %v", err)
	}
	if result.Agent.Status != model.AgentStatusActive {
		t.Errorf("expected active, got %s", result.Agent.Status)
	}
}

func TestActivate_succeedsWhenDomainVerified(t *testing.T) {
	repo := newStubAgentRepo()
	verifier := &stubDomainVerifier{verified: true}
	svc := newTestAgentService(repo, nil, nil, verifier)

	agent, _ := svc.Register(context.Background(), testRegisterRequest())
	result, err := svc.Activate(context.Background(), agent.ID)
	if err != nil {
		t.Fatalf("expected activation to succeed: %v", err)
	}
	if result.Agent.Status != model.AgentStatusActive {
		t.Errorf("expected active, got %s", result.Agent.Status)
	}
}

func TestRevoke_success(t *testing.T) {
	repo := newStubAgentRepo()
	svc := newTestAgentService(repo, nil, nil, nil)

	agent, _ := svc.Register(context.Background(), testRegisterRequest())
	svc.Activate(context.Background(), agent.ID)

	if err := svc.Revoke(context.Background(), agent.ID, ""); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	got, _ := svc.Get(context.Background(), agent.ID)
	if got.Status != model.AgentStatusRevoked {
		t.Errorf("expected revoked, got %s", got.Status)
	}
}

func TestRevoke_notFound(t *testing.T) {
	svc := newTestAgentService(newStubAgentRepo(), nil, nil, nil)
	err := svc.Revoke(context.Background(), uuid.New(), "")
	if err == nil {
		t.Error("expected error for missing agent")
	}
}

func TestUpdate_fields(t *testing.T) {
	repo := newStubAgentRepo()
	svc := newTestAgentService(repo, nil, nil, nil)

	agent, _ := svc.Register(context.Background(), testRegisterRequest())
	updated, err := svc.Update(context.Background(), agent.ID, &model.UpdateRequest{
		DisplayName: "Updated Name",
		Endpoint:    "https://new.example.com",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.DisplayName != "Updated Name" {
		t.Errorf("display name not updated: %q", updated.DisplayName)
	}
	if updated.Endpoint != "https://new.example.com" {
		t.Errorf("endpoint not updated: %q", updated.Endpoint)
	}
}

func TestUpdate_notFound(t *testing.T) {
	svc := newTestAgentService(newStubAgentRepo(), nil, nil, nil)
	_, err := svc.Update(context.Background(), uuid.New(), &model.UpdateRequest{DisplayName: "x"})
	if err == nil {
		t.Error("expected error for missing agent")
	}
}

func TestDelete_success(t *testing.T) {
	repo := newStubAgentRepo()
	svc := newTestAgentService(repo, nil, nil, nil)

	agent, _ := svc.Register(context.Background(), testRegisterRequest())
	if err := svc.Delete(context.Background(), agent.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := svc.Get(context.Background(), agent.ID)
	if err == nil {
		t.Error("expected agent to be deleted")
	}
}

func TestDelete_notFound(t *testing.T) {
	svc := newTestAgentService(newStubAgentRepo(), nil, nil, nil)
	err := svc.Delete(context.Background(), uuid.New())
	if err == nil {
		t.Error("expected error for missing agent")
	}
}

func TestResolve_activeAgent(t *testing.T) {
	repo := newStubAgentRepo()
	svc := newTestAgentService(repo, nil, nil, nil)

	agent, _ := svc.Register(context.Background(), testRegisterRequest())
	svc.Activate(context.Background(), agent.ID)

	resolved, err := svc.Resolve(context.Background(), "example.com", "finance", agent.AgentID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.Endpoint != "https://tax.example.com" {
		t.Errorf("unexpected endpoint: %s", resolved.Endpoint)
	}
}

func TestResolve_inactiveAgent(t *testing.T) {
	repo := newStubAgentRepo()
	svc := newTestAgentService(repo, nil, nil, nil)

	agent, _ := svc.Register(context.Background(), testRegisterRequest())
	// Don't activate — agent is pending

	_, err := svc.Resolve(context.Background(), "example.com", "finance", agent.AgentID)
	if err == nil {
		t.Error("expected error for pending agent")
	}
	if !strings.Contains(err.Error(), "not active") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ── stubEmailChecker ──────────────────────────────────────────────────────

type stubEmailChecker struct {
	verified bool
}

func (s *stubEmailChecker) IsEmailVerified(_ context.Context, _ uuid.UUID) (bool, error) {
	return s.verified, nil
}

// ── Free-tier (nap_hosted) tests ──────────────────────────────────────────

func napHostedRequest(userID uuid.UUID, username string) *model.RegisterRequest {
	return &model.RegisterRequest{
		RegistrationType: model.RegistrationTypeNAPHosted,
		DisplayName:      "My Hosted Agent",
		Endpoint:         "https://myagent.example.com",
		OwnerUserID:      &userID,
		Username:         username,
		Capability:       "finance>accounting",
	}
}

func TestRegister_napHosted_success(t *testing.T) {
	repo := newStubAgentRepo()
	svc := newTestAgentService(repo, nil, nil, nil)

	userID := uuid.New()
	agent, err := svc.Register(context.Background(), napHostedRequest(userID, "alice"))
	if err != nil {
		t.Fatalf("Register nap_hosted: %v", err)
	}
	if agent.RegistrationType != model.RegistrationTypeNAPHosted {
		t.Errorf("expected nap_hosted, got %s", agent.RegistrationType)
	}
	// New URI model: capability_node = full capability path,
	// trust_root = FreeTierConfig.TrustRoot ("nap"), NOT the username.
	// This prevents impersonation: anyone who picks username "amazon" should
	// not receive agent://amazon/finance/<id>.
	if agent.CapabilityNode != "finance>accounting" {
		t.Errorf("capability_node: expected finance>accounting, got %s", agent.CapabilityNode)
	}
	if agent.TrustRoot != "nap" {
		t.Errorf("trust_root: expected nap (FreeTierConfig.TrustRoot), got %s", agent.TrustRoot)
	}
	if agent.OwnerUserID == nil || *agent.OwnerUserID != userID {
		t.Errorf("owner_user_id mismatch")
	}
}

func TestRegister_napHosted_quotaEnforced(t *testing.T) {
	repo := newStubAgentRepo()
	svc := newTestAgentService(repo, nil, nil, nil)
	svc.SetFreeTierConfig(service.FreeTierConfig{TrustRoot: "nexusagentprotocol.com", MaxAgents: 2})

	userID := uuid.New()
	for i := 0; i < 2; i++ {
		_, err := svc.Register(context.Background(), napHostedRequest(userID, "alice"))
		if err != nil {
			t.Fatalf("register agent %d: %v", i, err)
		}
	}

	_, err := svc.Register(context.Background(), napHostedRequest(userID, "alice"))
	if err == nil {
		t.Fatal("expected quota error for 3rd agent")
	}
	if !strings.Contains(err.Error(), "agent limit") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRegister_napHosted_requiresOwnerUserID(t *testing.T) {
	svc := newTestAgentService(newStubAgentRepo(), nil, nil, nil)
	req := &model.RegisterRequest{
		RegistrationType: model.RegistrationTypeNAPHosted,
		DisplayName:      "Agent",
		Endpoint:         "https://example.com",
		Username:         "alice",
		// OwnerUserID intentionally absent
	}
	_, err := svc.Register(context.Background(), req)
	if err == nil {
		t.Error("expected error when owner_user_id missing")
	}
}

func TestActivate_napHosted_requiresEmailVerified(t *testing.T) {
	repo := newStubAgentRepo()
	svc := newTestAgentService(repo, nil, nil, nil)
	svc.SetEmailChecker(&stubEmailChecker{verified: false})

	userID := uuid.New()
	agent, err := svc.Register(context.Background(), napHostedRequest(userID, "alice"))
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	_, err = svc.Activate(context.Background(), agent.ID)
	if err == nil {
		t.Error("expected error when email not verified")
	}
	if !strings.Contains(err.Error(), "email") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestActivate_napHosted_succeedsWhenEmailVerified(t *testing.T) {
	repo := newStubAgentRepo()
	svc := newTestAgentService(repo, nil, nil, nil)
	svc.SetEmailChecker(&stubEmailChecker{verified: true})

	userID := uuid.New()
	agent, err := svc.Register(context.Background(), napHostedRequest(userID, "alice"))
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	result, err := svc.Activate(context.Background(), agent.ID)
	if err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if result.Agent.Status != model.AgentStatusActive {
		t.Errorf("expected active, got %s", result.Agent.Status)
	}
}

func TestActivate_napHosted_skipsEmailCheck_whenCheckerNil(t *testing.T) {
	repo := newStubAgentRepo()
	svc := newTestAgentService(repo, nil, nil, nil)
	// No email checker set — should activate without email verification gate

	userID := uuid.New()
	agent, err := svc.Register(context.Background(), napHostedRequest(userID, "alice"))
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	result, err := svc.Activate(context.Background(), agent.ID)
	if err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if result.Agent.Status != model.AgentStatusActive {
		t.Errorf("expected active, got %s", result.Agent.Status)
	}
}

func TestRegister_domain_requiresOwnerDomain(t *testing.T) {
	svc := newTestAgentService(newStubAgentRepo(), nil, nil, nil)
	req := &model.RegisterRequest{
		RegistrationType: model.RegistrationTypeDomain,
		Capability:       "finance",
		DisplayName:      "Agent",
		Endpoint:         "https://example.com",
		// OwnerDomain intentionally absent — should fail
	}
	_, err := svc.Register(context.Background(), req)
	if err == nil {
		t.Error("expected error when owner_domain missing for domain registration")
	}
}

// ── Revocation & Suspension Tests ────────────────────────────────────────

func TestRevoke_withReason(t *testing.T) {
	repo := newStubAgentRepo()
	svc := newTestAgentService(repo, nil, nil, nil)

	agent, _ := svc.Register(context.Background(), testRegisterRequest())
	svc.Activate(context.Background(), agent.ID)

	if err := svc.Revoke(context.Background(), agent.ID, "policy violation"); err != nil {
		t.Fatalf("Revoke with reason: %v", err)
	}

	got, _ := svc.Get(context.Background(), agent.ID)
	if got.Status != model.AgentStatusRevoked {
		t.Errorf("expected revoked, got %s", got.Status)
	}
	if got.RevocationReason != "policy violation" {
		t.Errorf("expected reason 'policy violation', got %q", got.RevocationReason)
	}
}

func TestSuspend_activeToSuspended(t *testing.T) {
	repo := newStubAgentRepo()
	svc := newTestAgentService(repo, nil, nil, nil)

	agent, _ := svc.Register(context.Background(), testRegisterRequest())
	svc.Activate(context.Background(), agent.ID)

	if err := svc.Suspend(context.Background(), agent.ID); err != nil {
		t.Fatalf("Suspend: %v", err)
	}

	got, _ := svc.Get(context.Background(), agent.ID)
	if got.Status != model.AgentStatusSuspended {
		t.Errorf("expected suspended, got %s", got.Status)
	}
	if got.SuspendedAt == nil {
		t.Error("expected suspended_at to be set")
	}
}

func TestRestore_suspendedToActive(t *testing.T) {
	repo := newStubAgentRepo()
	svc := newTestAgentService(repo, nil, nil, nil)

	agent, _ := svc.Register(context.Background(), testRegisterRequest())
	svc.Activate(context.Background(), agent.ID)
	svc.Suspend(context.Background(), agent.ID)

	if err := svc.Restore(context.Background(), agent.ID); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	got, _ := svc.Get(context.Background(), agent.ID)
	if got.Status != model.AgentStatusActive {
		t.Errorf("expected active, got %s", got.Status)
	}
	if got.SuspendedAt != nil {
		t.Error("expected suspended_at to be cleared")
	}
}

func TestRestore_failsOnRevoked(t *testing.T) {
	repo := newStubAgentRepo()
	svc := newTestAgentService(repo, nil, nil, nil)

	agent, _ := svc.Register(context.Background(), testRegisterRequest())
	svc.Activate(context.Background(), agent.ID)
	svc.Revoke(context.Background(), agent.ID, "")

	err := svc.Restore(context.Background(), agent.ID)
	if err == nil {
		t.Error("expected error restoring a revoked agent")
	}
}

func TestSuspend_failsOnPending(t *testing.T) {
	repo := newStubAgentRepo()
	svc := newTestAgentService(repo, nil, nil, nil)

	agent, _ := svc.Register(context.Background(), testRegisterRequest())

	err := svc.Suspend(context.Background(), agent.ID)
	if err == nil {
		t.Error("expected error suspending a pending agent")
	}
}

// ── Deprecation Tests ────────────────────────────────────────────────────

func TestDeprecate_success(t *testing.T) {
	repo := newStubAgentRepo()
	svc := newTestAgentService(repo, nil, nil, nil)

	agent, _ := svc.Register(context.Background(), testRegisterRequest())
	svc.Activate(context.Background(), agent.ID)

	req := &model.DeprecateRequest{
		SunsetDate:     "2026-12-01",
		ReplacementURI: "agent://example.com/finance/agent_new123",
	}
	if err := svc.Deprecate(context.Background(), agent.ID, req); err != nil {
		t.Fatalf("Deprecate: %v", err)
	}

	got, _ := svc.Get(context.Background(), agent.ID)
	if got.Status != model.AgentStatusDeprecated {
		t.Errorf("expected deprecated, got %s", got.Status)
	}
	if got.ReplacementURI != "agent://example.com/finance/agent_new123" {
		t.Errorf("unexpected replacement_uri: %q", got.ReplacementURI)
	}
}

func TestResolve_deprecatedIncludesAgent(t *testing.T) {
	repo := newStubAgentRepo()
	svc := newTestAgentService(repo, nil, nil, nil)

	agent, _ := svc.Register(context.Background(), testRegisterRequest())
	svc.Activate(context.Background(), agent.ID)
	svc.Deprecate(context.Background(), agent.ID, &model.DeprecateRequest{})

	// Resolve should still return deprecated agents.
	resolved, err := svc.Resolve(context.Background(), "example.com", "finance", agent.AgentID)
	if err != nil {
		t.Fatalf("Resolve deprecated: %v", err)
	}
	if resolved.Status != model.AgentStatusDeprecated {
		t.Errorf("expected deprecated status, got %s", resolved.Status)
	}
}

func TestDeprecate_failsOnPending(t *testing.T) {
	repo := newStubAgentRepo()
	svc := newTestAgentService(repo, nil, nil, nil)

	agent, _ := svc.Register(context.Background(), testRegisterRequest())

	err := svc.Deprecate(context.Background(), agent.ID, nil)
	if err == nil {
		t.Error("expected error deprecating a pending agent")
	}
}

// Suppress unused import warning for time package.
var _ = time.Now
