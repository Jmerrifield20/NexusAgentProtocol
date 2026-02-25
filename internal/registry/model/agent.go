package model

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmerrifield20/NexusAgentProtocol/pkg/agentcard"
	"github.com/jmerrifield20/NexusAgentProtocol/pkg/mcpmanifest"
)

// AgentStatus represents the lifecycle state of a registered agent.
type AgentStatus string

const (
	AgentStatusPending    AgentStatus = "pending"
	AgentStatusActive     AgentStatus = "active"
	AgentStatusRevoked    AgentStatus = "revoked"
	AgentStatusExpired    AgentStatus = "expired"
	AgentStatusSuspended  AgentStatus = "suspended"
	AgentStatusDeprecated AgentStatus = "deprecated"
)

// RegistrationType indicates how the agent is hosted.
const (
	RegistrationTypeDomain    = "domain"
	RegistrationTypeNAPHosted = "nap_hosted"
)

// TrustTier is a computed credibility label derived from registration type,
// activation status, and whether an mTLS certificate has been issued.
type TrustTier string

const (
	// TierUnverified — agent is pending, revoked, or expired.
	TierUnverified TrustTier = "unverified"
	// TierBasic — nap_hosted agent that has been activated.
	TierBasic TrustTier = "basic"
	// TierVerified — domain-verified (DNS-01) and active, but no mTLS cert.
	TierVerified TrustTier = "verified"
	// TierTrusted — domain-verified, active, and an mTLS cert has been issued by the registry CA.
	TierTrusted TrustTier = "trusted"
)

// Agent is the core domain model representing a registered agent identity.
type Agent struct {
	ID               uuid.UUID  `json:"id"                       db:"id"`
	TrustRoot        string     `json:"trust_root"               db:"trust_root"`
	CapabilityNode   string     `json:"capability_node"          db:"capability_node"`
	AgentID          string     `json:"agent_id"                 db:"agent_id"`
	DisplayName      string     `json:"display_name"             db:"display_name"`
	Description      string     `json:"description"              db:"description"`
	Endpoint         string     `json:"endpoint"                 db:"endpoint"`
	OwnerDomain      string     `json:"owner_domain"             db:"owner_domain"`
	Status           AgentStatus `json:"status"                  db:"status"`
	CertSerial       string     `json:"cert_serial,omitempty"    db:"cert_serial"`
	PublicKeyPEM     string     `json:"public_key_pem,omitempty" db:"public_key_pem"`
	Metadata         AgentMeta  `json:"metadata"                 db:"metadata"`
	CreatedAt        time.Time  `json:"created_at"               db:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"               db:"updated_at"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"     db:"expires_at"`
	OwnerUserID      *uuid.UUID `json:"owner_user_id,omitempty"  db:"owner_user_id"`
	RegistrationType string     `json:"registration_type"        db:"registration_type"`
	// Extended metadata — user-editable via UpdateRequest.
	Version     string   `json:"version"      db:"version"`
	Tags        []string `json:"tags"         db:"tags"`
	SupportURL  string   `json:"support_url"  db:"support_url"`
	// System-managed fields updated by health checks.
	LastSeenAt   *time.Time `json:"last_seen_at,omitempty" db:"last_seen_at"`
	HealthStatus string     `json:"health_status"          db:"health_status"`
	// Revocation / suspension fields.
	RevocationReason string     `json:"revocation_reason,omitempty" db:"revocation_reason"`
	SuspendedAt      *time.Time `json:"suspended_at,omitempty"      db:"suspended_at"`
	// Deprecation fields.
	DeprecatedAt   *time.Time `json:"deprecated_at,omitempty"   db:"deprecated_at"`
	SunsetDate     *time.Time `json:"sunset_date,omitempty"     db:"sunset_date"`
	ReplacementURI string     `json:"replacement_uri,omitempty" db:"replacement_uri"`
	// Skill / tool search columns — denormalised from metadata for indexed lookup.
	PrimarySkill string   `json:"primary_skill" db:"primary_skill"`
	SkillIDs     []string `json:"skill_ids"     db:"skill_ids"`
	ToolNames    []string `json:"tool_names"    db:"tool_names"`
	// TrustTier is computed at read time from status, registration_type, and cert_serial.
	// It is never stored in the database.
	TrustTier TrustTier `json:"trust_tier" db:"-"`
}

// AgentMeta holds extensible key-value metadata for an agent.
type AgentMeta map[string]string

