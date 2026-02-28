# Nexus Agent Protocol (NAP) — Integration Guide

> Connect your AI agent to the internet with a verified identity, resolvable address, and authenticated agent-to-agent communication.

---

## What is NAP?

NAP gives AI agents a stable, verifiable identity on the internet — similar to how domains identify websites, but for agents. It answers three questions:

1. **Who is this agent?** — verified identity backed by DNS-01 domain ownership or email verification; X.509 certificate issued by the Nexus CA
2. **Where does it live?** — a permanent `agent://` URI that resolves to an HTTPS endpoint
3. **Is it authorised to call me?** — mutual TLS and scoped RS256 JWT Task Tokens

### The `agent://` URI

Every NAP agent has a permanent, human-readable address:

```
agent://acme.com/finance/reconcile-invoices/agent_7x2v9qaabbccdd
         ───────  ───────  ─────────────────  ──────────────────
         trust    category  primary skill       unique agent ID
         root
```

| Segment | Meaning |
|---------|---------|
| `acme.com` | Trust root — your verified domain, or `nap` for NAP-hosted agents |
| `finance` | Category — top-level capability namespace |
| `reconcile-invoices` | Primary skill — derived from your capability node at registration |
| `agent_7x2v9q…` | Agent ID — opaque unique identifier assigned at registration |

The URI is permanent. The underlying endpoint (IP/URL) can change; the URI does not.

When the capability is top-level only (no sub-skill), the shorter 3-segment form is used:

```
agent://nap/assistant/agent_abc123
```

### The Registry

The Nexus Registry is the central identity coordinator for NAP. It:

- Verifies identity via DNS-01 challenge (domain agents) or email verification (NAP-hosted agents)
- Issues X.509 certificates to registered agents (4096-bit RSA, signed by the Nexus CA)
- Resolves `agent://` URIs to live HTTPS endpoints
- Maintains a Trust Ledger — an append-only hash chain of all registration events

### The Agent Card

Every NAP domain optionally publishes a discovery file at:

```
https://example.com/.well-known/agent-card.json
```

The Nexus registry also serves this on behalf of registered domains at:

```
https://api.nexusagentprotocol.com/.well-known/agent-card.json?domain=example.com
```

**Example agent-card.json:**

```json
{
  "schema_version": "1.0",
  "domain": "acme.com",
  "trust_root": "nexusagentprotocol.com",
  "updated_at": "2026-02-27T12:00:00Z",
  "agents": [
    {
      "uri": "agent://acme.com/finance/billing/agent_7x2v9q",
      "display_name": "Acme Billing Agent",
      "description": "Handles invoicing and payment reconciliation.",
      "endpoint": "https://agents.acme.com",
      "capability_node": "finance>billing",
      "protocols": ["https"],
      "status": "active"
    }
  ]
}
```

### Authentication — mTLS + Task Tokens

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
  │── POST https://agents.b.com/v1/task                               │
  │   Authorization: Bearer <JWT>  ──────────────────────────────────>│
  │<─ 200 OK ─────────────────────────────────────────────────────────│
```

- **mTLS cert** — proves to the registry that you are who you say you are
- **JWT Task Token** — a scoped, short-lived RS256 token the registry issues after verifying your cert
- **Bearer token on agent call** — the receiving agent validates the JWT against the registry JWKS endpoint

---

## Path A: NAP-Hosted Registration

Use this path if you do not own a domain or want to get started quickly. Your agent's trust root will be `nap`, giving a URI like `agent://nap/finance/billing/agent_xxx`.

**Requirement:** a verified email address.

### Step 1 — Create an account

```bash
curl -X POST https://api.nexusagentprotocol.com/api/v1/auth/signup \
  -H "Content-Type: application/json" \
  -d '{"email": "you@example.com", "password": "yourpassword"}'
```

Check your inbox and follow the verification link. Registration is blocked until your email is verified.

### Step 2 — Log in and get a user token

```bash
curl -X POST https://api.nexusagentprotocol.com/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "you@example.com", "password": "yourpassword"}'
```

```json
{ "token": "eyJ..." }
```

Save this token — you will use it to register and manage your agents.

### Step 3 — Register your agent

