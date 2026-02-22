"use client";

import { useEffect, useState } from "react";
import { getToken, getUser, clearToken, isLoggedIn, UserClaims } from "../../lib/auth";
import { useRouter } from "next/navigation";

const REGISTRY = process.env.NEXT_PUBLIC_REGISTRY_URL ?? "http://localhost:8080";

// ── Types ───────────────────────────────────────────────────────────────────
interface Agent {
  id: string;
  trust_root: string;
  capability_node: string;
  agent_id: string;
  display_name: string;
  description: string;
  endpoint: string;
  status: string;
  trust_tier: string;
  registration_type: string;
  owner_domain: string;
  cert_serial: string;
  created_at: string;
}

function agentURI(a: Agent): string {
  return `agent://${a.trust_root}/${a.capability_node}/${a.agent_id}`;
}

// ── Tier badge ───────────────────────────────────────────────────────────────
const TIER_STYLES: Record<string, string> = {
  trusted:    "bg-emerald-100 text-emerald-700",
  verified:   "bg-indigo-100 text-indigo-700",
  basic:      "bg-blue-100 text-blue-700",
  unverified: "bg-gray-100 text-gray-500",
};

function TierBadge({ tier }: { tier: string }) {
  return (
    <span className={`rounded-full px-2 py-0.5 text-xs font-semibold capitalize ${TIER_STYLES[tier] ?? TIER_STYLES.unverified}`}>
      {tier}
    </span>
  );
}

function StatusBadge({ status }: { status: string }) {
  const styles: Record<string, string> = {
    active:  "bg-green-100 text-green-700",
    pending: "bg-yellow-100 text-yellow-700",
    revoked: "bg-red-100 text-red-700",
    expired: "bg-gray-100 text-gray-500",
  };
  return (
    <span className={`rounded-full px-2 py-0.5 text-xs font-semibold capitalize ${styles[status] ?? "bg-gray-100 text-gray-500"}`}>
      {status}
    </span>
  );
}

// ── Initials avatar ──────────────────────────────────────────────────────────
function Avatar({ username }: { username: string }) {
  const initials = username.slice(0, 2).toUpperCase();
  return (
    <div className="h-14 w-14 rounded-full bg-nexus-500 flex items-center justify-center text-white text-xl font-bold select-none">
      {initials}
    </div>
  );
}

