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
│   ValidFor: 5 years    MaxPathLen: 0 (default — cannot issue further intermediates)
│   │
│   └── Leaf cert — agent://acme.com/finance/billing/agent_xyz
│       ValidFor: 1 year
│
├── Intermediate CA — gov.kr (sub-delegation enabled)
│   ValidFor: 5 years    MaxPathLen: 1 (can issue one level of sub-intermediates)
│   │
│   ├── Sub-Intermediate CA — molit.go.kr
│   │   ValidFor: 5 years    MaxPathLen: 0
│   │   │
│   │   └── Leaf cert — agent://molit.go.kr/transport/logistics/agent_abc
│   │       ValidFor: 1 year
│   │
│   └── Leaf cert — agent://gov.kr/governance/agent_def
│       ValidFor: 1 year   (top-level capability — 3-segment URI)
│
└── Intermediate CA — globalbank.io
    ValidFor: 5 years    MaxPathLen: 0
    │
    └── Leaf cert — agent://globalbank.io/finance/reconciliation/agent_abc
        ValidFor: 1 year
```

By default, **MaxPathLen=0** on intermediate CAs prevents sub-delegation.
The root admin can grant sub-delegation to specific registries by increasing
their `max_path_len` before issuing the intermediate CA (see Section 8).

---

## 3. Registry Discovery Priority

When resolving `agent://{trustRoot}/{category}/{agentID}`, a federated or root
registry applies the following discovery cascade:

| Priority | Mechanism | Format |
|----------|-----------|--------|
| 1 | **Federation table lookup** | `registered_registries` WHERE `trust_root = ?` AND `status = 'active'` |
| 2 | **DNS TXT record** | `_nap-registry.{trustRoot}` → `v=nap1 url=https://registry.example.com` |
| 3 | **Root registry fallback** | Configured `federation.root_registry_url` |

A suspended registry (status=`suspended`) is skipped at step 1.

**DNS results require root approval.** When a registry has `fedSvc` available
(i.e. it is the root registry), DNS-discovered endpoints at step 2 are
cross-referenced against the `registered_registries` table. The DNS result is
only accepted if the trust root has an **active** entry. Unapproved or
suspended trust roots are rejected and discovery falls through to step 3.
This ensures the root registry remains the single source of truth for
federation membership — publishing a `_nap-registry` TXT record alone is
not sufficient to join the federation.

Federated registries (which have `fedSvc = nil`) skip DNS discovery entirely
and fall through to the root registry fallback, which is the correct behavior
since the root is the authority.

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
   Agents receive URIs of the form `agent://acme.com/{category}/{primarySkill}/{agentID}`
   (or the 3-segment form `agent://acme.com/{category}/{agentID}` when no skill is derived).

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

- **MaxPathLen control** — By default, all intermediate CAs are issued with
  `MaxPathLen=0`, preventing sub-delegation. The root admin can grant
  sub-delegation to specific registries (see Section 8). A hard cap of
  `MaxAllowedPathLen=2` prevents excessively deep CA chains.
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
- **Sub-delegation revocation** — Setting `max_path_len` back to 0 and
  re-issuing the intermediate CA revokes sub-delegation authority. The
  operator must redeploy with the new certificate.

---

## 7. Sub-Delegation

Sub-delegation allows the root admin to grant specific registries the ability
to issue their own sub-intermediate CAs. This is useful for jurisdictional
authority — for example, a government registry (`gov.kr`) that needs to onboard
its own ministries (`molit.go.kr`, `moe.go.kr`).

### Workflow

```
1. Register Korea:    POST /federation/register {trust_root: "gov.kr", ...}
2. Approve:           POST /federation/registries/{id}/approve
3. Grant delegation:  PATCH /federation/registries/{id}/delegation {max_path_len: 1}
4. Issue CA:          POST /federation/issue-ca {trust_root: "gov.kr"}
                      → cert has MaxPathLen=1 baked in
5. Deliver cert+key to Korean counterpart (out of band)

Later, to revoke sub-delegation:
6. Revoke:            PATCH /federation/registries/{id}/delegation {max_path_len: 0}
7. Re-issue CA:       POST /federation/issue-ca {trust_root: "gov.kr"}
                      → new cert has MaxPathLen=0
8. Korean operator must redeploy with new cert
```

### Auto-Detection

When a federated registry boots with an intermediate CA cert that has
`MaxPathLen > 0`, it automatically enables root-like behaviour:
- The federation service is wired with the intermediate-mode issuer
- Root-only routes (approve, issue-ca, delegation) are enabled
- The registry can issue sub-intermediates to its sub-registries

No additional configuration is required — the X.509 certificate itself
carries the sub-delegation authority.

### Constraints

- `max_path_len` must be between 0 and `MaxAllowedPathLen` (currently 2)
- A sub-intermediate's `MaxPathLen` must be strictly less than its parent's
- Changes to `max_path_len` only take effect on the next CA re-issuance

---

## 8. API Reference

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

### PATCH /api/v1/federation/registries/:id/delegation *(root only)*
Update the sub-delegation depth for a registered registry.
The new value takes effect on the next CA re-issuance via `POST /federation/issue-ca`.

**Body:** `{ "max_path_len": 1 }`
**Response 200:** `{ "id": "...", "max_path_len": 1 }`
**Errors:** 400 if `max_path_len` is negative or exceeds `MaxAllowedPathLen` (2)
