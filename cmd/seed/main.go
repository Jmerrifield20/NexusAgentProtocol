// cmd/seed — populates the database with realistic mock data for development.
//
// Running twice is safe: existing rows are updated to match the seed definitions
// (ON CONFLICT ... DO UPDATE). To fully reset, truncate agents/users first:
//
//	psql $DATABASE_URL -c "TRUNCATE agents, certificates, dns_challenges CASCADE; DELETE FROM users WHERE id IN (SELECT id FROM users LIMIT 3);"
//
// Usage:
//
//	go run ./cmd/seed
//	DATABASE_URL=postgres://... go run ./cmd/seed
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jmerrifield20/NexusAgentProtocol/pkg/agentcard"
	"github.com/jmerrifield20/NexusAgentProtocol/pkg/mcpmanifest"
	"golang.org/x/crypto/bcrypt"
)

const defaultDB = "postgres://nexus:nexus@localhost:5432/nexus?sslmode=disable"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "seed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = defaultDB
	}

	ctx := context.Background()
	db, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		return fmt.Errorf("ping: %w", err)
	}
	fmt.Println("connected to database")

	if err := seedUsers(ctx, db); err != nil {
		return fmt.Errorf("seed users: %w", err)
	}
	if err := seedAgents(ctx, db); err != nil {
		return fmt.Errorf("seed agents: %w", err)
	}

	fmt.Println("\nseed complete")
	return nil
}

// ── Users ────────────────────────────────────────────────────────────────────

type seedUser struct {
	ID          uuid.UUID
	Email       string
	Username    string
	DisplayName string
	Password    string // plaintext; hashed before insert
}

var users = []seedUser{
	{
		ID:          uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Email:       "alice@acme.com",
		Username:    "alice",
		DisplayName: "Alice Chen",
		Password:    "nexus_dev",
	},
	{
		ID:          uuid.MustParse("00000000-0000-0000-0000-000000000002"),
		Email:       "bob@techcorp.io",
		Username:    "bob",
		DisplayName: "Bob Russo",
		Password:    "nexus_dev",
	},
	{
		ID:          uuid.MustParse("00000000-0000-0000-0000-000000000003"),
		Email:       "carol@nexusagentprotocol.com",
		Username:    "carol",
		DisplayName: "Carol Osei",
		Password:    "nexus_dev",
	},
}

func seedUsers(ctx context.Context, db *pgxpool.Pool) error {
	const q = `
		INSERT INTO users (id, email, password_hash, display_name, username, tier, email_verified)
		VALUES ($1, $2, $3, $4, $5, 'free', true)
		ON CONFLICT (id) DO UPDATE SET
			email          = EXCLUDED.email,
			password_hash  = EXCLUDED.password_hash,
			display_name   = EXCLUDED.display_name,
			username       = EXCLUDED.username,
			email_verified = true`

	for _, u := range users {
		hash, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hash password for %s: %w", u.Email, err)
		}
		if _, err := db.Exec(ctx, q, u.ID, u.Email, string(hash), u.DisplayName, u.Username); err != nil {
			return fmt.Errorf("insert user %s: %w", u.Email, err)
		}
		fmt.Printf("  user  %-32s  password: %s\n", u.Email, u.Password)
	}
	return nil
}

// ── Agents ───────────────────────────────────────────────────────────────────

type seedAgent struct {
	ID               uuid.UUID
	TrustRoot        string     // full domain for domain-verified; "nap" for free-hosted
	CapabilityNode   string     // full taxonomy path e.g. "finance>accounting"
	AgentID          string     // must start with "agent_"
	DisplayName      string
	Description      string
	Endpoint         string
	OwnerDomain      string     // domain-verified: same as TrustRoot; nap_hosted: ""
	Status           string     // pending | active
	CertSerial       string     // non-empty → trusted tier
	OwnerUserID      *uuid.UUID
	RegistrationType string     // domain | nap_hosted
	Version          string
	Tags             []string
	SupportURL       string
	CreatedAt        time.Time

	// A2A skills declared at registration. When empty the registry auto-derives
	// one skill from the capability taxonomy (same logic as the live path).
	Skills []agentcard.A2ASkill

	// MCP tool definitions. Stored as metadata["_mcp_tools"] so the endpoint
	// GET /api/v1/agents/:id/mcp-manifest.json can serve them.
	MCPTools []mcpmanifest.MCPTool
}

