"use client";

import { useEffect, useState } from "react";

interface Agent {
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
}

export default function AgentsPage() {
  const [agents, setAgents] = useState<Agent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [titleFilter, setTitleFilter] = useState("");
  const [domainFilter, setDomainFilter] = useState("");

  useEffect(() => {
    const base = process.env.NEXT_PUBLIC_REGISTRY_URL ?? "http://localhost:8080";
    fetch(`${base}/api/v1/agents`)
      .then((r) => {
        if (!r.ok) throw new Error(`HTTP ${r.status}`);
        return r.json();
      })
      .then((data) => setAgents(data.agents ?? []))
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, []);

  const filtered = agents.filter((a) => {
    const matchTitle = a.display_name.toLowerCase().includes(titleFilter.toLowerCase());
    const matchDomain = a.owner_domain?.toLowerCase().includes(domainFilter.toLowerCase()) ||
      a.trust_root.toLowerCase().includes(domainFilter.toLowerCase());
    return matchTitle && matchDomain;
  });

  return (
    <div>
      <h1 className="mb-6 text-3xl font-bold">Registered Agents</h1>

      <div className="mb-4 flex gap-3">
        <input
          type="text"
          placeholder="Search by title..."
          value={titleFilter}
          onChange={(e) => setTitleFilter(e.target.value)}
          className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-nexus-500 focus:outline-none"
        />
        <input
          type="text"
          placeholder="Search by domain..."
          value={domainFilter}
          onChange={(e) => setDomainFilter(e.target.value)}
          className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-nexus-500 focus:outline-none"
        />
      </div>

      {loading && <p className="text-gray-500">Loading agents...</p>}
      {error && (
        <p className="rounded bg-red-50 p-4 text-red-600">
          Failed to load agents: {error}
        </p>
      )}

      {!loading && !error && agents.length === 0 && (
        <p className="text-gray-500">No agents registered yet.</p>
      )}

      {!loading && !error && agents.length > 0 && filtered.length === 0 && (
        <p className="text-gray-500">No agents match your search.</p>
      )}

      <div className="divide-y divide-gray-100 rounded-xl border border-gray-200 bg-white shadow-sm">
        {filtered.map((agent) => (
          <div key={agent.id} className="flex items-center justify-between px-4 py-3 gap-4">
            <div className="min-w-0">
              <div className="flex items-center gap-2">
                <span className="font-medium text-sm text-gray-900">{agent.display_name}</span>
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
            <span
              className={`shrink-0 rounded-full px-2 py-0.5 text-xs font-medium ${
                agent.status === "active"
                  ? "bg-green-100 text-green-700"
                  : agent.status === "pending"
                  ? "bg-yellow-100 text-yellow-700"
                  : "bg-red-100 text-red-700"
              }`}
            >
              {agent.status}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}