// URI returns the agent:// URI for this agent.
//
// Format: agent://{org}/{category}/{agent_id}
//      or agent://{org}/{category}/{primary_skill}/{agent_id}   (when primary_skill is set)
//
//   - org           is trust_root — the full verified domain for domain-verified agents
//                   (e.g. "acme.com"), or "nap" for free-hosted agents.
//   - category      is the top-level segment of capability_node  (e.g. "finance")
//   - primary_skill is the slugified primary skill identifier, set once at registration
//   - agent_id      is the unique opaque identifier
func (a *Agent) URI() string {
	category := a.CapabilityNode
	if idx := strings.Index(category, ">"); idx != -1 {
		category = category[:idx]
	}
	if a.PrimarySkill != "" {
		return "agent://" + a.TrustRoot + "/" + category + "/" + a.PrimarySkill + "/" + a.AgentID
	}
	return "agent://" + a.TrustRoot + "/" + category + "/" + a.AgentID
}

// ComputeTrustTier derives the credibility tier from the agent's current state.
// The result depends only on fields already present in the agents table row.
func (a *Agent) ComputeTrustTier() TrustTier {
	if a.Status != AgentStatusActive && a.Status != AgentStatusDeprecated {
		return TierUnverified
	}
	if a.RegistrationType == RegistrationTypeDomain {
		if a.CertSerial != "" {
			return TierTrusted
		}
		return TierVerified
	}
	// nap_hosted, active
	return TierBasic
}

// Certificate represents an X.509 certificate issued to an agent.
type Certificate struct {
	ID        uuid.UUID  `json:"id"                   db:"id"`
	AgentID   uuid.UUID  `json:"agent_id"             db:"agent_id"`
	Serial    string     `json:"serial"               db:"serial"`
	PEM       string     `json:"pem"                  db:"pem"`
	IssuedAt  time.Time  `json:"issued_at"            db:"issued_at"`
	ExpiresAt time.Time  `json:"expires_at"           db:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty" db:"revoked_at"`
}

// RegisterRequest is the payload for creating a new agent registration.
type RegisterRequest struct {
	// Capability is the full three-level capability path using ">" as separator.
	// Examples: "finance", "finance>accounting", "finance>accounting>reconciliation"
	// All three levels are validated against the fixed Taxonomy.
	// Required for all registration types. The top-level segment becomes the
	// category in the URI: agent://{org}/{category}/{agent_id}
	Capability string `json:"capability"`

	// OrgName is deprecated for domain-verified agents — owner_domain is used
	// directly as the trust_root and URI org segment (e.g. "acme.com").
	// Retained for backward compatibility; ignored when OwnerDomain is set.
	OrgName string `json:"org_name"`

	// Category is accepted for backward-compatibility with single-level capability.
	// Ignored when Capability is set.
	Category string `json:"category"`

	// CapabilityNode is accepted for backward-compatibility; ignored when Capability
	// or Category is set.
	CapabilityNode string `json:"capability_node"`

	// TrustRoot is accepted for backward-compatibility; ignored when OrgName is set.
	TrustRoot string `json:"trust_root"`

	DisplayName      string     `json:"display_name"     binding:"required"`
	Description      string     `json:"description"`
	Endpoint         string     `json:"endpoint"`
	OwnerDomain      string     `json:"owner_domain"`
	PublicKeyPEM     string     `json:"public_key_pem"`
	Metadata         AgentMeta  `json:"metadata"`
	OwnerUserID      *uuid.UUID `json:"owner_user_id,omitempty"`
	RegistrationType string     `json:"registration_type"`
	// Username is set by the handler from the user JWT; not from the client body.
	Username string `json:"-"`

	// Skills is an optional set of A2A skill descriptors. When provided, they
	// are stored in agent metadata and used to populate the A2A agent card.
	// If omitted, skills are auto-derived from the capability taxonomy.
	Skills []agentcard.A2ASkill `json:"skills,omitempty"`

	// MCPTools declares the MCP tool definitions this agent exposes.
	// Stored in agent metadata and served at /api/v1/agents/:id/mcp-manifest.json.
	MCPTools []mcpmanifest.MCPTool `json:"mcp_tools,omitempty"`
}

// DeprecateRequest is the payload for deprecating an agent.
type DeprecateRequest struct {
	SunsetDate     string `json:"sunset_date"`     // RFC 3339 date, e.g. "2026-06-01"
	ReplacementURI string `json:"replacement_uri"`
}

// UpdateRequest is the payload for updating an existing agent registration.
type UpdateRequest struct {
	DisplayName  string    `json:"display_name"`
	Description  string    `json:"description"`
	Endpoint     string    `json:"endpoint"      binding:"omitempty,url"`
	PublicKeyPEM string    `json:"public_key_pem"`
	Metadata     AgentMeta `json:"metadata"`
	// Extended metadata fields.
	Version     string   `json:"version"`
	Tags        []string `json:"tags"`
	SupportURL  string   `json:"support_url"  binding:"omitempty,url"`
}
