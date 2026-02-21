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
  status: string;
  created_at: string;
}

export default function AgentsPage() {
  const [agents, setAgents] = useState<Agent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

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

  return (
    <div>
      <h1 className="mb-8 text-3xl font-bold">Registered Agents</h1>

      {loading && <p className="text-gray-500">Loading agents...</p>}
      {error && (
        <p className="rounded bg-red-50 p-4 text-red-600">
          Failed to load agents: {error}
        </p>
      )}

      {!loading && !error && agents.length === 0 && (
        <p className="text-gray-500">No agents registered yet.</p>
      )}

      <div className="grid gap-4">
        {agents.map((agent) => (
          <div
            key={agent.id}
            className="rounded-xl border border-gray-200 bg-white p-6 shadow-sm"
          >
            <div className="flex items-start justify-between">
              <div>
                <h3 className="font-semibold text-lg">{agent.display_name}</h3>
                <code className="mt-1 block text-sm text-nexus-500 font-mono">
                  agent://{agent.trust_root}/{agent.capability_node}/
                  {agent.agent_id}
                </code>
                {agent.description && (
                  <p className="mt-2 text-sm text-gray-500">
                    {agent.description}
                  </p>
                )}
              </div>
              <span
                className={`rounded-full px-3 py-1 text-xs font-medium ${
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
            <div className="mt-4 flex items-center gap-6 text-sm text-gray-400">
              <span>
                Endpoint:{" "}
                <a
                  href={agent.endpoint}
                  className="text-nexus-500 hover:underline"
                  target="_blank"
                  rel="noreferrer"
                >
                  {agent.endpoint}
                </a>
              </span>
              <span>
                Registered:{" "}
                {new Date(agent.created_at).toLocaleDateString()}
              </span>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
