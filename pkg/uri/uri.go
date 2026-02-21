// Package uri provides parsing and validation for the agent:// URI scheme.
//
// URI format: agent://[trust-root]/[capability-node]/[agent-id]
//
// Example:
//
//	agent://nexus.io/finance/taxes/agent_7x2v9qaabbccdd
package uri

import (
	"fmt"
	"net/url"
	"strings"
)

const scheme = "agent"

// URI represents a parsed agent:// URI.
type URI struct {
	TrustRoot      string // e.g. "nexus.io"
	CapabilityNode string // e.g. "finance/taxes"
	AgentID        string // e.g. "agent_7x2v9q..."
	raw            string
}

// Parse parses an agent:// URI string.
func Parse(raw string) (*URI, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid URI: %w", err)
	}

	if u.Scheme != scheme {
		return nil, fmt.Errorf("unsupported scheme %q: expected %q", u.Scheme, scheme)
	}

	trustRoot := u.Host
	if trustRoot == "" {
		return nil, fmt.Errorf("missing trust-root in URI %q", raw)
	}

	// Path: /[capability-node.../][agent-id]
	// url.Parse puts everything after the host in Path with a leading slash.
	path := strings.TrimPrefix(u.Path, "/")
	if path == "" {
		return nil, fmt.Errorf("missing capability-node and agent-id in URI %q", raw)
	}

	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("URI must contain at least one capability-node segment and an agent-id, got %q", path)
	}

	agentID := parts[len(parts)-1]
	capabilityNode := strings.Join(parts[:len(parts)-1], "/")

	if err := validateTrustRoot(trustRoot); err != nil {
		return nil, err
	}
	if err := validateAgentID(agentID); err != nil {
		return nil, err
	}

	return &URI{
		TrustRoot:      trustRoot,
		CapabilityNode: capabilityNode,
		AgentID:        agentID,
		raw:            raw,
	}, nil
}

// String returns the canonical agent:// URI string.
func (u *URI) String() string {
	return fmt.Sprintf("%s://%s/%s/%s", scheme, u.TrustRoot, u.CapabilityNode, u.AgentID)
}

// MustParse parses a URI and panics on error. Useful in tests and init blocks.
func MustParse(raw string) *URI {
	u, err := Parse(raw)
	if err != nil {
		panic(err)
	}
	return u
}

// validateTrustRoot checks that the trust-root is a valid hostname.
func validateTrustRoot(host string) error {
	if host == "" {
		return fmt.Errorf("trust-root must not be empty")
	}
	// Basic hostname validation: no spaces, no path characters.
	if strings.ContainsAny(host, " /\\?#") {
		return fmt.Errorf("trust-root %q contains invalid characters", host)
	}
	return nil
}

// validateAgentID checks that the agent-id matches the expected prefix pattern.
func validateAgentID(id string) error {
	if id == "" {
		return fmt.Errorf("agent-id must not be empty")
	}
	if !strings.HasPrefix(id, "agent_") {
		return fmt.Errorf("agent-id %q must start with \"agent_\"", id)
	}
	return nil
}
