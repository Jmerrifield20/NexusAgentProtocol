# NAP Federation Protocol

**Version:** 1.0
**Status:** Draft

---

## 1. Overview

The Nexus Agent Protocol (NAP) registry is designed to be both self-hostable and
globally federated. A single binary supports three operational modes controlled
by the `registry.role` configuration key.

| Role | Operated by | Capabilities |
|------|------------|--------------|
| `standalone` | White-label buyer | Full registry; isolated trust domain; no cross-registry resolution |
| `federated` | Licensed operator | Intermediate CA from root; cross-registry resolution via DNS or root fallback |
| `root` | nexusagentprotocol.com | Issues intermediate CAs; registry-of-registries; global resolution proxy |

The **moat** is not the code (Apache 2.0). It is:
1. The **root CA private key** — only the root registry can issue intermediate CAs.
2. The **`nap` namespace** — free-tier agents (`agent://nap/…`) are guaranteed to
   come from the root registry, ensuring email-verified identity.
3. The **"NAP Certified" trademark** — indicates a registry is operating under a
   valid intermediate CA issued by the root.

---

## 2. CA Hierarchy

```
Root CA (nexusagentprotocol.com)
│   ValidFor: 10 years   MaxPathLen: unrestricted
│
├── Intermediate CA — acme.com
│   ValidFor: 5 years    MaxPathLen: 0 (cannot issue further intermediates)
│   │
│   └── Leaf cert — agent://acme.com/finance/agent_xyz
│       ValidFor: 1 year
│
└── Intermediate CA — globalbank.io
    ValidFor: 5 years    MaxPathLen: 0
    │
    └── Leaf cert — agent://globalbank.io/finance/agent_abc
        ValidFor: 1 year
```

**MaxPathLen=0** on all intermediate CAs ensures that federated registries
cannot sub-delegate CA authority. Only the root can issue intermediate CAs.

---

## 3. Registry Discovery Priority

When resolving `agent://{trustRoot}/{category}/{agentID}`, a federated or root
registry applies the following discovery cascade:

| Priority | Mechanism | Format |
|----------|-----------|--------|
| 1 | **Federation table lookup** | `registered_registries` WHERE `trust_root = ?` AND `status = 'active'` |
| 2 | **DNS TXT record** | `_nap-registry.{trustRoot}` → `v=nap1 url=https://registry.example.com` |
| 3 | **Root registry fallback** | Configured `federation.root_registry_url` |

A suspended registry (status=`suspended`) is skipped at step 1; DNS and
root fallback also fail if the root registry actively blocks the trust root.

---

## 4. Operator Onboarding Flow

An operator (e.g. `acme.com`) joins the federation in six steps:

1. **Register** — `POST /api/v1/federation/register` on the root registry:
   ```json
   { "trust_root": "acme.com", "endpoint_url": "https://registry.acme.com", "contact_email": "ops@acme.com" }
   ```
   Status: `pending`.

2. **Review** — Root registry operator reviews the application via
   `GET /api/v1/federation/registries?status=pending`.

3. **Approve** — `POST /api/v1/federation/registries/{id}/approve`
   Status: `active`.

4. **Issue intermediate CA** — `POST /api/v1/federation/issue-ca`:
   ```json
   { "trust_root": "acme.com" }
   ```
   Response includes `cert_pem`, `key_pem` (one-time), `root_ca_pem`.
   The private key is **never persisted** by the root registry.

5. **Configure** — The operator saves `cert_pem` and `key_pem` to disk, then
   sets:
   ```yaml
   registry:
     role: federated
   federation:
     intermediate_ca_cert: /etc/nap/intermediate.crt
     intermediate_ca_key:  /etc/nap/intermediate.key
     root_registry_url: https://registry.nexusagentprotocol.com
   ```

6. **Start** — The federated registry boots, loads the intermediate CA, and
   begins issuing leaf certs signed by the `acme.com` intermediate.
   Agents receive URIs of the form `agent://acme.com/{category}/{agentID}`.

---

## 5. Cross-Registry Resolution Sequence

```
Client                  Acme Registry            Root Registry
  │                         │                         │
  │── GET /api/v1/resolve ──>│                         │
  │   trust_root=globalbank  │                         │
  │                         │── (1) table miss ────────│
  │                         │── (2) DNS TXT lookup     │
  │                         │   _nap-registry.globalbank.io
  │                         │   → url=https://registry.globalbank.io
  │                         │── GET /api/v1/resolve ──>registry.globalbank.io
  │                         │<── 200 agent JSON ───────│
  │<── 200 agent JSON ──────│
```

If DNS lookup fails and no table entry exists, the request falls back to the
configured `root_registry_url`, which can act as a global resolution proxy.

---

## 6. Security Notes

- **MaxPathLen=0** — Federated registries cannot issue further intermediates.
  Only the root CA can extend the chain.
- **Suspended blocking** — A suspended registry is excluded from table lookup.
  Operators should also publish a DNS revocation signal by removing or updating
  the `_nap-registry` TXT record.
- **Reserved namespaces** — `nap` and `nexusagentprotocol.com` are reserved
  trust roots. The registry rejects registration requests using these values.
- **Key delivery** — Intermediate CA private keys are returned once in the API
  response and never stored. If lost, a new intermediate CA must be issued.
- **mTLS verification** — In federated mode, peer certificates are verified
  against `rootCAPool` (Roots) with the intermediate cert in the Intermediates
  pool, ensuring the full chain is validated.

---

## 7. API Reference

### POST /api/v1/federation/register
Register a new federation member application.
Available in `federated` and `root` modes.

**Body:** `RegisterRequest`
**Response 201:** `RegisteredRegistry` (status=`pending`)

### GET /api/v1/federation/registries *(root only)*
List registered registries. Optional `?status=pending|active|suspended` filter.

**Response 200:** `{ "registries": [RegisteredRegistry] }`

### POST /api/v1/federation/issue-ca *(root only)*
Issue an intermediate CA certificate for an approved registry.

**Body:** `{ "trust_root": "acme.com" }`
**Response 200:** `IssueCAResponse` (includes `cert_pem`, `key_pem`, `root_ca_pem`)

### POST /api/v1/federation/registries/:id/approve *(root only)*
Approve a pending registry.
**Response 200:** `RegisteredRegistry` (status=`active`)

### POST /api/v1/federation/registries/:id/suspend *(root only)*
Suspend an active registry, blocking cross-registry resolution.
**Response 200:** `RegisteredRegistry` (status=`suspended`)
