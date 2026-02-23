package uri_test

import (
	"testing"

	"github.com/jmerrifield20/NexusAgentProtocol/pkg/uri"
)

func TestParse_valid(t *testing.T) {
	cases := []struct {
		input    string
		category string
		orgName  string
		agentID  string
	}{
		{
			input:    "agent://acme/finance/agent_7x2v9qaabbcc",
			category: "finance",
			orgName:  "acme",
			agentID:  "agent_7x2v9qaabbcc",
		},
		{
			input:    "agent://johndoe/research/agent_abc123",
			category: "research",
			orgName:  "johndoe",
			agentID:  "agent_abc123",
		},
		{
			input:    "agent://staples/officesupplies/agent_xyz",
			category: "officesupplies",
			orgName:  "staples",
			agentID:  "agent_xyz",
		},
		// domain-verified: full domain as org-name
		{
			input:    "agent://amazon.com/commerce/agent_3k8mwpqq",
			category: "commerce",
			orgName:  "amazon.com",
			agentID:  "agent_3k8mwpqq",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			u, err := uri.Parse(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if u.Category != tc.category {
				t.Errorf("Category: got %q, want %q", u.Category, tc.category)
			}
			if u.OrgName != tc.orgName {
				t.Errorf("OrgName: got %q, want %q", u.OrgName, tc.orgName)
			}
			if u.AgentID != tc.agentID {
				t.Errorf("AgentID: got %q, want %q", u.AgentID, tc.agentID)
			}
		})
	}
}

func TestParse_invalid(t *testing.T) {
	cases := []string{
		"https://acme/finance/agent_abc",     // wrong scheme
		"agent://acme/agent_abc",             // missing category (only 1 path segment)
		"agent:///finance/acme/agent_abc",    // empty org-name
		"agent://acme/finance/notanagent",    // agent-id without prefix
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
	raw := "agent://acme/finance/agent_7x2v9q"
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
