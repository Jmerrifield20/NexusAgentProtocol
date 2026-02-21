package agentcard_test

import (
	"testing"
	"time"

	"github.com/nexus-protocol/nexus/pkg/agentcard"
)

func TestParse_valid(t *testing.T) {
	data := []byte(`{
		"schema_version": "1.0",
		"domain": "example.com",
		"trust_root": "nexus.io",
		"updated_at": "2026-01-01T00:00:00Z",
		"agents": [
			{
				"uri": "agent://nexus.io/assistant/agent_abc",
				"display_name": "Test Agent",
				"endpoint": "https://example.com/agent",
				"capability_node": "assistant",
				"status": "active"
			}
		]
	}`)

	card, err := agentcard.Parse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if card.Domain != "example.com" {
		t.Errorf("Domain: got %q, want %q", card.Domain, "example.com")
	}
	if len(card.Agents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(card.Agents))
	}
}

func TestParse_missingRequiredFields(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{
			name: "missing schema_version",
			data: []byte(`{"domain":"example.com","trust_root":"nexus.io","agents":[]}`),
		},
		{
			name: "missing domain",
			data: []byte(`{"schema_version":"1.0","trust_root":"nexus.io","agents":[]}`),
		},
		{
			name: "missing trust_root",
			data: []byte(`{"schema_version":"1.0","domain":"example.com","agents":[]}`),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := agentcard.Parse(tc.data)
			if err == nil {
				t.Error("expected validation error but got nil")
			}
		})
	}
}

func TestValidate_agentMissingEndpoint(t *testing.T) {
	card := &agentcard.AgentCard{
		SchemaVersion: "1.0",
		Domain:        "example.com",
		TrustRoot:     "nexus.io",
		UpdatedAt:     time.Now(),
		Agents: []agentcard.AgentEntry{
			{URI: "agent://nexus.io/a/agent_x", Endpoint: ""},
		},
	}
	if err := card.Validate(); err == nil {
		t.Error("expected error for missing endpoint")
	}
}
