# NAP URI Standard

**Version:** 1.0
**Status:** Finalized

---

## 1. Format

```
agent://<trust_root>/<category>/<agent_id>
```

| Segment | Description | Example |
|---------|-------------|---------|
| `trust_root` | Verified domain or reserved namespace | `acme.com`, `nap` |
| `category` | Top-level capability category (first segment of capability path) | `finance`, `legal` |
| `agent_id` | Registry-generated opaque identifier | `agent_3k2mn7p4q...` |

---

## 2. Examples

| URI | Registration type | Trust level |
|-----|------------------|-------------|
| `agent://nap/finance/agent_abc123` | NAP-hosted (free tier) | Email-verified |
| `agent://acme.com/finance/agent_xyz789` | Domain-verified | DNS-01 proven |
| `agent://globalbank.io/legal/agent_mnop` | Domain-verified (federated) | Intermediate CA |

---

## 3. URI Stability Guarantee

Once issued, a NAP URI is **permanent**:
- The registry never reuses `agent_id` values.
- The `trust_root` and `category` segments are set at registration and cannot
  be changed.
- If an agent is revoked or deleted, its URI returns a 404/revoked status but
  is never reassigned.

---

## 4. Trust Root Semantics

### Free-tier (`nap`)
- Controlled exclusively by the root registry.
- Guarantees: registrant email address is verified.
- Does **not** guarantee the registrant owns any domain.
- URI: `agent://nap/{category}/{agent_id}`

### Domain-verified
- The `trust_root` is the registrant's **full verified domain** as proven by
  DNS-01 challenge (e.g. `acme.com`).
- `acme.com` and `acme.io` are distinct trust roots — no abbreviation or slug.
- URI: `agent://{owner_domain}/{category}/{agent_id}`

### Federated domain-verified
- Same as domain-verified but the leaf certificate is signed by an intermediate
  CA issued by the NAP root, not the root CA directly.
- Clients verify the full chain: leaf → intermediate → root CA.

---

## 5. Resolution Algorithm

```
Resolve(uri: agent://{trustRoot}/{category}/{agentID}):

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

## 6. Reserved Namespaces

The following trust roots are reserved and cannot be registered by operators:

| Namespace | Reserved for |
|-----------|--------------|
| `nap` | NAP-hosted free tier (root registry only) |
| `nexusagentprotocol.com` | Root registry itself |

Any `RegisterRequest` with a `trust_root` matching a reserved namespace is
rejected with a 400 error.

---

## 7. URI Construction

The `URI()` method on `model.Agent` produces the canonical URI:

```go
func (a *Agent) URI() string {
    category := TopLevelCategory(a.CapabilityNode) // first segment before ">"
    return "agent://" + a.TrustRoot + "/" + category + "/" + a.AgentID
}
```

Only the **top-level category** appears in the URI path, even when the agent
has a 2- or 3-level capability node (e.g. `finance>accounting>reconciliation`
produces `agent://acme.com/finance/agent_xyz`). The full capability path is
stored internally in `capability_node` and returned in API responses.

---

## 8. A2A Compatibility

NAP URIs are embedded as URI SANs in X.509 agent certificates (scheme `agent`),
making them verifiable at the TLS layer. The A2A agent card (`agent.json`)
also carries the NAP URI in the `nap_uri` field alongside the standard A2A
`url` (agent endpoint) field.

```json
{
  "name": "Acme Finance Agent",
  "url": "https://agent.acme.com",
  "nap_uri": "agent://acme.com/finance/agent_xyz789",
  "nap_trust_tier": "domain_verified",
  "nap_registry": "https://registry.nexusagentprotocol.com"
}
```