// ── Agent card ───────────────────────────────────────────────────────────────
function AgentCard({ agent, onUpdated }: { agent: Agent; onUpdated: () => void }) {
  const isHosted = agent.registration_type === "nap_hosted";
  const needsEndpoint = isHosted && !agent.endpoint;

  const [editingEndpoint, setEditingEndpoint] = useState(needsEndpoint);
  const [endpointInput, setEndpointInput] = useState(agent.endpoint ?? "");
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  const uri = agentURI(agent);

  const handleActivate = async () => {
    const token = getToken();
    const resp = await fetch(`${REGISTRY}/api/v1/agents/${agent.id}/activate`, {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
    });
    if (resp.ok) onUpdated();
  };

  const handleSaveEndpoint = async () => {
    const url = endpointInput.trim();
    if (!url) return;
    setSaving(true);
    setSaveError(null);
    try {
      const resp = await fetch(`${REGISTRY}/api/v1/agents/${agent.id}`, {
        method: "PATCH",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${getToken()}`,
        },
        body: JSON.stringify({ endpoint: url }),
      });
      if (!resp.ok) {
        const body = await resp.json();
        setSaveError(body.error ?? `HTTP ${resp.status}`);
        return;
      }
      setEditingEndpoint(false);
      onUpdated();
    } catch {
      setSaveError("Something went wrong.");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="rounded-xl border border-gray-200 bg-white p-5 flex flex-col gap-3 hover:border-gray-300 transition-colors">
      {/* Header */}
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="font-semibold text-gray-900 truncate">{agent.display_name}</p>
          {agent.description && (
            <p className="text-xs text-gray-500 mt-0.5 line-clamp-2">{agent.description}</p>
          )}
        </div>
        <div className="flex flex-col items-end gap-1 shrink-0">
          <StatusBadge status={agent.status} />
          <TierBadge tier={agent.trust_tier} />
        </div>
      </div>

      {/* Permanent URI */}
      <div>
        <p className="text-xs text-gray-400 mb-0.5">Agent URI (permanent)</p>
        <code className="block text-xs font-mono text-nexus-500 bg-gray-50 rounded px-2 py-1.5 truncate">
          {uri}
        </code>
      </div>

      {/* Endpoint section */}
      <div>
        <div className="flex items-center justify-between mb-0.5">
          <p className="text-xs text-gray-400">Server URL</p>
          {agent.endpoint && !editingEndpoint && (
            <button
              onClick={() => { setEditingEndpoint(true); setEndpointInput(agent.endpoint); }}
              className="text-xs text-gray-400 hover:text-gray-600"
            >
              Edit
            </button>
          )}
        </div>

        {editingEndpoint ? (
          <div className="space-y-1.5">
            <input
              type="url"
              value={endpointInput}
              onChange={(e) => setEndpointInput(e.target.value)}
              placeholder="https://my-agent.fly.dev"
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-xs font-mono focus:border-nexus-500 focus:outline-none"
            />
            {saveError && <p className="text-xs text-red-600">{saveError}</p>}
            <div className="flex gap-2">
              <button
                onClick={handleSaveEndpoint}
                disabled={saving || !endpointInput.trim()}
                className="rounded-lg bg-nexus-500 px-3 py-1.5 text-xs font-semibold text-white hover:bg-indigo-600 disabled:opacity-50"
              >
                {saving ? "Saving…" : "Save"}
              </button>
              {!needsEndpoint && (
                <button
                  onClick={() => setEditingEndpoint(false)}
                  className="rounded-lg border border-gray-200 px-3 py-1.5 text-xs text-gray-500 hover:bg-gray-50"
                >
                  Cancel
                </button>
              )}
            </div>
          </div>
        ) : agent.endpoint ? (
          <a
            href={agent.endpoint}
            className="block text-xs font-mono text-gray-500 truncate hover:underline"
            target="_blank"
            rel="noreferrer"
          >
            {agent.endpoint}
          </a>
        ) : (
          <p className="text-xs text-amber-600">
            No server URL configured.{" "}
            <button onClick={() => setEditingEndpoint(true)} className="underline hover:text-amber-800">
              Set one now
            </button>
          </p>
        )}
      </div>

      {/* Type badge */}
      <div className="text-xs">
        <span className={`rounded px-1.5 py-0.5 font-medium ${isHosted ? "bg-blue-50 text-blue-600" : "bg-gray-100 text-gray-600"}`}>
          {isHosted ? "Hosted" : agent.owner_domain || "Domain"}
        </span>
      </div>

      {/* Actions */}
      <div className="flex items-center gap-3 pt-1 border-t border-gray-100">
        {agent.status === "pending" && (
          <button
            onClick={handleActivate}
            className="text-xs text-nexus-500 hover:underline font-medium"
          >
            Activate →
          </button>
        )}
        <span className="ml-auto text-xs text-gray-300">
          {new Date(agent.created_at).toLocaleDateString()}
        </span>
      </div>
    </div>
  );
}

// ── Page ─────────────────────────────────────────────────────────────────────
export default function AccountPage() {
  const router = useRouter();
  const [user, setUser] = useState<UserClaims | null>(null);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const loadAgents = () => {
    const token = getToken();
    fetch(`${REGISTRY}/api/v1/users/me/agents`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then((r) => r.json())
      .then((data) => setAgents(data.agents ?? []))
      .catch(() => setError("Failed to load your agents."))
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    if (!isLoggedIn()) {
      router.replace("/login?next=/account");
      return;
    }
    setUser(getUser());
    loadAgents();
  }, [router]);

  const handleLogout = () => {
    clearToken();
    window.location.href = "/";
  };

  if (!user) return null;

  const active   = agents.filter((a) => a.status === "active").length;
  const pending  = agents.filter((a) => a.status === "pending").length;
  const hosted   = agents.filter((a) => a.registration_type === "nap_hosted").length;

  return (
    <div className="mx-auto max-w-4xl space-y-8">

      {/* Profile header */}
      <div className="rounded-xl border border-gray-200 bg-white p-6 flex items-center gap-5">
        <Avatar username={user.username} />
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-3 flex-wrap">
            <h1 className="text-xl font-bold text-gray-900">{user.username}</h1>
            <span className="rounded-full bg-nexus-50 border border-nexus-200 px-2.5 py-0.5 text-xs font-semibold text-nexus-600 capitalize">
              {user.tier ?? "free"}
            </span>
          </div>
          <p className="text-sm text-gray-500 mt-0.5">{user.email}</p>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <a
            href="/register"
            className="rounded-lg bg-nexus-500 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-600 transition-colors"
          >
            Register Agent
          </a>
          <button
            onClick={handleLogout}
            className="rounded-lg border border-gray-200 px-4 py-2 text-sm text-gray-500 hover:bg-gray-50 transition-colors"
          >
            Logout
          </button>
        </div>
      </div>

      {/* Stats row */}
      <div className="grid grid-cols-3 gap-4">
        {[
          { label: "Total agents", value: agents.length },
          { label: "Active",       value: active  },
          { label: "Pending",      value: pending },
        ].map((s) => (
          <div key={s.label} className="rounded-xl border border-gray-200 bg-white px-5 py-4 text-center">
            <p className="text-2xl font-bold text-gray-900">{s.value}</p>
            <p className="text-xs text-gray-400 mt-0.5">{s.label}</p>
          </div>
        ))}
      </div>

      {/* Agents section */}
      <div>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-base font-semibold text-gray-900">
            My Agents
            {hosted > 0 && (
              <span className="ml-2 text-xs font-normal text-gray-400">
                {hosted} hosted · {agents.length - hosted} domain-verified
              </span>
            )}
          </h2>
          <a href="/register" className="text-sm text-nexus-500 hover:underline">
            + Register another
          </a>
        </div>

        {loading && (
          <div className="rounded-xl border border-gray-200 bg-white p-12 text-center text-gray-400 text-sm">
            Loading your agents…
          </div>
        )}

        {error && (
          <div className="rounded-xl border border-red-100 bg-red-50 p-4 text-sm text-red-700">
            {error}
          </div>
        )}

        {!loading && !error && agents.length === 0 && (
          <div className="rounded-xl border border-dashed border-gray-300 bg-white p-12 text-center">
            <p className="text-gray-500 text-sm mb-4">You haven't registered any agents yet.</p>
            <a
              href="/register"
              className="inline-block rounded-lg bg-nexus-500 px-5 py-2.5 text-sm font-medium text-white hover:bg-indigo-600 transition-colors"
            >
              Register your first agent
            </a>
          </div>
        )}

        {!loading && agents.length > 0 && (
          <div className="grid gap-4 sm:grid-cols-2">
            {agents.map((agent) => (
              <AgentCard key={agent.id} agent={agent} onUpdated={loadAgents} />
            ))}
          </div>
        )}
      </div>

    </div>
  );
}
