// Package uri provides parsing and validation for the agent:// URI scheme.
//
// URI format: agent://[org-name]/[category]/[agent-id]
//          or agent://[org-name]/[category]/[primary-skill]/[agent-id]
//
// Examples:
//
//	agent://acme.com/finance/agent_7x2v9qaabbccdd                   (domain-verified, no skill)
//	agent://acme.com/finance/reconcile-invoices/agent_7x2v9qaabbccdd (domain-verified, with skill)
//	agent://nap/finance/agent_7x2v9qaabbccdd                         (free-hosted, no skill)
//	agent://nap/legal/contract-review/agent_abc123                   (free-hosted, with skill)
//
// For domain-verified agents, org-name is the full verified domain (e.g. "acme.com").
// For free-hosted agents, org-name is always "nap" (registry-controlled namespace).
// The category is the top-level capability namespace (e.g. "finance", "legal").
// The optional primary-skill is a slugified skill identifier (e.g. "reconcile-invoices").
// The agent-id is the unique identifier assigned at registration.
package uri

import (
	"fmt"
	"net/url"
	"strings"
)

const scheme = "agent"

// URI represents a parsed agent:// URI.
type URI struct {
	OrgName      string // e.g. "acme.com" or "nap" — verified domain (domain-verified) or "nap" (free-hosted) (url.Host)
	Category     string // e.g. "finance"   — top-level capability category (first path segment)
	PrimarySkill string // e.g. "reconcile-invoices" — optional slugified skill (second path segment when 4-part URI)
	AgentID      string // e.g. "agent_7x…" — unique agent identifier
	raw          string
}

// Parse parses an agent:// URI string.
//
// The expected structures are:
//
//	agent://{org-name}/{category}/{agent-id}                  (3-segment, legacy)
//	agent://{org-name}/{category}/{primary-skill}/{agent-id}  (4-segment, with skill)
//
// For URIs with more than 4 segments, only the first segment is used as category
// and the last as agent-id (forward-compatible).
func Parse(raw string) (*URI, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid URI: %w", err)
	}

	if u.Scheme != scheme {
		return nil, fmt.Errorf("unsupported scheme %q: expected %q", u.Scheme, scheme)
	}

	orgName := u.Host
	if orgName == "" {
		return nil, fmt.Errorf("missing org-name in URI %q", raw)
	}

	// Path: /{category}/{agent-id}
	path := strings.TrimPrefix(u.Path, "/")
	if path == "" {
		return nil, fmt.Errorf("missing category and agent-id in URI %q", raw)
	}

	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("URI must contain category and agent-id, got %q", path)
	}

	// category is always the first path segment; agent-id is always the last.
	// For 4-segment URIs the second segment is the primary-skill.
	// URIs with more than 4 segments are accepted for forward-compatibility
	// (primary-skill is omitted in that case).
	agentID := parts[len(parts)-1]
	category := parts[0]
	primarySkill := ""
	if len(parts) == 3 {
		primarySkill = parts[1]
	}

	if err := validateSegment("org-name", orgName); err != nil {
		return nil, err
	}
	if err := validateSegment("category", category); err != nil {
		return nil, err
	}
	if err := validateAgentID(agentID); err != nil {
		return nil, err
	}

	return &URI{
		OrgName:      orgName,
		Category:     category,
		PrimarySkill: primarySkill,
		AgentID:      agentID,
		raw:          raw,
	}, nil
}

// String returns the canonical agent:// URI string.
// When PrimarySkill is set, it is included as the second path segment.
func (u *URI) String() string {
	if u.PrimarySkill != "" {
		return fmt.Sprintf("%s://%s/%s/%s/%s", scheme, u.OrgName, u.Category, u.PrimarySkill, u.AgentID)
	}
	return fmt.Sprintf("%s://%s/%s/%s", scheme, u.OrgName, u.Category, u.AgentID)
}

// MustParse parses a URI and panics on error. Useful in tests and init blocks.
func MustParse(raw string) *URI {
	u, err := Parse(raw)
	if err != nil {
		panic(err)
	}
	return u
}

// validateSegment checks that a URI segment contains no illegal characters.
func validateSegment(name, value string) error {
	if value == "" {
		return fmt.Errorf("%s must not be empty", name)
	}
	if strings.ContainsAny(value, " \\?#") {
		return fmt.Errorf("%s %q contains invalid characters", name, value)
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
