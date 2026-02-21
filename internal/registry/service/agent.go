package service

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nexus-protocol/nexus/internal/identity"
	"github.com/nexus-protocol/nexus/internal/registry/model"
	"github.com/nexus-protocol/nexus/internal/trustledger"
	"go.uber.org/zap"
)

// ActivationResult is returned by Activate and includes the issued certificate
// if the service was configured with an Issuer.
type ActivationResult struct {
	Agent *model.Agent

	// CertPEM is the X.509 agent certificate in PEM format.
	// Non-empty only when an Issuer is configured.
	CertPEM string

	// KeyPEM is the agent's RSA private key in PEM format.
	// This is delivered ONCE at activation time — it is not persisted.
	KeyPEM string

	// Serial is the certificate serial number in hex.
	Serial string

	// ExpiresAt is the certificate expiry time.
	ExpiresAt time.Time

	// CAPEM is the CA certificate in PEM format.
	// Clients use this to configure their TLS trust store.
	CAPEM string
}

// agentRepo is the persistence interface for the agent service.
// *repository.AgentRepository satisfies this interface.
type agentRepo interface {
	Create(ctx context.Context, agent *model.Agent) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Agent, error)
	GetByAgentID(ctx context.Context, trustRoot, capNode, agentID string) (*model.Agent, error)
	List(ctx context.Context, trustRoot, capNode string, limit, offset int) ([]*model.Agent, error)
	ListByOwnerDomain(ctx context.Context, domain string, limit, offset int) ([]*model.Agent, error)
	Update(ctx context.Context, agent *model.Agent) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.AgentStatus) error
	ActivateWithCert(ctx context.Context, id uuid.UUID, serial, certPEM string) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// DomainVerifier checks whether a domain has completed DNS-01 verification.
// *DNSChallengeService satisfies this interface.
type DomainVerifier interface {
	IsDomainVerified(ctx context.Context, domain string) (bool, error)
}

// AgentService contains business logic for agent lifecycle management.
type AgentService struct {
	repo        agentRepo
	issuer      *identity.Issuer   // nil = no cert issuance
	ledger      trustledger.Ledger // nil = no ledger writes
	dnsVerifier DomainVerifier     // nil = skip domain verification gate
	logger      *zap.Logger
}

// NewAgentService creates a new AgentService.
// issuer, ledger, and dnsVerifier may each be nil to disable that feature.
func NewAgentService(repo agentRepo, issuer *identity.Issuer, ledger trustledger.Ledger, dnsVerifier DomainVerifier, logger *zap.Logger) *AgentService {
	return &AgentService{repo: repo, issuer: issuer, ledger: ledger, dnsVerifier: dnsVerifier, logger: logger}
}

// appendLedger appends an audit entry to the ledger in a non-fatal manner.
// If the ledger is nil or the append fails, the error is only logged.
func (s *AgentService) appendLedger(ctx context.Context, agentURI, action, actor string, payload any) {
	if s.ledger == nil {
		return
	}
	if _, err := s.ledger.Append(ctx, agentURI, action, actor, payload); err != nil {
		s.logger.Error("ledger append failed (non-fatal)",
			zap.String("action", action),
			zap.String("agent_uri", agentURI),
			zap.Error(err),
		)
	}
}

// Register creates a new agent registration in pending state.
func (s *AgentService) Register(ctx context.Context, req *model.RegisterRequest) (*model.Agent, error) {
	agentID, err := generateAgentID()
	if err != nil {
		return nil, fmt.Errorf("generate agent ID: %w", err)
	}

	capNode := normalizeCapabilityNode(req.CapabilityNode)

	agent := &model.Agent{
		TrustRoot:      req.OwnerDomain,
		CapabilityNode: capNode,
		AgentID:        agentID,
		DisplayName:    req.DisplayName,
		Description:    req.Description,
		Endpoint:       req.Endpoint,
		OwnerDomain:    req.OwnerDomain,
		Status:         model.AgentStatusPending,
		PublicKeyPEM:   req.PublicKeyPEM,
		Metadata:       req.Metadata,
	}

	if err := s.repo.Create(ctx, agent); err != nil {
		s.logger.Error("failed to create agent", zap.Error(err))
		return nil, fmt.Errorf("create agent: %w", err)
	}

	s.logger.Info("agent registered",
		zap.String("agent_id", agentID),
		zap.String("trust_root", req.TrustRoot),
		zap.String("capability_node", capNode),
	)

	s.appendLedger(ctx, agent.URI(), "register", req.OwnerDomain, map[string]string{
		"trust_root":      agent.TrustRoot,
		"capability_node": agent.CapabilityNode,
		"agent_id":        agent.AgentID,
		"owner_domain":    agent.OwnerDomain,
		"endpoint":        agent.Endpoint,
	})

	return agent, nil
}

// Get retrieves an agent by its internal UUID.
func (s *AgentService) Get(ctx context.Context, id uuid.UUID) (*model.Agent, error) {
	return s.repo.GetByID(ctx, id)
}

// Resolve looks up an active agent by its URI components.
func (s *AgentService) Resolve(ctx context.Context, trustRoot, capNode, agentID string) (*model.Agent, error) {
	agent, err := s.repo.GetByAgentID(ctx, trustRoot, normalizeCapabilityNode(capNode), agentID)
	if err != nil {
		return nil, err
	}
	if agent.Status != model.AgentStatusActive {
		return nil, fmt.Errorf("agent is not active (status: %s)", agent.Status)
	}
	return agent, nil
}

