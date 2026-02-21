package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	internaldns "github.com/nexus-protocol/nexus/internal/dns"
	"github.com/nexus-protocol/nexus/internal/registry/model"
	"github.com/nexus-protocol/nexus/internal/registry/repository"
	"github.com/nexus-protocol/nexus/internal/registry/service"
	"go.uber.org/zap"
)

// ── In-memory stub for challengeStore ──────────────────────────────────────

type stubChallengeStore struct {
	mu   sync.RWMutex
	rows map[uuid.UUID]*model.DNSChallenge
}

func newStubStore() *stubChallengeStore {
	return &stubChallengeStore{rows: make(map[uuid.UUID]*model.DNSChallenge)}
}

func (s *stubChallengeStore) Create(_ context.Context, ch *model.DNSChallenge) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch.ID = uuid.New()
	ch.CreatedAt = time.Now().UTC()
	cp := *ch
	s.rows[ch.ID] = &cp
	return nil
}

func (s *stubChallengeStore) GetByID(_ context.Context, id uuid.UUID) (*model.DNSChallenge, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ch, ok := s.rows[id]
	if !ok {
		return nil, repository.ErrChallengeNotFound
	}
	cp := *ch
	return &cp, nil
}

func (s *stubChallengeStore) MarkVerified(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch, ok := s.rows[id]
	if !ok {
		return repository.ErrChallengeNotFound
	}
	ch.Verified = true
	return nil
}

func (s *stubChallengeStore) FindVerifiedByDomain(_ context.Context, domain string) (*model.DNSChallenge, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, ch := range s.rows {
		if ch.Domain == domain && ch.Verified {
			cp := *ch
			return &cp, nil
		}
	}
	return nil, repository.ErrChallengeNotFound
}

// ── Helpers ────────────────────────────────────────────────────────────────

func newDNSSvc(store *stubChallengeStore, vfn func(context.Context, *internaldns.Challenge) error) *service.DNSChallengeService {
	return service.NewDNSChallengeService(store, vfn, zap.NewNop())
}

func successVerify(_ context.Context, _ *internaldns.Challenge) error { return nil }
func failVerify(_ context.Context, _ *internaldns.Challenge) error {
	return errors.New("TXT record not found")
}

// ── Tests ──────────────────────────────────────────────────────────────────

func TestStartChallenge_createsRecord(t *testing.T) {
	store := newStubStore()
	svc := newDNSSvc(store, successVerify)

	ch, err := svc.StartChallenge(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("StartChallenge: %v", err)
	}
	if ch.ID == uuid.Nil {
		t.Error("expected non-nil ID")
	}
	if ch.Domain != "example.com" {
		t.Errorf("Domain: got %q", ch.Domain)
	}
	if ch.TXTHost == "" {
		t.Error("TXTHost must not be empty")
	}
	if ch.TXTRecord == "" {
		t.Error("TXTRecord must not be empty")
	}
	if ch.Verified {
		t.Error("challenge must not be verified on creation")
	}
}

func TestStartChallenge_emptyDomain(t *testing.T) {
	svc := newDNSSvc(newStubStore(), successVerify)
	_, err := svc.StartChallenge(context.Background(), "")
	if err == nil {
		t.Error("expected error for empty domain")
	}
}

func TestGetChallenge_returnsChallenge(t *testing.T) {
	store := newStubStore()
	svc := newDNSSvc(store, successVerify)

	created, _ := svc.StartChallenge(context.Background(), "example.com")

	got, err := svc.GetChallenge(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetChallenge: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID mismatch: %v vs %v", got.ID, created.ID)
	}
	if got.TXTHost == "" {
		t.Error("TXTHost should be populated by GetChallenge")
	}
}

func TestGetChallenge_notFound(t *testing.T) {
	svc := newDNSSvc(newStubStore(), successVerify)
	_, err := svc.GetChallenge(context.Background(), uuid.New())
	if !errors.Is(err, service.ErrChallengeNotFound) {
		t.Errorf("expected ErrChallengeNotFound, got %v", err)
	}
}

func TestVerifyChallenge_success(t *testing.T) {
	store := newStubStore()
	svc := newDNSSvc(store, successVerify)

	ch, _ := svc.StartChallenge(context.Background(), "example.com")

	result, err := svc.VerifyChallenge(context.Background(), ch.ID)
	if err != nil {
		t.Fatalf("VerifyChallenge: %v", err)
	}
	if !result.Verified {
		t.Error("expected Verified=true")
	}
}

func TestVerifyChallenge_dnsFailure(t *testing.T) {
	store := newStubStore()
	svc := newDNSSvc(store, failVerify)

	ch, _ := svc.StartChallenge(context.Background(), "example.com")

	_, err := svc.VerifyChallenge(context.Background(), ch.ID)
	if !errors.Is(err, service.ErrVerificationFailed) {
		t.Errorf("expected ErrVerificationFailed, got %v", err)
	}
}

func TestVerifyChallenge_expired(t *testing.T) {
	store := newStubStore()
	svc := newDNSSvc(store, successVerify)

	ch, _ := svc.StartChallenge(context.Background(), "example.com")

	// Manually set the challenge as expired in the store.
	store.mu.Lock()
	store.rows[ch.ID].ExpiresAt = time.Now().Add(-time.Minute)
	store.mu.Unlock()

	_, err := svc.VerifyChallenge(context.Background(), ch.ID)
	if !errors.Is(err, service.ErrChallengeExpired) {
		t.Errorf("expected ErrChallengeExpired, got %v", err)
	}
}

func TestVerifyChallenge_idempotent(t *testing.T) {
	store := newStubStore()
	svc := newDNSSvc(store, successVerify)

	ch, _ := svc.StartChallenge(context.Background(), "example.com")
	if _, err := svc.VerifyChallenge(context.Background(), ch.ID); err != nil {
		t.Fatalf("first verify: %v", err)
	}
	// Second call should succeed without error (idempotent).
	if _, err := svc.VerifyChallenge(context.Background(), ch.ID); err != nil {
		t.Errorf("second verify (idempotent) failed: %v", err)
	}
}

func TestIsDomainVerified_trueAfterVerify(t *testing.T) {
	store := newStubStore()
	svc := newDNSSvc(store, successVerify)

	ch, _ := svc.StartChallenge(context.Background(), "example.com")
	if _, err := svc.VerifyChallenge(context.Background(), ch.ID); err != nil {
		t.Fatal(err)
	}

	ok, err := svc.IsDomainVerified(context.Background(), "example.com")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected domain to be verified")
	}
}

func TestIsDomainVerified_falseBeforeVerify(t *testing.T) {
	svc := newDNSSvc(newStubStore(), successVerify)

	ok, err := svc.IsDomainVerified(context.Background(), "example.com")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected domain not to be verified before challenge")
	}
}

func TestIsDomainVerified_falseAfterFailedVerify(t *testing.T) {
	store := newStubStore()
	svc := newDNSSvc(store, failVerify)

	ch, _ := svc.StartChallenge(context.Background(), "example.com")
	_, _ = svc.VerifyChallenge(context.Background(), ch.ID) // expected to fail

	ok, err := svc.IsDomainVerified(context.Background(), "example.com")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("domain should not be verified after failed DNS check")
	}
}
