package federation

import "time"

// Role defines the operational mode of a registry node.
type Role string

const (
	RoleStandalone Role = "standalone"
	RoleFederated  Role = "federated"
	RoleRoot       Role = "root"
)

// RegistryStatus represents the lifecycle state of a federated registry entry.
type RegistryStatus string

const (
	StatusPending   RegistryStatus = "pending"
	StatusActive    RegistryStatus = "active"
	StatusSuspended RegistryStatus = "suspended"
)

// RegisteredRegistry is a remote registry that has applied to join the NAP federation.
type RegisteredRegistry struct {
	ID             string
	TrustRoot      string
	EndpointURL    string
	IntermediateCA string
	Status         RegistryStatus
	RegisteredAt   time.Time
	UpdatedAt      time.Time
}

// RegisterRequest is the payload for a new federation member application.
type RegisterRequest struct {
	TrustRoot    string `json:"trust_root"`
	EndpointURL  string `json:"endpoint_url"`
	ContactEmail string `json:"contact_email"`
}

// IssueCARequest requests an intermediate CA certificate for an approved registry.
type IssueCARequest struct {
	TrustRoot string `json:"trust_root"`
}

// IssueCAResponse contains the intermediate CA credentials returned to an operator.
// The KeyPEM field is only ever delivered once and is never stored by the root registry.
type IssueCAResponse struct {
	TrustRoot string `json:"trust_root"`
	CertPEM   string `json:"cert_pem"`
	KeyPEM    string `json:"key_pem"`
	Serial    string `json:"serial"`
	ExpiresAt string `json:"expires_at"`
	RootCAPEM string `json:"root_ca_pem"`
	Warning   string `json:"warning"`
}
