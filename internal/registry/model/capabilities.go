package model

import (
	"fmt"
	"sort"
	"strings"
)

// Taxonomy is the suggested three-level capability hierarchy.
// It is served via GET /api/v1/capabilities as browsable suggestions;
// it is NOT enforced during registration. Any well-formed capability
// string is accepted (see ValidateCapabilityNode).
//
// Stored as a single ">"-delimited string in the capability_node column:
//
//	"finance"
//	"finance>accounting"
//	"finance>accounting>reconciliation"
//	"my-custom-domain>my-sub"
//
// The agent:// URI exposes only the top-level category:
//
//	agent://acme/finance/agent_xyz
var Taxonomy = map[string]map[string][]string{
	"commerce": {
		"catalog":     {"listing", "search", "variants", "pricing"},
		"orders":      {"checkout", "fulfillment", "returns", "tracking"},
		"payments":    {"invoicing", "billing", "subscriptions", "refunds"},
		"inventory":   {"stock", "replenishment", "warehousing", "forecasting"},
		"procurement": {"sourcing", "rfq", "vendor-management", "contracts"},
	},
	"communication": {
		"email":    {"campaigns", "transactional", "templates", "deliverability"},
		"chat":     {"support", "notifications", "bots", "moderation"},
		"calendar": {"scheduling", "reminders", "availability", "booking"},
		"crm":      {"contacts", "pipelines", "forecasting", "activity"},
		"social":   {"posting", "engagement", "monitoring", "analytics"},
	},
	"data": {
		"analytics":     {"dashboards", "reporting", "kpi", "cohort"},
		"engineering":   {"pipelines", "etl", "warehousing", "streaming"},
		"ml":            {"training", "inference", "evaluation", "feature-store"},
		"governance":    {"quality", "lineage", "catalog", "privacy"},
		"visualization": {"charts", "maps", "real-time", "exports"},
	},
	"education": {
		"tutoring":       {"k12", "higher-ed", "professional", "language"},
		"curriculum":     {"design", "assessment", "multimedia", "translation"},
		"administration": {"enrollment", "scheduling", "advising", "reporting"},
		"research":       {"pedagogy", "outcomes", "grants", "policy"},
		"skills":         {"coding", "writing", "math", "critical-thinking"},
	},
	"finance": {
		"accounting": {"reconciliation", "bookkeeping", "auditing", "reporting"},
		"tax":        {"filing", "compliance", "planning", "research"},
		"payments":   {"invoicing", "payroll", "billing", "settlement"},
		"trading":    {"equities", "crypto", "forex", "derivatives"},
		"banking":    {"lending", "deposits", "risk", "compliance"},
	},
	"healthcare": {
		"clinical":  {"diagnosis", "treatment", "monitoring", "documentation"},
		"admin":     {"billing", "scheduling", "records", "coding"},
		"research":  {"trials", "genomics", "epidemiology", "analysis"},
		"wellness":  {"nutrition", "fitness", "mental-health", "prevention"},
		"pharmacy":  {"dispensing", "interactions", "formulary", "adherence"},
	},
	"hr": {
		"recruiting":  {"sourcing", "screening", "interviewing", "onboarding"},
		"performance": {"reviews", "feedback", "okrs", "planning"},
		"benefits":    {"administration", "compliance", "enrollment", "wellness"},
		"learning":    {"training", "certification", "development", "lms"},
		"workforce":   {"planning", "analytics", "scheduling", "offboarding"},
	},
	"infrastructure": {
		"cloud":      {"provisioning", "cost", "scaling", "migration"},
		"networking": {"dns", "load-balancing", "vpn", "firewall"},
		"security":   {"iam", "vulnerability", "compliance", "incident"},
		"devops":     {"ci-cd", "deployment", "monitoring", "alerting"},
		"storage":    {"backup", "archival", "replication", "encryption"},
	},
	"legal": {
		"contracts":  {"drafting", "review", "negotiation", "management"},
		"compliance": {"regulatory", "privacy", "employment", "reporting"},
		"litigation": {"research", "filing", "discovery", "strategy"},
		"ip":         {"patents", "trademarks", "licensing", "enforcement"},
		"corporate":  {"governance", "mergers", "restructuring", "advisory"},
	},
	"logistics": {
		"shipping":    {"quoting", "labeling", "tracking", "returns"},
		"warehousing": {"receiving", "picking", "packing", "inventory"},
		"routing":     {"optimization", "scheduling", "multi-modal", "last-mile"},
		"customs":     {"classification", "compliance", "documentation", "duties"},
		"fleet":       {"tracking", "maintenance", "dispatch", "telematics"},
	},
	"real-estate": {
		"listings":            {"search", "valuation", "marketing", "syndication"},
		"transactions":        {"contracts", "closing", "title", "escrow"},
		"property-management": {"leasing", "maintenance", "rent", "inspections"},
		"investment":          {"analysis", "underwriting", "portfolio", "reporting"},
		"construction":        {"permitting", "scheduling", "budgeting", "inspection"},
	},
	"research": {
		"data-analysis": {"statistics", "visualization", "modeling", "forecasting"},
		"literature":    {"search", "summarization", "citation", "review"},
		"experiments":   {"design", "simulation", "reporting", "validation"},
		"science":       {"biology", "chemistry", "physics", "materials"},
		"social":        {"surveys", "ethnography", "policy", "economics"},
	},
}

