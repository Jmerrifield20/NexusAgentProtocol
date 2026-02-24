package federation

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// inMemoryFedRepo is a minimal in-memory federationRepo for delegation tests.
type inMemoryFedRepo struct {
	registries map[string]*RegisteredRegistry
}

func newInMemoryFedRepo() *inMemoryFedRepo {
	return &inMemoryFedRepo{registries: make(map[string]*RegisteredRegistry)}
}

func (r *inMemoryFedRepo) Create(_ context.Context, reg *RegisteredRegistry) error {
	id := uuid.New().String()
	reg.ID = id
	r.registries[id] = reg
	return nil
}

func (r *inMemoryFedRepo) GetByTrustRoot(_ context.Context, trustRoot string) (*RegisteredRegistry, error) {
	for _, reg := range r.registries {
		if reg.TrustRoot == trustRoot {
			return reg, nil
		}
	}
	return nil, ErrNotFound
}

func (r *inMemoryFedRepo) GetByID(_ context.Context, id uuid.UUID) (*RegisteredRegistry, error) {
	if reg, ok := r.registries[id.String()]; ok {
		return reg, nil
	}
	return nil, ErrNotFound
}

func (r *inMemoryFedRepo) List(_ context.Context, _ RegistryStatus, _, _ int) ([]*RegisteredRegistry, error) {
	return nil, nil
}

func (r *inMemoryFedRepo) UpdateStatus(_ context.Context, id uuid.UUID, status RegistryStatus) error {
	reg, ok := r.registries[id.String()]
	if !ok {
		return ErrNotFound
	}
	reg.Status = status
	return nil
}

func (r *inMemoryFedRepo) SetIntermediateCA(_ context.Context, id uuid.UUID, certPEM string) error {
	reg, ok := r.registries[id.String()]
	if !ok {
		return ErrNotFound
	}
	reg.IntermediateCA = certPEM
	return nil
}

func (r *inMemoryFedRepo) UpdateMaxPathLen(_ context.Context, id uuid.UUID, maxPathLen int) error {
	reg, ok := r.registries[id.String()]
	if !ok {
		return ErrNotFound
	}
	reg.MaxPathLen = maxPathLen
	return nil
}

func TestUpdateMaxPathLen_Validation(t *testing.T) {
	repo := newInMemoryFedRepo()
	svc := NewFederationService(repo, nil, zap.NewNop())

	// Register and approve a registry.
	reg, err := svc.Register(context.Background(), &RegisterRequest{
		TrustRoot:   "gov.kr",
		EndpointURL: "https://registry.gov.kr",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	id, _ := uuid.Parse(reg.ID)

	// Negative value must be rejected.
	if err := svc.UpdateMaxPathLen(context.Background(), id, -1); err == nil {
		t.Error("expected error for negative max_path_len, got nil")
	}

	// Value exceeding MaxAllowedPathLen must be rejected.
	if err := svc.UpdateMaxPathLen(context.Background(), id, MaxAllowedPathLen+1); err == nil {
		t.Errorf("expected error for max_path_len > %d, got nil", MaxAllowedPathLen)
	}

	// Valid values.
	for _, v := range []int{0, 1, MaxAllowedPathLen} {
		if err := svc.UpdateMaxPathLen(context.Background(), id, v); err != nil {
			t.Errorf("UpdateMaxPathLen(%d) unexpected error: %v", v, err)
		}
	}
}

func TestUpdateMaxPathLen_NotFound(t *testing.T) {
	repo := newInMemoryFedRepo()
	svc := NewFederationService(repo, nil, zap.NewNop())

	bogusID := uuid.New()
	err := svc.UpdateMaxPathLen(context.Background(), bogusID, 1)
	if err == nil {
		t.Error("expected error for non-existent registry, got nil")
	}
}