// List returns a paginated list of agents.
func (s *AgentService) List(ctx context.Context, trustRoot, capNode string, limit, offset int) ([]*model.Agent, error) {
	return s.repo.List(ctx, trustRoot, capNode, limit, offset)
}

// Update modifies an existing agent's mutable fields.
func (s *AgentService) Update(ctx context.Context, id uuid.UUID, req *model.UpdateRequest) (*model.Agent, error) {
	agent, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.DisplayName != "" {
		agent.DisplayName = req.DisplayName
	}
	if req.Description != "" {
		agent.Description = req.Description
	}
	if req.Endpoint != "" {
		agent.Endpoint = req.Endpoint
	}
	if req.PublicKeyPEM != "" {
		agent.PublicKeyPEM = req.PublicKeyPEM
	}
	if req.Metadata != nil {
		agent.Metadata = req.Metadata
	}

	if err := s.repo.Update(ctx, agent); err != nil {
		return nil, fmt.Errorf("update agent: %w", err)
	}

	s.appendLedger(ctx, agent.URI(), "update", agent.OwnerDomain, map[string]string{
		"agent_id":     agent.AgentID,
		"display_name": agent.DisplayName,
		"endpoint":     agent.Endpoint,
	})

	return agent, nil
}

// Activate transitions an agent from pending to active and, if an Issuer is
// configured, issues an X.509 agent identity certificate signed by the Nexus CA.
//
// The returned ActivationResult.KeyPEM contains the private key exactly once —
// it is not persisted anywhere. The caller must deliver it to the agent owner
// and instruct them to store it securely.
func (s *AgentService) Activate(ctx context.Context, id uuid.UUID) (*ActivationResult, error) {
	agent, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Gate activation on DNS-01 domain verification when a verifier is configured.
	if s.dnsVerifier != nil {
		verified, err := s.dnsVerifier.IsDomainVerified(ctx, agent.OwnerDomain)
		if err != nil {
			return nil, fmt.Errorf("check domain verification: %w", err)
		}
		if !verified {
			return nil, fmt.Errorf("domain %q ownership not verified; complete DNS-01 challenge first", agent.OwnerDomain)
		}
	}

	result := &ActivationResult{Agent: agent}

	if s.issuer != nil {
		cert, err := s.issuer.IssueAgentCert(agent.URI(), agent.OwnerDomain, 365*24*time.Hour)
		if err != nil {
			return nil, fmt.Errorf("issue agent cert: %w", err)
		}

		if err := s.repo.ActivateWithCert(ctx, id, cert.Serial, cert.CertPEM); err != nil {
			return nil, fmt.Errorf("activate with cert: %w", err)
		}

		result.CertPEM = cert.CertPEM
		result.KeyPEM = cert.KeyPEM
		result.Serial = cert.Serial
		result.ExpiresAt = cert.Cert.NotAfter
		result.CAPEM = s.issuer.CACertPEM()

		agent.Status = model.AgentStatusActive
		agent.CertSerial = cert.Serial
		agent.PublicKeyPEM = cert.CertPEM
	} else {
		if err := s.repo.UpdateStatus(ctx, id, model.AgentStatusActive); err != nil {
			return nil, err
		}
		agent.Status = model.AgentStatusActive
	}

	s.logger.Info("agent activated",
		zap.String("id", id.String()),
		zap.String("agent_uri", agent.URI()),
		zap.Bool("cert_issued", result.CertPEM != ""),
	)

	s.appendLedger(ctx, agent.URI(), "activate", "nexus-system", map[string]string{
		"agent_id":    agent.AgentID,
		"cert_serial": result.Serial,
	})

	return result, nil
}

// Revoke marks an agent as revoked.
func (s *AgentService) Revoke(ctx context.Context, id uuid.UUID) error {
	agent, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.repo.UpdateStatus(ctx, id, model.AgentStatusRevoked); err != nil {
		return err
	}

	s.appendLedger(ctx, agent.URI(), "revoke", "nexus-system", map[string]string{
		"agent_id": agent.AgentID,
	})

	return nil
}

// ListByOwnerDomain returns active agents for a given owner domain.
func (s *AgentService) ListByOwnerDomain(ctx context.Context, ownerDomain string, limit, offset int) ([]*model.Agent, error) {
	return s.repo.ListByOwnerDomain(ctx, ownerDomain, limit, offset)
}

// Delete permanently removes an agent record.
func (s *AgentService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// generateAgentID produces a unique, sortable Base32 agent identifier.
func generateAgentID() (string, error) {
	// 10 random bytes + 8-byte millisecond timestamp prefix = sortable & unique
	buf := make([]byte, 10)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	ts := time.Now().UnixMilli()
	tsBuf := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		tsBuf[i] = byte(ts & 0xff)
		ts >>= 8
	}
	combined := append(tsBuf, buf...)
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(combined)
	return "agent_" + strings.ToLower(encoded), nil
}

// normalizeCapabilityNode removes leading/trailing slashes and lowercases.
func normalizeCapabilityNode(cap string) string {
	return strings.ToLower(strings.Trim(cap, "/"))
}