// ValidateCapabilityNode validates the format of a capability string.
// The taxonomy is treated as suggestions only; any well-formed custom
// capability is accepted.
//
// Rules:
//   - Non-empty, max 200 characters total.
//   - At most 3 levels separated by ">".
//   - Each segment: non-empty, max 100 characters.
//   - Level 1 (category) appears in agent:// URIs, so it is restricted to
//     letters, digits, hyphens and underscores (slug-safe).
//   - Levels 2 and 3 may contain any printable characters except ">".
func ValidateCapabilityNode(cap string) error {
	if cap == "" {
		return fmt.Errorf("capability is required")
	}
	if len(cap) > 200 {
		return fmt.Errorf("capability too long (max 200 characters)")
	}
	parts := strings.Split(cap, ">")
	if len(parts) > 3 {
		return fmt.Errorf("capability may have at most 3 levels (category > subcategory > specialisation)")
	}
	for i, p := range parts {
		if p == "" {
			return fmt.Errorf("capability segment %d is empty", i+1)
		}
		if len(p) > 100 {
			return fmt.Errorf("capability segment %d too long (max 100 characters)", i+1)
		}
		if i == 0 {
			// First segment goes into agent:// URIs — must be slug-safe.
			for _, r := range p {
				if !isSlugRune(r) {
					return fmt.Errorf("category %q may only contain letters, digits, hyphens and underscores", p)
				}
			}
		}
	}
	return nil
}

// isSlugRune reports whether r is safe for use in an agent:// URI segment.
func isSlugRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '-' || r == '_'
}

// TopLevelCategory extracts the first segment of a capability string.
// "finance>accounting>reconciliation" → "finance"
func TopLevelCategory(cap string) string {
	if idx := strings.Index(cap, ">"); idx != -1 {
		return cap[:idx]
	}
	return cap
}

// FormatDisplay converts the internal ">"-delimited capability string to the
// human-readable " > " form shown in the UI.
// "finance>accounting>reconciliation" → "finance > accounting > reconciliation"
func FormatDisplay(cap string) string {
	return strings.ReplaceAll(cap, ">", " > ")
}

// SortedCategories returns taxonomy top-level keys in alphabetical order.
func SortedCategories() []string {
	cats := make([]string, 0, len(Taxonomy))
	for k := range Taxonomy {
		cats = append(cats, k)
	}
	sort.Strings(cats)
	return cats
}

// SortedSubcategories returns the subcategory keys for a category, alphabetically.
// Returns nil if the category does not exist.
func SortedSubcategories(category string) []string {
	subs, ok := Taxonomy[category]
	if !ok {
		return nil
	}
	keys := make([]string, 0, len(subs))
	for k := range subs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// SortedItems returns the leaf items for a category+subcategory pair, in their
// defined order (not alphabetical, to preserve logical grouping).
// Returns nil if the category or subcategory does not exist.
func SortedItems(category, sub string) []string {
	subs, ok := Taxonomy[category]
	if !ok {
		return nil
	}
	items, ok := subs[sub]
	if !ok {
		return nil
	}
	return items
}

// ErrValidation is returned by service methods when the caller supplies invalid
// input. Handlers should convert this to HTTP 400 rather than 500.
type ErrValidation struct{ Msg string }

func (e *ErrValidation) Error() string { return e.Msg }
