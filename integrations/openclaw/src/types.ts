/**
 * NAP (Nexus Agent Protocol) integration for OpenClaw.
 *
 * Adds an optional `nap` section to ~/.openclaw/openclaw.json that registers
 * the OpenClaw Gateway instance as a discoverable NAP agent.
 *
 * URI format — 4-segment (preferred):
 *   agent://<trust_root>/<category>/<primary_skill>/<agent_id>
 *
 *   Free hosted     → agent://nap/<category>/<primary_skill>/<id>
 *                     e.g.  agent://nap/finance/billing/agent_xxx
 *   Domain-verified → agent://<owner-domain>/<category>/<primary_skill>/<id>
 *                     e.g.  agent://acme.com/finance/billing/agent_xxx
 *
 * URI format — 3-segment (top-level capability only):
 *   agent://<trust_root>/<category>/<agent_id>
 *   e.g.  agent://nap/assistant/agent_xxx
 *
 * primary_skill is derived automatically from the capability path (last segment
 * when 2+ levels) or from the first declared skill ID. Top-level-only capabilities
 * (e.g. "assistant") produce the 3-segment form.
 *
 * trust_root = "nap" (free-hosted) or the full owner domain, e.g. "acme.com"
 * category   = top-level of capability path ("finance>accounting" → "finance")
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
   * Full capability path using ">" as separator between levels (up to 3 levels).
   * Required for both registration paths.
   * Examples: "assistant", "finance", "finance>accounting", "finance>accounting>reconciliation"
   *
   * The top-level segment becomes the URI category; the last segment becomes
   * the primary_skill URI segment (when 2+ levels are provided):
   *   "finance>accounting"              → agent://nap/finance/accounting/agent_xxx
   *   "finance>accounting>reconciliation" → agent://nap/finance/reconciliation/agent_xxx
   *   "assistant"                       → agent://nap/assistant/agent_xxx (3-segment)
   *
   * Providing a 2- or 3-level path is recommended — it makes your URI
   * self-describing and enables structured skill-based discovery.
   */
  capability?: string;

  /**
   * Optional explicit A2A skill declarations. When provided, skills[0].id
   * overrides the capability-derived primary_skill in the URI.
   *
   * Example:
   *   [{ id: "reconcile-invoices", name: "Reconcile Invoices",
   *      description: "Match purchase orders against invoices" }]
   *
   * If omitted, skills are auto-derived from the capability taxonomy.
   */
  skills?: Array<{ id: string; name: string; description?: string; tags?: string[] }>;

  /**
   * Optional MCP tool definitions this agent exposes. When provided, tool names
   * are indexed for structured discovery via GET /agents?tool=<name>.
   *
   * Example:
   *   [{ name: "parse_invoice", description: "Parse a PDF invoice into structured data",
   *      inputSchema: { type: "object", properties: { url: { type: "string" } } } }]
   */
  mcp_tools?: Array<{ name: string; description: string; inputSchema?: unknown }>;
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
  mcp_manifest_json?: string;
  mcp_manifest_note?: string;
  task_token?: string;
  certificate?: { serial: string; pem: string };
  private_key_pem?: string;
  ca_pem?: string;
  warning?: string;
}
