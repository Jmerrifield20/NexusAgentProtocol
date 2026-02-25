# NAP URI Standard

**Version:** 1.1
**Status:** Finalized

---

## 1. Format

NAP URIs come in two forms. The 4-segment form is preferred for new registrations
when a meaningful skill can be derived; the 3-segment form is preserved for agents
whose capability is top-level only.

**4-segment (preferred):**
```
agent://<trust_root>/<category>/<primary_skill>/<agent_id>
```

**3-segment (legacy / top-level capability only):**
```
agent://<trust_root>/<category>/<agent_id>
```

| Segment | Description | Example |
|---------|-------------|---------|
| `trust_root` | Verified domain or reserved namespace | `acme.com`, `nap` |
| `category` | Top-level capability category (first segment of capability path) | `finance`, `legal` |
| `primary_skill` | Slugified skill identifier — omitted when capability is top-level only | `reconcile-invoices`, `contract-review` |
| `agent_id` | Registry-generated opaque identifier | `agent_3k2mn7p4q...` |

---

## 2. Examples

| URI | Registration type | Trust level |
|-----|------------------|-------------|
| `agent://nap/finance/reconcile-invoices/agent_abc123` | NAP-hosted (free tier) | Email-verified |
| `agent://nap/assistant/agent_xyz` | NAP-hosted, top-level capability only | Email-verified |
| `agent://acme.com/finance/billing/agent_xyz789` | Domain-verified | DNS-01 proven |
| `agent://acme.com/legal/agent_mnop` | Domain-verified, top-level capability only | DNS-01 proven |
| `agent://globalbank.io/finance/reconciliation/agent_pqrs` | Domain-verified (federated) | Intermediate CA |

---

## 3. Primary Skill Derivation

`primary_skill` is a URI-safe slug derived **once at registration** and never
changed. It is derived in priority order:

1. **Declared skills** — if the registration request includes an explicit
   `skills` array, `skills[0].id` is slugified (lowercased, spaces → hyphens).
2. **Capability path** — if the capability node has 2 or more levels
   (e.g. `finance>accounting>reconciliation`), the **last segment** becomes the
   primary skill (`reconciliation`).
3. **Top-level only** — if the capability is a single segment (e.g. `assistant`),
   `primary_skill` is left empty and the 3-segment URI form is used.

| Capability | Declared skills | `primary_skill` | URI form |
|------------|----------------|-----------------|----------|
| `finance>accounting>reconciliation` | — | `reconciliation` | 4-segment |
| `finance>accounting` | — | `accounting` | 4-segment |
| `finance` | — | *(empty)* | 3-segment |
| `finance>accounting` | `[{id: "reconcile-invoices"}]` | `reconcile-invoices` | 4-segment |
| `assistant` | `[{id: "code review"}]` | `code-review` | 4-segment |

---

## 4. URI Stability Guarantee

Once issued, a NAP URI is **permanent**:
- The registry never reuses `agent_id` values.
- `trust_root`, `category`, and `primary_skill` are all set at registration and
  cannot be changed.
- If an agent is revoked or deleted, its URI returns a 404/revoked status but
  is never reassigned.

---

## 5. Trust Root Semantics

### Free-tier (`nap`)
- Controlled exclusively by the root registry.
- Guarantees: registrant email address is verified.
- Does **not** guarantee the registrant owns any domain.
- URI: `agent://nap/{category}/{primary_skill}/{agent_id}`  or  `agent://nap/{category}/{agent_id}`

### Domain-verified
- The `trust_root` is the registrant's **full verified domain** as proven by
  DNS-01 challenge (e.g. `acme.com`).
- `acme.com` and `acme.io` are distinct trust roots — no abbreviation or slug.
- URI: `agent://{owner_domain}/{category}/{primary_skill}/{agent_id}`

### Federated domain-verified
- Same as domain-verified but the leaf certificate is signed by an intermediate
  CA issued by the NAP root, not the root CA directly.
- Clients verify the full chain: leaf → intermediate → root CA.

---

## 6. Resolution Algorithm

The resolution API accepts `capability_node` as the top-level category (e.g.
`finance`). The `primary_skill` segment is not required for resolution — it is
informational only. Prefix matching in the registry ensures agents stored under
`finance>accounting>reconciliation` are found when querying with `finance`.

```
Resolve(uri: agent://{trustRoot}/{category}/[{primarySkill}/]{agentID}):

  1. Query local registry:
     GET /api/v1/resolve?trust_root={trustRoot}&cap_node={category}&agent_id={agentID}
     → 200: return agent record
     → 404: continue

  2. If RemoteResolver is configured:
     a. Table lookup: SELECT endpoint_url FROM registered_registries
                       WHERE trust_root = {trustRoot} AND status = 'active'
     b. DNS TXT: _nap-registry.{trustRoot} → "v=nap1 url=https://..."
     c. Root fallback: configured root_registry_url
     → Forward request to discovered endpoint
     → Return proxied agent record

  3. Return 404 Not Found
```

---

## 7. Reserved Namespaces

The following trust roots are reserved and cannot be registered by operators:

| Namespace | Reserved for |
|-----------|--------------|
| `nap` | NAP-hosted free tier (root registry only) |
| `nexusagentprotocol.com` | Root registry itself |

Any `RegisterRequest` with a `trust_root` matching a reserved namespace is
rejected with a 400 error.

---

## 8. URI Construction

The `URI()` method on `model.Agent` produces the canonical URI:

```go
func (a *Agent) URI() string {
    category := TopLevelCategory(a.CapabilityNode) // first segment before ">"
    if a.PrimarySkill != "" {
        return "agent://" + a.TrustRoot + "/" + category + "/" + a.PrimarySkill + "/" + a.AgentID
    }
    return "agent://" + a.TrustRoot + "/" + category + "/" + a.AgentID
}
```

The full capability path is stored internally in `capability_node` and returned
in API responses. The `primary_skill` column stores the derived slug and is
indexed for structured search (see Section 9).

---

## 9. Skill & Tool Search

Three indexed columns on the `agents` table enable structured discovery:

| Column | Type | Description |
|--------|------|-------------|
| `primary_skill` | `TEXT` | The skill slug encoded in the URI |
| `skill_ids` | `TEXT[]` | All declared skill IDs (GIN-indexed) |
| `tool_names` | `TEXT[]` | All declared MCP tool names (GIN-indexed) |

These are queryable via the `GET /api/v1/agents` endpoint:

```
GET /api/v1/agents?skill=reconcile-invoices   → agents declaring that skill
GET /api/v1/agents?tool=parse_invoice         → agents exposing that MCP tool
```

---

## 10. A2A Compatibility

NAP URIs are embedded as URI SANs in X.509 agent certificates (scheme `agent`),
making them verifiable at the TLS layer. The A2A agent card (`agent.json`)
also carries the NAP URI in the `nap_uri` field alongside the standard A2A
`url` (agent endpoint) field.

```json
{
  "name": "Acme Finance Agent",
  "url": "https://agent.acme.com",
  "nap_uri": "agent://acme.com/finance/billing/agent_xyz789",
  "nap_trust_tier": "trusted",
  "nap_registry": "https://registry.nexusagentprotocol.com"
}
```
