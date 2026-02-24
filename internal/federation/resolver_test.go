package federation

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// stubFedRepo is a minimal federationRepo for resolver tests.
type stubFedRepo struct {
	byTrustRoot map[string]*RegisteredRegistry
}

func (s *stubFedRepo) GetByTrustRoot(_ context.Context, trustRoot string) (*RegisteredRegistry, error) {
	if reg, ok := s.byTrustRoot[trustRoot]; ok {
		return reg, nil
	}
	return nil, fmt.Errorf("not found")
}

func (s *stubFedRepo) Create(context.Context, *RegisteredRegistry) error              { return nil }
func (s *stubFedRepo) GetByID(context.Context, uuid.UUID) (*RegisteredRegistry, error) { return nil, nil }
func (s *stubFedRepo) List(context.Context, RegistryStatus, int, int) ([]*RegisteredRegistry, error) {
	return nil, nil
}
func (s *stubFedRepo) UpdateStatus(context.Context, uuid.UUID, RegistryStatus) error { return nil }
func (s *stubFedRepo) SetIntermediateCA(context.Context, uuid.UUID, string) error     { return nil }
func (s *stubFedRepo) UpdateMaxPathLen(context.Context, uuid.UUID, int) error         { return nil }

func newTestFedSvc(repo federationRepo) *FederationService {
	return NewFederationService(repo, nil, zap.NewNop())
}

// TestDiscoverEndpoint_DNSRejectedWithoutApproval verifies that when fedSvc is
// present (root mode) and DNS returns a URL, but the trust root has no active
// entry in the federation table, DNS is rejected and we fall through to the
// root registry fallback.
func TestDiscoverEndpoint_DNSRejectedWithoutApproval(t *testing.T) {
	repo := &stubFedRepo{byTrustRoot: map[string]*RegisteredRegistry{}}
	fedSvc := newTestFedSvc(repo)

	rr := NewRemoteResolver(fedSvc, "https://root.example.com", true, 0, zap.NewNop())
	rr.dnsDiscoverFn = func(trustRoot string) (string, bool) {
		return "https://rogue.example.com", true
	}

	url, err := rr.discoverEndpoint(context.Background(), "rogue.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://root.example.com" {
		t.Errorf("expected root fallback URL, got %q", url)
	}
}

// TestDiscoverEndpoint_DNSAcceptedWithApproval verifies that when the trust root
// has an active entry in the federation table, the DNS-discovered URL is accepted.
// We use a counting repo where the first GetByTrustRoot call (step 1) returns a
// transient error, but the second call (step 2 cross-reference) succeeds.
func TestDiscoverEndpoint_DNSAcceptedWithApproval(t *testing.T) {
	countingRepo := &countingFedRepo{
		callCount: 0,
		registry: &RegisteredRegistry{
			TrustRoot:   "acme.com",
			EndpointURL: "https://table.acme.com",
			Status:      StatusActive,
		},
	}
	fedSvc := newTestFedSvc(countingRepo)
	rr := NewRemoteResolver(fedSvc, "https://root.example.com", true, 0, zap.NewNop())
	rr.dnsDiscoverFn = func(trustRoot string) (string, bool) {
		return "https://dns.acme.com", true
	}

	url, err := rr.discoverEndpoint(context.Background(), "acme.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Step 1: first GetByTrustRoot returns error → skip.
	// Step 2: DNS returns URL, cross-ref GetByTrustRoot succeeds with active → accept DNS URL.
	if url != "https://dns.acme.com" {
		t.Errorf("expected DNS URL https://dns.acme.com, got %q", url)
	}
}

// TestDiscoverEndpoint_DNSWorksWithoutFedSvc verifies that when fedSvc is nil
// (federated mode), DNS discovery works without any cross-reference check.
func TestDiscoverEndpoint_DNSWorksWithoutFedSvc(t *testing.T) {
	rr := NewRemoteResolver(nil, "https://root.example.com", true, 0, zap.NewNop())
	rr.dnsDiscoverFn = func(trustRoot string) (string, bool) {
		return "https://dns.example.com", true
	}

	url, err := rr.discoverEndpoint(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://dns.example.com" {
		t.Errorf("expected DNS URL, got %q", url)
	}
}

// TestDiscoverEndpoint_TableTakesPrecedence verifies that when the federation
// table has an active entry, its URL is returned immediately without consulting DNS.
func TestDiscoverEndpoint_TableTakesPrecedence(t *testing.T) {
	repo := &stubFedRepo{byTrustRoot: map[string]*RegisteredRegistry{
		"acme.com": {
			ID:          "00000000-0000-0000-0000-000000000001",
			TrustRoot:   "acme.com",
			EndpointURL: "https://table.acme.com",
			Status:      StatusActive,
		},
	}}
	fedSvc := newTestFedSvc(repo)

	dnsCalled := false
	rr := NewRemoteResolver(fedSvc, "https://root.example.com", true, 0, zap.NewNop())
	rr.dnsDiscoverFn = func(trustRoot string) (string, bool) {
		dnsCalled = true
		return "https://dns.acme.com", true
	}

	url, err := rr.discoverEndpoint(context.Background(), "acme.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://table.acme.com" {
		t.Errorf("expected table URL, got %q", url)
	}
	if dnsCalled {
		t.Error("DNS should not have been consulted when table has an active entry")
	}
}

// countingFedRepo returns an error on the first GetByTrustRoot call
// and succeeds on the second (simulating a transient failure in step 1
// while step 2's cross-reference succeeds).
type countingFedRepo struct {
	callCount int
	registry  *RegisteredRegistry
}

func (c *countingFedRepo) GetByTrustRoot(_ context.Context, _ string) (*RegisteredRegistry, error) {
	c.callCount++
	if c.callCount == 1 {
		return nil, fmt.Errorf("transient error")
	}
	return c.registry, nil
}

func (c *countingFedRepo) Create(context.Context, *RegisteredRegistry) error              { return nil }
func (c *countingFedRepo) GetByID(context.Context, uuid.UUID) (*RegisteredRegistry, error) { return nil, nil }
func (c *countingFedRepo) List(context.Context, RegistryStatus, int, int) ([]*RegisteredRegistry, error) {
	return nil, nil
}
func (c *countingFedRepo) UpdateStatus(context.Context, uuid.UUID, RegistryStatus) error { return nil }
func (c *countingFedRepo) SetIntermediateCA(context.Context, uuid.UUID, string) error     { return nil }
func (c *countingFedRepo) UpdateMaxPathLen(context.Context, uuid.UUID, int) error         { return nil }
