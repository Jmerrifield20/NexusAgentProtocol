"use client";

import { useEffect, useState, useCallback } from "react";

// Top-level categories — mirrors model/capabilities.go Taxonomy keys.
const CATEGORIES = [
  "commerce", "communication", "data", "education", "finance",
  "healthcare", "hr", "infrastructure", "legal", "logistics",
  "real-estate", "research",
];

// formatCapability converts "finance>accounting>reconciliation" → "finance > accounting > reconciliation"
function formatCapability(cap: string): string {
  return cap.replace(/>/g, " > ");
}

interface AgentFull {
  id: string;
  trust_root: string;
  capability_node: string;
  agent_id: string;
  display_name: string;
  description: string;
  endpoint: string;
  owner_domain: string;
  status: string;
  created_at: string;
  registration_type?: string;
  trust_tier?: string;
}

const TIER_STYLES: Record<string, { label: string; className: string }> = {
  trusted:    { label: "Trusted",    className: "bg-emerald-100 text-emerald-700" },
  verified:   { label: "Verified",   className: "bg-indigo-100 text-indigo-700" },
  basic:      { label: "Basic",      className: "bg-blue-100 text-blue-700" },
  unverified: { label: "Unverified", className: "bg-gray-100 text-gray-500" },
};

function TierBadge({ tier }: { tier?: string }) {
  if (!tier || !TIER_STYLES[tier]) return null;
  const s = TIER_STYLES[tier];
  return (
    <span className={`shrink-0 rounded-full px-2 py-0.5 text-xs font-medium ${s.className}`}>
      {s.label}
    </span>
  );
}

function StatusBadge({ status }: { status: string }) {
  return (
    <span className={`shrink-0 rounded-full px-2 py-0.5 text-xs font-medium ${
      status === "active"  ? "bg-green-100 text-green-700"  :
      status === "pending" ? "bg-yellow-100 text-yellow-700" :
                             "bg-red-100 text-red-700"
    }`}>
      {status}
    </span>
  );
}

// ── All-agents list ────────────────────────────────────────────────────────

function AllAgentsList({ agents }: { agents: AgentFull[] }) {
  if (agents.length === 0) {
    return <p className="text-gray-500">No agents found.</p>;
  }

  return (
    <div className="divide-y divide-gray-100 rounded-xl border border-gray-200 bg-white shadow-sm">
      {agents.map((agent) => (
        <a key={agent.id} href={`/agents/${agent.id}`} className="flex items-center justify-between px-4 py-3 gap-4 hover:bg-gray-50 transition-colors">
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <span className="font-medium text-sm text-gray-900">{agent.display_name}</span>
              <TierBadge tier={agent.trust_tier} />
              <code className="text-xs text-nexus-500 font-mono truncate">
                agent://{agent.trust_root}/{agent.capability_node.split(">")[0]}/{agent.agent_id}
              </code>
            </div>
            <div className="mt-0.5 flex items-center gap-3 text-xs text-gray-400">
              <span className="truncate">{agent.endpoint}</span>
              <span>{new Date(agent.created_at).toLocaleDateString()}</span>
            </div>
          </div>
          <StatusBadge status={agent.status} />
        </a>
      ))}
    </div>
  );
}

// ── Page ──────────────────────────────────────────────────────────────────

export default function AgentsPage() {
  const base = process.env.NEXT_PUBLIC_REGISTRY_URL ?? "http://localhost:8080";

  const [input, setInput] = useState("");
  const [agents, setAgents] = useState<AgentFull[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeQuery, setActiveQuery] = useState("");

  const fetchAgents = useCallback((q: string) => {
    setLoading(true);
    setError(null);
    const url = q.trim()
      ? `${base}/api/v1/agents?q=${encodeURIComponent(q.trim())}`
      : `${base}/api/v1/agents`;
    fetch(url)
      .then((r) => { if (!r.ok) throw new Error(`HTTP ${r.status}`); return r.json(); })
      .then((data) => { setAgents(data.agents ?? []); setActiveQuery(q.trim()); })
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, [base]);

  // Load all agents on mount
  useEffect(() => { fetchAgents(""); }, [fetchAgents]);

  const handleSearch = useCallback((e: React.FormEvent) => {
    e.preventDefault();
    fetchAgents(input);
  }, [fetchAgents, input]);

  const handleClear = () => {
    setInput("");
    fetchAgents("");
  };

  return (
    <div className="space-y-10">

      {/* Search */}
      <section>
        <h1 className="mb-1 text-3xl font-bold">Find Agents</h1>
        <p className="mb-5 text-sm text-gray-500">
          Search by name, description, capability, org domain, or tag.
        </p>

        <form onSubmit={handleSearch} className="flex gap-2">
          <input
            type="text"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder='e.g. "tax", "acme.com", "finance", "hipaa"'
            className="flex-1 rounded-lg border border-gray-300 px-4 py-2.5 text-sm focus:border-nexus-500 focus:outline-none"
          />
          <button
            type="submit"
            disabled={loading}
            className="rounded-lg bg-nexus-500 px-5 py-2.5 text-sm font-semibold text-white hover:bg-indigo-600 disabled:opacity-50"
          >
            Search
          </button>
          {activeQuery && (
            <button
              type="button"
              onClick={handleClear}
              className="rounded-lg border border-gray-300 px-4 py-2.5 text-sm text-gray-600 hover:bg-gray-50"
            >
              Clear
            </button>
          )}
        </form>

        {/* Category quick-filter chips */}
        {!activeQuery && (
          <div className="mt-3 flex flex-wrap gap-1.5">
            {CATEGORIES.map((cat) => (
              <button
                key={cat}
                type="button"
                onClick={() => { setInput(cat); fetchAgents(cat); }}
                className="rounded-full border border-gray-200 px-3 py-1 text-xs font-medium capitalize text-gray-500 transition-colors hover:border-nexus-400 hover:text-nexus-600"
              >
                {cat}
              </button>
            ))}
          </div>
        )}
      </section>

      {/* Results */}
      <section>
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-xl font-semibold text-gray-900">
            {activeQuery
              ? <span>Results for <span className="text-nexus-600">"{activeQuery}"</span></span>
              : "All Registered Agents"
            }
          </h2>
          {!loading && (
            <span className="text-sm text-gray-400">{agents.length} agent{agents.length !== 1 ? "s" : ""}</span>
          )}
        </div>

        {loading && <p className="text-gray-500">Searching…</p>}
        {error && <p className="rounded bg-red-50 p-4 text-red-600">Failed to load agents: {error}</p>}
        {!loading && !error && <AllAgentsList agents={agents} />}
      </section>

    </div>
  );
}
