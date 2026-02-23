// Package threat provides threat analysis for agent registration requests.
// It scores registrations against configurable rule sets and can reject
// high-risk registrations before they are written to the database.
package threat

import "context"

// Finding is a single rule match returned by the scorer.
type Finding struct {
	Rule        string  `json:"rule"`
	Description string  `json:"description"`
	Confidence  float64 `json:"confidence"`
}

// Report is the output of a threat analysis run.
type Report struct {
	// Score is the aggregate risk score (0–100).
	Score int `json:"score"`

	// Severity is a human-readable label derived from Score:
	//   0–14   → "none"
	//   15–34  → "low"
	//   35–64  → "medium"
	//   65–84  → "high"
	//   85–100 → "critical"
	Severity string `json:"severity"`

	// Findings lists every rule that triggered.
	Findings []Finding `json:"findings"`

	// Rejected is true when Score ≥ 85. Registrations with Rejected=true
	// should be denied by the caller.
	Rejected bool `json:"rejected"`
}

// Scorer analyses a registration request for threat indicators.
type Scorer interface {
	Score(ctx context.Context, name, description, endpoint string, caps []string) (*Report, error)
}

// severityLabel maps a 0–100 score to a severity string.
func severityLabel(score int) string {
	switch {
	case score >= 85:
		return "critical"
	case score >= 65:
		return "high"
	case score >= 35:
		return "medium"
	case score >= 15:
		return "low"
	default:
		return "none"
	}
}
