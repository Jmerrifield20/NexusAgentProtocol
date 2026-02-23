package threat

import (
	"context"
	"strings"
)

// ruleFunc is a function that inspects registration inputs and returns zero or
// more Findings if its rule matches.
type ruleFunc func(name, description, endpoint string, caps []string) []Finding

// RuleBasedScorer is the default Scorer implementation. It runs a fixed set of
// pattern-matching rules against the registration inputs and accumulates a score.
type RuleBasedScorer struct {
	rules []ruleFunc
}

// NewRuleBasedScorer returns a RuleBasedScorer loaded with the default rule set.
func NewRuleBasedScorer() *RuleBasedScorer {
	s := &RuleBasedScorer{}
	s.rules = []ruleFunc{
		ruleCapabilityKeywords,
		ruleDescriptionPhrases,
		ruleEndpointScheme,
		ruleNameKeywords,
	}
	return s
}

// Score implements Scorer.
func (s *RuleBasedScorer) Score(_ context.Context, name, description, endpoint string, caps []string) (*Report, error) {
	var findings []Finding
	for _, r := range s.rules {
		findings = append(findings, r(name, description, endpoint, caps)...)
	}

	total := 0
	for _, f := range findings {
		total += int(f.Confidence * 25)
	}
	if total > 100 {
		total = 100
	}

	if findings == nil {
		findings = []Finding{}
	}

	return &Report{
		Score:    total,
		Severity: severityLabel(total),
		Findings: findings,
		Rejected: total >= 85,
	}, nil
}

// ── Rules ─────────────────────────────────────────────────────────────────────

// suspiciousCapabilityKeywords are terms in capability names that suggest
// the agent may be claiming elevated system access.
var suspiciousCapabilityKeywords = []string{
	"shell", "exec", "sudo", "admin", "root", "system", "kernel", "daemon",
}

func ruleCapabilityKeywords(_, _, _ string, caps []string) []Finding {
	var findings []Finding
	for _, cap := range caps {
		lower := strings.ToLower(cap)
		for _, kw := range suspiciousCapabilityKeywords {
			if strings.Contains(lower, kw) {
				findings = append(findings, Finding{
					Rule:        "capability_keyword",
					Description: "Capability name contains suspicious keyword: " + kw,
					Confidence:  0.7,
				})
				break
			}
		}
	}
	return findings
}

// suspiciousDescriptionPhrases are substrings in descriptions that suggest
// the agent may be designed for harmful operations.
var suspiciousDescriptionPhrases = []string{
	"exfiltrat", "bypass", "escalat", "inject", "exploit", "malware",
	"arbitrary shell", "arbitrary command", "remote code", "privilege",
	"backdoor", "rootkit", "keylog",
}

func ruleDescriptionPhrases(_, description, _ string, _ []string) []Finding {
	var findings []Finding
	lower := strings.ToLower(description)
	for _, phrase := range suspiciousDescriptionPhrases {
		if strings.Contains(lower, phrase) {
			findings = append(findings, Finding{
				Rule:        "description_phrase",
				Description: "Description contains suspicious phrase: " + phrase,
				Confidence:  0.8,
			})
		}
	}
	return findings
}

// ruleEndpointScheme flags non-HTTPS endpoints (HTTP or bare IP patterns).
func ruleEndpointScheme(_, _, endpoint string, _ []string) []Finding {
	if endpoint == "" {
		return nil
	}
	lower := strings.ToLower(endpoint)
	if strings.HasPrefix(lower, "http://") && !strings.Contains(lower, "localhost") && !strings.Contains(lower, "127.0.0.1") {
		return []Finding{{
			Rule:        "endpoint_scheme",
			Description: "Endpoint uses non-HTTPS scheme in production context",
			Confidence:  0.4,
		}}
	}
	return nil
}

// suspiciousNameKeywords are terms in the agent display name that suggest
// the agent may be impersonating system components or privileged services.
var suspiciousNameKeywords = []string{
	"shell executor", "command executor", "root agent", "system agent",
	"admin agent", "sudo agent", "kernel agent",
}

func ruleNameKeywords(name, _, _ string, _ []string) []Finding {
	var findings []Finding
	lower := strings.ToLower(name)
	for _, kw := range suspiciousNameKeywords {
		if strings.Contains(lower, kw) {
			findings = append(findings, Finding{
				Rule:        "name_keyword",
				Description: "Display name contains suspicious keyword: " + kw,
				Confidence:  0.6,
			})
		}
	}
	return findings
}
