"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { getToken, isLoggedIn } from "../../../../lib/auth";
import {
  Agent, agentURI,
  TIER_STYLES, STATUS_STYLES, HEALTH_STYLES, healthLabel,
} from "../../../../lib/agent";

const REGISTRY = process.env.NEXT_PUBLIC_REGISTRY_URL ?? "http://localhost:8080";

// ── A2A card types ────────────────────────────────────────────────────────────

interface A2ASkill {
  id:          string;
  name:        string;
  description: string;
  tags?:       string[];
}

// ── Badges ────────────────────────────────────────────────────────────────────

function TierBadge({ tier }: { tier: string }) {
  return (
    <span className={`rounded-full px-2.5 py-0.5 text-xs font-semibold capitalize ${TIER_STYLES[tier] ?? TIER_STYLES.unverified}`}>
      {tier}
    </span>
  );
}

function StatusBadge({ status }: { status: string }) {
  return (
    <span className={`rounded-full px-2.5 py-0.5 text-xs font-semibold capitalize ${STATUS_STYLES[status] ?? "bg-gray-100 text-gray-500"}`}>
      {status}
    </span>
  );
}

function HealthBadge({ status }: { status: string }) {
  return (
    <span className={`rounded px-2 py-0.5 text-xs font-medium capitalize ${HEALTH_STYLES[status] ?? HEALTH_STYLES.unknown}`}>
      {healthLabel(status)}
    </span>
  );
}

// ── Read-only detail row ──────────────────────────────────────────────────────

function DetailRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="grid grid-cols-[9rem_1fr] gap-2 py-2.5 border-b border-gray-50 last:border-0">
      <p className="text-xs font-medium text-gray-400 pt-0.5">{label}</p>
      <div className="text-sm text-gray-800">{children}</div>
    </div>
  );
}

// ── Edit form ─────────────────────────────────────────────────────────────────

interface EditForm {
  display_name: string;
  description:  string;
  endpoint:     string;
  version:      string;
  tags:         string; // comma-separated in UI
  support_url:  string;

}

function formFromAgent(a: Agent): EditForm {
  return {
    display_name: a.display_name,
    description:  a.description,
    endpoint:     a.endpoint,
    version:      a.version ?? "",
    tags:         (a.tags ?? []).join(", "),
    support_url:  a.support_url ?? "",
  };
}

// ── Page ─────────────────────────────────────────────────────────────────────

