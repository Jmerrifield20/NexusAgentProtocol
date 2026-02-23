package federation

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/identity"
	"go.uber.org/zap"
)

// FederationService provides business logic for the registry-of-registries.
type FederationService struct {
	repo   federationRepo
	issuer *identity.Issuer // nil when this node cannot issue intermediate CAs
	logger *zap.Logger
}

// NewFederationService creates a FederationService.
// issuer may be nil for federated registries that cannot issue sub-CAs.
func NewFederationService(repo federationRepo, issuer *identity.Issuer, logger *zap.Logger) *FederationService {
	return &FederationService{repo: repo, issuer: issuer, logger: logger}
}

// Register records a new registry application in pending state.
func (s *FederationService) Register(ctx context.Context, req *RegisterRequest) (*RegisteredRegistry, error) {
	if req.TrustRoot == "" {
		return nil, fmt.Errorf("trust_root is required")
	}
	if req.EndpointURL == "" {
		return nil, fmt.Errorf("endpoint_url is required")
	}

	reg := &RegisteredRegistry{
		TrustRoot:   req.TrustRoot,
		EndpointURL: req.EndpointURL,
		Status:      StatusPending,
	}

	if err := s.repo.Create(ctx, reg); err != nil {
		return nil, fmt.Errorf("create registry: %w", err)
	}

	s.logger.Info("federation registry registered",
		zap.String("trust_root", reg.TrustRoot),
		zap.String("id", reg.ID),
	)
	return reg, nil
}

// Approve transitions a registry from pending to active.
func (s *FederationService) Approve(ctx context.Context, id uuid.UUID) (*RegisteredRegistry, error) {
	if err := s.repo.UpdateStatus(ctx, id, StatusActive); err != nil {
		return nil, fmt.Errorf("approve registry: %w", err)
	}
	return s.repo.GetByID(ctx, id)
}

// Suspend marks a registry as suspended, blocking cross-registry resolution.
func (s *FederationService) Suspend(ctx context.Context, id uuid.UUID) (*RegisteredRegistry, error) {
	if err := s.repo.UpdateStatus(ctx, id, StatusSuspended); err != nil {
		return nil, fmt.Errorf("suspend registry: %w", err)
	}
	return s.repo.GetByID(ctx, id)
}

// List returns registered registries filtered by status.
func (s *FederationService) List(ctx context.Context, status RegistryStatus, limit, offset int) ([]*RegisteredRegistry, error) {
	return s.repo.List(ctx, status, limit, offset)
}

// GetByTrustRoot looks up an active registry by its trust root domain.
func (s *FederationService) GetByTrustRoot(ctx context.Context, trustRoot string) (*RegisteredRegistry, error) {
	return s.repo.GetByTrustRoot(ctx, trustRoot)
}

// IssueIntermediateCA signs and returns an intermediate CA certificate for an
// approved federated registry. The private key is returned once in the response
// and is never stored by the root registry.
func (s *FederationService) IssueIntermediateCA(ctx context.Context, trustRoot string) (*IssueCAResponse, error) {
	if s.issuer == nil {
		return nil, fmt.Errorf("this registry cannot issue intermediate CAs (no root issuer configured)")
	}

	reg, err := s.repo.GetByTrustRoot(ctx, trustRoot)
	if err != nil {
		return nil, fmt.Errorf("registry not found for trust_root %q: %w", trustRoot, err)
	}
	if reg.Status != StatusActive {
		return nil, fmt.Errorf("registry %q must be active before issuing a CA cert (status: %s)", trustRoot, reg.Status)
	}

	issued, err := s.issuer.IssueIntermediateCert(trustRoot, 5*365*24*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("issue intermediate cert: %w", err)
	}

	// Persist the cert PEM (non-fatal — the operator already has it in the response).
	regID, parseErr := uuid.Parse(reg.ID)
	if parseErr == nil {
		if dbErr := s.repo.SetIntermediateCA(ctx, regID, issued.CertPEM); dbErr != nil {
			s.logger.Error("persist intermediate CA cert (non-fatal)", zap.Error(dbErr))
		}
	}

	s.logger.Info("intermediate CA issued",
		zap.String("trust_root", trustRoot),
		zap.String("serial", issued.Serial),
		zap.Time("expires_at", issued.Cert.NotAfter),
	)

	return &IssueCAResponse{
		TrustRoot: trustRoot,
		CertPEM:   issued.CertPEM,
		KeyPEM:    issued.KeyPEM,
		Serial:    issued.Serial,
		ExpiresAt: issued.Cert.NotAfter.UTC().Format(time.RFC3339),
		RootCAPEM: s.issuer.CACertPEM(),
		Warning:   "Store the key_pem securely. It will not be shown again.",
	}, nil
}