func ptr(u uuid.UUID) *uuid.UUID { return &u }

var alice = ptr(uuid.MustParse("00000000-0000-0000-0000-000000000001"))
var bob   = ptr(uuid.MustParse("00000000-0000-0000-0000-000000000002"))
var carol = ptr(uuid.MustParse("00000000-0000-0000-0000-000000000003"))

// rawSchema is a convenience helper for inline JSON Schema objects.
func rawSchema(s string) json.RawMessage { return json.RawMessage(s) }

var agents = []seedAgent{

	// ── Trusted (domain-verified + active + X.509 cert) ──────────────────────
	// URI: agent://acme.com/finance/agent_tax-advisor
	{
		ID:               uuid.MustParse("10000000-0000-0000-0000-000000000001"),
		TrustRoot:        "acme.com",
		CapabilityNode:   "finance>accounting",
		AgentID:          "agent_tax-advisor",
		DisplayName:      "ACME Tax Advisor",
		Description:      "Automates federal and state tax filings, identifies deductions, and answers tax queries for ACME Corp employees.",
		Endpoint:         "https://agents.acme.com/finance/tax",
		OwnerDomain:      "acme.com",
		Status:           "active",
		CertSerial:       "4a1b2c3d4e5f0001",
		RegistrationType: "domain",
		Version:          "2.1.0",
		Tags:             []string{"tax", "filing", "accounting", "usa"},
		SupportURL:       "https://docs.acme.com/agents/tax-advisor",
		CreatedAt:        daysAgo(120),
		Skills: []agentcard.A2ASkill{
			{ID: "tax-filing", Name: "Tax Filing", Description: "Prepare and file federal and state tax returns", Tags: []string{"tax", "filing", "irs"}},
			{ID: "deduction-analysis", Name: "Deduction Analysis", Description: "Identify eligible deductions and tax credits from financial records", Tags: []string{"tax", "optimization"}},
			{ID: "tax-query", Name: "Tax Q&A", Description: "Answer ad-hoc tax questions in plain English", Tags: []string{"tax", "qa"}},
		},
		MCPTools: []mcpmanifest.MCPTool{
			{
				Name:        "file_return",
				Description: "File a tax return for the specified fiscal year",
				InputSchema: rawSchema(`{"type":"object","required":["fiscal_year","entity_type"],"properties":{"fiscal_year":{"type":"integer","description":"e.g. 2024"},"entity_type":{"type":"string","enum":["individual","corporation","partnership"]}}}`),
			},
			{
				Name:        "estimate_deductions",
				Description: "Estimate eligible deductions from provided income and expense data",
				InputSchema: rawSchema(`{"type":"object","required":["income","expenses"],"properties":{"income":{"type":"number"},"expenses":{"type":"object","additionalProperties":{"type":"number"}}}}`),
			},
		},
	},
	// URI: agent://stripe.com/commerce/agent_checkout-bot
	{
		ID:               uuid.MustParse("10000000-0000-0000-0000-000000000002"),
		TrustRoot:        "stripe.com",
		CapabilityNode:   "commerce>payments",
		AgentID:          "agent_checkout-bot",
		DisplayName:      "Stripe Checkout Bot",
		Description:      "Handles payment intent creation, refund processing, and dispute resolution on behalf of Stripe merchants.",
		Endpoint:         "https://api.stripe.com/agents/checkout",
		OwnerDomain:      "stripe.com",
		Status:           "active",
		CertSerial:       "4a1b2c3d4e5f0002",
		RegistrationType: "domain",
		Version:          "1.4.2",
		Tags:             []string{"payments", "checkout", "refunds", "disputes"},
		SupportURL:       "https://stripe.com/docs/agents",
		CreatedAt:        daysAgo(200),
		Skills: []agentcard.A2ASkill{
			{ID: "payment-processing", Name: "Payment Processing", Description: "Create and manage payment intents", Tags: []string{"stripe", "payments"}},
			{ID: "refund-management", Name: "Refund Management", Description: "Process full and partial refunds", Tags: []string{"stripe", "refunds"}},
			{ID: "dispute-resolution", Name: "Dispute Resolution", Description: "Respond to chargebacks with supporting evidence", Tags: []string{"stripe", "disputes"}},
		},
		MCPTools: []mcpmanifest.MCPTool{
			{
				Name:        "create_payment_intent",
				Description: "Create a Stripe payment intent for the given amount and currency",
				InputSchema: rawSchema(`{"type":"object","required":["amount","currency"],"properties":{"amount":{"type":"integer","description":"Amount in smallest currency unit (e.g. cents)"},"currency":{"type":"string","description":"ISO 4217 code, e.g. usd"},"customer_id":{"type":"string"}}}`),
			},
			{
				Name:        "issue_refund",
				Description: "Issue a full or partial refund for a charge",
				InputSchema: rawSchema(`{"type":"object","required":["charge_id"],"properties":{"charge_id":{"type":"string"},"amount":{"type":"integer","description":"Amount to refund in smallest unit; omit for full refund"}}}`),
			},
		},
	},
	// URI: agent://salesforce.com/commerce/agent_pipeline-mgr
	{
		ID:               uuid.MustParse("10000000-0000-0000-0000-000000000003"),
		TrustRoot:        "salesforce.com",
		CapabilityNode:   "commerce>orders",
		AgentID:          "agent_pipeline-mgr",
		DisplayName:      "Salesforce Pipeline Manager",
		Description:      "Monitors CRM pipeline health, drafts follow-up emails, and escalates stalled deals to account executives.",
		Endpoint:         "https://agents.salesforce.com/commerce/pipeline",
		OwnerDomain:      "salesforce.com",
		Status:           "active",
		CertSerial:       "4a1b2c3d4e5f0003",
		RegistrationType: "domain",
		Version:          "3.0.1",
		Tags:             []string{"crm", "pipeline", "sales", "deals"},
		SupportURL:       "https://help.salesforce.com/agents",
		CreatedAt:        daysAgo(90),
		Skills: []agentcard.A2ASkill{
			{ID: "pipeline-monitoring", Name: "Pipeline Monitoring", Description: "Track deal health across pipeline stages", Tags: []string{"crm", "pipeline"}},
			{ID: "follow-up-drafting", Name: "Follow-up Drafting", Description: "Generate personalised follow-up emails for stalled deals", Tags: []string{"sales", "email"}},
		},
		MCPTools: []mcpmanifest.MCPTool{
			{
				Name:        "get_stalled_deals",
				Description: "Return deals that have not progressed in the last N days",
				InputSchema: rawSchema(`{"type":"object","required":["days_stalled"],"properties":{"days_stalled":{"type":"integer","minimum":1},"stage":{"type":"string"}}}`),
			},
			{
				Name:        "draft_followup",
				Description: "Draft a follow-up email for a specific deal",
				InputSchema: rawSchema(`{"type":"object","required":["deal_id"],"properties":{"deal_id":{"type":"string"},"tone":{"type":"string","enum":["professional","friendly","urgent"]}}}`),
			},
		},
	},

	// ── Verified (domain-verified + active, no cert) ──────────────────────────
	// URI: agent://techcorp.io/infrastructure/agent_code-reviewer
	{
		ID:               uuid.MustParse("20000000-0000-0000-0000-000000000001"),
		TrustRoot:        "techcorp.io",
		CapabilityNode:   "infrastructure>devops",
		AgentID:          "agent_code-reviewer",
		DisplayName:      "TechCorp Code Reviewer",
		Description:      "Reviews pull requests, flags security anti-patterns, and enforces style guidelines across TypeScript and Go codebases.",
		Endpoint:         "https://agents.techcorp.io/infra/review",
		OwnerDomain:      "techcorp.io",
		Status:           "active",
		RegistrationType: "domain",
		Version:          "1.0.0",
		Tags:             []string{"code-review", "security", "typescript", "go"},
		SupportURL:       "https://github.com/techcorp/agents",
		CreatedAt:        daysAgo(45),
		Skills: []agentcard.A2ASkill{
			{ID: "pr-review", Name: "Pull Request Review", Description: "Automated review of GitHub pull requests for bugs, style, and security", Tags: []string{"github", "pr"}},
			{ID: "security-audit", Name: "Security Audit", Description: "Detect OWASP top-10 patterns and common vulnerabilities in code", Tags: []string{"security", "owasp"}},
		},
		MCPTools: []mcpmanifest.MCPTool{
			{
				Name:        "review_pr",
				Description: "Review a GitHub pull request and return structured findings",
				InputSchema: rawSchema(`{"type":"object","required":["owner","repo","pr_number"],"properties":{"owner":{"type":"string"},"repo":{"type":"string"},"pr_number":{"type":"integer"}}}`),
			},
		},
	},
	// URI: agent://medcenter.org/healthcare/agent_patient-intake
	{
		ID:               uuid.MustParse("20000000-0000-0000-0000-000000000002"),
		TrustRoot:        "medcenter.org",
		CapabilityNode:   "healthcare>clinical",
		AgentID:          "agent_patient-intake",
		DisplayName:      "MedCenter Patient Intake",
		Description:      "Collects patient history, insurance details, and symptom information prior to physician consultations.",
		Endpoint:         "https://intake.medcenter.org/agent",
		OwnerDomain:      "medcenter.org",
		Status:           "active",
		RegistrationType: "domain",
		Version:          "1.2.0",
		Tags:             []string{"healthcare", "intake", "hipaa", "ehr"},
		SupportURL:       "https://medcenter.org/support",
		CreatedAt:        daysAgo(30),
		Skills: []agentcard.A2ASkill{
			{ID: "patient-history", Name: "Patient History Collection", Description: "Gather structured medical history via conversational intake", Tags: []string{"ehr", "clinical"}},
			{ID: "insurance-verification", Name: "Insurance Verification", Description: "Verify patient insurance eligibility in real time", Tags: []string{"insurance", "hipaa"}},
		},
		// No MCP tools — auto-derived skills only; this agent uses a custom webhook model.
	},
	// URI: agent://legalops.ai/legal/agent_contract-analyzer
	{
		ID:               uuid.MustParse("20000000-0000-0000-0000-000000000003"),
		TrustRoot:        "legalops.ai",
		CapabilityNode:   "legal>contracts",
		AgentID:          "agent_contract-analyzer",
		DisplayName:      "LegalOps Contract Analyzer",
		Description:      "Extracts key clauses, liability terms, and renewal dates from commercial contracts. Flags non-standard terms.",
		Endpoint:         "https://api.legalops.ai/agents/contracts",
		OwnerDomain:      "legalops.ai",
		Status:           "active",
		RegistrationType: "domain",
		Version:          "2.0.0",
		Tags:             []string{"legal", "contracts", "nda", "clause-extraction"},
		SupportURL:       "https://legalops.ai/docs",
		CreatedAt:        daysAgo(15),
		Skills: []agentcard.A2ASkill{
			{ID: "clause-extraction", Name: "Clause Extraction", Description: "Extract and categorise key clauses from contract documents", Tags: []string{"legal", "nlp"}},
			{ID: "risk-flagging", Name: "Risk Flagging", Description: "Identify non-standard or high-risk terms against a reference template", Tags: []string{"legal", "risk"}},
		},
		MCPTools: []mcpmanifest.MCPTool{
			{
				Name:        "analyze_contract",
				Description: "Extract clauses and flag risks from a contract document",
				InputSchema: rawSchema(`{"type":"object","required":["document_url"],"properties":{"document_url":{"type":"string","format":"uri"},"contract_type":{"type":"string","enum":["nda","saas","employment","vendor"]}}}`),
			},
			{
				Name:        "compare_to_template",
				Description: "Compare a contract to a standard template and return diffs",
				InputSchema: rawSchema(`{"type":"object","required":["document_url","template_id"],"properties":{"document_url":{"type":"string","format":"uri"},"template_id":{"type":"string"}}}`),
			},
		},
	},

	// ── Basic (nap_hosted + active) ───────────────────────────────────────────
	// URI: agent://nap/research/agent_research-bot
	{
		ID:               uuid.MustParse("30000000-0000-0000-0000-000000000001"),
		TrustRoot:        "nap",
		CapabilityNode:   "research",
		AgentID:          "agent_research-bot",
		DisplayName:      "Alice's Research Bot",
		Description:      "Searches academic papers, summarises findings, and generates literature reviews on demand.",
		Endpoint:         "https://alice-research.fly.dev",
		OwnerDomain:      "",
		Status:           "active",
		OwnerUserID:      alice,
		RegistrationType: "nap_hosted",
		Version:          "1.0.0",
		Tags:             []string{"research", "academia", "summarization"},
		CreatedAt:        daysAgo(10),
		Skills: []agentcard.A2ASkill{
			{ID: "literature-search", Name: "Literature Search", Description: "Search Semantic Scholar, arXiv, and PubMed for papers by topic", Tags: []string{"academia", "search"}},
			{ID: "literature-review", Name: "Literature Review", Description: "Generate a structured literature review from a set of papers", Tags: []string{"writing", "research"}},
		},
		MCPTools: []mcpmanifest.MCPTool{
			{
				Name:        "search_papers",
				Description: "Search academic databases for papers matching the given topic",
				InputSchema: rawSchema(`{"type":"object","required":["query"],"properties":{"query":{"type":"string"},"max_results":{"type":"integer","default":10},"source":{"type":"string","enum":["arxiv","pubmed","semantic_scholar","all"],"default":"all"}}}`),
			},
			{
				Name:        "generate_review",
				Description: "Generate a structured literature review from a list of paper IDs",
				InputSchema: rawSchema(`{"type":"object","required":["paper_ids"],"properties":{"paper_ids":{"type":"array","items":{"type":"string"}},"style":{"type":"string","enum":["narrative","systematic"],"default":"narrative"}}}`),
			},
		},
	},
	// URI: agent://nap/communication/agent_meeting-summariser
	{
		ID:               uuid.MustParse("30000000-0000-0000-0000-000000000002"),
		TrustRoot:        "nap",
		CapabilityNode:   "communication>meetings",
		AgentID:          "agent_meeting-summariser",
		DisplayName:      "Alice's Meeting Summariser",
		Description:      "Transcribes meeting recordings, extracts action items, and posts summaries to Slack.",
		Endpoint:         "https://alice-meetings.fly.dev",
		OwnerDomain:      "",
		Status:           "active",
		OwnerUserID:      alice,
		RegistrationType: "nap_hosted",
		Version:          "1.1.0",
		Tags:             []string{"meetings", "transcription", "slack", "action-items"},
		CreatedAt:        daysAgo(7),
		Skills: []agentcard.A2ASkill{
			{ID: "transcription", Name: "Meeting Transcription", Description: "Transcribe audio/video meeting recordings to text", Tags: []string{"audio", "transcription"}},
			{ID: "action-items", Name: "Action Item Extraction", Description: "Extract and assign action items from a meeting transcript", Tags: []string{"productivity", "meetings"}},
		},
		MCPTools: []mcpmanifest.MCPTool{
			{
				Name:        "transcribe_meeting",
				Description: "Transcribe a meeting recording from a URL",
				InputSchema: rawSchema(`{"type":"object","required":["recording_url"],"properties":{"recording_url":{"type":"string","format":"uri"},"language":{"type":"string","default":"en"}}}`),
			},
			{
				Name:        "extract_action_items",
				Description: "Extract action items from a meeting transcript",
				InputSchema: rawSchema(`{"type":"object","required":["transcript"],"properties":{"transcript":{"type":"string"},"format":{"type":"string","enum":["markdown","json"],"default":"markdown"}}}`),
			},
		},
	},
	// URI: agent://nap/data/agent_data-analyst
	{
		ID:               uuid.MustParse("30000000-0000-0000-0000-000000000003"),
		TrustRoot:        "nap",
		CapabilityNode:   "data>analytics",
		AgentID:          "agent_data-analyst",
		DisplayName:      "Bob's Data Analyst",
		Description:      "Runs SQL queries, builds visualisation specs, and explains statistical trends in plain English.",
		Endpoint:         "https://bob-analyst.railway.app",
		OwnerDomain:      "",
		Status:           "active",
		OwnerUserID:      bob,
		RegistrationType: "nap_hosted",
		Version:          "0.9.0",
		Tags:             []string{"sql", "analytics", "visualization", "statistics"},
		CreatedAt:        daysAgo(5),
		Skills: []agentcard.A2ASkill{
			{ID: "sql-analysis", Name: "SQL Analysis", Description: "Execute SQL queries against connected databases and explain results", Tags: []string{"sql", "database"}},
			{ID: "chart-generation", Name: "Chart Generation", Description: "Produce Vega-Lite or Chart.js visualisation specs from query results", Tags: []string{"visualization", "charts"}},
			{ID: "trend-explanation", Name: "Trend Explanation", Description: "Summarise statistical trends in plain English from tabular data", Tags: []string{"statistics", "nlp"}},
		},
		MCPTools: []mcpmanifest.MCPTool{
			{
				Name:        "run_query",
				Description: "Execute a SQL query on the connected database and return results",
				InputSchema: rawSchema(`{"type":"object","required":["query"],"properties":{"query":{"type":"string","description":"Read-only SQL SELECT statement"},"limit":{"type":"integer","default":100,"maximum":1000}}}`),
			},
			{
				Name:        "generate_chart_spec",
				Description: "Generate a Vega-Lite chart spec from tabular query results",
				InputSchema: rawSchema(`{"type":"object","required":["data","chart_type"],"properties":{"data":{"type":"array","items":{"type":"object"}},"chart_type":{"type":"string","enum":["bar","line","scatter","pie","histogram"]},"x_field":{"type":"string"},"y_field":{"type":"string"}}}`),
			},
		},
	},

	// ── Unverified (pending) ──────────────────────────────────────────────────
	// URI: agent://startup.io/infrastructure/agent_debug-helper
	{
		ID:               uuid.MustParse("40000000-0000-0000-0000-000000000001"),
		TrustRoot:        "startup.io",
		CapabilityNode:   "infrastructure>monitoring",
		AgentID:          "agent_debug-helper",
		DisplayName:      "Startup Debug Helper",
		Description:      "Analyses stack traces, searches GitHub issues, and proposes fixes for runtime errors.",
		Endpoint:         "https://debug.startup.io",
		OwnerDomain:      "startup.io",
		Status:           "pending",
		RegistrationType: "domain",
		Tags:             []string{"debugging", "monitoring", "stack-traces"},
		CreatedAt:        daysAgo(2),
		// No skills/tools declared yet — pending agents would auto-derive from capability.
	},
	// URI: agent://nap/communication/agent_content-writer
	{
		ID:               uuid.MustParse("40000000-0000-0000-0000-000000000002"),
		TrustRoot:        "nap",
		CapabilityNode:   "communication>content",
		AgentID:          "agent_content-writer",
		DisplayName:      "Carol's Content Writer",
		Description:      "Drafts blog posts, social copy, and email campaigns from a brief.",
		Endpoint:         "https://carol-content.vercel.app",
		OwnerDomain:      "",
		Status:           "pending",
		OwnerUserID:      carol,
		RegistrationType: "nap_hosted",
		Tags:             []string{"content", "copywriting", "blog", "email"},
		CreatedAt:        daysAgo(1),
		Skills: []agentcard.A2ASkill{
			{ID: "blog-writing", Name: "Blog Writing", Description: "Draft long-form blog posts from a topic brief", Tags: []string{"content", "blog"}},
			{ID: "social-copy", Name: "Social Copy", Description: "Generate platform-optimised social media copy", Tags: []string{"social", "copywriting"}},
		},
		// No MCP tools yet — pending registration.
	},
}

