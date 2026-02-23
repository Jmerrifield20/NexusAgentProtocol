"use client";

import { useEffect, useState, useCallback } from "react";

const CATEGORIES = [
  "commerce", "communication", "data", "education", "finance",
  "healthcare", "hr", "infrastructure", "legal", "logistics",
  "real-estate", "research",
];

// ── Types ──────────────────────────────────────────────────────────────────

// Full agent shape returned by GET /api/v1/agents
interface AgentFull {
  id: string;
  trust_root: string;
  capability_node: string;
  agent_id: string;
  display_name: string;
  description: string;
  endpoint: string;
  status: string;
  trust_tier?: string;
  created_at: string;
}

// Card shape returned by GET /api/v1/lookup
interface AgentCard {
  id: string;
  uri: string;
  display_name: string;
  description: string;
  capability_node: string;
  endpoint: string;
  trust_tier?: string;
}

// Unified display shape used by the result list
interface DisplayAgent {
  id: string;
  display_name: string;
  description?: string;
  uri: string;
  endpoint?: string;
  trust_tier?: string;
  status?: string;
  capability_node?: string;
}

function fromFull(a: AgentFull): DisplayAgent {
  const category = a.capability_node?.split(">")[0] ?? "";
  return {
    id: a.id,
    display_name: a.display_name,
    description: a.description,
    uri: `agent://${a.trust_root}/${category}/${a.agent_id}`,
    endpoint: a.endpoint,
    trust_tier: a.trust_tier,
    status: a.status,
    capability_node: a.capability_node,
  };
}

function fromCard(a: AgentCard): DisplayAgent {
  return {
    id: a.id,
    display_name: a.display_name,
    description: a.description,
    uri: a.uri,
    endpoint: a.endpoint,
    trust_tier: a.trust_tier,
    capability_node: a.capability_node,
  };
}

// ── Badges ─────────────────────────────────────────────────────────────────

const TIER_STYLES: Record<string, string> = {
  trusted:    "bg-emerald-100 text-emerald-700",
  verified:   "bg-indigo-100 text-indigo-700",
  basic:      "bg-blue-100 text-blue-700",
  unverified: "bg-gray-100 text-gray-500",
};

function TierBadge({ tier }: { tier?: string }) {
  if (!tier || !TIER_STYLES[tier]) return null;
  return (
    <span className={`shrink-0 rounded-full px-2 py-0.5 text-xs font-medium capitalize ${TIER_STYLES[tier]}`}>
      {tier}
    </span>
  );
}

function StatusBadge({ status }: { status?: string }) {
  if (!status) return null;
  return (
    <span className={`shrink-0 rounded-full px-2 py-0.5 text-xs font-medium capitalize ${
      status === "active"  ? "bg-green-100 text-green-700"  :
      status === "pending" ? "bg-yellow-100 text-yellow-700" :
                             "bg-red-100 text-red-700"
    }`}>
      {status}
    </span>
  );
}

// ── Result list ────────────────────────────────────────────────────────────

function AgentList({ agents }: { agents: DisplayAgent[] }) {
  if (agents.length === 0) {
    return <p className="text-sm text-gray-500">No agents found.</p>;
  }
  return (
    <div className="divide-y divide-gray-100 rounded-xl border border-gray-200 bg-white shadow-sm">
      {agents.map((agent) => (
        <a
          key={agent.id}
          href={`/agents/${agent.id}`}
          className="flex items-start justify-between gap-4 px-4 py-3 hover:bg-gray-50 transition-colors"
        >
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <span className="font-medium text-sm text-gray-900">{agent.display_name}</span>
              <TierBadge tier={agent.trust_tier} />
              <StatusBadge status={agent.status} />
            </div>
            <code className="mt-0.5 block text-xs text-nexus-500 font-mono truncate">{agent.uri}</code>
            {agent.capability_node && (
              <p className="mt-0.5 text-xs text-gray-400">
                {agent.capability_node.replace(/>/g, " > ")}
              </p>
            )}
          </div>
          <span className="shrink-0 text-xs text-gray-400 font-mono truncate max-w-[180px] text-right">
            {agent.endpoint}
          </span>
        </a>
      ))}
    </div>
  );
}

// ── Page ──────────────────────────────────────────────────────────────────

type SearchMode = "domain" | "capability" | "user";

