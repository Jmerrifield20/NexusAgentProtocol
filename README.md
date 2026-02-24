# Nexus Agent Protocol (NAP)

> A permanent, verifiable address for every AI agent on the internet.

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go 1.22+](https://img.shields.io/badge/Go-1.22+-00ADD8.svg)](https://golang.org)

NAP gives AI agents a stable identity that survives infrastructure changes. Register once, get an `agent://` URI that any NAP-aware client can resolve to your current endpoint — with cryptographic proof of who you are.

```
agent://acme.com/finance/agent_7x2v9q
         └── your domain  └── what you do  └── unique ID
```

---

## Why NAP?

Today, agents are addressed by their hosting URL. When you move providers, change domains, or scale horizontally, every caller has to update their config. NAP separates **identity** from **location**.

- **Permanent address** — your `agent://` URI never changes, even if your endpoint does.
- **Cryptographic ownership** — domain-verified agents prove ownership via DNS-01. Callers know they're talking to the real you.
- **Discoverable** — agents are listed in a searchable directory, categorised by capability.
- **A2A compatible** — every activated agent gets an [Agent2Agent](https://google.github.io/A2A/)-spec card with declared skills and a NAP endorsement JWT, deployable at `/.well-known/agent.json`.
- **MCP ready** — declare your tools at registration and get a [Model Context Protocol](https://modelcontextprotocol.io/) manifest served at a stable URL.
- **Safe** — registrations are automatically scored for threat patterns. Dangerous agents are rejected before they enter the directory.
- **Auditable** — every lifecycle event (register, activate, revoke, suspend, deprecate) is appended to a Merkle-chain trust ledger anyone can verify.
- **Continuously monitored** — active agents are health-probed on a schedule. Degraded endpoints are flagged automatically.
- **Observable** — Prometheus metrics at `/metrics` give you request rates, latency histograms, health-check results, and agent counts by status.
- **Webhook-driven** — subscribe to lifecycle events (`agent.registered`, `agent.revoked`, `agent.health_degraded`, etc.) and get HMAC-signed POST notifications in real time.

---

## Register Your Agent

### Hosted (no infrastructure required)

Sign up at [nexusagentprotocol.com](https://registry.nexusagentprotocol.com), verify your email, and register under the `nap` namespace:

```bash
# 1 — Sign up
curl -s -X POST https://registry.nexusagentprotocol.com/api/v1/auth/signup \
  -H "Content-Type: application/json" \
  -d '{"email":"you@example.com","password":"hunter2","display_name":"Alice"}'

# 2 — Log in (after verifying your email)
TOKEN=$(curl -s -X POST https://registry.nexusagentprotocol.com/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"you@example.com","password":"hunter2"}' | jq -r .token)

# 3 — Register
curl -s -X POST https://registry.nexusagentprotocol.com/api/v1/agents \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "registration_type": "nap_hosted",
    "display_name":      "My Assistant",
    "capability":        "assistant",
    "endpoint":          "https://my-agent.fly.dev",
    "skills": [
      {"id":"answer","name":"Answer Questions","description":"Answers product questions","tags":["assistant"]}
    ]
  }'

# Returns: agent://nap/assistant/agent_xxx
```

### Domain-verified (your domain, your trust root)

Own a domain? Prove it with a DNS TXT record and your agent gets a verified `agent://yourdomain.com/...` address — the highest trust tier:

```bash
# 1 — Register
curl -s -X POST https://registry.nexusagentprotocol.com/api/v1/agents \
  -H "Content-Type: application/json" \
  -d '{
    "display_name":  "Acme Tax Agent",
    "capability":    "finance>accounting",
    "endpoint":      "https://agents.acme.com/tax",
    "owner_domain":  "acme.com"
  }'

# 2 — Start a DNS-01 challenge
curl -s -X POST https://registry.nexusagentprotocol.com/api/v1/dns/challenge \
  -d '{"domain":"acme.com"}'
# Publish the returned TXT record under _nexus-challenge.acme.com, then:

# 3 — Verify
curl -s -X POST https://registry.nexusagentprotocol.com/api/v1/dns/challenge/<ID>/verify

# 4 — Activate (receive X.509 cert + A2A agent card + MCP manifest)
curl -s -X POST https://registry.nexusagentprotocol.com/api/v1/agents/<UUID>/activate

# Returns: agent://acme.com/finance/agent_xxx  (Trusted tier)
```

---

## What Activation Gives You

After activation your agent receives:

- **A2A agent card** — deploy at `/.well-known/agent.json` so any A2A client can discover and trust your agent. Includes your declared skills and a NAP endorsement JWT signed by the registry CA.
- **MCP manifest** — if you declared `mcp_tools` at registration, a manifest is served at `/api/v1/agents/:id/mcp-manifest.json` and returned in the activation response. Point Claude Desktop or any MCP client at it.
- **X.509 certificate** (domain-verified only) — signed by the Nexus CA. Private key returned once, never stored.
- **NAP endorsement JWT** — RS256 token embedded in your agent card. Callers verify it against the registry's JWKS endpoint.

```bash
# Fetch your A2A card any time
curl https://registry.nexusagentprotocol.com/api/v1/agents/<UUID>/agent.json

# Fetch your MCP manifest
curl https://registry.nexusagentprotocol.com/api/v1/agents/<UUID>/mcp-manifest.json
```

---

## Resolve Any Agent

Any NAP client can resolve an `agent://` URI to its current endpoint:

```bash
curl "https://registry.nexusagentprotocol.com/api/v1/resolve?\
trust_root=acme.com&capability_node=finance&agent_id=agent_7x2v9q"
# {"endpoint":"https://agents.acme.com/tax","status":"active","cert_serial":"3f9a..."}
```

### Go SDK

```bash
go get github.com/jmerrifield20/NexusAgentProtocol/pkg/client
```

```go
c, _ := client.New("https://registry.nexusagentprotocol.com")

// Resolve any agent URI
result, err := c.Resolve(ctx, "agent://acme.com/finance/agent_7x2v9q")
fmt.Println(result.Endpoint) // https://agents.acme.com/tax

// Fetch the A2A card
card, err := c.GetAgentCard(ctx, agentID)

// Fetch the MCP manifest
manifest, err := c.GetMCPManifest(ctx, agentID)
```

---

## Trust Tiers

Every agent carries a computed trust tier so callers know how much verification has been done:

| Tier | How earned |
|------|-----------|
| **Trusted** | Domain-verified (DNS-01) + active + mTLS certificate issued |
| **Verified** | Domain-verified + active, no cert |
| **Basic** | NAP-hosted + email-verified + active |
| **Unverified** | Pending, revoked, suspended, or expired |

Gate your integrations on tier. Only accept `agent://` callers at the trust level your use case requires.

---

## Developer Portal

The registry ships with a Next.js web portal for browsing the agent directory, managing your registrations, and reading the full API docs:

- **Agent directory** — search by capability (e.g. `finance>accounting`) or by org domain.
- **Per-agent pages** — skills, integration links (A2A card, MCP manifest), and live health status.
- **Account management** — register, activate, suspend, restore, deprecate, and revoke agents via the UI.
- **API docs** — full reference at `/developers`.

---

## Run Your Own Registry

NAP is designed to be federated. **This repository is the full registry implementation** — clone it, deploy it under your own domain, and your instance joins the same `agent://` address space.

```bash
git clone https://github.com/jmerrifield20/NexusAgentProtocol.git
cd NexusAgentProtocol
make dev          # starts Postgres + registry + web portal locally
curl http://localhost:8080/healthz
# {"status":"ok"}
```

Agents registered on your instance get addresses rooted at your domain (`agent://yourdomain.com/...`). Any NAP client resolves them by querying your registry directly — no cross-registry coordination, no fees, no permission needed. The URI is self-routing: the trust root is the domain, and the domain is where to resolve.

See the [self-hosting guide](docs/self-hosting.md) for production deployment, TLS configuration, and SMTP setup.

---

## Project Structure

```
NexusAgentProtocol/
├── cmd/registry/      # Registry server binary (HTTP :8080, mTLS :8443)
├── internal/
│   ├── registry/      # Handler → Service → Repository → Model
│   ├── identity/      # X.509 CA, cert issuance, mTLS, OIDC, JWT
│   ├── trustledger/   # Merkle-chain audit log (Postgres-backed)
│   ├── dns/           # DNS-01 challenge verification
│   ├── threat/        # Registration threat scoring
│   ├── health/        # Continuous endpoint health checker
│   ├── webhooks/      # Webhook subscriptions and event dispatch
│   ├── federation/    # Cross-registry resolution and CA hierarchy
│   └── users/         # Accounts, email verification, OAuth
├── pkg/
│   ├── client/        # Go SDK
│   ├── agentcard/     # A2A agent card types
│   ├── mcpmanifest/   # MCP manifest types
│   └── uri/           # agent:// URI parsing
├── migrations/        # SQL migrations (golang-migrate)
├── web/               # Next.js developer portal
└── integrations/
    └── openclaw/      # TypeScript SDK (@openclaw/nap)
```

---

## Privacy

The registry is a **pure phonebook**. It maps `agent://` URIs to endpoints — nothing more.

- Resolve queries are not logged. The registry does not know who is calling whom.
- Agent-to-agent traffic never passes through the registry. After the lookup, agents talk directly.
- The only data retained is registration metadata, Trust Ledger lifecycle events, abuse reports, and webhook subscription URLs.

---

## Contributing

Bug fixes, new features, and documentation improvements are all welcome. See [CONTRIBUTING.md](CONTRIBUTING.md).

## Security

Report vulnerabilities to security@nexusagentprotocol.com rather than opening a public issue.

## License

Apache 2.0 — see [LICENSE](LICENSE).
