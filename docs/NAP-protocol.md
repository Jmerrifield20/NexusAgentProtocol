# Nexus Agent Protocol (NAP)

> A open standard for agent identity, discovery, and authenticated agent-to-agent communication over the internet.

---

## What is NAP?

NAP is a protocol that gives AI agents a stable, verifiable identity on the internet — similar to how domains identify websites, but for agents. It answers three questions that no existing protocol does together:

1. **Who is this agent?** — verified domain ownership, X.509 certificate
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
agent://nexusagentprotocol.com/ecommerce/retail/agent_7x2v9qaabbccdd
         ────────  ────────────────  ──────────────────
         trust      capability node    unique agent ID
         root       (hierarchy)
```

| Part | Meaning |
|------|---------|
| `nexusagentprotocol.com` | Trust root — the registry that issued the identity |
| `ecommerce/retail` | Capability node — what the agent does (hierarchical) |
| `agent_7x2v9q…` | Unique agent ID assigned at registration |

The URI is permanent. The underlying endpoint (IP/URL) can change; the URI does not.

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
      "uri": "agent://nexusagentprotocol.com/ecommerce/retail/agent_7x2v9q",
      "display_name": "Acme Store",
      "description": "Handles orders, inventory, and invoicing for Acme.",
      "endpoint": "https://agents.example.com",
      "capability_node": "ecommerce/retail",
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
import "github.com/nexus-protocol/nexus/pkg/client"
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

    "github.com/nexus-protocol/nexus/pkg/client"
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
        "agent://nexusagentprotocol.com/finance/billing/agent_7x2v9q",
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
    "github.com/nexus-protocol/nexus/internal/identity"
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
| `GET` | `/api/v1/agents?trust_root=X&capability_node=Y` | None | Search agents |

### Agent Lifecycle

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/v1/dns/challenge` | None | Start DNS-01 domain verification |
| `POST` | `/api/v1/dns/challenge/:id/verify` | None | Verify the TXT record |
| `POST` | `/api/v1/agents` | DNS verified | Register a new agent |
| `POST` | `/api/v1/agents/:id/activate` | mTLS | Issue X.509 cert; returns cert + private key |
| `DELETE` | `/api/v1/agents/:id` | mTLS | Revoke agent |

### Resolution

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/v1/resolve?uri=agent://…` | None | Resolve URI → endpoint |
| `POST` | `/api/v1/token` | mTLS | Exchange cert for JWT Task Token |

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
result, err := c.Resolve(ctx, "agent://nexusagentprotocol.com/finance/billing/agent_7x2v9q")
// result.Endpoint → "https://billing.example.com"

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

// List agents
agents, err := c.ListAgents(ctx, "nexusagentprotocol.com", "ecommerce")
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

Capability nodes are hierarchical dot-separated paths. Use the most specific node that describes your agent:

```
assistant/
  general          — general-purpose conversational assistant
  coding           — software development assistant
  research         — research and summarisation

finance/
  billing          — invoicing and payment processing
  accounting       — bookkeeping and financial reporting
  tax              — tax calculation and filing

ecommerce/
  retail           — product catalogue and storefront
  orders           — order management and fulfilment
  inventory        — stock management

data/
  analytics        — data analysis and reporting
  etl              — data pipeline and transformation

media/
  image            — image generation and editing
  video            — video generation and editing
  audio            — audio generation and transcription
```

Use the `--capability` flag in `nap claim` to set your node, e.g. `--capability assistant/general`.

---

## Trust Ledger

Every registration and revocation event is appended to an append-only hash chain (similar to a blockchain but without consensus overhead). This provides:

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

---

*NAP is an open protocol. The registry source is at `github.com/nexus-protocol/nexus`.*
