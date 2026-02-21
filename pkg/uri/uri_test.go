package uri_test

import (
	"testing"

	"github.com/nexus-protocol/nexus/pkg/uri"
)

func TestParse_valid(t *testing.T) {
	cases := []struct {
		input          string
		trustRoot      string
		capabilityNode string
		agentID        string
	}{
		{
			input:          "agent://nexus.io/finance/taxes/agent_7x2v9qaabbcc",
			trustRoot:      "nexus.io",
			capabilityNode: "finance/taxes",
			agentID:        "agent_7x2v9qaabbcc",
		},
		{
			input:          "agent://nexus.io/assistant/agent_abc123",
			trustRoot:      "nexus.io",
			capabilityNode: "assistant",
			agentID:        "agent_abc123",
		},
		{
			input:          "agent://registry.example.com/a/b/c/agent_xyz",
			trustRoot:      "registry.example.com",
			capabilityNode: "a/b/c",
			agentID:        "agent_xyz",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			u, err := uri.Parse(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if u.TrustRoot != tc.trustRoot {
				t.Errorf("TrustRoot: got %q, want %q", u.TrustRoot, tc.trustRoot)
			}
			if u.CapabilityNode != tc.capabilityNode {
				t.Errorf("CapabilityNode: got %q, want %q", u.CapabilityNode, tc.capabilityNode)
			}
			if u.AgentID != tc.agentID {
				t.Errorf("AgentID: got %q, want %q", u.AgentID, tc.agentID)
			}
		})
	}
}

func TestParse_invalid(t *testing.T) {
	cases := []string{
		"https://nexus.io/finance/taxes/agent_abc",  // wrong scheme
		"agent://nexus.io/agent_abc",                // missing capability node
		"agent:///finance/taxes/agent_abc",          // empty trust root
		"agent://nexus.io/finance/taxes/notanagent", // agent_id without prefix
		"not-a-uri",
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc, func(t *testing.T) {
			_, err := uri.Parse(tc)
			if err == nil {
				t.Errorf("expected error for %q but got nil", tc)
			}
		})
	}
}

func TestURI_String(t *testing.T) {
	raw := "agent://nexus.io/finance/taxes/agent_7x2v9q"
	u, err := uri.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got := u.String(); got != raw {
		t.Errorf("String(): got %q, want %q", got, raw)
	}
}

func TestMustParse_panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected MustParse to panic on invalid URI")
		}
	}()
	uri.MustParse("not-a-uri")
}
