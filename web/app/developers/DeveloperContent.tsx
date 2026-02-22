"use client";

import { useState, useCallback } from "react";

// ── Chevron icon ────────────────────────────────────────────────────────────
function ChevronIcon({ open }: { open: boolean }) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      className={`shrink-0 transition-transform duration-200 ${open ? "rotate-0" : "-rotate-90"}`}
    >
      <polyline points="6 9 12 15 18 9" />
    </svg>
  );
}

// ── Section (accordion) ─────────────────────────────────────────────────────
function Section({
  id,
  title,
  open,
  onToggle,
  children,
}: {
  id: string;
  title: string;
  open: boolean;
  onToggle: () => void;
  children: React.ReactNode;
}) {
  return (
    <section id={id} className="scroll-mt-24">
      <button
        onClick={onToggle}
        className="w-full flex items-center justify-between pb-2 border-b border-gray-200 mb-4 group text-left"
        aria-expanded={open}
      >
        <h2 className="text-xl font-bold text-gray-900 group-hover:text-nexus-500 transition-colors">
          {title}
        </h2>
        <span className="ml-3 text-gray-400 group-hover:text-nexus-500 transition-colors">
          <ChevronIcon open={open} />
        </span>
      </button>
      {open && (
        <div className="space-y-4 text-sm text-gray-700 leading-relaxed">
          {children}
        </div>
      )}
    </section>
  );
}

// ── Code block ──────────────────────────────────────────────────────────────
function Code({ children }: { children: string }) {
  return (
    <pre className="overflow-x-auto rounded-lg bg-gray-900 p-5 text-sm font-mono text-gray-100 leading-relaxed">
      {children}
    </pre>
  );
}

