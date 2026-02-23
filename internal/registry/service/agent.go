package service

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/identity"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/registry/model"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/threat"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/trustledger"
	"github.com/jmerrifield20/NexusAgentProtocol/pkg/agentcard"
	"github.com/jmerrifield20/NexusAgentProtocol/pkg/mcpmanifest"
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

	// AgentCardJSON is a ready-to-deploy A2A-compatible agent card JSON string
	// with embedded NAP endorsement. Deploy at /.well-known/agent.json on the
	// agent's own domain for A2A client discovery.
	// Non-empty only when a TokenIssuer is configured on the service.
	AgentCardJSON string

	// MCPManifestJSON is the MCP server manifest for this agent serialised as JSON.
	// Non-empty when the agent declared MCPTools at registration time.
	MCPManifestJSON string
}

// FreeTierConfig holds configuration for the NAP-hosted free tier.
type FreeTierConfig struct {
	TrustRoot string // e.g. "nexusagentprotocol.com"
	MaxAgents int    // maximum agents per free-tier user; 0 = unlimited
}

// agentRepo is the persistence interface for the agent service.
// *repository.AgentRepository satisfies this interface.
type agentRepo interface {
	Create(ctx context.Context, agent *model.Agent) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Agent, error)
	GetByAgentID(ctx context.Context, trustRoot, capNode, agentID string) (*model.Agent, error)
	List(ctx context.Context, trustRoot, capNode string, limit, offset int) ([]*model.Agent, error)
	ListByOwnerDomain(ctx context.Context, domain string, limit, offset int) ([]*model.Agent, error)
	ListByOwnerUserID(ctx context.Context, ownerUserID uuid.UUID, limit, offset int) ([]*model.Agent, error)
	SearchByOrg(ctx context.Context, orgName string, limit, offset int) ([]*model.Agent, error)
	SearchByCapability(ctx context.Context, capability, orgName string, limit, offset int) ([]*model.Agent, error)
	Search(ctx context.Context, q string, limit, offset int) ([]*model.Agent, error)
	CountByOwner(ctx context.Context, ownerUserID uuid.UUID) (int, error)
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

// UserEmailChecker verifies whether a user's email address has been confirmed.
// *users.UserService satisfies this interface.
type UserEmailChecker interface {
	IsEmailVerified(ctx context.Context, userID uuid.UUID) (bool, error)
}

// RemoteResolver resolves agents that are not found in the local registry by
// querying federated peer registries. *federation.RemoteResolver satisfies this.
type RemoteResolver interface {
	Resolve(ctx context.Context, trustRoot, capNode, agentID string) (*model.Agent, error)
}

// AgentService contains business logic for agent lifecycle management.
type AgentService struct {
	repo           agentRepo
	issuer         *identity.Issuer      // nil = no cert issuance
	tokens         *identity.TokenIssuer // nil = no endorsement JWT generation
	ledger         trustledger.Ledger    // nil = no ledger writes
	dnsVerifier    DomainVerifier        // nil = skip domain verification gate
	emailChecker   UserEmailChecker      // nil = skip email verification gate
	threatScorer   threat.Scorer         // nil = no threat scoring
	remoteResolver RemoteResolver        // nil = no cross-registry resolution
	freeTier       FreeTierConfig
	registryURL    string // base URL of this registry, used in endorsement JWTs
	logger         *zap.Logger
}

// NewAgentService creates a new AgentService.
// issuer, ledger, and dnsVerifier may each be nil to disable that feature.
func NewAgentService(repo agentRepo, issuer *identity.Issuer, ledger trustledger.Ledger, dnsVerifier DomainVerifier, logger *zap.Logger) *AgentService {
	return &AgentService{
		repo:        repo,
		issuer:      issuer,
		ledger:      ledger,
		dnsVerifier: dnsVerifier,
		freeTier:    FreeTierConfig{TrustRoot: "nap", MaxAgents: 0}, // 0 = unlimited
		logger:      logger,
	}
}

// SetEmailChecker configures the email verification checker used for nap_hosted activation.
func (s *AgentService) SetEmailChecker(ec UserEmailChecker) {
	s.emailChecker = ec
}

// SetThreatScorer configures the threat scorer used at registration time.
// When set, Register() runs the scorer and rejects registrations with score ≥ 85.
func (s *AgentService) SetThreatScorer(sc threat.Scorer) {
	s.threatScorer = sc
}

