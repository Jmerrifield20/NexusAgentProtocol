/**
 * NAP (Nexus Agent Protocol) integration for OpenClaw.
 *
 * Adds an optional `nap` section to ~/.openclaw/openclaw.json that registers
 * the OpenClaw Gateway instance as a discoverable NAP agent.
 *
 * URI format (both paths):  agent://<org>/<category>/<agent-id>
 *
 * Free hosted   → agent://nap/<category>/<id>
 *                 e.g.  agent://nap/assistant/0195fa3c-…
 *   ("nap" is the registry-controlled namespace — prevents username impersonation)
 *
 * Domain-verified → agent://<owner-domain>/<category>/<id>
 *                   e.g.  agent://acme.com/finance/0195fa3c-…
 *
 * org       = "nap" (free-hosted) or the full owner domain, e.g. "acme.com" (domain-verified)
 * category  = top-level of capability path ("finance>accounting" → "finance" in URI)
 */

// ── Config (goes in ~/.openclaw/openclaw.json under "nap") ───────────────────

export interface NAPConfig {
  /** Set to true to enable NAP registration. Default: false. */
  enabled: boolean;

  /**
   * NAP registry base URL. Override for self-hosted registries.
   * @default "https://registry.nexusagentprotocol.com"
   */
  registry_url?: string;

  /** Human-readable name shown in agent listings. */
  display_name: string;

  /** Optional description of what this assistant does. */
  description?: string;

  /**
   * Publicly reachable URL of this OpenClaw Gateway instance.
   * Required for callers to reach the agent after resolution.
   * Example: "https://alice.tailnet.ts.net:18789"
   *          "https://assistant.acme.com"
   * Leave empty to register as pending and set later via the web UI.
   */
  endpoint?: string;

  /**
   * If set, uses the domain-verified registration path (DNS-01 challenge).
   * The user must own this domain and complete verification.
   * If omitted, uses the free hosted tier (email-verified account).
   */
  owner_domain?: string;

  /**
   * Full capability path using ">" as separator between levels.
   * Required for both registration paths.
   * Examples: "assistant", "finance", "finance>accounting", "finance>accounting>reconciliation"
   *
   * Only the top-level segment appears in the agent:// URI:
   *   "finance>accounting" → agent://<org>/finance/<id>
   *
   * Sub-categories are indexed for search but not encoded in the address,
   * keeping URIs stable as capability paths evolve.
   */
  capability?: string;
}

// ── Persisted state (written to ~/.openclaw/nap.json) ────────────────────────

export interface NAPState {
  /** UUID assigned by the registry at registration. */
  agent_id: string;

  /** Stable agent:// URI — never changes even if endpoint moves. */
  agent_uri: string;

  /**
   * User JWT (nap_hosted only). Used to authenticate PATCH calls.
   * Expires after 24 hours — refreshed automatically on startup.
   */
  user_token?: string;

  /**
   * Task token (domain-verified only). Issued at activation.
   * Required for revoke/delete operations.
   */
  task_token?: string;

  /** The A2A-compatible agent card JSON string, ready to serve. */
  agent_card_json?: string;

  /** Email address used to log in (nap_hosted). Stored to enable token refresh. */
  email?: string;

  /** ISO timestamp of initial registration. */
  registered_at: string;

  /** Current agent status as last known by the registry. */
  status: 'pending' | 'active' | 'revoked' | 'expired';

  /** Last time the endpoint was successfully synced to NAP. */
  endpoint_synced_at?: string;
}

// ── API response shapes ───────────────────────────────────────────────────────

export interface NAPAuthResponse {
  token: string;
  user: {
    id: string;
    email: string;
    username: string;
    tier: string;
  };
  note?: string;
}

export interface NAPRegisterResponse {
  id: string;
  agent_id: string;
  agent_uri: string;
  status: string;
  uri: string;
}

export interface NAPActivateResponse {
  status: string;
  agent_card_json?: string;
  agent_card_note?: string;
  task_token?: string;
  certificate?: { serial: string; pem: string };
  private_key_pem?: string;
  ca_pem?: string;
  warning?: string;
}