```bash
curl -X POST https://api.nexusagentprotocol.com/api/v1/agents \
  -H "Authorization: Bearer $USER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "registration_type": "nap_hosted",
    "display_name": "My Assistant",
    "description": "A general-purpose assistant agent.",
    "capability": "assistant>general",
    "endpoint": "https://myagent.example.com"
  }'
```

```json
{
  "agent": {
    "id": "3f9a1b2c-...",
    "agent_uri": "agent://nap/assistant/general/agent_7x2v9q",
    "status": "pending"
  }
}
```

Note the `id` — you will need it to activate.

### Step 4 — Activate and receive your certificate

Activation is gated on your email being verified. Once it is:

```bash
curl -X POST https://api.nexusagentprotocol.com/api/v1/agents/$AGENT_ID/activate \
  -H "Authorization: Bearer $USER_TOKEN"
```

```json
{
  "cert_pem": "-----BEGIN CERTIFICATE-----\n...",
  "private_key_pem": "-----BEGIN RSA PRIVATE KEY-----\n...",
  "ca_pem": "-----BEGIN CERTIFICATE-----\n..."
}
```

> **Important:** `private_key_pem` is returned once and never stored by the registry. Save it securely (file mode `0600`).

Save the three PEM values to `~/.nap/certs/nap/`:
- `cert.pem`
- `key.pem`
- `ca.pem`

Your agent is now `active` with URI `agent://nap/assistant/general/agent_7x2v9q`.

---

## Path B: Domain-Verified Registration

Use this path to register an agent under your own domain. Your agent's trust root will be your domain, giving a URI like `agent://acme.com/finance/billing/agent_xxx`.

**Requirement:** ownership of a domain with the ability to add DNS TXT records.

### Step 1 — Start the DNS-01 challenge

```bash
curl -X POST https://api.nexusagentprotocol.com/api/v1/dns/challenge \
  -H "Content-Type: application/json" \
  -d '{"domain": "acme.com"}'
```

```json
{
  "id": "chal_abc123",
  "domain": "acme.com",
  "txt_host": "_nexus-agent-challenge.acme.com",
  "txt_record": "nexus-agent-challenge=abc123xyz",
  "status": "pending"
}
```

### Step 2 — Publish the TXT record

Add the following DNS record to your domain:

```
Type:  TXT
Host:  _nexus-agent-challenge.acme.com
Value: nexus-agent-challenge=abc123xyz
```

This is the same mechanism used by Let's Encrypt. DNS propagation typically takes 1–5 minutes.

### Step 3 — Verify the challenge

```bash
curl -X POST https://api.nexusagentprotocol.com/api/v1/dns/challenge/$CHALLENGE_ID/verify
```

Returns `{"status": "verified"}` when the TXT record is found, or `{"status": "pending"}` if DNS has not yet propagated — retry after a short wait.

### Step 4 — Register your agent

Once the challenge is verified, register your agent:

```bash
curl -X POST https://api.nexusagentprotocol.com/api/v1/agents \
  -H "Content-Type: application/json" \
  -d '{
    "registration_type": "domain_verified",
    "trust_root": "acme.com",
    "display_name": "Acme Billing Agent",
    "description": "Handles invoicing and payment reconciliation.",
    "capability": "finance>billing",
    "endpoint": "https://agents.acme.com"
  }'
```

```json
{
  "agent": {
    "id": "7b3c9d1e-...",
    "agent_uri": "agent://acme.com/finance/billing/agent_7x2v9q",
    "status": "pending"
  }
}
```

### Step 5 — Activate and receive your certificate

```bash
curl -X POST https://api.nexusagentprotocol.com/api/v1/agents/$AGENT_ID/activate
```

```json
{
  "cert_pem": "-----BEGIN CERTIFICATE-----\n...",
  "private_key_pem": "-----BEGIN RSA PRIVATE KEY-----\n...",
  "ca_pem": "-----BEGIN CERTIFICATE-----\n..."
}
```

> **Important:** `private_key_pem` is returned once and never stored by the registry. Save it securely (file mode `0600`).

Save the three PEM values to `~/.nap/certs/acme.com/`:
- `cert.pem`
- `key.pem`
- `ca.pem`

Your agent is now `active` with URI `agent://acme.com/finance/billing/agent_7x2v9q`.

---

## Connecting Your Agent

The steps below are identical regardless of which registration path you used. The only difference is the cert directory path (`~/.nap/certs/nap/` vs `~/.nap/certs/yourdomain.com/`).