// ── Endpoint card ───────────────────────────────────────────────────────────
function Endpoint({
  method,
  path,
  description,
  auth,
  badge,
}: {
  method: string;
  path: string;
  description: string;
  auth?: boolean;
  badge?: { label: string; className: string };
}) {
  const methodColor: Record<string, string> = {
    GET: "bg-blue-100 text-blue-700",
    POST: "bg-green-100 text-green-700",
    PATCH: "bg-yellow-100 text-yellow-700",
    DELETE: "bg-red-100 text-red-700",
  };
  return (
    <div className="rounded-lg border border-gray-200 bg-white p-4">
      <div className="flex items-center gap-3 mb-2 flex-wrap">
        <span className={`rounded px-2 py-0.5 text-xs font-bold font-mono ${methodColor[method] ?? "bg-gray-100 text-gray-700"}`}>
          {method}
        </span>
        <code className="text-sm font-mono text-gray-800">{path}</code>
        {auth && (
          <span className="rounded-full bg-amber-100 px-2 py-0.5 text-xs text-amber-700 font-medium">
            requires auth
          </span>
        )}
        {badge && (
          <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${badge.className}`}>
            {badge.label}
          </span>
        )}
      </div>
      <p className="text-sm text-gray-500">{description}</p>
    </div>
  );
}

// ── Tier badge colours ──────────────────────────────────────────────────────
const TIER_BADGES: Record<string, { label: string; className: string }> = {
  trusted:    { label: "Trusted",    className: "bg-emerald-100 text-emerald-700" },
  verified:   { label: "Verified",   className: "bg-indigo-100 text-indigo-700" },
  basic:      { label: "Basic",      className: "bg-blue-100 text-blue-700" },
  unverified: { label: "Unverified", className: "bg-gray-100 text-gray-500" },
};

// ── Nav items ───────────────────────────────────────────────────────────────
const NAV = [
  { id: "overview",         label: "Overview" },
  { id: "concepts",         label: "Core Concepts" },
  { id: "quickstart",       label: "Quick Start" },
  { id: "agent-lifecycle",  label: "Agent Lifecycle" },
  { id: "trust-tiers",      label: "Trust Tiers" },
  { id: "dns-verification", label: "DNS-01 Verification" },
  { id: "a2a",              label: "A2A Compatibility" },
  { id: "api-reference",    label: "API Reference" },
  { id: "trust-ledger",     label: "Trust Ledger" },
  { id: "auth",             label: "Authentication" },
  { id: "sdk",              label: "Go SDK" },
];

const ALL_IDS = NAV.map((n) => n.id);

// ── Main component ──────────────────────────────────────────────────────────
export default function DeveloperContent() {
  // All sections open by default.
  const [openSections, setOpenSections] = useState<Record<string, boolean>>(
    () => Object.fromEntries(ALL_IDS.map((id) => [id, true]))
  );

  const toggle = useCallback((id: string) => {
    setOpenSections((prev) => ({ ...prev, [id]: !prev[id] }));
  }, []);

  const allOpen  = ALL_IDS.every((id) => openSections[id]);
  const allClosed = ALL_IDS.every((id) => !openSections[id]);

  const expandAll  = () => setOpenSections(Object.fromEntries(ALL_IDS.map((id) => [id, true])));
  const collapseAll = () => setOpenSections(Object.fromEntries(ALL_IDS.map((id) => [id, false])));

  const s = (id: string) => ({
    id,
    open: openSections[id] ?? true,
    onToggle: () => toggle(id),
  });

  return (
    <div className="mx-auto max-w-7xl">
      <div className="flex gap-10">

        {/* Sidebar nav */}
        <aside className="hidden lg:block w-52 shrink-0">
          <div className="sticky top-8">
            <div className="mb-3 flex items-center justify-between">
              <p className="text-xs font-semibold uppercase tracking-widest text-gray-400">On this page</p>
            </div>
            <nav className="space-y-1 mb-4">
              {NAV.map((item) => (
                <div key={item.id} className="flex items-center gap-1">
                  <a
                    href={`#${item.id}`}
                    className="flex-1 rounded-md px-3 py-1.5 text-sm text-gray-600 hover:bg-gray-100 hover:text-nexus-500"
                  >
                    {item.label}
                  </a>
                  <button
                    onClick={() => toggle(item.id)}
                    className="p-1 rounded text-gray-300 hover:text-gray-500 hover:bg-gray-100"
                    title={openSections[item.id] ? "Collapse" : "Expand"}
                    aria-label={openSections[item.id] ? `Collapse ${item.label}` : `Expand ${item.label}`}
                  >
                    <ChevronIcon open={openSections[item.id] ?? true} />
                  </button>
                </div>
              ))}
            </nav>
            <div className="flex gap-2 border-t border-gray-100 pt-3">
              <button
                onClick={expandAll}
                disabled={allOpen}
                className="flex-1 rounded text-xs py-1 text-gray-500 hover:text-gray-800 hover:bg-gray-100 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
              >
                Expand all
              </button>
              <button
                onClick={collapseAll}
                disabled={allClosed}
                className="flex-1 rounded text-xs py-1 text-gray-500 hover:text-gray-800 hover:bg-gray-100 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
              >
                Collapse all
              </button>
            </div>
          </div>
        </aside>

        {/* Main content */}
        <div className="min-w-0 flex-1 space-y-12">

          <div className="flex items-end justify-between gap-4">
            <div>
              <h1 className="text-4xl font-extrabold tracking-tight text-gray-900">Developer Docs</h1>
              <p className="mt-3 text-lg text-gray-500">
                Everything you need to register agents, verify domains, get NAP-certified, resolve URIs, and integrate with the Nexus Agent Protocol.
              </p>
            </div>
            {/* Mobile expand/collapse controls */}
            <div className="flex gap-2 lg:hidden shrink-0">
              <button onClick={expandAll}   disabled={allOpen}   className="text-xs text-gray-500 hover:text-gray-800 disabled:opacity-30">Expand all</button>
              <button onClick={collapseAll} disabled={allClosed} className="text-xs text-gray-500 hover:text-gray-800 disabled:opacity-30">Collapse all</button>
            </div>
          </div>

          {/* Overview */}
          <Section title="Overview" {...s("overview")}>
            <p>
              The Nexus Agent Protocol gives AI agents a permanent, verifiable address on the internet using the{" "}
              <code className="rounded bg-gray-100 px-1.5 py-0.5 font-mono text-nexus-500">agent://</code> URI scheme.
              Think of it as DNS for agents — a neutral registry that maps logical addresses to live endpoints, with cryptographic proof of ownership baked in.
            </p>
            <p>
              The registry exposes a JSON REST API on port <strong>8080</strong> (HTTP) and port <strong>8443</strong> (mTLS).
              All endpoints live under <code className="rounded bg-gray-100 px-1 py-0.5 font-mono">/api/v1/</code>.
            </p>
            <p>
              Every activated agent receives a <strong>NAP Certification</strong> — a cryptographically signed endorsement JWT embedded
              in an A2A-compatible agent card. This card can be deployed at{" "}
              <code className="rounded bg-gray-100 px-1 py-0.5 font-mono">/.well-known/agent.json</code> on the agent's domain
              so that both NAP-aware and A2A-compatible clients can discover and trust it.
            </p>
            <div className="rounded-lg border border-blue-100 bg-blue-50 px-4 py-3 text-blue-800">
              <strong>Base URL (local dev):</strong>{" "}
              <code className="font-mono">http://localhost:8080/api/v1</code>
            </div>
          </Section>

          {/* Core Concepts */}
          <Section title="Core Concepts" {...s("concepts")}>
            <div className="grid gap-4 sm:grid-cols-2">
              {[
                { term: "Trust Root",          def: "The org that anchors the agent's identity. For domain-verified agents this is the full verified domain (e.g. acme.com), proved via DNS-01 — so agent://acme.com/… can only ever be ACME, not acme.io or amazon.fakeaccount.com. For hosted agents it is always nap — the registry-controlled namespace." },
                { term: "Capability Node",      def: "A label describing what the agent does — e.g. finance or support. Appears as the second segment of the agent:// URI (after the org namespace)." },
                { term: "Agent ID",             def: "A short unique identifier generated at registration — e.g. agent_7x2v9q. Stable even if the endpoint URL changes." },
                { term: "Endpoint",             def: "The physical HTTPS/gRPC URL where the agent currently listens. This is what resolve returns — callers use this to make requests." },
                { term: "Trust Tier",           def: "A computed credibility label: Trusted (domain-verified + mTLS cert), Verified (domain-verified, no cert), Basic (NAP-hosted, activated), or Unverified (pending / revoked). Included in every agent listing." },
                { term: "DNS-01 Challenge",     def: "The domain ownership proof mechanism. The registry issues a TXT record value; you publish it under _nexus-challenge.<domain> and call verify." },
                { term: "Identity Certificate", def: "An X.509 cert signed by the Nexus CA, issued at activation for domain-verified agents. The private key is returned once and never stored by the registry." },
                { term: "NAP Endorsement",      def: "A CA-signed RS256 JWT included in every activated agent's agent card. It attests the agent URI, trust tier, and cert serial. Verifiable via the registry's JWKS endpoint." },
                { term: "A2A Card",             def: "A JSON file compatible with the Google Agent2Agent protocol, extended with nap:* fields. Deploy at /.well-known/agent.json on your domain so A2A clients can discover your agent." },
                { term: "Trust Ledger",         def: "An append-only hash chain recording every registration and activation event. Anyone can independently verify the full history of any agent." },
                { term: "Task Token",           def: "A short-lived RS256 JWT issued at activation. Required for protected operations like revoke and delete." },
                { term: "Registration Type",    def: 'Either domain (company-owned domain, DNS-01 verified) or nap_hosted (free tier, hosted under nexusagentprotocol.com, email-verified).' },
              ].map((c) => (
                <div key={c.term} className="rounded-lg border border-gray-200 bg-white p-4">
                  <p className="font-semibold text-gray-900 mb-1">{c.term}</p>
                  <p className="text-xs text-gray-500 leading-relaxed">{c.def}</p>
                </div>
              ))}
            </div>
          </Section>

          {/* Quick Start */}
          <Section title="Quick Start" {...s("quickstart")}>
            <div className="flex gap-2 flex-wrap mb-4">
              <span className="rounded-full border border-gray-300 bg-white px-3 py-1 text-xs font-semibold text-gray-700">Path A — NAP Hosted (no domain required)</span>
              <span className="rounded-full border border-gray-300 bg-white px-3 py-1 text-xs font-semibold text-gray-700">Path B — Domain-Verified (company domain)</span>
            </div>

            <div className="rounded-lg border border-blue-100 bg-blue-50 p-4 space-y-4">
              <p className="font-semibold text-blue-900">Path A — NAP Hosted</p>
              <p className="text-xs text-blue-800">
                Sign up at <code className="font-mono">/signup</code>, verify your email, then register agents under{" "}
                <code className="font-mono">nexusagentprotocol.com</code>. No domain ownership required.
              </p>
              <div>
                <p className="font-medium text-gray-800 mb-2 text-xs">1 — Sign up and get a user token</p>
                <Code>{`curl -s -X POST http://localhost:8080/api/v1/auth/signup \\
  -H "Content-Type: application/json" \\
  -d '{"email":"you@example.com","password":"hunter2","display_name":"Alice"}'

# Check your email for the verification link, then:
curl -s -X POST "http://localhost:8080/api/v1/auth/verify-email?token=<TOKEN>"

# Log in to get a JWT:
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \\
  -H "Content-Type: application/json" \\
  -d '{"email":"you@example.com","password":"hunter2"}' | jq -r .token)`}</Code>
              </div>
              <div>
                <p className="font-medium text-gray-800 mb-2 text-xs">2 — Register a hosted agent</p>
                <Code>{`curl -s -X POST http://localhost:8080/api/v1/agents \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer $TOKEN" \\
  -d '{
    "registration_type": "nap_hosted",
    "display_name":      "My Assistant",
    "description":       "Answers questions about my product",
    "endpoint":          "https://api.example.com/assistant"
  }'
# Returns: id, agent_id, uri (agent://nap/assistant/agent_xxx)`}</Code>
              </div>
              <div>
                <p className="font-medium text-gray-800 mb-2 text-xs">3 — Activate and receive your NAP Certification</p>
                <Code>{`curl -s -X POST http://localhost:8080/api/v1/agents/<UUID>/activate
# Returns agent_card_json — your A2A-compatible NAP certified card.
# Deploy it at https://api.example.com/.well-known/agent.json`}</Code>
              </div>
            </div>

            <div className="rounded-lg border border-gray-200 bg-gray-50 p-4 space-y-4">
              <p className="font-semibold text-gray-900">Path B — Domain-Verified</p>
              <div>
                <p className="font-medium text-gray-800 mb-2 text-xs">1 — Register</p>
                <Code>{`curl -s -X POST http://localhost:8080/api/v1/agents \\
  -H "Content-Type: application/json" \\
  -d '{
    "trust_root":      "acme.com",
    "capability_node": "finance",
    "display_name":    "Acme Tax Agent",
    "description":     "Handles tax filing queries",
    "endpoint":        "https://agents.acme.com/tax",
    "owner_domain":    "acme.com"
  }'`}</Code>
              </div>
              <div>
                <p className="font-medium text-gray-800 mb-2 text-xs">2 — Complete DNS-01 verification (see DNS-01 section)</p>
              </div>
              <div>
                <p className="font-medium text-gray-800 mb-2 text-xs">3 — Activate</p>
                <Code>{`curl -s -X POST http://localhost:8080/api/v1/agents/<UUID>/activate
# Returns X.509 cert + private key + agent_card_json (Trusted tier)`}</Code>
              </div>
            </div>
          </Section>

          {/* Agent Lifecycle */}
          <Section title="Agent Lifecycle" {...s("agent-lifecycle")}>
            <p>Every agent moves through the following states:</p>
            <div className="flex items-center gap-2 flex-wrap">
              {["pending", "→", "active", "→", "revoked / expired"].map((s, i) => (
                s === "→"
                  ? <span key={i} className="text-gray-400 font-mono">→</span>
                  : <span key={i} className="rounded-full border border-gray-300 bg-white px-3 py-1 text-xs font-semibold text-gray-700">{s}</span>
              ))}
            </div>
            <div className="space-y-3">
              {[
                { state: "pending",          color: "bg-yellow-100 text-yellow-700", desc: "Created by POST /agents. Domain ownership (or email for nap_hosted) has not yet been verified. The agent is not resolvable." },
                { state: "active",           color: "bg-green-100 text-green-700",   desc: "Activated after verification passes. For domain agents, an X.509 cert and NAP endorsement are issued. For hosted agents, email verification is required." },
                { state: "revoked",          color: "bg-red-100 text-red-700",       desc: "Manually revoked via POST /agents/:id/revoke. The agent remains in the registry but resolve returns an error." },
                { state: "expired",          color: "bg-gray-100 text-gray-700",     desc: "The agent's certificate has passed its validity window. Re-activation is required." },
              ].map((s) => (
                <div key={s.state} className="flex items-start gap-3 rounded-lg border border-gray-200 bg-white p-4">
                  <span className={`mt-0.5 shrink-0 rounded-full px-2.5 py-0.5 text-xs font-semibold ${s.color}`}>{s.state}</span>
                  <p className="text-sm text-gray-600">{s.desc}</p>
                </div>
              ))}
            </div>
          </Section>

          {/* Trust Tiers */}
          <Section title="Trust Tiers" {...s("trust-tiers")}>
            <p>
              Every agent in the registry carries a <strong>trust tier</strong> — a computed credibility label that tells callers how
              much verification has been performed. Tiers are derived from the agent's registration type, activation status,
              and whether an mTLS certificate has been issued. They are never manually set.
            </p>
            <div className="space-y-3">
              {[
                {
                  tier: "trusted",
                  how:  "Domain-verified (DNS-01) + active + mTLS certificate issued by the Nexus CA",
                  desc: "The highest level of assurance. The owner has proven DNS control of their domain and holds a CA-signed X.509 certificate. NAP endorsement is always included.",
                },
                {
                  tier: "verified",
                  how:  "Domain-verified (DNS-01) + active, no mTLS cert",
                  desc: "Domain ownership proved but cert issuance was not requested or not yet completed.",
                },
                {
                  tier: "basic",
                  how:  "nap_hosted registration + active + email verified",
                  desc: "No domain ownership required. Identity is bound to a verified email address under nexusagentprotocol.com. NAP endorsement is included.",
                },
                {
                  tier: "unverified",
                  how:  "Any agent that is pending, revoked, or expired",
                  desc: "The agent has not completed the activation process or has been deactivated. Not resolvable.",
                },
              ].map((t) => (
                <div key={t.tier} className="rounded-lg border border-gray-200 bg-white p-4">
                  <div className="flex items-center gap-3 mb-2">
                    <span className={`rounded-full px-2.5 py-0.5 text-xs font-semibold ${TIER_BADGES[t.tier].className}`}>
                      {TIER_BADGES[t.tier].label}
                    </span>
                    <code className="text-xs text-gray-500 font-mono">{t.how}</code>
                  </div>
                  <p className="text-sm text-gray-600">{t.desc}</p>
                </div>
              ))}
            </div>
            <p>
              The <code className="font-mono rounded bg-gray-100 px-1">trust_tier</code> field is returned on every agent object and
              in the NAP discovery card. Use it to gate access — for example, only accepting requests from{" "}
              <code className="font-mono rounded bg-gray-100 px-1">trusted</code> or{" "}
              <code className="font-mono rounded bg-gray-100 px-1">verified</code> agents.
            </p>
            <Code>{`{
  "id":                "550e8400-...",
  "uri":               "agent://acme.com/finance/agent_7x2v9q",
  "display_name":      "Acme Tax Agent",
  "status":            "active",
  "trust_tier":        "trusted",
  "registration_type": "domain",
  "cert_serial":       "3f9a...",
  ...
}`}</Code>
          </Section>

          {/* DNS-01 Verification */}
          <Section title="DNS-01 Verification" {...s("dns-verification")}>
            <p>
              Before a domain-verified agent can be activated, you must prove you control the{" "}
              <code className="font-mono rounded bg-gray-100 px-1">owner_domain</code>.
              This uses the same DNS-01 mechanism as Let's Encrypt.
            </p>
            <div className="space-y-4">
              <div>
                <p className="font-medium text-gray-800 mb-2">Step 1 — Start a challenge</p>
                <Code>{`curl -s -X POST http://localhost:8080/api/v1/dns/challenge \\
  -H "Content-Type: application/json" \\
  -d '{"domain": "acme.com"}'

# Response:
{
  "id":         "a1b2c3d4-...",
  "domain":     "acme.com",
  "txt_host":   "_nexus-challenge.acme.com",
  "txt_record": "nexus-verify=abc123xyz...",
  "expires_at": "2026-02-21T05:00:00Z"
}`}</Code>
              </div>
              <div>
                <p className="font-medium text-gray-800 mb-2">Step 2 — Publish the TXT record</p>
                <p>
                  Add a DNS TXT record at the host shown in{" "}
                  <code className="font-mono rounded bg-gray-100 px-1">txt_host</code> with the value from{" "}
                  <code className="font-mono rounded bg-gray-100 px-1">txt_record</code>. Allow time for DNS propagation (typically 1–5 minutes).
                </p>
                <Code>{`# Example (varies by DNS provider)
_nexus-challenge.acme.com.  IN  TXT  "nexus-verify=abc123xyz..."`}</Code>
              </div>
              <div>
                <p className="font-medium text-gray-800 mb-2">Step 3 — Trigger verification</p>
                <Code>{`curl -s -X POST http://localhost:8080/api/v1/dns/challenge/<CHALLENGE_ID>/verify`}</Code>
                <p>The registry performs a live DNS lookup. On success the challenge is marked verified and you can activate any agents under that domain.</p>
              </div>
              <div>
                <p className="font-medium text-gray-800 mb-2">Step 4 — Activate the agent</p>
                <Code>{`curl -s -X POST http://localhost:8080/api/v1/agents/<AGENT_UUID>/activate`}</Code>
              </div>
            </div>
            <div className="rounded-lg border border-amber-100 bg-amber-50 px-4 py-3 text-amber-800 text-xs">
              Challenges expire after <strong>15 minutes</strong>. If verification times out, start a new challenge.
            </div>
          </Section>

          {/* A2A Compatibility */}
          <Section title="A2A Compatibility & NAP Certification" {...s("a2a")}>
            <p>
              The Nexus Agent Protocol is compatible with the{" "}
              <strong>Google Agent2Agent (A2A) protocol</strong>. Every activated agent receives a ready-to-deploy
              JSON file — the <strong>NAP-certified agent card</strong> — that A2A clients can read natively.
              NAP-aware clients additionally verify the embedded <strong>NAP Endorsement</strong> JWT to confirm
              the agent's trust tier.
            </p>
            <div className="rounded-lg border border-emerald-100 bg-emerald-50 px-4 py-3 text-emerald-800 text-sm">
              <strong>NAP Certification</strong> = A2A-compatible agent card + CA-signed endorsement JWT verifiable
              via the registry's JWKS endpoint.
            </div>
            <p className="font-medium text-gray-800">What the activation response contains</p>
            <Code>{`POST /api/v1/agents/<UUID>/activate  →  200 OK

{
  "status": "activated",
  "agent":  { ... },

  // X.509 cert (domain-verified agents only)
  "certificate":     { "serial": "3f9a...", "pem": "-----BEGIN CERTIFICATE-----..." },
  "private_key_pem": "-----BEGIN RSA PRIVATE KEY-----...",
  "ca_pem":          "-----BEGIN CERTIFICATE-----...",
  "warning":         "Store private_key_pem securely. It will not be shown again.",

  // NAP Certification — your deployable A2A agent card
  "agent_card_json": "{ \\"name\\": \\"Acme Tax Agent\\", ... }",
  "agent_card_note": "Deploy agent_card_json at https://yourdomain/.well-known/agent.json"
}`}</Code>
            <p className="font-medium text-gray-800">The agent card format</p>
            <Code>{`{
  // Standard A2A fields
  "name":        "Acme Tax Agent",
  "description": "Handles tax filing queries",
  "url":         "https://agents.acme.com/tax",
  "version":     "1.0",
  "capabilities": { "streaming": false, "pushNotifications": false, "stateTransitionHistory": false },

  // NAP extension fields (ignored by plain A2A clients)
  "nap:uri":         "agent://acme.com/finance/agent_7x2v9q",
  "nap:trust_tier":  "trusted",
  "nap:registry":    "https://registry.nexusagentprotocol.com",
  "nap:cert_serial": "3f9a...",
  "nap:endorsement": "<RS256 JWT signed by the Nexus CA>"
}`}</Code>
            <p className="font-medium text-gray-800">Deploying the card</p>
            <Code>{`# Write the card to your web server
echo '<agent_card_json value>' > /var/www/html/.well-known/agent.json

# Verify A2A clients can reach it
curl https://agents.acme.com/.well-known/agent.json`}</Code>
            <p className="font-medium text-gray-800">Verifying the NAP Endorsement</p>
            <Code>{`# Fetch the registry public key
curl https://registry.nexusagentprotocol.com/.well-known/jwks.json

# Decode and verify the endorsement JWT
jwt decode <nap:endorsement value>
# {
#   "iss":              "https://registry.nexusagentprotocol.com",
#   "sub":              "agent://acme.com/finance/agent_7x2v9q",
#   "nap:uri":          "agent://acme.com/finance/agent_7x2v9q",
#   "nap:trust_tier":   "trusted",
#   "nap:cert_serial":  "3f9a...",
#   "nap:registry":     "https://registry.nexusagentprotocol.com",
#   "exp":              1771234567
# }`}</Code>
            <p className="font-medium text-gray-800">NAP Registry discovery card</p>
            <Code>{`curl "https://registry.nexusagentprotocol.com/.well-known/agent-card.json?domain=acme.com"
# {
#   "schema_version": "1.0",
#   "domain":         "acme.com",
#   "trust_root":     "acme.com",
#   "agents": [
#     {
#       "uri":            "agent://acme.com/finance/agent_7x2v9q",
#       "display_name":   "Acme Tax Agent",
#       "endpoint":       "https://agents.acme.com/tax",
#       "status":         "active",
#       "nap:trust_tier": "trusted"
#     }
#   ]
# }`}</Code>
          </Section>

          {/* API Reference */}
          <Section title="API Reference" {...s("api-reference")}>
            <p className="font-semibold text-gray-800">User Auth</p>
            <div className="space-y-2">
              <Endpoint method="POST" path="/api/v1/auth/signup"              description='Create a new user account. Body: {"email","password","display_name?"}. Returns user JWT.' badge={{ label: "free tier", className: "bg-blue-100 text-blue-700" }} />
              <Endpoint method="POST" path="/api/v1/auth/login"               description='Authenticate with email and password. Body: {"email","password"}. Returns user JWT.' badge={{ label: "free tier", className: "bg-blue-100 text-blue-700" }} />
              <Endpoint method="POST" path="/api/v1/auth/verify-email"        description="Consume an email verification token (query param ?token= or body). Enables nap_hosted agent activation." badge={{ label: "free tier", className: "bg-blue-100 text-blue-700" }} />
              <Endpoint method="POST" path="/api/v1/auth/resend-verification" description="Resend the verification email. Body or query param: email address." />
              <Endpoint method="GET"  path="/api/v1/auth/oauth/:provider"          description="Redirect to GitHub or Google OAuth flow." />
              <Endpoint method="GET"  path="/api/v1/auth/oauth/:provider/callback" description="OAuth callback — exchanges code for a user JWT." />
            </div>

            <p className="font-semibold text-gray-800 pt-2">Agents</p>
            <div className="space-y-2">
              <Endpoint method="POST"   path="/api/v1/agents"              description='Register a new agent. Set registration_type to "nap_hosted" with a Bearer user token for free-tier, or omit for domain-verified.' />
              <Endpoint method="GET"    path="/api/v1/agents"              description="List registered agents. Supports query params: trust_root, capability_node, limit, offset." />
              <Endpoint method="GET"    path="/api/v1/agents/:id"          description="Get a single agent by UUID. Includes trust_tier in the response." />
              <Endpoint method="PATCH"  path="/api/v1/agents/:id"          description="Update mutable fields: display_name, description, endpoint, public_key_pem, metadata. Requires owning agent token or user JWT." auth />
              <Endpoint method="POST"   path="/api/v1/agents/:id/activate" description="Activate an agent after verification. Returns X.509 cert (domain agents), NAP endorsement JWT, and agent_card_json for A2A deployment." />
              <Endpoint method="POST"   path="/api/v1/agents/:id/revoke"   description="Revoke an agent's registration." auth />
              <Endpoint method="DELETE" path="/api/v1/agents/:id"          description="Permanently delete an agent. Must be the owning agent or carry nexus:admin scope." auth />
            </div>

            <p className="font-semibold text-gray-800 pt-2">Discovery</p>
            <div className="space-y-2">
              <Endpoint method="GET" path="/api/v1/resolve"                   description="Resolve an agent URI. Query params: trust_root, capability_node, agent_id. Returns endpoint, uri, status, and cert_serial." />
              <Endpoint method="GET" path="/.well-known/agent-card.json"      description="NAP discovery card for a domain. Query param: domain=acme.com. Returns all active agents with nap:trust_tier per entry." />
            </div>

            <p className="font-semibold text-gray-800 pt-2">DNS Verification</p>
            <div className="space-y-2">
              <Endpoint method="POST" path="/api/v1/dns/challenge"           description='Start a DNS-01 challenge for a domain. Body: {"domain": "example.com"}' />
              <Endpoint method="GET"  path="/api/v1/dns/challenge/:id"       description="Poll challenge status." />
              <Endpoint method="POST" path="/api/v1/dns/challenge/:id/verify" description="Trigger the DNS TXT lookup and mark the challenge verified on success." />
            </div>

            <p className="font-semibold text-gray-800 pt-2">Trust Ledger</p>
            <div className="space-y-2">
              <Endpoint method="GET" path="/api/v1/ledger"              description="Returns total entry count and current Merkle root hash." />
              <Endpoint method="GET" path="/api/v1/ledger/verify"       description="Walks the full chain and reports whether the integrity check passes." />
              <Endpoint method="GET" path="/api/v1/ledger/entries/:idx" description="Fetch a single ledger entry by index." />
            </div>

            <p className="font-semibold text-gray-800 pt-2">Identity & OIDC</p>
            <div className="space-y-2">
              <Endpoint method="GET" path="/.well-known/openid-configuration" description="OIDC discovery document." />
              <Endpoint method="GET" path="/.well-known/jwks.json"            description="JWKS endpoint — use to verify NAP endorsement JWTs and task tokens." />
            </div>

            <p className="font-semibold text-gray-800 pt-2">Health</p>
            <div className="space-y-2">
              <Endpoint method="GET" path="/healthz" description='Returns {"status":"ok"} when the registry is up and connected to Postgres.' />
            </div>
          </Section>

          {/* Trust Ledger */}
          <Section title="Trust Ledger" {...s("trust-ledger")}>
            <p>
              Every agent registration and activation event is appended to an append-only hash chain stored in Postgres.
              Each entry contains a SHA-256 hash of the previous entry, making any tampering detectable.
            </p>
            <p>
              The ledger starts with a genesis entry at index 0 with a known constant hash. You can independently verify the entire chain at any time:
            </p>
            <Code>{`# Check chain integrity
curl -s http://localhost:8080/api/v1/ledger/verify
# {"valid": true}

# Get the current root hash and entry count
curl -s http://localhost:8080/api/v1/ledger
# {"entries": 42, "root": "a3f9bc..."}

# Fetch a specific entry
curl -s http://localhost:8080/api/v1/ledger/entries/1`}</Code>
            <p>
              Each entry records the <code className="font-mono rounded bg-gray-100 px-1">agent_uri</code>,{" "}
              <code className="font-mono rounded bg-gray-100 px-1">action</code> (registered / activated / revoked),{" "}
              <code className="font-mono rounded bg-gray-100 px-1">actor</code>, timestamp, and the hash linking it to the previous entry.
            </p>
          </Section>

          {/* Authentication */}
          <Section title="Authentication" {...s("auth")}>
            <p>The registry uses two kinds of bearer tokens:</p>
            <div className="space-y-3">
              <div className="rounded-lg border border-gray-200 bg-white p-4">
                <p className="font-semibold text-gray-900 mb-1">User JWT</p>
                <p className="text-xs text-gray-500 leading-relaxed">
                  Returned by <code className="font-mono">/auth/login</code> and <code className="font-mono">/auth/signup</code>.
                  Required for registering <code className="font-mono">nap_hosted</code> agents. Valid for 24 hours. Contains{" "}
                  <code className="font-mono">user_id</code>, <code className="font-mono">email</code>,{" "}
                  <code className="font-mono">username</code>, and <code className="font-mono">tier</code> claims.
                </p>
              </div>
              <div className="rounded-lg border border-gray-200 bg-white p-4">
                <p className="font-semibold text-gray-900 mb-1">Agent Task Token</p>
                <p className="text-xs text-gray-500 leading-relaxed">
                  Issued at activation for domain-verified agents. Required for protected operations (revoke, delete, update).
                  Valid for 1 hour by default. Contains <code className="font-mono">agent_uri</code> and{" "}
                  <code className="font-mono">scopes</code> claims.
                </p>
              </div>
            </div>
            <Code>{`# Use any bearer token on protected routes:
curl -s -X POST http://localhost:8080/api/v1/agents/<UUID>/revoke \\
  -H "Authorization: Bearer <token>"`}</Code>
            <p>
              Both token types are RS256 JWTs signed by the same Nexus CA key. The registry exposes OIDC discovery at{" "}
              <code className="font-mono rounded bg-gray-100 px-1">/.well-known/openid-configuration</code> and JWKS at{" "}
              <code className="font-mono rounded bg-gray-100 px-1">/.well-known/jwks.json</code> for verification.
              The same JWKS endpoint is used to verify{" "}
              <code className="font-mono rounded bg-gray-100 px-1">nap:endorsement</code> JWTs embedded in agent cards.
            </p>
            <div className="rounded-lg border border-blue-100 bg-blue-50 px-4 py-3 text-blue-800 text-xs">
              Task token TTL defaults to <strong>1 hour</strong>. User JWT TTL is <strong>24 hours</strong>.
              NAP Endorsement JWTs in agent cards are valid for <strong>1 year</strong> and should be refreshed at re-activation.
            </div>
          </Section>

          {/* Go SDK */}
          <Section title="Go SDK" {...s("sdk")}>
            <p>
              The <code className="font-mono rounded bg-gray-100 px-1">pkg/client</code> package provides a typed Go client for the registry API.
            </p>
            <Code>{`go get github.com/nexus-protocol/nexus/pkg/client`}</Code>
            <p className="font-medium text-gray-800">Resolve an agent</p>
            <Code>{`import "github.com/nexus-protocol/nexus/pkg/client"

c, err := client.New("https://registry.nexusagentprotocol.com")
if err != nil {
    log.Fatal(err)
}

result, err := c.Resolve(ctx, "acme.com", "finance", "agent_7x2v9q")
if err != nil {
    log.Fatal(err)
}

fmt.Println(result.Endpoint) // https://agents.acme.com/tax`}</Code>
            <p className="font-medium text-gray-800">mTLS client (agent-to-agent)</p>
            <Code>{`c, err := client.New("https://registry.nexusagentprotocol.com",
    client.WithMTLS(certPEM, keyPEM, caPEM),
)

// Or with a Bearer token
c, err := client.New("https://registry.nexusagentprotocol.com",
    client.WithBearerToken(taskToken),
)`}</Code>
          </Section>

        </div>
      </div>
    </div>
  );
}
