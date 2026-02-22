# @openclaw/nap — Nexus Agent Protocol Integration

Gives any Node.js application (or an [OpenClaw](https://github.com/openclaw/openclaw)
Gateway specifically) a stable, globally-resolvable `agent://` URI backed by the
Nexus Agent Protocol (NAP) registry.

---

## Table of Contents

1. [Conceptual model](#1-conceptual-model)
2. [Registration paths](#2-registration-paths)
3. [Installation](#3-installation)
4. [Configuration reference](#4-configuration-reference)
5. [State file reference](#5-state-file-reference)
6. [Step-by-step: free hosted tier](#6-step-by-step-free-hosted-tier)
7. [Step-by-step: domain-verified tier](#7-step-by-step-domain-verified-tier)
8. [Gateway integration](#8-gateway-integration)
9. [Public API reference](#9-public-api-reference)
10. [Backend API reference](#10-backend-api-reference)
11. [Error handling](#11-error-handling)
12. [Token lifecycle](#12-token-lifecycle)
13. [Self-hosted registry](#13-self-hosted-registry)
14. [Troubleshooting](#14-troubleshooting)

---

## 1. Conceptual model

Three independent layers work together — understanding which is which prevents
confusion:

```
┌─────────────────────────────────────────────────────────────────┐
│  NAP Registry  (this package talks to this)                     │
│  registry.nexusagentprotocol.com                                │
│  • Global index of agent:// URIs                                │
│  • Issues signed endorsement JWTs                               │
│  • Resolves agent:// → HTTPS endpoint                           │
└──────────────────────────────┬──────────────────────────────────┘
                               │ produces
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│  A2A Agent Card  (your server serves this)                      │
│  https://yourdomain.com/.well-known/agent.json                  │
│  • Describes capabilities, protocols, auth requirements         │
│  • Embeds NAP endorsement JWT (signed by registry)              │
│  • Readable by any A2A-compatible client                        │
└──────────────────────────────┬──────────────────────────────────┘
                               │ describes
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│  Your HTTPS Endpoint  (callers talk to this)                    │
│  https://yourdomain.com  (or Tailscale address, etc.)           │
│  • Handles actual agent task requests over HTTPS                │
│  • Transport layer — unchanged by NAP                           │
└─────────────────────────────────────────────────────────────────┘
```

**What NAP does not do:** NAP does not proxy traffic, inspect payloads, or
sit between caller and agent at runtime. After resolution, callers talk
directly to your HTTPS endpoint.

**What `agent://` is:** A stable name, not a transport URL. The registry
resolves it to your current HTTPS endpoint. If your endpoint changes (new
Tailscale address, new server) you PATCH the registry once and all existing
`agent://` references continue to work.

---

## 2. Registration paths

All `agent://` URIs share the same three-segment format:

```
agent://<org>/<category>/<agent-id>
```

| Segment | Description |
|---------|-------------|
| `org` | The entity that owns this agent — `nap` for free-hosted agents, or the **full verified domain** for domain-verified agents (e.g. `acme.com`). DNS-01 proves ownership, so `agent://acme.com/…` can only be ACME — `acme.io` and `amazon.fakeaccount.com` get different addresses. |
| `category` | Top-level capability from the NAP taxonomy (e.g. `finance`, `assistant`, `devops`). Sub-categories like `finance>accounting` are searchable but only the top-level appears in the URI, keeping addresses stable as capability paths evolve. |
| `agent-id` | Opaque unique ID assigned at registration. Never changes. |

### Free hosted tier

```
agent://nap/<category>/<uuid>
```

Example: `agent://nap/assistant/0195fa3c-…`

- Requires a free NAP account (email + password)
- Email must be verified before activation
- Supports up to 3 agents per account
- Identity claim: email-verified
- The `nap` org segment is a registry-controlled namespace — it signals to any
  caller that this is an email-verified hosted agent, not a domain-verified org.
  This prevents impersonation: a user who picks username `amazon` cannot produce
  `agent://amazon/shopping/<id>` — that URI requires DNS-01 proof of `amazon.com` ownership.

### Domain-verified tier

```
agent://<owner-domain>/<category>/<uuid>
```

Example: `agent://acme.com/finance/0195fa3c-…`

- No account needed — the DNS challenge proves domain ownership
- Requires a DNS-01 TXT record (same mechanism as Let's Encrypt)
- Unlimited agents under your org
- Identity claim: DNS-verified domain ownership (strongest)
- Domain agents receive an X.509 client certificate at activation (mTLS)

---

## 3. Installation

**Requirements:** Node.js 22 or later (uses native `fetch` and
`readline/promises`). No runtime npm dependencies.

```bash
# From within the OpenClaw repo (or your own project):
npm install ./integrations/openclaw

# Or, once published:
npm install @openclaw/nap
```

Build the TypeScript sources:

```bash
cd integrations/openclaw
npm install        # installs TypeScript dev dep
npm run build      # outputs to dist/
```

---

## 4. Configuration reference

The `NAPConfig` object is the single point of configuration. In an OpenClaw
deployment it lives under the `"nap"` key in
`~/.openclaw/openclaw.json`. In any other project, construct it however suits
your config system.

```ts
interface NAPConfig {
  enabled: boolean;          // Must be true to activate any NAP behaviour
  registry_url?: string;     // Override for self-hosted. Default: "https://registry.nexusagentprotocol.com"
  display_name: string;      // Human-readable name shown in listings
  description?: string;      // Optional one-line description of what this agent does
  endpoint?: string;         // Publicly reachable HTTPS URL of your gateway
                             // Leave empty to set later via: openclaw nap update-endpoint
  owner_domain?: string;     // Set to use domain-verified path. Omit for free hosted.
  capability?: string;       // Top-level URI category. Required for both paths.
                             // Full path stored (e.g. "finance>accounting") but only
                             // top-level appears in URI: agent://<org>/finance/<id>
}
```

### Minimal free-hosted config

```json
{
  "nap": {
    "enabled": true,
    "display_name": "Alice's Assistant",
    "capability": "assistant",
    "endpoint": "https://alice.tailnet.ts.net:18789"
  }
}
```

Produces URI: `agent://nap/assistant/<uuid>`

### Domain-verified config

```json
{
  "nap": {
    "enabled": true,
    "display_name": "ACME Corp Assistant",
    "description": "Internal productivity assistant for ACME employees",
    "owner_domain": "acme.com",
    "capability": "finance>accounting",
    "endpoint": "https://assistant.acme.com"
  }
}
```

Produces URI: `agent://acme.com/finance/<uuid>` — only `finance` (the top-level) appears in the URI; `accounting` is indexed for search but not encoded in the address.

### Field notes

| Field | Required | Notes |
|-------|----------|-------|
| `enabled` | Yes | Guards all startup behaviour; set to `false` to disable without removing config |
| `display_name` | Yes | Max 128 chars |
| `capability` | Yes | Full taxonomy path e.g. `"finance"`, `"finance>accounting"`, `"finance>accounting>reconciliation"`. Top-level segment becomes the second URI segment (after org). |
| `registry_url` | No | Trailing slash is stripped automatically |
| `description` | No | Shown in registry search results |
| `endpoint` | No | Can be set after registration via PATCH; must be a full HTTPS URL |
| `owner_domain` | No | Presence switches to domain-verified path. Proves ownership via DNS-01 challenge. |

---

## 5. State file reference

All registration state is written to `~/.openclaw/nap.json` with mode `0600`
(owner-readable only). The file is created on first `registerAgent()` call and
updated by `activateAgent()`.

### Full state shape

```ts
interface NAPState {
  agent_id: string;            // UUID assigned by registry — never changes
  agent_uri: string;           // Stable agent:// URI — never changes
  status: 'pending'            // Registered, awaiting email/DNS verification
         | 'active'            // Fully verified and live
         | 'revoked'           // Manually revoked
         | 'expired';          // Inactive for 90 days (domain-verified only)

  // Free-hosted only
  user_token?: string;         // 24-hour JWT. Used to auth PATCH/activate calls.
  email?: string;              // Stored to enable re-auth prompts when token expires

  // Domain-verified only (issued at activation)
  task_token?: string;         // Opaque token. Required for revoke/delete operations.

  // Set after activation
  agent_card_json?: string;    // A2A-compatible agent card, ready to serve verbatim

  registered_at: string;       // ISO 8601 timestamp
  endpoint_synced_at?: string; // ISO 8601 timestamp of last successful PATCH
}
```

### Example — free hosted, active

```json
{
  "agent_id": "0195fa3c-7d2e-7d2e-8b1c-0195fa3c7d2e",
  "agent_uri": "agent://nap/assistant/0195fa3c-7d2e-7d2e-8b1c-0195fa3c7d2e",
  "status": "active",
  "user_token": "eyJhbGci...",
  "email": "alice@example.com",
  "agent_card_json": "{\"name\":\"Alice's Assistant\",\"url\":\"https://alice.tailnet.ts.net:18789\",\"nap_endorsement\":\"eyJ...\"}",
  "registered_at": "2026-02-22T14:00:00.000Z",
  "endpoint_synced_at": "2026-02-22T14:00:12.000Z"
}
```

### Inspecting state

```bash
cat ~/.openclaw/nap.json | jq '{uri: .agent_uri, status: .status}'
cat ~/.openclaw/nap.json | jq '.agent_card_json | fromjson'
```

---

## 6. Step-by-step: free hosted tier

### Step 1 — Register

```bash
openclaw nap register
# or programmatically:
```

```ts
import { registerAgent } from '@openclaw/nap/register';

const state = await registerAgent(config, {
  email: 'alice@example.com',
  password: 'supersecret123',
  log: console.log,
});

console.log(state.agent_uri);   // agent://nap/assistant/…
console.log(state.status);      // "pending"
```

What happens internally:
1. Calls `POST /api/v1/auth/signup` with email + password
2. If email already exists (HTTP 409), falls through to `POST /api/v1/auth/login`
3. Calls `POST /api/v1/agents` with `registration_type: "nap_hosted"` + Bearer token
4. Writes `~/.openclaw/nap.json` with `status: "pending"`
5. Returns the state object

**If signup succeeds:** The registry sends a verification email. The link points
to your frontend's `/verify-email?token=<token>` page.

**If the account already existed:** Login is attempted automatically — no
separate signup needed.

### Step 2 — Verify email

Click the link in the verification email. It hits your frontend at:

```
https://yourfrontend.com/verify-email?token=<one-time-token>
```

Which POSTs to the backend:

```
POST /api/v1/auth/verify-email
Content-Type: application/json

{"token": "<one-time-token>"}
```

Alternatively, pass the token as a query param:

```
POST /api/v1/auth/verify-email?token=<one-time-token>
```

### Step 3 — Activate

```bash
openclaw nap activate
# or programmatically:
```

```ts
import { activateAgent } from '@openclaw/nap/register';

const state = await activateAgent(config, { log: console.log });

console.log(state.status);           // "active"
console.log(state.agent_card_json);  // A2A card JSON string
```

What happens internally:
1. Loads state from `~/.openclaw/nap.json`
2. Checks `user_token` is still fresh (24-hour window)
3. Calls `POST /api/v1/agents/<id>/activate` with Bearer token
4. Receives `agent_card_json` (A2A card) and optionally `task_token`
5. Writes updated state with `status: "active"`

---

## 7. Step-by-step: domain-verified tier

### Step 1 — Register

```ts
import { registerAgent } from '@openclaw/nap/register';

const state = await registerAgent({
  enabled: true,
  display_name: 'ACME Assistant',
  owner_domain: 'acme.com',
  capability: 'assistant',
  endpoint: 'https://assistant.acme.com',
}, { log: console.log });

// state.status === "pending"
```

The log output prints the DNS challenge instructions:

```
Registered: agent://acme.com/assistant/<uuid>
Next: complete DNS-01 verification for acme.com
  POST https://registry.nexusagentprotocol.com/api/v1/dns/challenge
  body: {"domain": "acme.com"}
```

### Step 2 — Complete DNS-01 challenge

**Start the challenge:**

```bash
curl -X POST https://registry.nexusagentprotocol.com/api/v1/dns/challenge \
  -H "Content-Type: application/json" \
  -d '{"domain": "acme.com"}'
```

Response:

```json
{
  "id": "<challenge-uuid>",
  "domain": "acme.com",
  "txt_host": "_nap-challenge.acme.com",
  "txt_value": "nap-challenge=<token>",
  "status": "pending",
  "expires_at": "2026-02-22T15:00:00Z"
}
```

**Add the DNS TXT record** at your DNS provider:

```
Host:  _nap-challenge.acme.com
Type:  TXT
Value: nap-challenge=<token>
TTL:   300 (or lowest available)
```

**Trigger verification:**

```bash
curl -X POST https://registry.nexusagentprotocol.com/api/v1/dns/challenge/<challenge-uuid>/verify
```

Poll status:

```bash
curl https://registry.nexusagentprotocol.com/api/v1/dns/challenge/<challenge-uuid>
# Wait for: {"status": "verified"}
```

### Step 3 — Activate

```bash
openclaw nap activate
```

For domain-verified agents, the activation response includes an X.509
certificate and private key pair for mTLS:

```
⚠️  Store this private key securely — it will NOT be shown again:
-----BEGIN RSA PRIVATE KEY-----
...
-----END RSA PRIVATE KEY-----
Certificate serial: 0x1A2B3C...
```

Save the private key to a secure location (e.g. `~/.openclaw/agent.key`).
It will never be retrievable again from the registry.

---

## 8. Gateway integration

### Minimal — Node.js `http` server

```ts
import http from 'node:http';
import { napStartupHook, createAgentCardHandler, startNAPSync } from '@openclaw/nap/gateway';
import type { NAPConfig } from '@openclaw/nap/types';

const napConfig: NAPConfig = { /* ... */ };
const currentEndpoint = 'https://mygateway.example.com';

// 1. Run startup hook once — syncs endpoint, logs status
let napState = await napStartupHook(napConfig, currentEndpoint);

// 2. Create the /.well-known/agent.json handler (can handle null state)
let agentCardHandler = createAgentCardHandler(napState);

// 3. Start periodic sync (1 hour default) and update the handler on change
const stopSync = startNAPSync(
  napConfig,
  () => currentEndpoint,
  (updatedState) => {
    napState = updatedState;
    agentCardHandler = createAgentCardHandler(updatedState);
  },
);

// 4. Wire before your main router
const server = http.createServer((req, res) => {
  if (agentCardHandler(req, res)) return;  // returns true = handled
  mainRouter(req, res);
});

// 5. Clean up on shutdown
process.on('SIGTERM', () => {
  stopSync();
  server.close();
});
```

### Express

```ts
import express from 'express';
import { napStartupHook, createAgentCardHandler } from '@openclaw/nap/gateway';

const app = express();
const napState = await napStartupHook(napConfig, endpoint);
const agentCardHandler = createAgentCardHandler(napState);

// Register before other routes
app.use((req, res, next) => {
  // createAgentCardHandler returns true if it handled the request
  if (!agentCardHandler(req as any, res as any)) next();
});

// ... rest of routes
```

### Fastify

```ts
import Fastify from 'fastify';
import { napStartupHook, createAgentCardHandler } from '@openclaw/nap/gateway';

const fastify = Fastify();
const napState = await napStartupHook(napConfig, endpoint);
const agentCardHandler = createAgentCardHandler(napState);

fastify.addHook('onRequest', async (request, reply) => {
  const handled = agentCardHandler(request.raw, reply.raw);
  if (handled) {
    reply.hijack();
  }
});
```

### What `napStartupHook` does at boot

1. Returns `null` immediately if `config.enabled === false`
2. Returns `null` if no `~/.openclaw/nap.json` exists (not yet registered)
3. If `status !== 'active'`: logs a reminder and returns the pending state
4. If `status === 'active'` and a `user_token` exists and `currentEndpoint` is
   non-empty: calls `PATCH /api/v1/agents/<id>` to sync the endpoint — but
   catches and logs any error rather than failing startup
5. Returns the loaded `NAPState`

### What `createAgentCardHandler` serves

```
GET /.well-known/agent.json
→ 200  Content-Type: application/json
       Cache-Control: public, max-age=3600
       <agent_card_json verbatim>

HEAD /.well-known/agent.json
→ 200  (no body)

GET /.well-known/agent.json  (when card not yet available)
→ 404  {"error": "agent card not yet available"}

POST/PUT/DELETE /.well-known/agent.json
→ (not intercepted, falls through to your router)
```

Any other URL returns `false` and is not intercepted.

### `startNAPSync` options

```ts
startNAPSync(
  config,
  () => getCurrentEndpoint(),   // Called each tick — allows dynamic endpoint
  (state) => { /* update your in-memory state */ },
  {
    log: myLogger.info,          // Optional custom logger
    intervalMs: 60 * 60 * 1000, // Default: 1 hour
  },
);
```

The timer uses `.unref()` so it does not prevent the process from exiting
if everything else has shut down.

---

## 9. Public API reference

All exports are available from the package root or from sub-path exports.

```ts
// All exports
import { ... } from '@openclaw/nap';

// Sub-path exports (tree-shake friendly)
import { registerAgent, activateAgent } from '@openclaw/nap/register';
import { onboardWizard, activateWizard } from '@openclaw/nap/onboard';
import { napStartupHook, createAgentCardHandler, startNAPSync } from '@openclaw/nap/gateway';
import { loadState, saveState, clearState, isTokenFresh } from '@openclaw/nap/state';  // Note: from state.js not a sub-path
import { NAPClient, NAPError } from '@openclaw/nap';
import type { NAPConfig, NAPState, NAPAuthResponse, NAPRegisterResponse, NAPActivateResponse } from '@openclaw/nap/types';
```

---

### `registerAgent(config, opts): Promise<NAPState>`

Core registration function. Writes `~/.openclaw/nap.json`.

```ts
registerAgent(
  config: NAPConfig,
  opts?: {
    email?: string;     // Required for nap_hosted path
    password?: string;  // Required for nap_hosted path
    log?: (msg: string) => void;  // Default: console.log
  }
): Promise<NAPState>
```

**Throws:**
- `Error('email and password are required for the free hosted tier')` if
  `owner_domain` is not set and email/password are omitted
- `NAPError` for any HTTP failure from the registry
- Re-throws signup errors except HTTP 409 (already exists → falls back to login)

---

### `activateAgent(config, opts): Promise<NAPState>`

Activates a registered agent. Loads state from disk, calls the activate endpoint,
writes updated state back.

```ts
activateAgent(
  config: NAPConfig,
  opts?: {
    log?: (msg: string) => void;
  }
): Promise<NAPState>
```

**Throws:**
- `Error('No NAP registration found. Run: openclaw nap register')` if no state file
- `Error('NAP user token has expired. Re-authenticate with: openclaw nap login')` if
  token is stale (free-hosted only)
- `NAPError` for HTTP failures

**Side effects:**
- Prints the private key PEM to the log if `result.private_key_pem` is present
  (domain-verified only, issued once)
- Writes `status: "active"` + `agent_card_json` + `task_token` to state file

---

### `onboardWizard(config, opts): Promise<void>`

Interactive CLI registration wizard. Reads from `stdin`/`stdout`. Prompts for
any fields missing from `config`.

```ts
onboardWizard(
  config: Partial<NAPConfig>,  // Pre-populated fields skip their prompts
  opts?: {
    log?: (msg: string) => void;
  }
): Promise<void>
```

Wizard flow:
1. Checks for existing registration (offers to overwrite)
2. Asks for registration type (1=hosted, 2=domain) unless `owner_domain` is set
3. Prompts for `display_name`, `description`, `endpoint`
4. For hosted: prompts email + password (password uses best-effort TTY hiding)
5. For domain: prompts `owner_domain` + `capability`
6. Calls `registerAgent()` and prints next-step instructions

---

### `activateWizard(config, opts): Promise<void>`

Thin wrapper around `activateAgent()` for CLI use. Logs state before activating.

```ts
activateWizard(
  config: NAPConfig,
  opts?: { log?: (msg: string) => void }
): Promise<void>
```

---

### `napStartupHook(config, currentEndpoint, opts): Promise<NAPState | null>`

Run once at Gateway startup. See [Gateway integration](#8-gateway-integration).

```ts
napStartupHook(
  config: NAPConfig,
  currentEndpoint: string,
  opts?: { log?: (msg: string) => void }
): Promise<NAPState | null>
```

Returns `null` when NAP is disabled or not yet registered.
Never throws — endpoint sync errors are caught and logged.

---

### `createAgentCardHandler(state): (req, res) => boolean`

Returns a raw Node.js HTTP handler for `/.well-known/agent.json`.

```ts
createAgentCardHandler(
  state: NAPState | null
): (req: IncomingMessage, res: ServerResponse) => boolean
```

Returns `true` if the request was handled, `false` if it should fall through.
Safe to call with `null` state — returns a 404 handler.

---

### `startNAPSync(config, getEndpoint, onStateUpdate, opts): () => void`

Starts a background timer that syncs the endpoint to the registry.
Returns a cleanup function.

```ts
startNAPSync(
  config: NAPConfig,
  getEndpoint: () => string,
  onStateUpdate: (state: NAPState) => void,
  opts?: {
    log?: (msg: string) => void;
    intervalMs?: number;          // Default: 3_600_000 (1 hour)
  }
): () => void   // call to stop the timer
```

Only syncs when:
- State file exists and `status === 'active'`
- A `user_token` is present (free-hosted only; domain agents have static endpoints)
- `getEndpoint()` returns a non-empty string

---

### `loadState(): Promise<NAPState | null>`

Reads `~/.openclaw/nap.json`. Returns `null` if the file does not exist or
cannot be parsed.

### `saveState(state: NAPState): Promise<void>`

Writes `~/.openclaw/nap.json` with mode `0600`.

### `clearState(): Promise<void>`

Deletes `~/.openclaw/nap.json`. Silent if already absent.

### `isTokenFresh(state: NAPState): boolean`

Returns `true` if `user_token` decodes to a JWT with `exp` more than 5 minutes
in the future. Returns `false` if the token is missing, malformed, or expired.

---

### `NAPClient`

Low-level REST client. Use this if you need to call the registry directly from
your own code without the higher-level helpers.

```ts
const client = new NAPClient(registryURL?);  // defaults to production registry

// Auth
await client.signup(email, password, displayName?): Promise<NAPAuthResponse>
await client.login(email, password): Promise<NAPAuthResponse>

// Agents
await client.registerHosted(userToken, displayName, endpoint, description?): Promise<NAPRegisterResponse>
await client.registerDomain(ownerDomain, capability, displayName, endpoint, description?): Promise<NAPRegisterResponse>
await client.activate(agentUUID, userToken?): Promise<NAPActivateResponse>
await client.updateEndpoint(agentUUID, endpoint, token): Promise<void>
```

All methods throw `NAPError` on non-2xx responses.

---

### `NAPError`

```ts
class NAPError extends Error {
  name: 'NAPError';
  status: number;   // HTTP status code
  message: string;  // Error message from registry "error" field, or "HTTP <status>"
}
```

---

## 10. Backend API reference

These are the registry endpoints this package calls. Useful if you need to
call them directly (e.g. from curl, Postman, or a custom script).

**Base URL:** `https://registry.nexusagentprotocol.com`

### Auth

#### `POST /api/v1/auth/signup`

```json
// Request
{ "email": "alice@example.com", "password": "supersecret123", "display_name": "Alice" }

// 201 Created
{ "token": "<jwt>", "user": { "id": "<uuid>", "email": "...", "username": "alice", "tier": "free" } }

// 409 Conflict — email already registered
{ "error": "email already in use" }
```

#### `POST /api/v1/auth/login`

```json
// Request
{ "email": "alice@example.com", "password": "supersecret123" }

// 200 OK
{ "token": "<jwt>", "user": { "id": "<uuid>", "email": "...", "username": "alice", "tier": "free" } }

// 401 Unauthorized
{ "error": "invalid credentials" }
```

#### `POST /api/v1/auth/verify-email`

```json
// Request (body or ?token= query param)
{ "token": "<one-time-token-from-email>" }

// 200 OK
{ "message": "email verified" }

// 400 Bad Request
{ "error": "invalid or expired token" }
```

#### `POST /api/v1/auth/resend-verification`

```json
// Request
{ "email": "alice@example.com" }

// 200 OK
{ "message": "verification email sent" }
```

### Agents

#### `POST /api/v1/agents` — free hosted

```
Authorization: Bearer <user-jwt>
```

```json
// Request
{
  "registration_type": "nap_hosted",
  "display_name": "Alice's Assistant",
  "description": "Personal productivity assistant",
  "endpoint": "https://alice.tailnet.ts.net:18789"
}

// 201 Created
{
  "id": "<uuid>",
  "agent_id": "<uuid>",
  "agent_uri": "agent://nap/assistant/<uuid>",
  "uri": "agent://nap/assistant/<uuid>",
  "status": "pending"
}

// 403 Forbidden — quota exceeded (3 agents per free account)
{ "error": "free tier agent limit reached" }
```

#### `POST /api/v1/agents` — domain-verified

No auth header required.

```json
// Request
{
  "trust_root": "acme.com",
  "owner_domain": "acme.com",
  "capability_node": "assistant",
  "display_name": "ACME Assistant",
  "description": "Internal assistant",
  "endpoint": "https://assistant.acme.com"
}

// 201 Created
{
  "id": "<uuid>",
  "agent_uri": "agent://acme.com/assistant/<uuid>",
  "status": "pending"
}
```

#### `POST /api/v1/agents/<uuid>/activate`

```
Authorization: Bearer <user-jwt>   (free-hosted only; omit for domain-verified)
```

```json
// 200 OK — free hosted
{
  "status": "active",
  "agent_card_json": "{\"name\":\"Alice's Assistant\",\"url\":\"...\",\"nap_endorsement\":\"eyJ...\"}",
  "task_token": "<opaque-token>"
}

// 200 OK — domain-verified (additional fields)
{
  "status": "active",
  "agent_card_json": "...",
  "task_token": "<opaque-token>",
  "private_key_pem": "-----BEGIN RSA PRIVATE KEY-----\n...",
  "certificate": { "serial": "0x1A2B3C", "pem": "-----BEGIN CERTIFICATE-----\n..." },
  "ca_pem": "-----BEGIN CERTIFICATE-----\n..."
}

// 403 Forbidden — email not verified (free-hosted)
{ "error": "email not verified" }

// 403 Forbidden — DNS challenge not complete (domain-verified)
{ "error": "domain not verified" }
```

#### `PATCH /api/v1/agents/<uuid>`

```
Authorization: Bearer <user-jwt>
```

```json
// Request
{ "endpoint": "https://new-address.tailnet.ts.net:18789" }

// 200 OK  (no body)
```

### DNS Challenge

#### `POST /api/v1/dns/challenge`

```json
// Request
{ "domain": "acme.com" }

// 201 Created
{
  "id": "<uuid>",
  "domain": "acme.com",
  "txt_host": "_nap-challenge.acme.com",
  "txt_value": "nap-challenge=<token>",
  "status": "pending",
  "expires_at": "2026-02-22T15:00:00Z"
}
```

#### `GET /api/v1/dns/challenge/<uuid>`

```json
// 200 OK
{ "id": "<uuid>", "domain": "acme.com", "status": "pending|verified|failed", "expires_at": "..." }
```

#### `POST /api/v1/dns/challenge/<uuid>/verify`

Triggers a live DNS TXT lookup. Returns the updated challenge status.

```json
// 200 OK — verified
{ "status": "verified" }

// 200 OK — TXT record not yet visible
{ "status": "pending", "error": "TXT record not found" }
```

---

## 11. Error handling

```ts
import { NAPClient, NAPError } from '@openclaw/nap';

const client = new NAPClient();

try {
  await client.login('alice@example.com', 'wrongpassword');
} catch (err) {
  if (err instanceof NAPError) {
    console.error(err.status);   // 401
    console.error(err.message);  // "invalid credentials"
  } else {
    throw err;  // network error, JSON parse error, etc.
  }
}
```

### Status codes to handle

| Code | Meaning | Recovery |
|------|---------|----------|
| 400 | Bad request / validation error | Fix request payload |
| 401 | Invalid credentials | Re-prompt for password |
| 403 | Quota exceeded, email not verified, or DNS not verified | Follow the error message |
| 409 | Email already registered | Fall back to login (handled automatically by `registerAgent`) |
| 422 | Unprocessable — missing required field | Fix request payload |
| 429 | Rate limited | Back off and retry |
| 500 | Registry internal error | Retry with exponential backoff |

---

## 12. Token lifecycle

### Free-hosted user token

- Issued by `POST /api/v1/auth/signup` or `/login`
- **Lifetime: 24 hours**
- Stored in `~/.openclaw/nap.json` as `user_token`
- Used to authenticate agent registration and endpoint PATCH calls
- `isTokenFresh(state)` decodes the JWT `exp` claim and returns `false`
  if within 5 minutes of expiry

When the token expires:

```
[NAP] Warning: could not sync endpoint — NAP user token has expired.
Re-authenticate with: openclaw nap login
```

To re-authenticate (not yet implemented as a CLI command — open PR):

```ts
const client = new NAPClient(config.registry_url);
const auth = await client.login(email, password);
const state = await loadState();
if (state) {
  state.user_token = auth.token;
  await saveState(state);
}
```

### Domain-verified task token

- Issued once by `POST /api/v1/agents/<uuid>/activate`
- **Does not expire** (long-lived opaque token)
- Stored in `~/.openclaw/nap.json` as `task_token`
- Required for future revoke/delete operations (not yet implemented)

---

## 13. Self-hosted registry

You can run the NAP registry yourself. Set `registry_url` in your config:

```json
{
  "nap": {
    "enabled": true,
    "registry_url": "https://registry.internal.acme.com",
    "display_name": "Internal Assistant"
  }
}
```

The registry source is at the root of the `Agent-registration` repository.
Run with Docker:

```bash
docker-compose up -d
```

The default local URL is `http://localhost:8080`.

For local development and testing:

```json
{
  "nap": {
    "enabled": true,
    "registry_url": "http://localhost:8080",
    "display_name": "Dev Assistant",
    "endpoint": "http://localhost:18789"
  }
}
```

Note: the registry requires HTTPS for production `endpoint` values, but
accepts `http://localhost` for local development.

---

## 14. Troubleshooting

### "No NAP registration found"

`~/.openclaw/nap.json` is missing. Run `openclaw nap register`.

### "NAP user token has expired"

Your 24-hour user JWT has expired. Re-authenticate:

```bash
# Not yet a built-in command; use the API directly:
curl -X POST https://registry.nexusagentprotocol.com/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"you@example.com","password":"yourpassword"}'
# Then manually update "user_token" in ~/.openclaw/nap.json
```

### "email not verified" on activate

You must click the verification link sent to your email before activating.
Resend it:

```bash
curl -X POST https://registry.nexusagentprotocol.com/api/v1/auth/resend-verification \
  -H "Content-Type: application/json" \
  -d '{"email":"you@example.com"}'
```

### "domain not verified" on activate

The DNS-01 challenge is not complete. Check:

```bash
dig TXT _nap-challenge.yourdomain.com
# Should show: "nap-challenge=<token>"
```

If the record shows but verify still fails, DNS propagation may not be
complete. Wait 5 minutes and retry:

```bash
curl -X POST https://registry.nexusagentprotocol.com/api/v1/dns/challenge/<id>/verify
```

### `/.well-known/agent.json` returns 404

The agent is registered but not yet activated. Run `openclaw nap activate`.

### Agent card shows stale endpoint

The `startNAPSync` timer runs hourly by default. Force a sync:

```ts
// Restart the Gateway, or call updateEndpoint directly:
const client = new NAPClient(config.registry_url);
await client.updateEndpoint(state.agent_id, newEndpoint, state.user_token!);
```

### TypeScript: `exactOptionalPropertyTypes` errors

This package uses strict TypeScript (`exactOptionalPropertyTypes: true`).
When constructing `NAPConfig`, only set optional fields conditionally:

```ts
// Wrong — assigns undefined to an optional-but-not-undefinable field
const config: NAPConfig = { enabled: true, display_name: 'x', registry_url: undefined };

// Correct — omit the key entirely
const config: NAPConfig = { enabled: true, display_name: 'x' };
if (registryURL) config.registry_url = registryURL;
```

---

## License

MIT