export default function AgentsPage() {
  const base = process.env.NEXT_PUBLIC_REGISTRY_URL ?? "http://localhost:8080";

  const [mode, setMode] = useState<SearchMode>("capability");
  const [domainInput, setDomainInput]         = useState("");
  const [capabilityInput, setCapabilityInput] = useState("");
  const [usernameInput, setUsernameInput]     = useState("");

  const [agents, setAgents]     = useState<DisplayAgent[]>([]);
  const [loading, setLoading]   = useState(true);
  const [error, setError]       = useState<string | null>(null);
  const [activeLabel, setActiveLabel] = useState<string | null>(null);

  // ── Loaders ──

  const loadAll = useCallback(() => {
    setLoading(true);
    setError(null);
    setActiveLabel(null);
    fetch(`${base}/api/v1/agents`)
      .then((r) => { if (!r.ok) throw new Error(`HTTP ${r.status}`); return r.json(); })
      .then((data) => setAgents((data.agents ?? []).map(fromFull)))
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false));
  }, [base]);

  const searchDomain = useCallback((domain: string) => {
    const d = domain.trim();
    if (!d) { loadAll(); return; }
    setLoading(true);
    setError(null);
    fetch(`${base}/api/v1/lookup?org=${encodeURIComponent(d)}`)
      .then((r) => { if (!r.ok) throw new Error(`HTTP ${r.status}`); return r.json(); })
      .then((data) => {
        setAgents((data.agents ?? []).map(fromCard));
        setActiveLabel(`domain: ${d}`);
      })
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false));
  }, [base, loadAll]);

  const searchCapability = useCallback((cap: string) => {
    const c = cap.trim();
    if (!c) { loadAll(); return; }
    setLoading(true);
    setError(null);
    fetch(`${base}/api/v1/lookup?capability=${encodeURIComponent(c)}`)
      .then((r) => { if (!r.ok) throw new Error(`HTTP ${r.status}`); return r.json(); })
      .then((data) => {
        setAgents((data.agents ?? []).map(fromCard));
        setActiveLabel(`capability: ${c}`);
      })
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false));
  }, [base, loadAll]);

  const searchByUser = useCallback((username: string) => {
    const u = username.trim();
    if (!u) { loadAll(); return; }
    setLoading(true);
    setError(null);
    fetch(`${base}/api/v1/agents?username=${encodeURIComponent(u)}`)
      .then((r) => { if (!r.ok) throw new Error(`HTTP ${r.status}`); return r.json(); })
      .then((data) => {
        setAgents((data.agents ?? []).map(fromFull));
        setActiveLabel(`user: ${u}`);
      })
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false));
  }, [base, loadAll]);

  // Initial load
  useEffect(() => { loadAll(); }, [loadAll]);

  const handleClear = () => {
    setDomainInput("");
    setCapabilityInput("");
    setUsernameInput("");
    loadAll();
  };

  // ── Render ──

  return (
    <div className="space-y-8">

      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold">Find Agents</h1>
        <p className="mt-1 text-sm text-gray-500">
          Search the registry by organisation domain or by what the agent does.
        </p>
      </div>

      {/* Search panel */}
      <div className="rounded-xl border border-gray-200 bg-white p-5 space-y-4">

        {/* Tab switcher */}
        <div className="flex gap-1 rounded-lg bg-gray-100 p-1 w-fit">
          {(["capability", "domain", "user"] as SearchMode[]).map((m) => (
            <button
              key={m}
              type="button"
              onClick={() => setMode(m)}
              className={`rounded-md px-4 py-1.5 text-sm font-medium transition-colors ${
                mode === m
                  ? "bg-white text-gray-900 shadow-sm"
                  : "text-gray-500 hover:text-gray-700"
              }`}
            >
              {m === "domain" ? "By Domain" : m === "capability" ? "By Capability" : "By User"}
            </button>
          ))}
        </div>

        {/* Domain search */}
        {mode === "domain" && (
          <form
            onSubmit={(e) => { e.preventDefault(); searchDomain(domainInput); }}
            className="space-y-3"
          >
            <p className="text-xs text-gray-400">
              Enter an organisation domain to see all its registered agents.
            </p>
            <div className="flex gap-2">
              <input
                type="text"
                value={domainInput}
                onChange={(e) => setDomainInput(e.target.value)}
                placeholder="e.g. acme.com"
                className="flex-1 rounded-lg border border-gray-300 px-4 py-2.5 text-sm font-mono focus:border-nexus-500 focus:outline-none"
              />
              <button
                type="submit"
                disabled={loading}
                className="rounded-lg bg-nexus-500 px-5 py-2.5 text-sm font-semibold text-white hover:bg-indigo-600 disabled:opacity-50"
              >
                Search
              </button>
              {activeLabel && (
                <button
                  type="button"
                  onClick={handleClear}
                  className="rounded-lg border border-gray-300 px-4 py-2.5 text-sm text-gray-600 hover:bg-gray-50"
                >
                  Clear
                </button>
              )}
            </div>
          </form>
        )}

        {/* Capability search */}
        {mode === "capability" && (
          <form
            onSubmit={(e) => { e.preventDefault(); searchCapability(capabilityInput); }}
            className="space-y-3"
          >
            <p className="text-xs text-gray-400">
              Search by what an agent does — use a category, subcategory, or keyword.
            </p>
            <div className="flex gap-2">
              <input
                type="text"
                value={capabilityInput}
                onChange={(e) => setCapabilityInput(e.target.value)}
                placeholder="e.g. accounting, finance, legal"
                className="flex-1 rounded-lg border border-gray-300 px-4 py-2.5 text-sm focus:border-nexus-500 focus:outline-none"
              />
              <button
                type="submit"
                disabled={loading}
                className="rounded-lg bg-nexus-500 px-5 py-2.5 text-sm font-semibold text-white hover:bg-indigo-600 disabled:opacity-50"
              >
                Search
              </button>
              {activeLabel && (
                <button
                  type="button"
                  onClick={handleClear}
                  className="rounded-lg border border-gray-300 px-4 py-2.5 text-sm text-gray-600 hover:bg-gray-50"
                >
                  Clear
                </button>
              )}
            </div>
            {/* Category chips */}
            {!activeLabel && (
              <div className="flex flex-wrap gap-1.5 pt-1">
                {CATEGORIES.map((cat) => (
                  <button
                    key={cat}
                    type="button"
                    onClick={() => { setCapabilityInput(cat); searchCapability(cat); }}
                    className="rounded-full border border-gray-200 px-3 py-1 text-xs font-medium capitalize text-gray-500 hover:border-nexus-400 hover:text-nexus-600 transition-colors"
                  >
                    {cat}
                  </button>
                ))}
              </div>
            )}
          </form>
        )}

        {/* User search */}
        {mode === "user" && (
          <form
            onSubmit={(e) => { e.preventDefault(); searchByUser(usernameInput); }}
            className="space-y-3"
          >
            <p className="text-xs text-gray-400">
              Enter a username to see all their active registered agents.
            </p>
            <div className="flex gap-2">
              <input
                type="text"
                value={usernameInput}
                onChange={(e) => setUsernameInput(e.target.value)}
                placeholder="e.g. alice"
                className="flex-1 rounded-lg border border-gray-300 px-4 py-2.5 text-sm font-mono focus:border-nexus-500 focus:outline-none"
              />
              <button
                type="submit"
                disabled={loading}
                className="rounded-lg bg-nexus-500 px-5 py-2.5 text-sm font-semibold text-white hover:bg-indigo-600 disabled:opacity-50"
              >
                Search
              </button>
              {activeLabel && (
                <button
                  type="button"
                  onClick={handleClear}
                  className="rounded-lg border border-gray-300 px-4 py-2.5 text-sm text-gray-600 hover:bg-gray-50"
                >
                  Clear
                </button>
              )}
            </div>
            {activeLabel?.startsWith("user: ") && (
              <a
                href={`/users/${encodeURIComponent(usernameInput.trim())}`}
                className="inline-block text-xs text-nexus-500 hover:underline"
              >
                View {usernameInput.trim()}&apos;s profile →
              </a>
            )}
          </form>
        )}
      </div>

      {/* Results */}
      <section>
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-xl font-semibold text-gray-900">
            {activeLabel
              ? <>Results for <span className="text-nexus-600">{activeLabel}</span></>
              : "All Registered Agents"
            }
          </h2>
          {!loading && (
            <span className="text-sm text-gray-400">
              {agents.length} agent{agents.length !== 1 ? "s" : ""}
            </span>
          )}
        </div>

        {loading && <p className="text-sm text-gray-500">Searching…</p>}
        {error   && <p className="rounded bg-red-50 p-4 text-sm text-red-600">Failed to load agents: {error}</p>}
        {!loading && !error && <AgentList agents={agents} />}
      </section>

    </div>
  );
}