### Go SDK — connect with your certs

```go
import "github.com/jmerrifield20/NexusAgentProtocol/pkg/client"

c, err := client.NewFromCertDir(
    "https://api.nexusagentprotocol.com",
    os.ExpandEnv("$HOME/.nap/certs/acme.com"),  // or /nap for NAP-hosted
)
if err != nil {
    log.Fatal(err)
}
```

### Call another agent

```go
var reply map[string]any
err = c.CallAgent(ctx,
    "agent://acme.com/finance/billing/agent_7x2v9q",
    http.MethodPost, "/v1/invoice",
    map[string]any{"amount": 100, "currency": "USD"},
    &reply,
)
```

`CallAgent` handles resolve → token exchange → authenticated HTTP call in one step.

**Using curl directly:**

```bash
# 1. Resolve the URI to an endpoint
curl "https://api.nexusagentprotocol.com/api/v1/resolve?uri=agent://acme.com/finance/billing/agent_7x2v9q"
# → {"endpoint": "https://agents.acme.com", "status": "active"}

# 2. Exchange your cert for a JWT Task Token
curl -X POST https://api.nexusagentprotocol.com/api/v1/token \
  --cert ~/.nap/certs/acme.com/cert.pem \
  --key  ~/.nap/certs/acme.com/key.pem  \
  --cacert ~/.nap/certs/acme.com/ca.pem
# → {"token": "eyJ..."}

# 3. Call the agent
curl -X POST https://agents.acme.com/v1/invoice \
  -H "Authorization: Bearer $TASK_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"amount": 100, "currency": "USD"}'
```

### Accept incoming NAP calls

Add the NAP middleware to your HTTP server. It validates the incoming JWT against the Nexus JWKS endpoint:

```go
import (
    "github.com/gin-gonic/gin"
    "github.com/jmerrifield20/NexusAgentProtocol/internal/identity"
)

router := gin.New()

// Only valid NAP JWT holders can call this endpoint
router.POST("/agent/*path", identity.RequireToken(oidcIssuerURL), yourHandler)
```

The JWKS endpoint is:

```
https://api.nexusagentprotocol.com/.well-known/jwks.json
```

The OIDC discovery document is at:

```
https://api.nexusagentprotocol.com/.well-known/openid-configuration
```

---

## Discovering Agents

All discovery endpoints are public and require no authentication.

### Free-text search

Search across agent name, description, capability, tags, and agent ID:

```bash
curl "https://api.nexusagentprotocol.com/api/v1/agents?q=billing"
```

```json
{
  "agents": [
    {
      "id": "7b3c9d1e-...",
      "agent_uri": "agent://acme.com/finance/billing/agent_7x2v9q",
      "display_name": "Acme Billing Agent",
      "description": "Handles invoicing and payment reconciliation.",
      "endpoint": "https://agents.acme.com",
      "capability_node": "finance>billing",
      "status": "active"
    }
  ],
  "count": 1
}
```

### Filter by capability

Use `?capability_node=` to browse by capability. This is a **prefix match** — passing a top-level category returns all agents within it:

```bash
# All finance agents (billing, accounting, tax, etc.)
curl "https://api.nexusagentprotocol.com/api/v1/agents?capability_node=finance"

# Only billing agents
curl "https://api.nexusagentprotocol.com/api/v1/agents?capability_node=finance>billing"
```

Combine with `?trust_root=` to scope to a specific organisation:

```bash
# All finance agents registered under acme.com
curl "https://api.nexusagentprotocol.com/api/v1/agents?trust_root=acme.com&capability_node=finance"
```

### Filter by skill ID

Find agents that declare a specific skill. Skill IDs are exact-match and indexed:

```bash
curl "https://api.nexusagentprotocol.com/api/v1/agents?skill=reconcile-invoices"
```

### Filter by MCP tool name

Find agents that expose a specific MCP tool:

```bash
curl "https://api.nexusagentprotocol.com/api/v1/agents?tool=parse_invoice"
```

### Filter by owner

Find all agents registered by a specific NAP user:

```bash
curl "https://api.nexusagentprotocol.com/api/v1/agents?username=acmecorp"
```

### Pagination

All list endpoints support `?limit=` (max 200, default 50) and `?offset=`:

```bash
# First page
curl "https://api.nexusagentprotocol.com/api/v1/agents?capability_node=finance&limit=20&offset=0"

# Second page
curl "https://api.nexusagentprotocol.com/api/v1/agents?capability_node=finance&limit=20&offset=20"
```

### Look up a single agent

```bash
curl "https://api.nexusagentprotocol.com/api/v1/agents/$AGENT_ID"
```

```json
{
  "agent": {
    "id": "7b3c9d1e-...",
    "agent_uri": "agent://acme.com/finance/billing/agent_7x2v9q",
    "display_name": "Acme Billing Agent",
    "capability_node": "finance>billing",
    "primary_skill": "billing",
    "endpoint": "https://agents.acme.com",
    "status": "active"
  },
  "owner": {
    "username": "acmecorp",
    "display_name": "Acme Corp",
    "avatar_url": "https://..."
  }
}
```

### List all agents for a domain

```bash
curl "https://api.nexusagentprotocol.com/.well-known/agent-card.json?domain=acme.com"
```

### Browse the capability taxonomy

To see all valid capability nodes before registering or searching:

```bash
curl "https://api.nexusagentprotocol.com/api/v1/capabilities"
```

```json
{
  "categories": [
    {
      "name": "finance",
      "subcategories": [
        {
          "name": "accounting",
          "items": ["reconciliation"]
        },
        {
          "name": "billing",
          "items": []
        }
      ]
    }
  ]
}
```

---

## Agent Lifecycle

### States

```
                  ┌──────────┐
                  │ pending  │
                  └────┬─────┘
                       │ activate
                  ┌────▼─────┐
          ┌───────│  active  │───────┐
          │       └────┬─────┘       │
          │ suspend    │ deprecate   │ revoke
     ┌────▼──────┐ ┌───▼──────┐ ┌───▼──────┐
     │ suspended │ │deprecated│ │ revoked  │
     └────┬──────┘ └──────────┘ └──────────┘
          │ restore   (resolvable) (terminal)
     ┌────▼─────┐
     │  active  │
     └──────────┘
```

| Status | Resolvable | Reversible | Description |
|--------|-----------|------------|-------------|
| `pending` | No | — | Registered, awaiting verification |
| `active` | Yes | — | Fully verified and live |
| `suspended` | No | Yes | Temporarily offline; can be restored |
| `deprecated` | Yes (with warnings) | No | Marked for retirement; sunset headers returned on resolve |
| `revoked` | No | No | Permanently removed from resolution |
| `expired` | No | No | Certificate or registration expired |

### Suspend and Restore

Suspension temporarily removes an agent from resolution — useful during a security incident, maintenance, or credential rotation.

```bash
# Suspend
curl -X POST https://api.nexusagentprotocol.com/api/v1/agents/$AGENT_ID/suspend \
  -H "Authorization: Bearer $TOKEN"

# Restore
curl -X POST https://api.nexusagentprotocol.com/api/v1/agents/$AGENT_ID/restore \
  -H "Authorization: Bearer $TOKEN"
```

### Deprecation

Deprecation signals to callers that an agent is being retired. The agent remains resolvable, but resolve responses include warning headers so callers can migrate.

```bash
curl -X POST https://api.nexusagentprotocol.com/api/v1/agents/$AGENT_ID/deprecate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"sunset_date": "2026-09-01", "replacement_uri": "agent://acme.com/finance/billing/agent_new"}'
```

When a deprecated agent is resolved, the response includes:

| Header | Value |
|--------|-------|
| `X-NAP-Deprecated` | `true` |
| `Sunset` | `2026-09-01` |
| `X-NAP-Replacement` | `agent://acme.com/finance/billing/agent_new` |

### Revocation

Revocation permanently removes an agent from resolution and records a reason in the Trust Ledger.

```bash
curl -X POST https://api.nexusagentprotocol.com/api/v1/agents/$AGENT_ID/revoke \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"reason": "compromised credentials"}'
```

### Certificate Revocation List

To check whether a certificate has been revoked:

```bash
curl https://api.nexusagentprotocol.com/api/v1/crl
```

```json
{
  "entries": [
    {"cert_serial": "3f9a...", "reason": "compromised", "revoked_at": "2026-02-20T12:00:00Z"}
  ],
  "count": 1,
  "generated_at": "2026-02-27T10:00:00Z"
}
```

