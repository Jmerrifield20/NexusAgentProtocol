package model

import (
	"time"

	"github.com/google/uuid"
)

// AgentStatus represents the lifecycle state of a registered agent.
type AgentStatus string

const (
	AgentStatusPending AgentStatus = "pending"
	AgentStatusActive  AgentStatus = "active"
	AgentStatusRevoked AgentStatus = "revoked"
	AgentStatusExpired AgentStatus = "expired"
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
	// TrustTier is computed at read time from status, registration_type, and cert_serial.
	// It is never stored in the database.
	TrustTier        TrustTier  `json:"trust_tier"               db:"-"`
}

// AgentMeta holds extensible key-value metadata for an agent.
type AgentMeta map[string]string

// URI returns the agent:// URI for this agent.
func (a *Agent) URI() string {
	return "agent://" + a.TrustRoot + "/" + a.CapabilityNode + "/" + a.AgentID
}

// ComputeTrustTier derives the credibility tier from the agent's current state.
// The result depends only on fields already present in the agents table row.
func (a *Agent) ComputeTrustTier() TrustTier {
	if a.Status != AgentStatusActive {
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
	TrustRoot        string     `json:"trust_root"`
	CapabilityNode   string     `json:"capability_node"`
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
}

// UpdateRequest is the payload for updating an existing agent registration.
type UpdateRequest struct {
	DisplayName  string    `json:"display_name"`
	Description  string    `json:"description"`
	Endpoint     string    `json:"endpoint" binding:"omitempty,url"`
	PublicKeyPEM string    `json:"public_key_pem"`
	Metadata     AgentMeta `json:"metadata"`
}