export default function AgentDetailPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();

  const [agent, setAgent]     = useState<Agent | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [skills, setSkills]   = useState<A2ASkill[]>([]);

  const [form, setForm]       = useState<EditForm | null>(null);
  const [saving, setSaving]   = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saveOk, setSaveOk]   = useState(false);

  const [actionError, setActionError] = useState<string | null>(null);

  // ── Load agent ──

  const loadAgent = async () => {
    const [resp, cardResp] = await Promise.all([
      fetch(`${REGISTRY}/api/v1/agents/${params.id}`, {
        headers: { Authorization: `Bearer ${getToken()}` },
      }),
      fetch(`${REGISTRY}/api/v1/agents/${params.id}/agent.json`).catch(() => null),
    ]);
    if (resp.status === 404) { setNotFound(true); setLoading(false); return; }
    if (!resp.ok) { setLoading(false); return; }
    const data = await resp.json() as Agent;
    setAgent(data);
    setForm(formFromAgent(data));
    if (cardResp?.ok) {
      const card = await cardResp.json() as { skills?: A2ASkill[] };
      setSkills(card.skills ?? []);
    }
    setLoading(false);
  };

  useEffect(() => {
    if (!isLoggedIn()) {
      router.replace("/login?next=/account");
      return;
    }
    loadAgent();
  }, [params.id]); // eslint-disable-line react-hooks/exhaustive-deps

  // ── Save edits ──

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!form || !agent) return;
    setSaving(true);
    setSaveError(null);
    setSaveOk(false);

    const tags = form.tags.split(",").map((t) => t.trim()).filter(Boolean);

    const resp = await fetch(`${REGISTRY}/api/v1/agents/${agent.id}`, {
      method: "PATCH",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${getToken()}`,
      },
      body: JSON.stringify({
        display_name: form.display_name,
        description:  form.description,
        endpoint:     form.endpoint,
        version:      form.version,
        tags,
        support_url:  form.support_url,
      }),
    }).catch(() => null);

    if (!resp || !resp.ok) {
      const body = resp ? await resp.json().catch(() => ({})) as { error?: string } : {};
      setSaveError(body.error ?? "Save failed.");
    } else {
      setSaveOk(true);
      await loadAgent();
    }
    setSaving(false);
  };

  // ── Actions: activate / revoke ──

  const doAction = async (path: string, label: string) => {
    setActionError(null);
    const resp = await fetch(`${REGISTRY}/api/v1/agents/${params.id}/${path}`, {
      method: "POST",
      headers: { Authorization: `Bearer ${getToken()}` },
    }).catch(() => null);
    if (!resp || !resp.ok) {
      const body = resp ? await resp.json().catch(() => ({})) as { error?: string } : {};
      setActionError(body.error ?? `${label} failed.`);
    } else {
      await loadAgent();
    }
  };

  // ── Render ──

  if (loading) {
    return (
      <div className="max-w-2xl mx-auto mt-16 text-center text-gray-400 text-sm">
        Loading…
      </div>
    );
  }

  if (notFound || !agent || !form) {
    return (
      <div className="max-w-2xl mx-auto mt-16 text-center">
        <p className="text-gray-500">Agent not found.</p>
        <a href="/account" className="mt-4 inline-block text-sm text-nexus-500 hover:underline">
          ← Back to account
        </a>
      </div>
    );
  }

  const isHosted  = agent.registration_type === "nap_hosted";
  const uri       = agentURI(agent);
  const set = (f: keyof EditForm) =>
    (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) =>
      setForm((prev) => prev ? { ...prev, [f]: e.target.value } : prev);

  return (
    <div className="mx-auto max-w-2xl space-y-6">

      {/* Back nav */}
      <a href="/account" className="inline-flex items-center gap-1 text-sm text-gray-400 hover:text-gray-600 transition-colors">
        ← My Agents
      </a>

      {/* Agent header */}
      <div className="rounded-xl border border-gray-200 bg-white p-6">
        <div className="flex items-start justify-between gap-4 flex-wrap">
          <div>
            <h1 className="text-xl font-bold text-gray-900">{agent.display_name}</h1>
            {agent.description && (
              <p className="text-sm text-gray-500 mt-1 max-w-xl">{agent.description}</p>
            )}
          </div>
          <div className="flex items-center gap-2 flex-wrap">
            <StatusBadge status={agent.status} />
            <TierBadge   tier={agent.trust_tier} />
          </div>
        </div>
        <div className="mt-4">
          <p className="text-xs text-gray-400 mb-1">Agent URI</p>
          <code className="block text-sm font-mono text-nexus-500 bg-gray-50 rounded px-3 py-2 break-all">
            {uri}
          </code>
        </div>
      </div>

      {/* Details */}
      <div className="rounded-xl border border-gray-200 bg-white px-6 py-4">
        <p className="text-xs font-semibold uppercase tracking-wide text-gray-400 mb-2">Overview</p>

        <DetailRow label="Server URL">
          {agent.endpoint ? (
            <a
              href={agent.endpoint}
              target="_blank"
              rel="noreferrer"
              className="font-mono text-xs text-gray-700 hover:underline break-all"
            >
              {agent.endpoint}
            </a>
          ) : (
            <span className="text-amber-500 text-xs">Not set</span>
          )}
        </DetailRow>

        <DetailRow label="Type">
          <span className={`rounded px-1.5 py-0.5 text-xs font-medium ${isHosted ? "bg-blue-50 text-blue-600" : "bg-gray-100 text-gray-700"}`}>
            {isHosted ? "NAP Hosted" : "Domain Verified"}
          </span>
        </DetailRow>

        {!isHosted && agent.owner_domain && (
          <DetailRow label="Domain">
            <span className="font-mono text-xs">{agent.owner_domain}</span>
          </DetailRow>
        )}

        <DetailRow label="Certificate">
          {agent.cert_serial ? (
            <span className="font-mono text-xs text-gray-600">#{agent.cert_serial}</span>
          ) : (
            <span className="text-xs text-gray-400">None issued</span>
          )}
        </DetailRow>

        {agent.version && (
          <DetailRow label="Version">
            <span className="font-mono text-xs bg-gray-100 rounded px-1.5 py-0.5">v{agent.version}</span>
          </DetailRow>
        )}

        {agent.tags?.length > 0 && (
          <DetailRow label="Tags">
            <div className="flex flex-wrap gap-1">
              {agent.tags.map((t) => (
                <span key={t} className="rounded-full bg-indigo-50 text-indigo-600 px-2 py-0.5 text-xs">
                  {t}
                </span>
              ))}
            </div>
          </DetailRow>
        )}

        {skills.length > 0 && (
          <DetailRow label="Skills">
            <div className="flex flex-col gap-2">
              {skills.map((sk) => (
                <div key={sk.id} className="rounded-lg border border-gray-100 bg-gray-50 px-3 py-2">
                  <p className="text-xs font-semibold text-gray-800">{sk.name}</p>
                  {sk.description && (
                    <p className="text-xs text-gray-500 mt-0.5">{sk.description}</p>
                  )}
                  {sk.tags && sk.tags.length > 0 && (
                    <div className="flex flex-wrap gap-1 mt-1.5">
                      {sk.tags.map((t) => (
                        <span key={t} className="rounded-full bg-indigo-50 text-indigo-600 px-2 py-0.5 text-xs">
                          {t}
                        </span>
                      ))}
                    </div>
                  )}
                </div>
              ))}
            </div>
          </DetailRow>
        )}

        {agent.support_url && (
          <DetailRow label="Support URL">
            <a href={agent.support_url} target="_blank" rel="noreferrer"
              className="text-xs text-nexus-500 hover:underline break-all font-mono">
              {agent.support_url}
            </a>
          </DetailRow>
        )}

        <DetailRow label="Health">
          <HealthBadge status={agent.health_status || "unknown"} />
          {agent.last_seen_at && (
            <span className="ml-2 text-xs text-gray-400">
              last seen {new Date(agent.last_seen_at).toLocaleString()}
            </span>
          )}
        </DetailRow>

        <DetailRow label="Registered">
          <span className="text-xs text-gray-600">
            {new Date(agent.created_at).toLocaleDateString(undefined, {
              day: "numeric", month: "long", year: "numeric",
            })}
          </span>
        </DetailRow>

        {agent.updated_at && agent.updated_at !== agent.created_at && (
          <DetailRow label="Last updated">
            <span className="text-xs text-gray-600">
              {new Date(agent.updated_at).toLocaleDateString(undefined, {
                day: "numeric", month: "long", year: "numeric",
              })}
            </span>
          </DetailRow>
        )}
      </div>

      {/* Integration links */}
      {agent.status === "active" && (
        <div className="rounded-xl border border-gray-200 bg-white px-6 py-4">
          <p className="text-xs font-semibold uppercase tracking-wide text-gray-400 mb-3">Integration</p>
          <div className="space-y-3">
            <div className="flex items-center justify-between gap-4">
              <div>
                <p className="text-xs font-medium text-gray-700">A2A Agent Card</p>
                <p className="text-xs text-gray-400">A2A-spec JSON with skills and NAP endorsement. Deploy at <code className="font-mono">/.well-known/agent.json</code></p>
              </div>
              <a
                href={`${REGISTRY}/api/v1/agents/${agent.id}/agent.json`}
                target="_blank"
                rel="noreferrer"
                className="shrink-0 rounded-lg border border-gray-200 px-3 py-1.5 text-xs font-medium text-gray-600 hover:border-nexus-400 hover:text-nexus-500 transition-colors"
              >
                View JSON
              </a>
            </div>
            <div className="flex items-center justify-between gap-4 border-t border-gray-50 pt-3">
              <div>
                <p className="text-xs font-medium text-gray-700">MCP Manifest</p>
                <p className="text-xs text-gray-400">Model Context Protocol tool definitions (if declared at registration)</p>
              </div>
              <a
                href={`${REGISTRY}/api/v1/agents/${agent.id}/mcp-manifest.json`}
                target="_blank"
                rel="noreferrer"
                className="shrink-0 rounded-lg border border-gray-200 px-3 py-1.5 text-xs font-medium text-gray-600 hover:border-nexus-400 hover:text-nexus-500 transition-colors"
              >
                View JSON
              </a>
            </div>
          </div>
        </div>
      )}

      {/* Edit form */}
      <div className="rounded-xl border border-gray-200 bg-white px-6 py-5">
        <p className="text-xs font-semibold uppercase tracking-wide text-gray-400 mb-4">Edit Details</p>

        <form onSubmit={handleSave} className="space-y-4">

          <div className="grid grid-cols-2 gap-4">
            <div className="col-span-2 sm:col-span-1">
              <label className="block text-xs font-medium text-gray-600 mb-1">Display Name</label>
              <input
                type="text"
                value={form.display_name}
                onChange={set("display_name")}
                placeholder="My Tax Agent"
                className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-nexus-500 focus:outline-none"
              />
            </div>
            <div className="col-span-2 sm:col-span-1">
              <label className="block text-xs font-medium text-gray-600 mb-1">Version</label>
              <input
                type="text"
                value={form.version}
                onChange={set("version")}
                placeholder="1.0.0"
                className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-nexus-500 focus:outline-none"
              />
            </div>
          </div>

          <div>
            <label className="block text-xs font-medium text-gray-600 mb-1">Description</label>
            <textarea
              value={form.description}
              onChange={set("description")}
              rows={3}
              placeholder="What does this agent do?"
              className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-nexus-500 focus:outline-none resize-none"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-gray-600 mb-1">Server URL (endpoint)</label>
            <input
              type="url"
              value={form.endpoint}
              onChange={set("endpoint")}
              placeholder="https://my-agent.fly.dev"
              className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm font-mono focus:border-nexus-500 focus:outline-none"
            />
            <p className="mt-1 text-xs text-gray-400">
              Publicly reachable HTTPS URL where other agents send requests.
            </p>
          </div>

          <div>
            <label className="block text-xs font-medium text-gray-600 mb-1">Tags</label>
            <input
              type="text"
              value={form.tags}
              onChange={set("tags")}
              placeholder="tax, filing, usa"
              className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-nexus-500 focus:outline-none"
            />
            <p className="mt-1 text-xs text-gray-400">Comma-separated keywords for search and filtering.</p>
          </div>

          <div>
            <label className="block text-xs font-medium text-gray-600 mb-1">Support / Contact URL</label>
            <input
              type="url"
              value={form.support_url}
              onChange={set("support_url")}
              placeholder="https://docs.example.com/support"
              className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm font-mono focus:border-nexus-500 focus:outline-none"
            />
          </div>

          {saveError && <p className="text-xs text-red-600">{saveError}</p>}
          {saveOk    && <p className="text-xs text-emerald-600">Changes saved.</p>}

          <div className="flex gap-2 pt-1">
            <button
              type="submit"
              disabled={saving}
              className="rounded-lg bg-nexus-500 px-5 py-2 text-sm font-semibold text-white hover:bg-indigo-600 disabled:opacity-50 transition-colors"
            >
              {saving ? "Saving…" : "Save changes"}
            </button>
          </div>
        </form>
      </div>

      {/* Actions */}
      {(agent.status === "pending" || agent.status === "active") && (
        <div className="rounded-xl border border-gray-200 bg-white px-6 py-5">
          <p className="text-xs font-semibold uppercase tracking-wide text-gray-400 mb-4">Actions</p>

          <div className="flex flex-wrap gap-3">
            {agent.status === "pending" && (
              <button
                onClick={() => doAction("activate", "Activate")}
                className="rounded-lg bg-emerald-500 px-4 py-2 text-sm font-medium text-white hover:bg-emerald-600 transition-colors"
              >
                Activate agent
              </button>
            )}

            {agent.status === "active" && (
              <button
                onClick={() => {
                  if (window.confirm("Revoke this agent? It will stop being discoverable.")) {
                    doAction("revoke", "Revoke");
                  }
                }}
                className="rounded-lg border border-red-200 px-4 py-2 text-sm font-medium text-red-600 hover:bg-red-50 transition-colors"
              >
                Revoke agent
              </button>
            )}
          </div>

          {actionError && <p className="mt-3 text-xs text-red-600">{actionError}</p>}
        </div>
      )}

    </div>
  );
}
