"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import {
  Agent, agentURI,
  TIER_STYLES, STATUS_STYLES, HEALTH_STYLES, healthLabel,
} from "../../../lib/agent";
import { isLoggedIn } from "../../../lib/auth";

const REGISTRY = process.env.NEXT_PUBLIC_REGISTRY_URL ?? "http://localhost:8080";

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

// ── Detail row ────────────────────────────────────────────────────────────────

function DetailRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="grid grid-cols-[9rem_1fr] gap-2 py-2.5 border-b border-gray-50 last:border-0">
      <p className="text-xs font-medium text-gray-400 pt-0.5">{label}</p>
      <div className="text-sm text-gray-800">{children}</div>
    </div>
  );
}

// ── Copy button ───────────────────────────────────────────────────────────────

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);

  const copy = async () => {
    await navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <button
      onClick={copy}
      className="ml-2 rounded px-2 py-0.5 text-xs text-gray-400 border border-gray-200 hover:border-nexus-400 hover:text-nexus-500 transition-colors"
    >
      {copied ? "Copied!" : "Copy"}
    </button>
  );
}

// ── Page ──────────────────────────────────────────────────────────────────────

export default function PublicAgentDetailPage() {
  const params = useParams<{ id: string }>();

  const [agent, setAgent]     = useState<Agent | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [loggedIn, setLoggedIn] = useState(false);

  useEffect(() => {
    setLoggedIn(isLoggedIn());

    fetch(`${REGISTRY}/api/v1/agents/${params.id}`)
      .then((r) => {
        if (r.status === 404) { setNotFound(true); setLoading(false); return null; }
        if (!r.ok) throw new Error(`HTTP ${r.status}`);
        return r.json() as Promise<Agent>;
      })
      .then((data) => { if (data) { setAgent(data); } })
      .catch(() => setNotFound(true))
      .finally(() => setLoading(false));
  }, [params.id]);

  if (loading) {
    return (
      <div className="max-w-2xl mx-auto mt-16 text-center text-gray-400 text-sm">
        Loading…
      </div>
    );
  }

  if (notFound || !agent) {
    return (
      <div className="max-w-2xl mx-auto mt-16 text-center">
        <p className="text-2xl font-bold text-gray-800 mb-2">Agent not found</p>
        <p className="text-gray-500 text-sm mb-6">
          This agent may have been revoked or the ID is incorrect.
        </p>
        <a href="/agents" className="text-sm text-nexus-500 hover:underline">
          ← Back to agent directory
        </a>
      </div>
    );
  }

  const uri      = agentURI(agent);
  const isHosted = agent.registration_type === "nap_hosted";

  return (
    <div className="mx-auto max-w-2xl space-y-6">

      {/* Back nav */}
      <a
        href="/agents"
        className="inline-flex items-center gap-1 text-sm text-gray-400 hover:text-gray-600 transition-colors"
      >
        ← Agent directory
      </a>

      {/* Header card */}
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

        {/* URI block */}
        <div className="mt-4">
          <div className="flex items-center gap-1 mb-1">
            <p className="text-xs text-gray-400">Agent URI</p>
            <CopyButton text={uri} />
          </div>
          <code className="block text-sm font-mono text-nexus-500 bg-gray-50 rounded px-3 py-2 break-all">
            {uri}
          </code>
        </div>
      </div>

      {/* Overview */}
      <div className="rounded-xl border border-gray-200 bg-white px-6 py-4">
        <p className="text-xs font-semibold uppercase tracking-wide text-gray-400 mb-2">Overview</p>

        <DetailRow label="Capability">
          <span className="font-mono text-xs bg-gray-100 rounded px-1.5 py-0.5">
            {agent.capability_node.replace(/>/g, " > ")}
          </span>
        </DetailRow>

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
            <span className="text-xs text-gray-400">Not set</span>
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

        {agent.support_url && (
          <DetailRow label="Support">
            <a
              href={agent.support_url}
              target="_blank"
              rel="noreferrer"
              className="text-xs text-nexus-500 hover:underline break-all font-mono"
            >
              {agent.support_url}
            </a>
          </DetailRow>
        )}

        {agent.pricing_info && (
          <DetailRow label="Pricing">
            <span className="text-xs text-gray-600 whitespace-pre-wrap">{agent.pricing_info}</span>
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

        <DetailRow label="Certificate">
          {agent.cert_serial ? (
            <span className="font-mono text-xs text-gray-600">#{agent.cert_serial}</span>
          ) : (
            <span className="text-xs text-gray-400">None issued</span>
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

      {/* Connect section */}
      <div className="rounded-xl border border-gray-200 bg-white px-6 py-4">
        <p className="text-xs font-semibold uppercase tracking-wide text-gray-400 mb-3">Connect to this agent</p>
        <p className="text-xs text-gray-500 mb-3">
          Resolve this agent&apos;s endpoint at any time using the NAP resolver:
        </p>
        <code className="block text-xs font-mono bg-gray-50 rounded px-3 py-2 text-gray-700 break-all">
          GET {REGISTRY}/api/v1/resolve?trust_root={agent.trust_root}&capability_node={agent.capability_node}&agent_id={agent.agent_id}
        </code>
        <p className="mt-3 text-xs text-gray-400">
          The registry never proxies traffic between agents — it returns the endpoint and steps aside.
        </p>
      </div>

      {/* Manage link — only shown when logged in */}
      {loggedIn && (
        <div className="text-right">
          <a
            href={`/account/agents/${agent.id}`}
            className="text-xs text-gray-400 hover:text-nexus-500 transition-colors"
          >
            Manage this agent →
          </a>
        </div>
      )}

    </div>
  );
}