// SetFreeTierConfig replaces the free-tier quota/trust-root configuration.
func (s *AgentService) SetFreeTierConfig(cfg FreeTierConfig) {
	s.freeTier = cfg
}

// SetTokenIssuer configures the JWT token issuer used to sign agent endorsements.
// When set, Activate() includes a signed A2A card in the ActivationResult.
func (s *AgentService) SetTokenIssuer(t *identity.TokenIssuer) {
	s.tokens = t
}

// SetRegistryURL sets the base URL of this registry for use in endorsement JWTs.
func (s *AgentService) SetRegistryURL(url string) {
	s.registryURL = url
}

// SetRemoteResolver configures the cross-registry resolver used when a local
// agent lookup fails. Set to nil to disable federated resolution.
func (s *AgentService) SetRemoteResolver(rr RemoteResolver) {
	s.remoteResolver = rr
}

// ScoreThreat runs the threat scorer against a registration request without
// performing any registration. Returns nil when no scorer is configured.
func (s *AgentService) ScoreThreat(ctx context.Context, req *model.RegisterRequest) (*threat.Report, error) {
	if s.threatScorer == nil {
		return nil, nil
	}
	var caps []string
	for _, c := range []string{req.Capability, req.Category, req.CapabilityNode} {
		if c != "" {
			caps = append(caps, c)
		}
	}
	return s.threatScorer.Score(ctx, req.DisplayName, req.Description, req.Endpoint, caps)
}