### Continuous Health Monitoring

The registry continuously probes active agent endpoints to detect outages. Your endpoint must remain reachable.

1. Every 5 minutes, the registry sends a `HEAD` request to your registered endpoint. If `HEAD` fails, it falls back to `GET`. Any 2xx response is a pass.
2. After 3 consecutive failures, your agent's health status is set to `degraded`. It remains resolvable but callers may see a `health: degraded` field in the resolve response.
3. Once your endpoint responds successfully again, health is restored to `healthy` automatically.

---

## Security Model

### Threat mitigations

| Threat | Mitigation |
|--------|-----------|
| Impersonating an agent | mTLS — requires the CA-issued private key |
| DNS hijacking during registration | DNS-01 challenge uses a random token with short expiry |
| Stolen JWT | Short TTL (default 1h); scoped to specific agent |
| Registry compromise | Trust Ledger provides tamper-evident audit trail |
| Man-in-the-middle | TLS on all connections; HTTPS-only endpoints enforced |
| Replayed tokens | JWT `jti` claim; `exp` enforcement |
| Abusive or malicious agent | Abuse reporting system; admin review and resolution workflow |
| Compromised credentials | Suspend immediately (reversible); revoke with reason for permanent removal |
| Stale or abandoned agents | Deprecation with sunset date and replacement URI; health checker detects unresponsive endpoints |
| Revoked cert still trusted | Public CRL at `/api/v1/crl` lists all revoked cert serials |

### Privacy model

The Nexus registry is a **pure phonebook**. Its job is to map `agent://` URIs to endpoints — and nothing else.

**What the registry does not do:**

| What you might expect | What actually happens |
|-----------------------|----------------------|
| Log resolve queries | Not logged. No record is kept of who looked up whom. |
| Show agent owners their caller list | Not possible. Resolve calls leave no trace. |
| Track query frequency or patterns | Not tracked. No analytics are collected on lookups. |
| Proxy agent-to-agent traffic | Never. After the lookup, agents communicate directly. |

**What the registry does store:**

1. Registration metadata — URI, endpoint, capability node, primary skill, skill IDs, MCP tool names, display name, status
2. Trust Ledger entries — lifecycle events: `register`, `activate`, `revoke`, `suspend`, `restore`, `deprecate`
3. X.509 certificates — the public cert issued at activation (private key is never stored)
4. User accounts — email and password hash (NAP-hosted agents only)
5. Abuse reports — reporter, reason, resolution status (no agent traffic is inspected)
6. Webhook subscriptions — subscriber URL and event filter

**The flow after a lookup:**

```
Agent A                    Nexus Registry              Agent B
   │                            │                          │
   │── GET /resolve?uri=... ───>│                          │
   │<── { endpoint: "https://…" }                          │
   │                            │  (registry is done)      │
   │──────── POST https://agents.b.com/v1/... ────────────>│
   │<──────── 200 OK ──────────────────────────────────────│
```

The registry has no visibility into what Agent A and Agent B say to each other.

---

## API Reference

All endpoints are on `https://api.nexusagentprotocol.com`.

### Auth (NAP-hosted path)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/v1/auth/signup` | None | Create an account (email + password) |
| `POST` | `/api/v1/auth/login` | None | Log in; returns user JWT |
| `POST` | `/api/v1/auth/verify-email` | None | Consume email verification token |
| `POST` | `/api/v1/auth/resend-verification` | None | Re-send verification email |
| `GET` | `/api/v1/auth/oauth/:provider` | None | Begin OAuth flow (GitHub or Google) |

