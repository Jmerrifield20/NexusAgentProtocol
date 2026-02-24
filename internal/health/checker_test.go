package health

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ── Stubs ────────────────────────────────────────────────────────────────

type stubLister struct {
	agents []EndpointAgent
}

func (s *stubLister) ListActiveEndpoints(_ context.Context) ([]EndpointAgent, error) {
	return s.agents, nil
}

type stubUpdater struct {
	statuses map[uuid.UUID]string
}

func (s *stubUpdater) UpdateHealthStatus(_ context.Context, id uuid.UUID, status string, _ time.Time) error {
	s.statuses[id] = status
	return nil
}

// ── Tests ────────────────────────────────────────────────────────────────

func TestProbeEndpoint_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	checker := New(nil, nil, Config{ProbeTimeout: 5 * time.Second}, zap.NewNop())
	if !checker.probeEndpoint(context.Background(), srv.URL) {
		t.Error("expected probe to succeed")
	}
}

func TestProbeEndpoint_failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	checker := New(nil, nil, Config{ProbeTimeout: 5 * time.Second}, zap.NewNop())
	if checker.probeEndpoint(context.Background(), srv.URL) {
		t.Error("expected probe to fail")
	}
}

func TestCheckAll_degradesAfterThreshold(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	agentID := uuid.New()
	lister := &stubLister{agents: []EndpointAgent{
		{ID: agentID, URI: "agent://test/cap/id", Endpoint: srv.URL},
	}}
	updater := &stubUpdater{statuses: make(map[uuid.UUID]string)}

	checker := New(lister, updater, Config{
		ProbeTimeout:  5 * time.Second,
		FailThreshold: 3,
	}, zap.NewNop())

	// Run 3 times to hit the threshold.
	for i := 0; i < 3; i++ {
		checker.CheckAll(context.Background())
	}

	if updater.statuses[agentID] != "degraded" {
		t.Errorf("expected degraded, got %q", updater.statuses[agentID])
	}
}

func TestCheckAll_recoversOnSuccess(t *testing.T) {
	failCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if failCount < 3 {
			failCount++
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	agentID := uuid.New()
	lister := &stubLister{agents: []EndpointAgent{
		{ID: agentID, URI: "agent://test/cap/id", Endpoint: srv.URL},
	}}
	updater := &stubUpdater{statuses: make(map[uuid.UUID]string)}

	checker := New(lister, updater, Config{
		ProbeTimeout:  5 * time.Second,
		FailThreshold: 3,
	}, zap.NewNop())

	// Fail 3 times, then succeed.
	for i := 0; i < 4; i++ {
		checker.CheckAll(context.Background())
	}

	if updater.statuses[agentID] != "healthy" {
		t.Errorf("expected healthy after recovery, got %q", updater.statuses[agentID])
	}
}
