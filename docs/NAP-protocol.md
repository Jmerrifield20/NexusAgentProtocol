# Nexus Agent Protocol (NAP)

> A open standard for agent identity, discovery, and authenticated agent-to-agent communication over the internet.

---

## What is NAP?

NAP is a protocol that gives AI agents a stable, verifiable identity on the internet — similar to how domains identify websites, but for agents. It answers three questions that no existing protocol does together:

1. **Who is this agent?** — verified identity (DNS-01 domain ownership for org agents; email verification for personal agents), X.509 certificate issued by the Nexus CA
2. **Where does it live?** — resolvable `agent://` URI → HTTPS endpoint
3. **Is it authorised to call me?** — mTLS + scoped JWT Task Tokens

---

## The Landscape (February 2026)

| Protocol | Discovery URL | Live deployments |
|----------|--------------|-----------------|
| **Google A2A** | `/.well-known/agent.json` | 0 (announced, not deployed) |
| **Anthropic MCP** | `/.well-known/mcp.json` | ~1 (Notion) |
| **OpenAI Plugins** (deprecated) | `/.well-known/ai-plugin.json` | ~2 (Taskade, Zapier — legacy) |
| **NAP** | `/.well-known/agent-card.json` | first-mover |

NAP is the only protocol that combines a central registry, DNS-01 domain ownership verification, and mutual TLS identity in a single coherent design.

---

## Core Concepts

### 1. The `agent://` URI

Every NAP agent has a permanent, human-readable address:

```
agent://acme.com/finance/reconcile-invoices/agent_7x2v9qaabbccdd
         ───────  ───────  ─────────────────  ──────────────────
         trust    category  primary skill       unique agent ID
         root
```

| Part | Meaning |
|------|---------|
| `acme.com` | Trust root — verified domain (or `nap` for free-hosted agents) |
| `finance` | Category — top-level capability namespace |
| `reconcile-invoices` | Primary skill — slugified skill ID, derived at registration |
| `agent_7x2v9q…` | Agent ID — opaque unique identifier assigned at registration |

The URI is permanent. The underlying endpoint (IP/URL) can change; the URI does not.

When a meaningful sub-skill cannot be derived (top-level capability only), the 3-segment form is used:

```
agent://nap/assistant/agent_abc123
```

See the [NAP URI Standard](./spec/nap-uri-standard.md) for the full derivation rules.

### 2. The Registry

The Nexus Registry is the central NIC (Name and Identity Coordinator) for NAP. It:

- Verifies domain ownership via **DNS-01 challenge** (same mechanism as Let's Encrypt)
- Issues **X.509 certificates** to registered agents (4096-bit RSA, signed by the Nexus CA)
- Resolves `agent://` URIs to live HTTPS endpoints
- Maintains a **Trust Ledger** — an append-only hash chain of all registration events

### 3. DNS-01 Domain Verification

Before an agent can be registered, the operator must prove they control the domain:

```
Registry generates:  _nexus-agent-challenge.example.com  TXT  "nexus-agent-challenge=TOKEN"
Operator publishes:  the TXT record in their DNS
Registry verifies:   DNS lookup confirms the record
```

This is the same mechanism used by Let's Encrypt for TLS certificates.

### 4. The Agent Card

Every NAP domain **optionally** publishes a discovery file at:

```
https://example.com/.well-known/agent-card.json
```

This lists all active agents for that domain. The Nexus registry also serves this on behalf of registered domains at:

```
https://registry.nexusagentprotocol.com/.well-known/agent-card.json?domain=example.com
```

**Example agent-card.json:**

```json
{
  "schema_version": "1.0",
  "domain": "example.com",
  "trust_root": "nexusagentprotocol.com",
  "updated_at": "2026-02-20T12:00:00Z",
  "agents": [
    {
      "uri": "agent://acme.com/ecommerce/retail/agent_7x2v9q",
      "display_name": "Acme Store",
      "description": "Handles orders, inventory, and invoicing for Acme.",
      "endpoint": "https://agents.acme.com",
      "capability_node": "ecommerce>retail",
      "protocols": ["https"],
      "status": "active"
    }
  ]
}
```

### 5. Authentication — mTLS + Task Tokens

NAP uses a two-layer authentication model:

```
Agent A                         Nexus Registry                     Agent B
  │                                   │                               │
  │── POST /token (mTLS cert) ───────>│                               │
  │<─ JWT Task Token ─────────────────│                               │
  │                                   │                               │
  │── GET /resolve?uri=agent://... ──>│                               │
  │<─ { endpoint: "https://..." } ────│                               │
  │                                   │                               │
  │── POST https://agents.b.com/v1/order                              │
  │   Authorization: Bearer <JWT>  ──────────────────────────────────>│
  │<─ 200 OK ─────────────────────────────────────────────────────────│
```

- **mTLS cert** — proves to the registry that you are who you say you are
- **JWT Task Token** — a scoped, short-lived token (RS256) the registry issues after verifying your cert
- **Bearer token on agent call** — the receiving agent validates the JWT against the registry's JWKS endpoint

---

## Integration Guide for Your Bot

### Step 1 — Obtain the Go SDK

```go
import "github.com/jmerrifield20/NexusAgentProtocol/pkg/client"
```

### Step 2 — Register Your Bot (one-time)

Use the `nap` CLI:

```bash
nap claim yourdomain.com \
  --registry https://registry.nexusagentprotocol.com \
  --capability "assistant/general" \
  --name "My Bot" \
  --endpoint https://yourdomain.com/agent
```

This will:
1. Ask you to add a DNS TXT record to prove you own `yourdomain.com`
2. Register the agent in the Nexus registry
3. Issue an X.509 certificate
4. Write cert files to `~/.nap/certs/yourdomain.com/`:
   - `cert.pem` — your agent's certificate
   - `key.pem` — private key (keep secret, mode 0600)
   - `ca.pem` — the Nexus CA certificate

### Step 3 — Connect Your Bot to NAP

```go
package main

import (
    "context"
    "log"
    "net/http"
    "os"

    "github.com/jmerrifield20/NexusAgentProtocol/pkg/client"
)

func main() {
    // Load certs written by 'nap claim'
    c, err := client.NewFromCertDir(
        "https://registry.nexusagentprotocol.com",
        os.ExpandEnv("$HOME/.nap/certs/yourdomain.com"),
    )
    if err != nil {
        log.Fatal(err)
    }

    ctx := context.Background()

    // Call any other NAP agent — resolve + auth + call in one step
    var reply map[string]any
    err = c.CallAgent(ctx,
        "agent://acme.com/finance/billing/agent_7x2v9q",
        http.MethodPost, "/v1/invoice",
        map[string]any{"amount": 100, "currency": "USD"},
        &reply,
    )
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("response: %+v", reply)
}
```

### Step 4 — Accept Incoming NAP Calls

Add the NAP middleware to your existing HTTP server. It validates the incoming JWT against the Nexus JWKS endpoint:

```go
import (
    "github.com/gin-gonic/gin"
    "github.com/jmerrifield20/NexusAgentProtocol/internal/identity"
)

router := gin.New()

// Protect your agent endpoint — only valid NAP JWT holders can call it
router.POST("/agent/*path", identity.RequireToken(oidcIssuerURL), yourHandler)
```

The JWKS endpoint is `https://registry.nexusagentprotocol.com/.well-known/jwks.json`.

---

## API Reference (Registry)

All endpoints are on the Nexus registry at `https://registry.nexusagentprotocol.com`.

### Agent Discovery

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/.well-known/agent-card.json?domain=X` | None | List all active agents for a domain |
| `GET` | `/api/v1/agents/:id` | None | Get agent details by UUID |
| `GET` | `/api/v1/agents?trust_root=X&capability_node=Y` | None | Filter agents by org / capability |
| `GET` | `/api/v1/agents?skill=reconcile-invoices` | None | Find agents by declared skill ID (indexed) |
| `GET` | `/api/v1/agents?tool=parse_invoice` | None | Find agents by MCP tool name (indexed) |
| `GET` | `/api/v1/agents?q=billing` | None | Full-text search across name, description, tags |

### Agent Lifecycle

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/v1/dns/challenge` | None | Start DNS-01 domain verification |
| `POST` | `/api/v1/dns/challenge/:id/verify` | None | Verify the TXT record |
| `POST` | `/api/v1/agents` | DNS verified | Register a new agent |
| `POST` | `/api/v1/agents/:id/activate` | mTLS | Issue X.509 cert; returns cert + private key |
| `DELETE` | `/api/v1/agents/:id` | mTLS | Revoke agent |
| `POST` | `/api/v1/agents/:id/suspend` | Agent/User | Suspend agent (reversible; blocks resolution) |
| `POST` | `/api/v1/agents/:id/restore` | Agent/User | Restore a suspended agent to active |
| `POST` | `/api/v1/agents/:id/deprecate` | Agent/User | Mark agent as deprecated with optional sunset date |

### Revocation & Trust

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/v1/crl` | None | Certificate Revocation List (revoked cert serials) |
| `POST` | `/api/v1/agents/:id/report-abuse` | User | Report an agent for abuse (max 3 open per user) |
| `GET` | `/api/v1/admin/abuse-reports` | Admin | List abuse reports (filterable by status) |
| `PATCH` | `/api/v1/admin/abuse-reports/:id` | Admin | Resolve or dismiss an abuse report |

### Resolution

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/v1/resolve?uri=agent://…` | None | Resolve URI → endpoint |
| `POST` | `/api/v1/resolve/batch` | None | Resolve up to 100 URIs in one request |
| `POST` | `/api/v1/token` | mTLS | Exchange cert for JWT Task Token |

### Webhooks

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/v1/webhooks` | User | Subscribe to lifecycle events |
| `GET` | `/api/v1/webhooks` | User | List your webhook subscriptions |
| `DELETE` | `/api/v1/webhooks/:id` | User | Delete a webhook subscription |

### Observability

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/metrics` | None | Prometheus metrics (request rates, latency, agent counts) |

### Trust Ledger

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/v1/ledger` | None | Ledger root hash + length |
| `GET` | `/api/v1/ledger/verify` | None | Verify ledger integrity |
| `GET` | `/api/v1/ledger/entries/:idx` | None | Get a specific ledger entry |

### OIDC / JWKS

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/.well-known/openid-configuration` | OIDC discovery document |
| `GET` | `/.well-known/jwks.json` | Public keys for JWT verification |

---

## SDK Quick Reference

```go
// Connect (unauthenticated — for resolution only)
c, err := client.New("https://registry.nexusagentprotocol.com")

// Connect (authenticated — load certs from disk)
c, err := client.NewFromCertDir(registryURL, "~/.nap/certs/yourdomain.com")

// Resolve a URI to an endpoint
result, err := c.Resolve(ctx, "agent://acme.com/finance/billing/agent_7x2v9q")
// result.Endpoint → "https://billing.acme.com"

// Batch resolve (up to 100 URIs)
results, err := c.ResolveBatch(ctx, []string{
    "agent://acme.com/finance/billing/agent_1",
    "agent://acme.com/legal/contracts/agent_2",
})

// Call another agent (resolve + auth + HTTP in one step)
err = c.CallAgent(ctx, uri, method, path, reqBody, &respBody)

// DNS challenge flow (for scripted registration)
challenge, err := c.StartDNSChallenge(ctx, "yourdomain.com")
// publish challenge.TXTHost / challenge.TXTRecord in your DNS
err = c.VerifyDNSChallenge(ctx, challenge.ID)  // returns ErrVerificationPending if not ready

// Register + activate
agent, err := c.RegisterAgent(ctx, client.RegisterAgentRequest{...})
certs, err := c.ActivateAgent(ctx, agent.ID)
// certs.PrivateKeyPEM is delivered once and never stored by the registry

// List agents by capability
agents, err := c.ListAgents(ctx, "acme.com", "finance")

// Lifecycle management
err = c.RevokeAgent(ctx, agentID, "compromised credentials")  // with reason
err = c.SuspendAgent(ctx, agentID)                             // reversible
err = c.RestoreAgent(ctx, agentID)                             // undo suspend

// Certificate Revocation List
crl, err := c.GetCRL(ctx)
// crl.Entries → [{CertSerial, Reason, RevokedAt}]
```

---

## MCP Bridge (Claude Desktop / Claude API)

If your bot is Claude-based, you can expose NAP as MCP tools without writing any code:

```json
// ~/.claude/claude_desktop_config.json
{
  "mcpServers": {
    "nap": {
      "command": "/path/to/nap-mcp-bridge",
      "args": [
        "--registry", "https://registry.nexusagentprotocol.com",
        "--cert-dir", "/Users/you/.nap/certs/yourdomain.com"
      ]
    }
  }
}
```

This gives Claude four tools:

| Tool | What it does |
|------|-------------|
| `resolve_agent` | Translate an `agent://` URI to its live HTTPS endpoint |
| `list_agents` | Search the registry by capability or trust root |
| `fetch_agent_card` | Read any domain's `/.well-known/agent-card.json` |
| `call_agent` | Resolve + authenticate + call an agent in one step |

---

## Capability Node Taxonomy

Capability nodes are hierarchical paths using `>` as separator (up to 3 levels).
Use the most specific node that describes your agent — the last segment of your
capability path automatically becomes your agent's `primary_skill` URI segment.

```
assistant>
  general          — agent://nap/assistant/general/agent_xxx
  coding           — agent://nap/assistant/coding/agent_xxx
  research         — agent://nap/assistant/research/agent_xxx

finance>
  billing          — agent://nap/finance/billing/agent_xxx
  accounting>
    reconciliation — agent://nap/finance/reconciliation/agent_xxx
  tax              — agent://nap/finance/tax/agent_xxx

ecommerce>
  retail           — agent://nap/ecommerce/retail/agent_xxx
  orders           — agent://nap/ecommerce/orders/agent_xxx
  inventory        — agent://nap/ecommerce/inventory/agent_xxx

data>
  analytics        — agent://nap/data/analytics/agent_xxx
  etl              — agent://nap/data/etl/agent_xxx

media>
  image            — agent://nap/media/image/agent_xxx
  video            — agent://nap/media/video/agent_xxx
  audio            — agent://nap/media/audio/agent_xxx
```

A top-level-only capability (e.g. `assistant`) produces a 3-segment URI
(`agent://nap/assistant/agent_xxx`) with no primary skill. Provide a 2- or
3-level path, or declare explicit skills at registration, to get the 4-segment form.

Use the `--capability` flag in `nap claim` to set your node, e.g. `--capability finance>billing`.

---

## Trust Ledger

Every lifecycle event is appended to an append-only hash chain (similar to a blockchain but without consensus overhead). This provides:

- **Auditability** — anyone can verify the full history of the registry
- **Tamper evidence** — any modification to a past entry breaks the chain
- **Root hash** — a single hash represents the entire registry state at any point in time

The genesis entry has hash `0000…0000` (64 zeros). Every subsequent entry hashes `previousHash + timestamp + payload`.

Verify the ledger at any time:

```bash
curl https://registry.nexusagentprotocol.com/api/v1/ledger/verify
# {"valid": true, "length": 42, "root": "a3f2..."}
```

---

## Security Model

| Threat | Mitigation |
|--------|-----------|
| Impersonating an agent | mTLS — requires the CA-issued private key |
| DNS hijacking during registration | DNS-01 challenge uses a random token; short expiry |
| Stolen JWT | Short TTL (default 1h); scoped to specific agent |
| Registry compromise | Trust Ledger provides tamper-evident audit trail |
| Man-in-the-middle | TLS on all connections; HTTPS-only endpoints enforced |
| Replayed tokens | JWT `jti` (token ID) claim; `exp` enforcement |
| Abusive or malicious agent | Abuse reporting system; admin review and resolution workflow |
| Compromised agent credentials | Suspend immediately (reversible); revoke with reason for permanent removal |
| Stale or abandoned agents | Deprecation with sunset date and replacement URI; health checker detects unresponsive endpoints |
| Revoked cert still trusted | Public CRL endpoint at `/api/v1/crl` lists all revoked cert serials |

---

## Privacy Model

The Nexus registry is a **pure phonebook**. Its job is to map `agent://` URIs to endpoints — and nothing else.

### What the registry does not do

| What you might expect | What actually happens |
|-----------------------|----------------------|
| Log resolve queries | Not logged. No record is kept of who looked up whom. |
| Show agent owners their caller list | Not possible. Resolve calls leave no trace. |
| Track query frequency or patterns | Not tracked. No analytics are collected on lookups. |
| Proxy agent-to-agent traffic | Never. After the lookup, agents communicate directly. |

### What the registry does store

The only persistent data is:

1. **Registration metadata** — agent URI, endpoint URL, capability node, primary skill, skill IDs, MCP tool names, display name, status
2. **Trust Ledger entries** — lifecycle events: `register`, `activate`, `revoke`, `suspend`, `restore`, `deprecate`, `update`
3. **X.509 certificates** — the public cert issued at activation (private key is never stored)
4. **User accounts** — email and password hash for free-tier hosted agents
5. **Abuse reports** — reporter, reason, resolution status (no agent traffic is inspected)
6. **Webhook subscriptions** — subscriber URL and event filter (delivery payloads contain only agent metadata)

### The flow after a lookup

```
Agent A                    Nexus Registry              Agent B
   │                            │                          │
   │── GET /resolve?uri=... ───>│                          │
   │<── { endpoint: "https://…" }                          │
   │                            │  (registry is done)      │
   │──────── POST https://agents.b.com/v1/... ────────────>│
   │<──────── 200 OK ──────────────────────────────────────│
```

The registry sees the resolve request and nothing after it. It has no visibility into what Agent A and Agent B say to each other.

### Why this matters

An agent's endpoint is public by design — it is listed in the registry so others can call it. What is **not** public, and what NAP deliberately never captures, is the social graph of which agents talk to which other agents. That information belongs to the agents themselves, not to the registry operator.

This is a **deliberate design choice**, not a limitation. A coordination layer should coordinate — not surveil.

---

## Agent Lifecycle States

An agent transitions through the following statuses:

```
                  ┌──────────┐
                  │ pending  │
                  └────┬─────┘
                       │ activate
                  ┌────▼─────┐
          ┌───────│  active   │───────┐
          │       └────┬─────┘       │
          │ suspend    │ deprecate   │ revoke
     ┌────▼─────┐ ┌───▼──────┐ ┌───▼──────┐
     │suspended │ │deprecated│ │ revoked  │
     └────┬─────┘ └──────────┘ └──────────┘
          │ restore                (terminal)
     ┌────▼─────┐
     │  active   │
     └──────────┘
```

| Status | Resolvable | Reversible | Description |
|--------|-----------|------------|-------------|
| `pending` | No | -- | Registered, awaiting verification |
| `active` | Yes | -- | Fully verified and live |
| `suspended` | No | Yes (restore) | Temporarily blocked; can be restored to active |
| `deprecated` | Yes (with warnings) | No | Marked for retirement; sunset headers returned on resolve |
| `revoked` | No | No | Permanently removed from resolution |
| `expired` | No | No | Certificate or registration expired |

### Suspend and Restore

Suspension is a reversible action for temporarily taking an agent offline (e.g. during a security incident, maintenance, or credential rotation):

```bash
# Suspend
curl -X POST https://registry.nexusagentprotocol.com/api/v1/agents/<UUID>/suspend \
  -H "Authorization: Bearer $TOKEN"

# Restore
curl -X POST https://registry.nexusagentprotocol.com/api/v1/agents/<UUID>/restore \
  -H "Authorization: Bearer $TOKEN"
```

Suspended agents are excluded from resolve queries. Only suspended agents can be restored; revoked agents cannot.

### Deprecation

Deprecation signals to callers that an agent is being retired. The agent remains resolvable, but resolve responses include warning headers:

```bash
curl -X POST https://registry.nexusagentprotocol.com/api/v1/agents/<UUID>/deprecate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"sunset_date": "2026-06-01", "replacement_uri": "agent://acme.com/finance/agent_new"}'
```

When a deprecated agent is resolved, the response includes:

| Header | Value |
|--------|-------|
| `X-NAP-Deprecated` | `true` |
| `Sunset` | `2026-06-01` |
| `X-NAP-Replacement` | `agent://acme.com/finance/agent_new` |

### Revocation with Reason

Revocation now accepts an optional reason string that is recorded in the Trust Ledger:

```bash
curl -X POST https://registry.nexusagentprotocol.com/api/v1/agents/<UUID>/revoke \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"reason": "compromised credentials"}'
```

### Certificate Revocation List (CRL)

The CRL endpoint returns all revoked agent certificate serials:

```bash
curl https://registry.nexusagentprotocol.com/api/v1/crl
```

```json
{
  "entries": [
    {"cert_serial": "3f9a...", "reason": "compromised", "revoked_at": "2026-02-20T12:00:00Z"}
  ],
  "count": 1,
  "generated_at": "2026-02-24T10:00:00Z"
}
```

---

## Continuous Health Monitoring

The registry continuously probes active agent endpoints to detect outages. Agents that fail to respond are flagged as degraded.

### How it works

1. Every `check_interval` (default 5 minutes), the health checker probes all active agent endpoints.
2. A probe sends `HEAD` to the endpoint. If that fails, it falls back to `GET`. Any 2xx response is a success.
3. After `fail_threshold` (default 3) consecutive failures, the agent's health status is set to `degraded`.
4. Once a degraded agent responds successfully, it is restored to `healthy`.
5. Only **transitions** (healthy to degraded, or degraded to healthy) write to the database and Trust Ledger.

### Configuration

```yaml
health:
  check_interval: "5m"    # how often to probe endpoints
  probe_timeout: "10s"    # per-probe HTTP timeout
  fail_threshold: 3       # consecutive failures before degrading
```

---

## Abuse Reporting

Users can report agents for abuse. Reports are reviewed by registry administrators.

```bash
# Report an agent (requires user token)
curl -X POST https://registry.nexusagentprotocol.com/api/v1/agents/<UUID>/report-abuse \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"reason": "impersonation", "details": "This agent claims to be from our company."}'
```

- Each user can have at most **3 open reports** per agent (prevents spam).
- Report statuses: `open` → `investigating` → `resolved` or `dismissed`.
- Administrators review reports at `GET /api/v1/admin/abuse-reports` and resolve them with `PATCH /api/v1/admin/abuse-reports/:id`.

---

## Webhook Events

Subscribe to agent lifecycle events and receive HMAC-signed HTTP POST notifications.

### Subscribe

```bash
curl -X POST https://registry.nexusagentprotocol.com/api/v1/webhooks \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://hooks.example.com/nap",
    "events": ["agent.registered", "agent.revoked", "agent.health_degraded"]
  }'
```

### Event types

| Event | Fired when |
|-------|-----------|
| `agent.registered` | A new agent is registered |
| `agent.activated` | An agent is activated |
| `agent.revoked` | An agent is revoked |
| `agent.suspended` | An agent is suspended |
| `agent.deprecated` | An agent is deprecated |
| `agent.health_degraded` | Health checker detects an unresponsive endpoint |

### Delivery

- Each delivery includes an `X-NAP-Signature` header containing an HMAC-SHA256 signature of the payload, computed with the subscription secret.
- Failed deliveries are retried up to 3 times with exponential backoff (1s, 5s, 25s).
- Delivery history is recorded per subscription.

### Manage subscriptions

```bash
# List your subscriptions
curl https://registry.nexusagentprotocol.com/api/v1/webhooks \
  -H "Authorization: Bearer $TOKEN"

# Delete a subscription
curl -X DELETE https://registry.nexusagentprotocol.com/api/v1/webhooks/<ID> \
  -H "Authorization: Bearer $TOKEN"
```

---

## Batch Resolve

Resolve up to 100 `agent://` URIs in a single request:

```bash
curl -X POST https://registry.nexusagentprotocol.com/api/v1/resolve/batch \
  -H "Content-Type: application/json" \
  -d '{"uris": ["agent://acme.com/finance/agent_1", "agent://nap/assistant/agent_2"]}'
```

```json
{
  "results": [
    {"uri": "agent://acme.com/finance/agent_1", "endpoint": "https://agents.acme.com", "status": "active"},
    {"uri": "agent://nap/assistant/agent_2", "error": "agent not found"}
  ],
  "count": 2
}
```

Partial failures are OK — each result has its own `error` field. Resolution runs with bounded concurrency (10 parallel lookups).

---

## Prometheus Metrics

The registry exposes a `/metrics` endpoint in Prometheus exposition format.

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `nap_agents_total` | Gauge | `status` | Total agents by status |
| `nap_requests_total` | Counter | `method`, `path`, `status` | HTTP requests by method, path, and status code |
| `nap_request_duration_seconds` | Histogram | `method`, `path` | Request latency distribution |
| `nap_health_checks_total` | Counter | `result` | Health check probe results |
| `nap_ledger_entries_total` | Counter | -- | Trust Ledger entries appended |
| `nap_webhook_deliveries_total` | Counter | `success` | Webhook delivery attempts |

---

## Comparison with A2A and MCP

| | NAP | Google A2A | Anthropic MCP |
|-|-----|-----------|--------------|
| **Primary use case** | Agent-to-agent internet calls | Agent-to-agent (enterprise) | Model-to-local-tool calls |
| **Transport** | HTTPS + mTLS | HTTPS | stdio / SSE |
| **Identity** | X.509 + DNS-01 | Unspecified | None (local trust) |
| **Discovery** | `/.well-known/agent-card.json` | `/.well-known/agent.json` | Manual config |
| **Central registry** | Yes (Nexus) | No | No |
| **Live deployments** | First-mover | 0 | ~1 (Notion) |
| **Token standard** | RS256 JWT | Unspecified | None |
| **Audit trail** | Trust Ledger | None | None |

---

## Quick Start Checklist

- [ ] Run `nap claim yourdomain.com` to register your agent
- [ ] Add DNS TXT record when prompted (verifies ownership)
- [ ] Save certs from `~/.nap/certs/yourdomain.com/` securely
- [ ] Call other agents with `client.CallAgent(ctx, agentURI, ...)`
- [ ] Protect your own endpoint with `identity.RequireToken(issuerURL)`
- [ ] Optionally: deploy `nap-mcp-bridge` for Claude Desktop integration
- [ ] Optionally: publish `/.well-known/agent-card.json` on your own domain
- [ ] Optionally: subscribe to webhook events for real-time notifications
- [ ] Optionally: monitor `/metrics` with Prometheus/Grafana

---

*NAP is an open protocol. The registry source is at `github.com/jmerrifield20/NexusAgentProtocol`.*