### Agent Lifecycle

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/v1/dns/challenge` | None | Start DNS-01 domain verification |
| `GET` | `/api/v1/dns/challenge/:id` | None | Poll challenge status |
| `POST` | `/api/v1/dns/challenge/:id/verify` | None | Trigger DNS TXT lookup |
| `POST` | `/api/v1/agents` | None / User JWT | Register a new agent |
| `GET` | `/api/v1/agents/:id` | None | Get agent details |
| `PATCH` | `/api/v1/agents/:id` | mTLS / User JWT | Update agent metadata |
| `POST` | `/api/v1/agents/:id/activate` | mTLS / User JWT | Issue X.509 cert; returns cert + private key |
| `POST` | `/api/v1/agents/:id/revoke` | mTLS | Revoke agent (records reason; writes to Trust Ledger) |
| `DELETE` | `/api/v1/agents/:id` | mTLS | Permanently delete agent record |
| `POST` | `/api/v1/agents/:id/suspend` | mTLS / User JWT | Suspend agent (reversible; blocks resolution) |
| `POST` | `/api/v1/agents/:id/restore` | mTLS / User JWT | Restore a suspended agent to active |
| `POST` | `/api/v1/agents/:id/deprecate` | mTLS / User JWT | Mark deprecated with optional sunset date |
| `GET` | `/api/v1/users/me/agents` | User JWT | List all agents owned by the authenticated user |

### Discovery

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/.well-known/agent-card.json?domain=X` | None | List all active agents for a domain |
| `GET` | `/api/v1/agents` | None | Filter agents by trust root, capability, skill, tool, or free text |
| `GET` | `/api/v1/agents?q=billing` | None | Full-text search across name, description, tags, agent ID |
| `GET` | `/api/v1/agents?skill=reconcile-invoices` | None | Find agents by declared skill ID |
| `GET` | `/api/v1/agents?tool=parse_invoice` | None | Find agents by MCP tool name |
| `GET` | `/api/v1/agents/:id/agent.json` | None | A2A-spec agent card for a single agent |
| `GET` | `/api/v1/agents/:id/mcp-manifest.json` | None | MCP manifest for a single agent |
| `GET` | `/api/v1/capabilities` | None | Full capability taxonomy as nested JSON |

### Resolution

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/v1/resolve?uri=agent://…` | None | Resolve URI → endpoint |
| `POST` | `/api/v1/resolve/batch` | None | Resolve up to 100 URIs in one request |
| `POST` | `/api/v1/token` | mTLS | Exchange cert for JWT Task Token |

### Revocation & Trust

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/v1/crl` | None | Certificate Revocation List (all revoked cert serials) |
| `POST` | `/api/v1/agents/:id/report-abuse` | User JWT | Report an agent for abuse |

### Webhooks

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/v1/webhooks` | User JWT | Subscribe to lifecycle events |
| `GET` | `/api/v1/webhooks` | User JWT | List your webhook subscriptions |
| `DELETE` | `/api/v1/webhooks/:id` | User JWT | Delete a webhook subscription |

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

## SDK Quick Reference (Go)

```go
// Connect — no auth (resolution and discovery only)
c, err := client.New("https://api.nexusagentprotocol.com")

// Connect — authenticated (load certs from disk)
c, err := client.NewFromCertDir(registryURL, "~/.nap/certs/acme.com")

// Resolve a URI to an endpoint
result, err := c.Resolve(ctx, "agent://acme.com/finance/billing/agent_7x2v9q")
// result.Endpoint → "https://agents.acme.com"

// Batch resolve (up to 100 URIs)
results, err := c.ResolveBatch(ctx, []string{
    "agent://acme.com/finance/billing/agent_1",
    "agent://nap/assistant/general/agent_2",
})

// Call another agent (resolve + auth + HTTP in one step)
err = c.CallAgent(ctx, uri, method, path, reqBody, &respBody)

// DNS challenge flow
challenge, err := c.StartDNSChallenge(ctx, "acme.com")
// publish challenge.TXTHost / challenge.TXTRecord in your DNS
err = c.VerifyDNSChallenge(ctx, challenge.ID)

// Register + activate
agent, err := c.RegisterAgent(ctx, client.RegisterAgentRequest{...})
certs, err := c.ActivateAgent(ctx, agent.ID)
// certs.PrivateKeyPEM is delivered once and never stored by the registry

// List agents by capability
agents, err := c.ListAgents(ctx, "acme.com", "finance")

// Lifecycle management
err = c.RevokeAgent(ctx, agentID, "compromised credentials")
err = c.SuspendAgent(ctx, agentID)
err = c.RestoreAgent(ctx, agentID)

// Certificate Revocation List
crl, err := c.GetCRL(ctx)
// crl.Entries → [{CertSerial, Reason, RevokedAt}]
```

---

## Webhooks

Subscribe to agent lifecycle events and receive HMAC-signed HTTP POST notifications.

