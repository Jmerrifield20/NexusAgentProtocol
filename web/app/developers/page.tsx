import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Developer Docs — Nexus Agentic Protocol",
};

function Section({ id, title, children }: { id: string; title: string; children: React.ReactNode }) {
  return (
    <section id={id} className="scroll-mt-24">
      <h2 className="mb-4 text-xl font-bold text-gray-900 border-b border-gray-200 pb-2">{title}</h2>
      <div className="space-y-4 text-sm text-gray-700 leading-relaxed">{children}</div>
    </section>
  );
}

function Code({ children }: { children: string }) {
  return (
    <pre className="overflow-x-auto rounded-lg bg-gray-900 p-5 text-sm font-mono text-gray-100 leading-relaxed">
      {children}
    </pre>
  );
}

function Endpoint({ method, path, description, auth }: { method: string; path: string; description: string; auth?: boolean }) {
  const methodColor: Record<string, string> = {
    GET: "bg-blue-100 text-blue-700",
    POST: "bg-green-100 text-green-700",
    PATCH: "bg-yellow-100 text-yellow-700",
    DELETE: "bg-red-100 text-red-700",
  };
  return (
    <div className="rounded-lg border border-gray-200 bg-white p-4">
      <div className="flex items-center gap-3 mb-2">
        <span className={`rounded px-2 py-0.5 text-xs font-bold font-mono ${methodColor[method] ?? "bg-gray-100 text-gray-700"}`}>
          {method}
        </span>
        <code className="text-sm font-mono text-gray-800">{path}</code>
        {auth && (
          <span className="ml-auto rounded-full bg-amber-100 px-2 py-0.5 text-xs text-amber-700 font-medium">
            requires auth
          </span>
        )}
      </div>
      <p className="text-sm text-gray-500">{description}</p>
    </div>
  );
}

const NAV = [
  { id: "overview", label: "Overview" },
  { id: "concepts", label: "Core Concepts" },
  { id: "quickstart", label: "Quick Start" },
  { id: "agent-lifecycle", label: "Agent Lifecycle" },
  { id: "dns-verification", label: "DNS-01 Verification" },
  { id: "api-reference", label: "API Reference" },
  { id: "trust-ledger", label: "Trust Ledger" },
  { id: "auth", label: "Authentication" },
  { id: "sdk", label: "Go SDK" },
];