func seedAgents(ctx context.Context, db *pgxpool.Pool) error {
	const q = `
		INSERT INTO agents (
			id, trust_root, capability_node, agent_id,
			display_name, description, endpoint, owner_domain,
			status, cert_serial, metadata,
			version, tags, support_url,
			created_at, updated_at,
			owner_user_id, registration_type
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8,
			$9, $10, $11,
			$12, $13, $14,
			$15, $15,
			$16, $17
		)
		ON CONFLICT (id) DO UPDATE SET
			trust_root        = EXCLUDED.trust_root,
			capability_node   = EXCLUDED.capability_node,
			agent_id          = EXCLUDED.agent_id,
			display_name      = EXCLUDED.display_name,
			description       = EXCLUDED.description,
			endpoint          = EXCLUDED.endpoint,
			owner_domain      = EXCLUDED.owner_domain,
			status            = EXCLUDED.status,
			cert_serial       = EXCLUDED.cert_serial,
			metadata          = EXCLUDED.metadata,
			version           = EXCLUDED.version,
			tags              = EXCLUDED.tags,
			support_url       = EXCLUDED.support_url,
			owner_user_id     = EXCLUDED.owner_user_id,
			registration_type = EXCLUDED.registration_type,
			updated_at        = now()`

	fmt.Println()
	for _, a := range agents {
		topLevel := a.CapabilityNode
		if idx := strings.Index(topLevel, ">"); idx != -1 {
			topLevel = topLevel[:idx]
		}
		uri := "agent://" + a.TrustRoot + "/" + topLevel + "/" + a.AgentID
		tier := computeTier(a)

		// Build metadata JSONB — mirrors what service.Register() stores at runtime.
		meta := map[string]string{}
		if len(a.Skills) > 0 {
			b, _ := json.Marshal(a.Skills)
			meta["_skills"] = string(b)
		}
		if len(a.MCPTools) > 0 {
			b, _ := json.Marshal(a.MCPTools)
			meta["_mcp_tools"] = string(b)
		}
		metaJSON, _ := json.Marshal(meta)

		ownerDomain := a.OwnerDomain
		certSerial := a.CertSerial
		version := a.Version
		supportURL := a.SupportURL

		if _, err := db.Exec(ctx, q,
			a.ID, a.TrustRoot, a.CapabilityNode, a.AgentID,
			a.DisplayName, a.Description, a.Endpoint, ownerDomain,
			a.Status, certSerial, string(metaJSON),
			version, a.Tags, supportURL,
			a.CreatedAt,
			a.OwnerUserID, a.RegistrationType,
		); err != nil {
			return fmt.Errorf("upsert agent %s: %w", uri, err)
		}

		skillCount := len(a.Skills)
		toolCount := len(a.MCPTools)
		fmt.Printf("  agent %-12s  %-52s  %-28s  skills:%d  mcp-tools:%d\n",
			tier, uri, a.DisplayName, skillCount, toolCount)
	}
	return nil
}

// computeTier mirrors model.Agent.ComputeTrustTier for the seed output log.
func computeTier(a seedAgent) string {
	if a.Status != "active" {
		return "unverified  "
	}
	if a.RegistrationType == "domain" {
		if a.CertSerial != "" {
			return "trusted     "
		}
		return "verified    "
	}
	return "basic       "
}

func daysAgo(n int) time.Time {
	return time.Now().UTC().Add(-time.Duration(n) * 24 * time.Hour)
}