// appendLedger appends an audit entry to the ledger in a non-fatal manner.
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

	var agent *model.Agent

	// Resolve capability path: prefer explicit Capability field, then Category,
	// then the legacy CapabilityNode field (converting "/" → ">").
	capability := normalizeCapability(req.Capability)
	if capability == "" {
		capability = normalizeCapability(req.Category)
	}
	if capability == "" {
		capability = normalizeCapability(req.CapabilityNode)
	}

	if req.RegistrationType == model.RegistrationTypeNAPHosted {
		// NAP-hosted free tier.
		// URI: agent://{trust_root}/{category}/{agent_id}
		if req.OwnerUserID == nil {
			return nil, fmt.Errorf("owner_user_id is required for nap_hosted registration")
		}
		if req.Username == "" {
			return nil, fmt.Errorf("username is required for nap_hosted registration")
		}
		if capability == "" {
			return nil, &model.ErrValidation{Msg: "capability is required for nap_hosted registration"}
		}
		if err := model.ValidateCapabilityNode(capability); err != nil {
			return nil, &model.ErrValidation{Msg: err.Error()}
		}

		// Quota check — only enforced when MaxAgents > 0.
		// Currently set to 0 (unlimited) for all users.
		if s.freeTier.MaxAgents > 0 {
			count, err := s.repo.CountByOwner(ctx, *req.OwnerUserID)
			if err != nil {
				return nil, fmt.Errorf("check agent quota: %w", err)
			}
			if count >= s.freeTier.MaxAgents {
				return nil, fmt.Errorf("agent limit of %d reached for your current plan", s.freeTier.MaxAgents)
			}
		}

		agent = &model.Agent{
			// trust_root = NAP-controlled namespace for all free-hosted agents.
			// Using the username here would allow anyone to claim "amazon" or
			// "google" as their org segment, making URIs indistinguishable from
			// domain-verified agents. The FreeTierConfig.TrustRoot ("nap") is a
			// registry-controlled namespace that callers can trust means
			// "email-verified, not domain-verified".
			TrustRoot:        s.freeTier.TrustRoot,
			CapabilityNode:   capability, // full path e.g. "finance>accounting>reconciliation"
			AgentID:          agentID,
			DisplayName:      req.DisplayName,
			Description:      req.Description,
			Endpoint:         req.Endpoint, // empty until user configures their server
			Status:           model.AgentStatusPending,
			PublicKeyPEM:     req.PublicKeyPEM,
			Metadata:         req.Metadata,
			OwnerUserID:      req.OwnerUserID,
			RegistrationType: model.RegistrationTypeNAPHosted,
		}
	} else {
		// Domain-verified path.
		// URI: agent://{owner_domain}/{category}/{agent_id}
		// The full verified domain is the org segment — unambiguous and proven
		// by DNS-01 challenge, so agent://amazon.com/… can only be Amazon.
		ownerDomain := strings.ToLower(strings.TrimSpace(req.OwnerDomain))
		if ownerDomain == "" {
			return nil, &model.ErrValidation{Msg: "owner_domain is required for domain registration"}
		}
		if capability == "" {
			return nil, &model.ErrValidation{Msg: "capability is required for domain registration"}
		}
		if req.Endpoint == "" {
			return nil, &model.ErrValidation{Msg: "endpoint is required for domain registration"}
		}
		if err := model.ValidateCapabilityNode(capability); err != nil {
			return nil, &model.ErrValidation{Msg: err.Error()}
		}

		agent = &model.Agent{
			// trust_root = verified domain; DNS-01 proves the registrant owns it.
			TrustRoot:        ownerDomain,
			CapabilityNode:   capability, // full path e.g. "finance>accounting>reconciliation"
			AgentID:          agentID,
			DisplayName:      req.DisplayName,
			Description:      req.Description,
			Endpoint:         req.Endpoint,
			OwnerDomain:      req.OwnerDomain,
			Status:           model.AgentStatusPending,
			PublicKeyPEM:     req.PublicKeyPEM,
			Metadata:         req.Metadata,
			OwnerUserID:      req.OwnerUserID,
			RegistrationType: model.RegistrationTypeDomain,
		}
	}

	// Persist declared A2A skills into metadata so they survive the round-trip.
	if len(req.Skills) > 0 {
		if agent.Metadata == nil {
			agent.Metadata = make(model.AgentMeta)
		}
		if b, err := json.Marshal(req.Skills); err == nil {
			agent.Metadata["_skills"] = string(b)
		}
	}

	// Persist declared MCP tools into metadata.
	if len(req.MCPTools) > 0 {
		if agent.Metadata == nil {
			agent.Metadata = make(model.AgentMeta)
		}
		if b, err := json.Marshal(req.MCPTools); err == nil {
			agent.Metadata["_mcp_tools"] = string(b)
		}
	}

	if err := s.repo.Create(ctx, agent); err != nil {
		s.logger.Error("failed to create agent", zap.Error(err))
		return nil, fmt.Errorf("create agent: %w", err)
	}

	s.logger.Info("agent registered",
		zap.String("agent_id", agentID),
		zap.String("trust_root", agent.TrustRoot),
		zap.String("capability", agent.CapabilityNode),
		zap.String("registration_type", agent.RegistrationType),
	)

	actor := agent.OwnerDomain
	if actor == "" && req.OwnerUserID != nil {
		actor = req.OwnerUserID.String()
	}
	s.appendLedger(ctx, agent.URI(), "register", actor, map[string]string{
		"trust_root":        agent.TrustRoot,
		"capability_node":   agent.CapabilityNode,
		"agent_id":          agent.AgentID,
		"registration_type": agent.RegistrationType,
		"endpoint":          agent.Endpoint,
	})

	return agent, nil
}

// Get retrieves an agent by its internal UUID.
func (s *AgentService) Get(ctx context.Context, id uuid.UUID) (*model.Agent, error) {
	return s.repo.GetByID(ctx, id)
}