export default function DeveloperPage() {
  return (
    <div className="mx-auto max-w-7xl">
      <div className="flex gap-10">

        {/* Sidebar nav */}
        <aside className="hidden lg:block w-52 shrink-0">
          <div className="sticky top-8">
            <p className="mb-3 text-xs font-semibold uppercase tracking-widest text-gray-400">On this page</p>
            <nav className="space-y-1">
              {NAV.map((item) => (
                <a
                  key={item.id}
                  href={`#${item.id}`}
                  className="block rounded-md px-3 py-1.5 text-sm text-gray-600 hover:bg-gray-100 hover:text-nexus-500"
                >
                  {item.label}
                </a>
              ))}
            </nav>
          </div>
        </aside>

        {/* Main content */}
        <div className="min-w-0 flex-1 space-y-12">

          <div>
            <h1 className="text-4xl font-extrabold tracking-tight text-gray-900">Developer Docs</h1>
            <p className="mt-3 text-lg text-gray-500">
              Everything you need to register agents, verify domains, resolve URIs, and integrate with the Nexus Agentic Protocol.
            </p>
          </div>

          {/* Overview */}
          <Section id="overview" title="Overview">
            <p>
              The Nexus Agentic Protocol gives AI agents a permanent, verifiable address on the internet using the{" "}
              <code className="rounded bg-gray-100 px-1.5 py-0.5 font-mono text-nexus-500">agent://</code> URI scheme.
              Think of it as DNS for agents — a neutral registry that maps logical addresses to live endpoints, with cryptographic proof of ownership baked in.
            </p>
            <p>
              The registry exposes a JSON REST API on port <strong>8080</strong> (HTTP) and port <strong>8443</strong> (mTLS).
              All endpoints live under <code className="rounded bg-gray-100 px-1 py-0.5 font-mono">/api/v1/</code>.
            </p>
            <div className="rounded-lg border border-blue-100 bg-blue-50 px-4 py-3 text-blue-800">
              <strong>Base URL (local dev):</strong>{" "}
              <code className="font-mono">http://localhost:8080/api/v1</code>
            </div>
          </Section>

          {/* Core Concepts */}
          <Section id="concepts" title="Core Concepts">
            <div className="grid gap-4 sm:grid-cols-2">
              {[
                {
                  term: "Trust Root",
                  def: "The domain that anchors the agent's identity — e.g. acme.com. This is the same as the owner domain and is verified via DNS-01 before activation.",
                },
                {
                  term: "Capability Node",
                  def: "A short label describing what the agent does — e.g. finance or support. Appears as the second segment of the agent:// URI.",
                },
                {
                  term: "Agent ID",
                  def: "A short unique identifier generated at registration — e.g. agent_7x2v9q. Stable even if the endpoint URL changes.",
                },
                {
                  term: "Endpoint",
                  def: "The physical HTTPS/gRPC URL where the agent currently listens. This is what resolve returns — callers use this to make requests.",
                },
                {
                  term: "DNS-01 Challenge",
                  def: "The domain ownership proof mechanism. The registry issues a TXT record value; you publish it under _nexus-challenge.<domain> and call verify.",
                },
                {
                  term: "Identity Certificate",
                  def: "An X.509 cert signed by the Nexus CA, issued at activation. The private key is returned once and never stored by the registry.",
                },
                {
                  term: "Trust Ledger",
                  def: "An append-only hash chain recording every registration and activation event. Anyone can independently verify the full history of any agent.",
                },
                {
                  term: "Task Token",
                  def: "A short-lived RS256 JWT issued at activation. Required for protected operations like revoke and delete.",
                },
              ].map((c) => (
                <div key={c.term} className="rounded-lg border border-gray-200 bg-white p-4">
                  <p className="font-semibold text-gray-900 mb-1">{c.term}</p>
                  <p className="text-xs text-gray-500 leading-relaxed">{c.def}</p>
                </div>
              ))}
            </div>
          </Section>

          {/* Quick Start */}
          <Section id="quickstart" title="Quick Start">
            <p>Register and activate an agent in three curl commands.</p>

            <p className="font-medium text-gray-800">1 — Register</p>
            <Code>{`curl -s -X POST http://localhost:8080/api/v1/agents \\
  -H "Content-Type: application/json" \\
  -d '{
    "capability_node": "finance",
    "display_name":    "Acme Tax Agent",
    "description":     "Handles tax filing queries",
    "endpoint":        "https://agents.acme.com/tax",
    "owner_domain":    "acme.com"
  }'`}</Code>
            <p className="text-xs text-gray-400">
              Returns a JSON object containing <code className="font-mono">id</code> (UUID) and <code className="font-mono">agent_id</code> (short ID). Save both.
            </p>

            <p className="font-medium text-gray-800">2 — Activate (local dev — DNS verification bypassed)</p>
            <Code>{`curl -s -X POST http://localhost:8080/api/v1/agents/<UUID>/activate`}</Code>
            <p className="text-xs text-gray-400">
              Returns the signed X.509 certificate, CA cert, and private key. <strong>The private key is shown once — store it securely.</strong>
            </p>

            <p className="font-medium text-gray-800">3 — Resolve</p>
            <Code>{`curl -s "http://localhost:8080/api/v1/resolve?trust_root=acme.com&capability_node=finance&agent_id=agent_7x2v9q"`}</Code>
            <p className="text-xs text-gray-400">
              Returns <code className="font-mono">endpoint</code>, <code className="font-mono">uri</code>, and <code className="font-mono">status</code>.
            </p>
          </Section>

          {/* Agent Lifecycle */}
          <Section id="agent-lifecycle" title="Agent Lifecycle">
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
                { state: "pending", color: "bg-yellow-100 text-yellow-700", desc: "Created by POST /agents. Domain ownership has not yet been verified. The agent is not resolvable." },
                { state: "active", color: "bg-green-100 text-green-700", desc: "Activated after DNS-01 verification passes. An X.509 cert is issued and the agent is resolvable." },
                { state: "revoked", color: "bg-red-100 text-red-700", desc: "Manually revoked via POST /agents/:id/revoke. The agent remains in the registry but resolve returns an error." },
                { state: "expired", color: "bg-gray-100 text-gray-700", desc: "The agent's certificate has passed its validity window. Re-activation is required." },
              ].map((s) => (
                <div key={s.state} className="flex items-start gap-3 rounded-lg border border-gray-200 bg-white p-4">
                  <span className={`mt-0.5 shrink-0 rounded-full px-2.5 py-0.5 text-xs font-semibold ${s.color}`}>{s.state}</span>
                  <p className="text-sm text-gray-600">{s.desc}</p>
                </div>
              ))}
            </div>
          </Section>

          {/* DNS-01 Verification */}
          <Section id="dns-verification" title="DNS-01 Verification">
            <p>
              Before an agent can be activated in production, you must prove you control the <code className="font-mono rounded bg-gray-100 px-1">owner_domain</code>.
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
                <p>Add a DNS TXT record at the host shown in <code className="font-mono rounded bg-gray-100 px-1">txt_host</code> with the value from <code className="font-mono rounded bg-gray-100 px-1">txt_record</code>. Allow time for DNS propagation (typically 1–5 minutes).</p>
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

          {/* API Reference */}
          <Section id="api-reference" title="API Reference">
            <p className="font-semibold text-gray-800">Agents</p>
            <div className="space-y-2">
              <Endpoint method="POST" path="/api/v1/agents" description="Register a new agent. Returns the agent object including its UUID and generated agent_id." />
              <Endpoint method="GET" path="/api/v1/agents" description="List registered agents. Supports query params: trust_root, capability_node, limit, offset." />
              <Endpoint method="GET" path="/api/v1/agents/:id" description="Get a single agent by UUID." />
              <Endpoint method="PATCH" path="/api/v1/agents/:id" description="Update mutable fields: display_name, description, endpoint, public_key_pem, metadata." />
              <Endpoint method="POST" path="/api/v1/agents/:id/activate" description="Activate an agent after DNS-01 verification. Issues X.509 cert and private key." />
              <Endpoint method="POST" path="/api/v1/agents/:id/revoke" description="Revoke an agent's registration." auth />
              <Endpoint method="DELETE" path="/api/v1/agents/:id" description="Permanently delete an agent. Must be the owning agent or carry nexus:admin scope." auth />
            </div>

            <p className="font-semibold text-gray-800 pt-2">Resolution</p>
            <div className="space-y-2">
              <Endpoint method="GET" path="/api/v1/resolve" description="Resolve an agent URI. Query params: trust_root, capability_node, agent_id. Returns endpoint and status." />
            </div>

            <p className="font-semibold text-gray-800 pt-2">DNS Verification</p>
            <div className="space-y-2">
              <Endpoint method="POST" path="/api/v1/dns/challenge" description='Start a DNS-01 challenge for a domain. Body: {"domain": "example.com"}' />
              <Endpoint method="GET" path="/api/v1/dns/challenge/:id" description="Poll challenge status." />
              <Endpoint method="POST" path="/api/v1/dns/challenge/:id/verify" description="Trigger the DNS TXT lookup and mark the challenge verified on success." />
            </div>

            <p className="font-semibold text-gray-800 pt-2">Trust Ledger</p>
            <div className="space-y-2">
              <Endpoint method="GET" path="/api/v1/ledger" description="Returns total entry count and current Merkle root hash." />
              <Endpoint method="GET" path="/api/v1/ledger/verify" description="Walks the full chain and reports whether the integrity check passes." />
              <Endpoint method="GET" path="/api/v1/ledger/entries/:idx" description="Fetch a single ledger entry by index." />
            </div>

            <p className="font-semibold text-gray-800 pt-2">Health</p>
            <div className="space-y-2">
              <Endpoint method="GET" path="/healthz" description='Returns {"status":"ok"} when the registry is up and connected to Postgres.' />
            </div>
          </Section>

          {/* Trust Ledger */}
          <Section id="trust-ledger" title="Trust Ledger">
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
              Each entry records the <code className="font-mono rounded bg-gray-100 px-1">agent_uri</code>, <code className="font-mono rounded bg-gray-100 px-1">action</code> (registered / activated / revoked), <code className="font-mono rounded bg-gray-100 px-1">actor</code>, timestamp, and the hash linking it to the previous entry.
            </p>
          </Section>

          {/* Authentication */}
          <Section id="auth" title="Authentication">
            <p>
              Most endpoints are open — registration and resolution require no credentials. Protected operations (revoke, delete) require a <strong>Bearer JWT Task Token</strong> issued at activation.
            </p>
            <Code>{`# The activate response includes a task token:
{
  "status": "activated",
  "agent":  { ... },
  "certificate": { "pem": "...", "expires_at": "..." },
  "private_key_pem": "-----BEGIN RSA PRIVATE KEY-----\n...",
  "warning": "Store private_key_pem securely. It will not be shown again."
}

# Use the token on protected routes:
curl -s -X POST http://localhost:8080/api/v1/agents/<UUID>/revoke \\
  -H "Authorization: Bearer <task_token>"`}</Code>
            <p>
              Tokens are RS256 JWTs signed by the Nexus CA. They carry the agent's URI and scope claims. The registry exposes OIDC discovery at{" "}
              <code className="font-mono rounded bg-gray-100 px-1">/.well-known/openid-configuration</code> and JWKS at{" "}
              <code className="font-mono rounded bg-gray-100 px-1">/.well-known/jwks.json</code> for token verification.
            </p>
            <div className="rounded-lg border border-blue-100 bg-blue-50 px-4 py-3 text-blue-800 text-xs">
              In local dev the registry runs with <code className="font-mono">tokens != nil</code> so auth is enforced. The token TTL defaults to <strong>1 hour</strong>.
            </div>
          </Section>

          {/* Go SDK */}
          <Section id="sdk" title="Go SDK">
            <p>
              The <code className="font-mono rounded bg-gray-100 px-1">pkg/client</code> package provides a typed Go client for the registry API.
            </p>
            <Code>{`go get github.com/nexus-protocol/nexus/pkg/client`}</Code>

            <p className="font-medium text-gray-800">Resolve an agent</p>
            <Code>{`import "github.com/nexus-protocol/nexus/pkg/client"

c, err := client.New("https://registry.nexus.io")
if err != nil {
    log.Fatal(err)
}

result, err := c.Resolve(ctx, "acme.com", "finance", "agent_7x2v9q")
if err != nil {
    log.Fatal(err)
}

fmt.Println(result.Endpoint) // https://agents.acme.com/tax`}</Code>

            <p className="font-medium text-gray-800">mTLS client (agent-to-agent)</p>
            <Code>{`c, err := client.New("https://registry.nexus.io",
    client.WithMTLS(certPEM, keyPEM, caPEM),
)

// Or with a Bearer token
c, err := client.New("https://registry.nexus.io",
    client.WithBearerToken(taskToken),
)`}</Code>
          </Section>

        </div>
      </div>
    </div>
  );
}
