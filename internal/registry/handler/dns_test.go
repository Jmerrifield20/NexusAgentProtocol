package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	internaldns "github.com/jmerrifield20/NexusAgentProtocol/internal/dns"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/registry/handler"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/registry/model"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/registry/repository"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/registry/service"
	"go.uber.org/zap"
)

// ── Stub challenge store ─────────────────────────────────────────────────

type stubChallengeStore struct {
	mu   sync.RWMutex
	rows map[uuid.UUID]*model.DNSChallenge
}

func newStubChallengeStore() *stubChallengeStore {
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

func (s *stubChallengeStore) DeleteExpired(_ context.Context) (int64, error) {
	return 0, nil
}

func setupDNSRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	store := newStubChallengeStore()
	svc := service.NewDNSChallengeService(store, func(_ context.Context, _ *internaldns.Challenge) error {
		return nil
	}, zap.NewNop())
	h := handler.NewDNSHandler(svc, zap.NewNop())
	v1 := r.Group("/api/v1")
	h.Register(v1)
	return r
}

func setupDNSRouterWithVerifyFail(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	store := newStubChallengeStore()
	svc := service.NewDNSChallengeService(store, func(_ context.Context, _ *internaldns.Challenge) error {
		return errors.New("TXT record not found")
	}, zap.NewNop())
	h := handler.NewDNSHandler(svc, zap.NewNop())
	v1 := r.Group("/api/v1")
	h.Register(v1)
	return r
}

func TestStartChallenge_201(t *testing.T) {
	router := setupDNSRouter(t)

	body := `{"domain":"example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dns/challenge", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["domain"] != "example.com" {
		t.Errorf("unexpected domain: %v", resp["domain"])
	}
}

func TestStartChallenge_400_missingDomain(t *testing.T) {
	router := setupDNSRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/dns/challenge", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetChallenge_404(t *testing.T) {
	router := setupDNSRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dns/challenge/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestVerifyChallenge_422_whenDNSFails(t *testing.T) {
	router := setupDNSRouterWithVerifyFail(t)

	// Start a challenge first
	body := `{"domain":"example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dns/challenge", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	id := resp["id"].(string)

	// Try to verify — DNS lookup fails
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/dns/challenge/"+id+"/verify", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", w2.Code, w2.Body.String())
	}
}
