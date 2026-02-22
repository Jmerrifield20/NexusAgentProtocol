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
export {};
//# sourceMappingURL=types.js.map