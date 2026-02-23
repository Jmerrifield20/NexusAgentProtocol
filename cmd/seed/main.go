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
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
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
}

func ptr(u uuid.UUID) *uuid.UUID { return &u }

var alice = ptr(uuid.MustParse("00000000-0000-0000-0000-000000000001"))
var bob   = ptr(uuid.MustParse("00000000-0000-0000-0000-000000000002"))
var carol = ptr(uuid.MustParse("00000000-0000-0000-0000-000000000003"))

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
			$9, $10, '{}',
			$11, $12, $13,
			$14, $14,
			$15, $16
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

		ownerDomain := a.OwnerDomain
		certSerial := a.CertSerial
		version := a.Version
		supportURL := a.SupportURL

		if _, err := db.Exec(ctx, q,
			a.ID, a.TrustRoot, a.CapabilityNode, a.AgentID,
			a.DisplayName, a.Description, a.Endpoint, ownerDomain,
			a.Status, certSerial,
			version, a.Tags, supportURL,
			a.CreatedAt,
			a.OwnerUserID, a.RegistrationType,
		); err != nil {
			return fmt.Errorf("upsert agent %s: %w", uri, err)
		}
		fmt.Printf("  agent %-12s  %-52s  %s\n", tier, uri, a.DisplayName)
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