```bash
curl -X POST https://api.nexusagentprotocol.com/api/v1/webhooks \
  -H "Authorization: Bearer $USER_TOKEN" \
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

Each delivery includes an `X-NAP-Signature` header — HMAC-SHA256 of the payload body, computed with your subscription secret. Failed deliveries are retried up to 3 times with exponential backoff (1s, 5s, 25s).

### Batch Resolve

Resolve up to 100 `agent://` URIs in a single request:

```bash
curl -X POST https://api.nexusagentprotocol.com/api/v1/resolve/batch \
  -H "Content-Type: application/json" \
  -d '{"uris": ["agent://acme.com/finance/billing/agent_1", "agent://nap/assistant/general/agent_2"]}'
```

```json
{
  "results": [
    {"uri": "agent://acme.com/finance/billing/agent_1", "endpoint": "https://agents.acme.com", "status": "active"},
    {"uri": "agent://nap/assistant/general/agent_2", "error": "agent not found"}
  ],
  "count": 2
}
```

Partial failures are fine — each result has its own `error` field.

---

## Capability Node Taxonomy

Capability nodes are hierarchical paths using `>` as the separator (up to 3 levels). Use the most specific node that describes your agent. The last segment of your capability path automatically becomes your agent's `primary_skill` URI segment.

```
assistant>
  general          → agent://nap/assistant/general/agent_xxx
  coding           → agent://nap/assistant/coding/agent_xxx
  research         → agent://nap/assistant/research/agent_xxx

finance>
  billing          → agent://nap/finance/billing/agent_xxx
  accounting>
    reconciliation → agent://nap/finance/reconciliation/agent_xxx
  tax              → agent://nap/finance/tax/agent_xxx

ecommerce>
  retail           → agent://nap/ecommerce/retail/agent_xxx
  orders           → agent://nap/ecommerce/orders/agent_xxx
  inventory        → agent://nap/ecommerce/inventory/agent_xxx

data>
  analytics        → agent://nap/data/analytics/agent_xxx
  etl              → agent://nap/data/etl/agent_xxx

media>
  image            → agent://nap/media/image/agent_xxx
  video            → agent://nap/media/video/agent_xxx
  audio            → agent://nap/media/audio/agent_xxx
```

A top-level-only capability (e.g. `assistant`) produces a 3-segment URI (`agent://nap/assistant/agent_xxx`) with no primary skill. Provide a 2- or 3-level path, or declare explicit skills at registration, to get the 4-segment form.

The full taxonomy is available at:

```bash
curl https://api.nexusagentprotocol.com/api/v1/capabilities
```

---

## Quick Start Checklists

### NAP-Hosted (no domain required)

- [ ] `POST /api/v1/auth/signup` — create your account
- [ ] Verify your email via the link in your inbox
- [ ] `POST /api/v1/auth/login` — get your user token
- [ ] `POST /api/v1/agents` with `registration_type: nap_hosted` — register your agent
- [ ] `POST /api/v1/agents/:id/activate` — receive your cert and private key
- [ ] Save `cert.pem`, `key.pem`, `ca.pem` securely (mode `0600`)
- [ ] Call other agents with `client.CallAgent(ctx, agentURI, ...)`
- [ ] Protect your endpoint with `identity.RequireToken(issuerURL)`
- [ ] Optionally: subscribe to webhook events for lifecycle notifications
- [ ] Optionally: publish `/.well-known/agent-card.json` on your own domain

### Domain-Verified

- [ ] `POST /api/v1/dns/challenge` — start DNS-01 verification for your domain
- [ ] Publish the TXT record in your DNS
- [ ] `POST /api/v1/dns/challenge/:id/verify` — confirm the record is live
- [ ] `POST /api/v1/agents` with `registration_type: domain_verified` — register your agent
- [ ] `POST /api/v1/agents/:id/activate` — receive your cert and private key
- [ ] Save `cert.pem`, `key.pem`, `ca.pem` securely (mode `0600`)
- [ ] Call other agents with `client.CallAgent(ctx, agentURI, ...)`
- [ ] Protect your endpoint with `identity.RequireToken(issuerURL)`
- [ ] Optionally: publish `/.well-known/agent-card.json` on your own domain
- [ ] Optionally: subscribe to webhook events for lifecycle notifications

---

*NAP is an open protocol. Registry source: `github.com/jmerrifield20/NexusAgentProtocol`*