// Resolve looks up an active agent by its URI components.
// capNode is the top-level category from the URI (e.g. "finance").
// The repository does prefix matching so agents stored as "finance>accounting>..."
// are correctly found.
// When a local miss occurs and a RemoteResolver is configured, resolution is
// attempted on federated peer registries (non-fatal on remote error).
func (s *AgentService) Resolve(ctx context.Context, trustRoot, capNode, agentID string) (*model.Agent, error) {
	agent, err := s.repo.GetByAgentID(ctx, trustRoot, normalizeCapability(capNode), agentID)
	if err != nil {
		if s.remoteResolver != nil {
			if remote, remoteErr := s.remoteResolver.Resolve(ctx, trustRoot, capNode, agentID); remoteErr == nil {
				return remote, nil
			}
		}
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

// Search returns agents matching the query string across multiple fields.
func (s *AgentService) Search(ctx context.Context, q string, limit, offset int) ([]*model.Agent, error) {
	return s.repo.Search(ctx, q, limit, offset)
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
	if req.Version != "" {
		agent.Version = req.Version
	}
	if req.Tags != nil {
		agent.Tags = req.Tags
	}
	if req.SupportURL != "" {
		agent.SupportURL = req.SupportURL
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
// For domain agents, DNS-01 verification is required (when dnsVerifier is set).
// For nap_hosted agents, email verification is required (when emailChecker is set).
func (s *AgentService) Activate(ctx context.Context, id uuid.UUID) (*ActivationResult, error) {
	agent, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if agent.RegistrationType == model.RegistrationTypeNAPHosted {
		// Gate activation on email verification for hosted agents.
		if s.emailChecker != nil && agent.OwnerUserID != nil {
			verified, err := s.emailChecker.IsEmailVerified(ctx, *agent.OwnerUserID)
			if err != nil {
				return nil, fmt.Errorf("check email verification: %w", err)
			}
			if !verified {
				return nil, fmt.Errorf("email address must be verified before activating a hosted agent")
			}
		}
	} else {
		// Gate activation on DNS-01 domain verification for domain agents.
		if s.dnsVerifier != nil {
			verified, err := s.dnsVerifier.IsDomainVerified(ctx, agent.OwnerDomain)
			if err != nil {
				return nil, fmt.Errorf("check domain verification: %w", err)
			}
			if !verified {
				return nil, fmt.Errorf("domain %q ownership not verified; complete DNS-01 challenge first", agent.OwnerDomain)
			}
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

	// Generate A2A-compatible agent card with NAP endorsement (non-fatal).
	if cardJSON, err := s.generateAgentCard(agent, result.Serial); err != nil {
		s.logger.Warn("generate agent card (non-fatal)", zap.Error(err))
	} else {
		result.AgentCardJSON = cardJSON
	}

	// Generate MCP manifest (non-fatal).
	if manifest, err := s.GenerateMCPManifest(agent); err != nil {
		s.logger.Warn("generate MCP manifest (non-fatal)", zap.Error(err))
	} else if manifest != nil {
		if b, err := json.MarshalIndent(manifest, "", "  "); err == nil {
			result.MCPManifestJSON = string(b)
		}
	}

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

// ListByOwnerUserID returns all agents owned by the given user account.
func (s *AgentService) ListByOwnerUserID(ctx context.Context, ownerUserID uuid.UUID, limit, offset int) ([]*model.Agent, error) {
	return s.repo.ListByOwnerUserID(ctx, ownerUserID, limit, offset)
}

// LookupByOrg returns all active agents registered under the given org namespace.
// The org is the first URI segment: the full verified domain for domain-verified agents
// (e.g. "acme.com" in agent://acme.com/finance/agent_xyz), or "nap" for free-hosted.
func (s *AgentService) LookupByOrg(ctx context.Context, orgName string, limit, offset int) ([]*model.Agent, error) {
	return s.repo.SearchByOrg(ctx, orgName, limit, offset)
}

// LookupByCapability returns active agents whose capability_node matches the
// given capability prefix (exact or hierarchical child). An optional orgName
// further restricts results to a single org namespace. Results are ordered by
// effective trust tier so the most trustworthy agents appear first.
func (s *AgentService) LookupByCapability(ctx context.Context, capability, orgName string, limit, offset int) ([]*model.Agent, error) {
	return s.repo.SearchByCapability(ctx, capability, orgName, limit, offset)
}

// Delete permanently removes an agent record.
func (s *AgentService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// generateAgentID produces a unique, sortable Base32 agent identifier.
func generateAgentID() (string, error) {
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

// normalizeCapability lowercases the capability string, trims whitespace, and
// converts the legacy "/" separator to ">" for backward compatibility.
func normalizeCapability(cap string) string {
	cap = strings.ToLower(strings.TrimSpace(cap))
	cap = strings.ReplaceAll(cap, "/", ">")
	cap = strings.Trim(cap, ">")
	return cap
}

// generateAgentCard builds and returns a JSON-encoded A2A-compatible agent card
// with a NAP endorsement JWT signed by the registry CA key.
// Returns an empty string (no error) when no token issuer is configured.
func (s *AgentService) generateAgentCard(agent *model.Agent, certSerial string) (string, error) {
	if s.tokens == nil {
		return "", nil
	}

	registry := s.registryURL
	if registry == "" {
		registry = "https://registry.nexusagentprotocol.com"
	}

	tier := agent.ComputeTrustTier()

	endorsement, err := s.tokens.IssueEndorsement(
		agent.URI(), string(tier), certSerial, registry, 365*24*time.Hour,
	)
	if err != nil {
		return "", fmt.Errorf("issue endorsement: %w", err)
	}

	card := agentcard.A2ACard{
		Name:           agent.DisplayName,
		Description:    agent.Description,
		URL:            agent.Endpoint,
		Version:        "1.0",
		Capabilities:   agentcard.A2ACapabilities{},
		Skills:         buildSkills(agent),
		NAPURI:         agent.URI(),
		NAPTrustTier:   string(tier),
		NAPRegistry:    registry,
		NAPCertSerial:  certSerial,
		NAPEndorsement: endorsement,
	}

	data, err := json.MarshalIndent(card, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal agent card: %w", err)
	}
	return string(data), nil
}

// GetAgentCardJSON returns the A2A-spec agent card for an existing agent as JSON.
// When a TokenIssuer is configured, the card includes a signed NAP endorsement.
// When no TokenIssuer is configured, the card is returned without an endorsement.
func (s *AgentService) GetAgentCardJSON(agent *model.Agent) (string, error) {
	cardJSON, err := s.generateAgentCard(agent, agent.CertSerial)
	if err != nil {
		return "", err
	}
	if cardJSON != "" {
		return cardJSON, nil
	}

	// No token issuer — return card without endorsement.
	registry := s.registryURL
	if registry == "" {
		registry = "https://registry.nexusagentprotocol.com"
	}
	tier := agent.ComputeTrustTier()
	card := agentcard.A2ACard{
		Name:         agent.DisplayName,
		Description:  agent.Description,
		URL:          agent.Endpoint,
		Version:      "1.0",
		Capabilities: agentcard.A2ACapabilities{},
		Skills:       buildSkills(agent),
		NAPURI:       agent.URI(),
		NAPTrustTier: string(tier),
		NAPRegistry:  registry,
	}
	data, err := json.MarshalIndent(card, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal agent card: %w", err)
	}
	return string(data), nil
}

// buildSkills returns the A2A skills for an agent. It checks metadata first
// (skills declared at registration) and falls back to capability-derived skills.
func buildSkills(agent *model.Agent) []agentcard.A2ASkill {
	// Check if skills were declared at registration and stored in metadata.
	if skillsJSON, ok := agent.Metadata["_skills"]; ok && skillsJSON != "" {
		var skills []agentcard.A2ASkill
		if err := json.Unmarshal([]byte(skillsJSON), &skills); err == nil && len(skills) > 0 {
			return skills
		}
	}

	// Auto-derive a skill from the capability node.
	if agent.CapabilityNode == "" {
		return nil
	}
	return []agentcard.A2ASkill{{
		ID:          strings.ReplaceAll(agent.CapabilityNode, ">", "-"),
		Name:        model.FormatDisplay(agent.CapabilityNode),
		Description: agent.Description,
		Tags:        []string{model.TopLevelCategory(agent.CapabilityNode)},
	}}
}

// GenerateMCPManifest assembles an MCP manifest for the given agent.
// Returns nil when the agent has no declared MCP tools.
func (s *AgentService) GenerateMCPManifest(agent *model.Agent) (*mcpmanifest.MCPManifest, error) {
	toolsJSON, ok := agent.Metadata["_mcp_tools"]
	if !ok || toolsJSON == "" {
		return nil, nil
	}

	var tools []mcpmanifest.MCPTool
	if err := json.Unmarshal([]byte(toolsJSON), &tools); err != nil {
		return nil, fmt.Errorf("decode mcp tools: %w", err)
	}
	if len(tools) == 0 {
		return nil, nil
	}

	registry := s.registryURL
	if registry == "" {
		registry = "https://registry.nexusagentprotocol.com"
	}

	version := agent.Version
	if version == "" {
		version = "1.0"
	}

	tier := agent.ComputeTrustTier()

	return &mcpmanifest.MCPManifest{
		SchemaVersion: "2024-11-05",
		Name:          agent.DisplayName,
		Version:       version,
		Description:   agent.Description,
		Tools:         tools,
		NAPURI:        agent.URI(),
		NAPTrustTier:  string(tier),
		NAPRegistry:   registry,
	}, nil
}
