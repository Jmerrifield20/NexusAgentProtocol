"use client";

import { useEffect, useState, useCallback } from "react";

interface AgentCard {
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

// ── Domain lookup results (card view from /api/v1/lookup) ──────────────────

function DomainResults({ domain, cards, loading, error }: {
  domain: string;
  cards: AgentCard[];
  loading: boolean;
  error: string | null;
}) {
  if (loading) return <p className="text-gray-500 py-4">Searching {domain}...</p>;
  if (error)   return <p className="rounded bg-red-50 p-4 text-red-600">Error: {error}</p>;
  if (cards.length === 0) return (
    <p className="text-gray-500 py-4">
      No verified agents found for <strong>{domain}</strong>.
    </p>
  );

  return (
    <div className="space-y-3">
      <p className="text-sm text-gray-500">
        {cards.length} verified agent{cards.length !== 1 ? "s" : ""} registered under <strong>{domain}</strong>
      </p>
      <div className="divide-y divide-gray-100 rounded-xl border border-gray-200 bg-white shadow-sm">
        {cards.map((card) => (
          <div key={card.uri} className="px-5 py-4">
            <div className="flex items-start justify-between gap-4">
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-2 mb-1">
                  <span className="font-semibold text-gray-900">{card.display_name}</span>
                  <TierBadge tier={card.trust_tier} />
                  <span className="text-xs text-gray-400 font-mono">{card.capability_node}</span>
                </div>
                {card.description && (
                  <p className="text-sm text-gray-600 mb-2">{card.description}</p>
                )}
                <code className="text-xs font-mono text-nexus-500 break-all">{card.uri}</code>
              </div>
            </div>
            <div className="mt-2">
              <a
                href={card.endpoint}
                className="text-xs text-gray-400 hover:underline break-all"
                target="_blank"
                rel="noreferrer"
              >
                {card.endpoint}
              </a>
            </div>
          </div>
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
        <div key={agent.id} className="flex items-center justify-between px-4 py-3 gap-4">
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <span className="font-medium text-sm text-gray-900">{agent.display_name}</span>
              <TierBadge tier={agent.trust_tier} />
              <code className="text-xs text-nexus-500 font-mono truncate">
                agent://{agent.trust_root}/{agent.capability_node}/{agent.agent_id}
              </code>
            </div>
            <div className="mt-0.5 flex items-center gap-3 text-xs text-gray-400">
              <a href={agent.endpoint} className="hover:underline truncate" target="_blank" rel="noreferrer">
                {agent.endpoint}
              </a>
              <span>{new Date(agent.created_at).toLocaleDateString()}</span>
            </div>
          </div>
          <StatusBadge status={agent.status} />
        </div>
      ))}
    </div>
  );
}

// ── Page ──────────────────────────────────────────────────────────────────

export default function AgentsPage() {
  const base = process.env.NEXT_PUBLIC_REGISTRY_URL ?? "http://localhost:8080";

  // Domain lookup state
  const [domainInput, setDomainInput] = useState("");
  const [searchedDomain, setSearchedDomain] = useState("");
  const [cards, setCards] = useState<AgentCard[]>([]);
  const [lookupLoading, setLookupLoading] = useState(false);
  const [lookupError, setLookupError] = useState<string | null>(null);

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

  const handleLookup = useCallback(async (e: React.FormEvent) => {
    e.preventDefault();
    const domain = domainInput.trim();
    if (!domain) return;

    setSearchedDomain(domain);
    setLookupLoading(true);
    setLookupError(null);
    setCards([]);

    try {
      const resp = await fetch(`${base}/api/v1/lookup?domain=${encodeURIComponent(domain)}`);
      if (!resp.ok) {
        const body = await resp.json();
        throw new Error(body.error ?? `HTTP ${resp.status}`);
      }
      const data = await resp.json();
      setCards(data.agents ?? []);
    } catch (e: unknown) {
      setLookupError(e instanceof Error ? e.message : String(e));
    } finally {
      setLookupLoading(false);
    }
  }, [base, domainInput]);

  const clearLookup = () => {
    setSearchedDomain("");
    setCards([]);
    setLookupError(null);
    setDomainInput("");
  };

  return (
    <div className="space-y-10">

      {/* Domain lookup */}
      <section>
        <h1 className="mb-1 text-3xl font-bold">Find Agents by Domain</h1>
        <p className="mb-5 text-gray-500 text-sm">
          Enter a domain to see all verified agents registered under it.
        </p>
        <form onSubmit={handleLookup} className="flex gap-2">
          <input
            type="text"
            value={domainInput}
            onChange={(e) => setDomainInput(e.target.value)}
            placeholder="acme.com"
            className="flex-1 rounded-lg border border-gray-300 px-4 py-2.5 text-sm focus:border-nexus-500 focus:outline-none font-mono"
          />
          <button
            type="submit"
            disabled={lookupLoading || !domainInput.trim()}
            className="rounded-lg bg-nexus-500 px-5 py-2.5 text-sm font-semibold text-white hover:bg-indigo-600 disabled:opacity-50"
          >
            Search
          </button>
          {searchedDomain && (
            <button
              type="button"
              onClick={clearLookup}
              className="rounded-lg border border-gray-300 px-4 py-2.5 text-sm text-gray-600 hover:bg-gray-50"
            >
              Clear
            </button>
          )}
        </form>

        {searchedDomain && (
          <div className="mt-5">
            <DomainResults
              domain={searchedDomain}
              cards={cards}
              loading={lookupLoading}
              error={lookupError}
            />
          </div>
        )}
      </section>

      {/* All agents directory */}
      {!searchedDomain && (
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
