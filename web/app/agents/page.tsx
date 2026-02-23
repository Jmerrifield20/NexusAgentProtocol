"use client";

import { useEffect, useState, useCallback } from "react";

type SearchMode = "domain" | "capability";

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

interface AgentCard {
  id: string;
  uri: string;
  display_name: string;
  description: string;
  capability_node: string;
  endpoint: string;
  trust_tier: string;
  metadata?: Record<string, string>;
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

// ── Shared agent card list (used by both domain + capability results) ────────

function AgentCardList({ cards, loading, loadingLabel, emptyLabel, summaryLabel }: {
  cards: AgentCard[];
  loading: boolean;
  loadingLabel: string;
  emptyLabel: React.ReactNode;
  summaryLabel: React.ReactNode;
}) {
  if (loading) return <p className="text-gray-500 py-4">{loadingLabel}</p>;
  if (cards.length === 0) return <p className="text-gray-500 py-4">{emptyLabel}</p>;

  return (
    <div className="space-y-3">
      <p className="text-sm text-gray-500">{summaryLabel}</p>
      <div className="divide-y divide-gray-100 rounded-xl border border-gray-200 bg-white shadow-sm">
        {cards.map((card) => (
          <a key={card.uri} href={`/agents/${card.id}`} className="block px-5 py-4 hover:bg-gray-50 transition-colors">
            <div className="flex flex-wrap items-center gap-2 mb-1">
              <span className="font-semibold text-gray-900">{card.display_name}</span>
              <TierBadge tier={card.trust_tier} />
              <span className="text-xs text-gray-400 font-mono bg-gray-50 rounded px-1.5 py-0.5">
                {formatCapability(card.capability_node)}
              </span>
            </div>
            {card.description && (
              <p className="text-sm text-gray-600 mb-2">{card.description}</p>
            )}
            <code className="block text-xs font-mono text-nexus-500 break-all mb-1">{card.uri}</code>
            {card.endpoint && (
              <span className="text-xs text-gray-400 break-all">{card.endpoint}</span>
            )}
          </a>
        ))}
      </div>
    </div>
  );
}

// ── All-agents list (full list from /api/v1/agents) ───────────────────────

function AllAgentsList({ agents, titleFilter }: {
  agents: AgentFull[];
  titleFilter: string;
}) {
  const filtered = agents.filter((a) =>
    a.display_name.toLowerCase().includes(titleFilter.toLowerCase())
  );

  if (filtered.length === 0 && titleFilter) {
    return <p className="text-gray-500">No agents match your search.</p>;
  }
  if (agents.length === 0) {
    return <p className="text-gray-500">No agents registered yet.</p>;
  }

  return (
    <div className="divide-y divide-gray-100 rounded-xl border border-gray-200 bg-white shadow-sm">
      {filtered.map((agent) => (
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

  // Search mode
  const [mode, setMode] = useState<SearchMode>("domain");

  // Shared search state
  const [input, setInput] = useState("");
  const [searched, setSearched] = useState("");
  const [cards, setCards] = useState<AgentCard[]>([]);
  const [loading, setLoading] = useState(false);
  const [searchError, setSearchError] = useState<string | null>(null);

  // All-agents state
  const [allAgents, setAllAgents] = useState<AgentFull[]>([]);
  const [allLoading, setAllLoading] = useState(true);
  const [allError, setAllError] = useState<string | null>(null);
  const [titleFilter, setTitleFilter] = useState("");

  // Load full list on mount
  useEffect(() => {
    fetch(`${base}/api/v1/agents`)
      .then((r) => { if (!r.ok) throw new Error(`HTTP ${r.status}`); return r.json(); })
      .then((data) => setAllAgents(data.agents ?? []))
      .catch((e) => setAllError(e.message))
      .finally(() => setAllLoading(false));
  }, [base]);

  const handleSearch = useCallback(async (e: React.FormEvent) => {
    e.preventDefault();
    const value = input.trim();
    if (!value) return;

    setSearched(value);
    setLoading(true);
    setSearchError(null);
    setCards([]);

    try {
      const param = mode === "domain"
        ? `org=${encodeURIComponent(value)}`
        : `capability=${encodeURIComponent(value)}`;
      const resp = await fetch(`${base}/api/v1/lookup?${param}`);
      if (!resp.ok) {
        const body = await resp.json();
        throw new Error(body.error ?? `HTTP ${resp.status}`);
      }
      const data = await resp.json();
      setCards(data.agents ?? []);
    } catch (e: unknown) {
      setSearchError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, [base, input, mode]);

  const clearSearch = () => {
    setSearched("");
    setCards([]);
    setSearchError(null);
    setInput("");
  };

  const switchMode = (m: SearchMode) => {
    setMode(m);
    clearSearch();
  };

  return (
    <div className="space-y-10">

      {/* Search panel */}
      <section>
        <h1 className="mb-1 text-3xl font-bold">Find Agents</h1>
        <p className="mb-5 text-gray-500 text-sm">
          Discover agents by their Nexus org namespace, or by the capability they provide.
        </p>

        {/* Mode toggle */}
        <div className="mb-4 flex gap-1 rounded-lg border border-gray-200 bg-gray-50 p-1 w-fit">
          {(["domain", "capability"] as SearchMode[]).map((m) => (
            <button
              key={m}
              onClick={() => switchMode(m)}
              className={`rounded-md px-4 py-1.5 text-sm font-medium transition-colors ${
                mode === m
                  ? "bg-white text-gray-900 shadow-sm"
                  : "text-gray-500 hover:text-gray-700"
              }`}
            >
              {m === "domain" ? "By Org" : "By Capability"}
            </button>
          ))}
        </div>

        <form onSubmit={handleSearch} className="flex gap-2">
          <input
            type="text"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder={mode === "domain" ? "acme" : "finance>accounting"}
            className="flex-1 rounded-lg border border-gray-300 px-4 py-2.5 text-sm focus:border-nexus-500 focus:outline-none font-mono"
          />
          <button
            type="submit"
            disabled={loading || !input.trim()}
            className="rounded-lg bg-nexus-500 px-5 py-2.5 text-sm font-semibold text-white hover:bg-indigo-600 disabled:opacity-50"
          >
            Search
          </button>
          {searched && (
            <button
              type="button"
              onClick={clearSearch}
              className="rounded-lg border border-gray-300 px-4 py-2.5 text-sm text-gray-600 hover:bg-gray-50"
            >
              Clear
            </button>
          )}
        </form>

        {/* Category quick-select chips — only shown in capability mode */}
        {mode === "capability" && !searched && (
          <div className="mt-3 flex flex-wrap gap-1.5">
            {CATEGORIES.map((cat) => (
              <button
                key={cat}
                type="button"
                onClick={() => setInput(cat)}
                className={`rounded-full border px-3 py-1 text-xs font-medium transition-colors capitalize ${
                  input === cat
                    ? "bg-nexus-500 text-white border-nexus-500"
                    : "border-gray-200 text-gray-500 hover:border-nexus-400 hover:text-nexus-600"
                }`}
              >
                {cat}
              </button>
            ))}
          </div>
        )}

        {searchError && (
          <p className="mt-3 rounded bg-red-50 p-4 text-red-600">Error: {searchError}</p>
        )}

        {searched && !searchError && (
          <div className="mt-5">
            <AgentCardList
              cards={cards}
              loading={loading}
              loadingLabel={`Searching for "${searched}"…`}
              emptyLabel={
                mode === "domain"
                  ? <span>No active agents found for org <strong>{searched}</strong>.</span>
                  : <span>No active agents found with capability <strong>{formatCapability(searched)}</strong>.</span>
              }
              summaryLabel={
                mode === "domain"
                  ? <span>{cards.length} agent{cards.length !== 1 ? "s" : ""} under org <strong>{searched}</strong></span>
                  : <span>{cards.length} agent{cards.length !== 1 ? "s" : ""} with capability <strong>{formatCapability(searched)}</strong></span>
              }
            />
          </div>
        )}
      </section>

      {/* All agents directory */}
      {!searched && (
        <section>
          <div className="mb-4 flex items-center justify-between gap-3">
            <h2 className="text-xl font-semibold text-gray-900">All Registered Agents</h2>
            <input
              type="text"
              placeholder="Filter by name..."
              value={titleFilter}
              onChange={(e) => setTitleFilter(e.target.value)}
              className="w-56 rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-nexus-500 focus:outline-none"
            />
          </div>

          {allLoading && <p className="text-gray-500">Loading agents...</p>}
          {allError && (
            <p className="rounded bg-red-50 p-4 text-red-600">
              Failed to load agents: {allError}
            </p>
          )}
          {!allLoading && !allError && (
            <AllAgentsList agents={allAgents} titleFilter={titleFilter} />
          )}
        </section>
      )}

    </div>
  );
}
