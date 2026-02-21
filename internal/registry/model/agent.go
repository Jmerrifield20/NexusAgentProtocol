package model

import (
	"time"

	"github.com/google/uuid"
)

// AgentStatus represents the lifecycle state of a registered agent.
type AgentStatus string

const (
	AgentStatusPending  AgentStatus = "pending"
	AgentStatusActive   AgentStatus = "active"
	AgentStatusRevoked  AgentStatus = "revoked"
	AgentStatusExpired  AgentStatus = "expired"
)

// Agent is the core domain model representing a registered agent identity.
type Agent struct {
	ID             uuid.UUID   `json:"id" db:"id"`
	TrustRoot      string      `json:"trust_root" db:"trust_root"`
	CapabilityNode string      `json:"capability_node" db:"capability_node"`
	AgentID        string      `json:"agent_id" db:"agent_id"`
	DisplayName    string      `json:"display_name" db:"display_name"`
	Description    string      `json:"description" db:"description"`
	Endpoint       string      `json:"endpoint" db:"endpoint"`
	OwnerDomain    string      `json:"owner_domain" db:"owner_domain"`
	Status         AgentStatus `json:"status" db:"status"`
	CertSerial     string      `json:"cert_serial,omitempty" db:"cert_serial"`
	PublicKeyPEM   string      `json:"public_key_pem,omitempty" db:"public_key_pem"`
	Metadata       AgentMeta   `json:"metadata" db:"metadata"`
	CreatedAt      time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at" db:"updated_at"`
	ExpiresAt      *time.Time  `json:"expires_at,omitempty" db:"expires_at"`
}

// AgentMeta holds extensible key-value metadata for an agent.
type AgentMeta map[string]string

// URI returns the agent:// URI for this agent.
func (a *Agent) URI() string {
	return "agent://" + a.TrustRoot + "/" + a.CapabilityNode + "/" + a.AgentID
}

// Certificate represents an X.509 certificate issued to an agent.
type Certificate struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	AgentID   uuid.UUID  `json:"agent_id" db:"agent_id"`
	Serial    string     `json:"serial" db:"serial"`
	PEM       string     `json:"pem" db:"pem"`
	IssuedAt  time.Time  `json:"issued_at" db:"issued_at"`
	ExpiresAt time.Time  `json:"expires_at" db:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty" db:"revoked_at"`
}

// RegisterRequest is the payload for creating a new agent registration.
type RegisterRequest struct {
	TrustRoot      string    `json:"trust_root" binding:"required"`
	CapabilityNode string    `json:"capability_node" binding:"required"`
	DisplayName    string    `json:"display_name" binding:"required"`
	Description    string    `json:"description"`
	Endpoint       string    `json:"endpoint" binding:"required,url"`
	OwnerDomain    string    `json:"owner_domain" binding:"required"`
	PublicKeyPEM   string    `json:"public_key_pem"`
	Metadata       AgentMeta `json:"metadata"`
}

// UpdateRequest is the payload for updating an existing agent registration.
type UpdateRequest struct {
	DisplayName  string    `json:"display_name"`
	Description  string    `json:"description"`
	Endpoint     string    `json:"endpoint" binding:"omitempty,url"`
	PublicKeyPEM string    `json:"public_key_pem"`
	Metadata     AgentMeta `json:"metadata"`
}
