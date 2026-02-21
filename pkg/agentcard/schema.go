// Package agentcard defines the agent-card.json schema used in NAP .well-known discovery.
//
// Every NAP-compliant domain must serve an agent-card.json at:
//
//	https://[domain]/.well-known/agent-card.json
package agentcard

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// AgentCard is the JSON structure served at /.well-known/agent-card.json.
type AgentCard struct {
	// SchemaVersion is the agent-card schema version (currently "1.0").
	SchemaVersion string `json:"schema_version"`

	// Domain is the authoritative domain of this agent card.
	Domain string `json:"domain"`

	// Agents lists the agent registrations for this domain.
	Agents []AgentEntry `json:"agents"`

	// Capabilities lists domain-wide capability declarations.
	Capabilities []string `json:"capabilities,omitempty"`

	// TrustRoot is the Nexus registry that issued the agent certificates.
	TrustRoot string `json:"trust_root"`

	// PublicKeyPEM is the domain-level public key for verifying agent signatures.
	PublicKeyPEM string `json:"public_key_pem,omitempty"`

	// UpdatedAt is the ISO-8601 timestamp of the last update.
	UpdatedAt time.Time `json:"updated_at"`

	// ContactEmail is an optional administrative contact.
	ContactEmail string `json:"contact_email,omitempty"`
}

// AgentEntry is a single agent listing within an agent card.
type AgentEntry struct {
	URI            string            `json:"uri"`
	DisplayName    string            `json:"display_name"`
	Description    string            `json:"description,omitempty"`
	Endpoint       string            `json:"endpoint"`
	Protocols      []string          `json:"protocols,omitempty"` // e.g. ["https", "grpc", "ws"]
	CapabilityNode string            `json:"capability_node"`
	Status         string            `json:"status"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// Parse decodes an AgentCard from JSON bytes.
func Parse(data []byte) (*AgentCard, error) {
	var card AgentCard
	if err := json.Unmarshal(data, &card); err != nil {
		return nil, fmt.Errorf("decode agent-card: %w", err)
	}
	if err := card.Validate(); err != nil {
		return nil, err
	}
	return &card, nil
}

// Fetch retrieves and parses the agent-card.json from the given domain.
func Fetch(domain string) (*AgentCard, error) {
	targetURL := "https://" + domain + "/.well-known/agent-card.json"
	if _, err := url.ParseRequestURI(targetURL); err != nil {
		return nil, fmt.Errorf("invalid domain %q: %w", domain, err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(targetURL) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("fetch agent-card: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("agent-card fetch returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB limit
	if err != nil {
		return nil, fmt.Errorf("read agent-card body: %w", err)
	}

	return Parse(body)
}

// Validate checks required fields of an AgentCard.
func (c *AgentCard) Validate() error {
	if c.SchemaVersion == "" {
		return fmt.Errorf("agent-card: schema_version is required")
	}
	if c.Domain == "" {
		return fmt.Errorf("agent-card: domain is required")
	}
	if c.TrustRoot == "" {
		return fmt.Errorf("agent-card: trust_root is required")
	}
	for i, a := range c.Agents {
		if a.URI == "" {
			return fmt.Errorf("agent-card: agents[%d].uri is required", i)
		}
		if a.Endpoint == "" {
			return fmt.Errorf("agent-card: agents[%d].endpoint is required", i)
		}
	}
	return nil
}
