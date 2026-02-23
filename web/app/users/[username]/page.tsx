"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { Agent, agentURI, TIER_STYLES, STATUS_STYLES } from "../../../lib/agent";

const REGISTRY = process.env.NEXT_PUBLIC_REGISTRY_URL ?? "http://localhost:8080";

// ── Types ──────────────────────────────────────────────────────────────────────

interface PublicProfile {
  username:         string;
  display_name:     string;
  bio:              string;
  avatar_url:       string;
  website_url:      string;
  email_verified:   boolean;
  verified_domains: string[];
  agent_count:      number;
  member_since:     string;
}

// ── Helpers ────────────────────────────────────────────────────────────────────

function Avatar({ name, avatarURL }: { name: string; avatarURL: string }) {
  if (avatarURL) {
    return (
      <img
        src={avatarURL}
        alt={name}
        className="h-16 w-16 rounded-full object-cover border border-gray-200"
      />
    );
  }
  return (
    <div className="h-16 w-16 rounded-full bg-nexus-500 flex items-center justify-center text-white text-2xl font-bold select-none shrink-0">
      {name.slice(0, 2).toUpperCase()}
    </div>
  );
}

function TierBadge({ tier }: { tier?: string }) {
  if (!tier) return null;
  return (
    <span className={`rounded-full px-2 py-0.5 text-xs font-medium capitalize ${TIER_STYLES[tier] ?? TIER_STYLES.unverified}`}>
      {tier}
    </span>
  );
}

function StatusBadge({ status }: { status?: string }) {
  if (!status) return null;
  return (
    <span className={`rounded-full px-2 py-0.5 text-xs font-medium capitalize ${STATUS_STYLES[status] ?? "bg-gray-100 text-gray-500"}`}>
      {status}
    </span>
  );
}

// ── Page ──────────────────────────────────────────────────────────────────────

export default function UserProfilePage() {
  const params = useParams<{ username: string }>();
  const username = params.username;

  const [profile, setProfile]   = useState<PublicProfile | null>(null);
  const [agents, setAgents]     = useState<Agent[]>([]);
  const [loading, setLoading]   = useState(true);
  const [notFound, setNotFound] = useState(false);

  useEffect(() => {
    const profileReq = fetch(`${REGISTRY}/api/v1/users/${encodeURIComponent(username)}`)
      .then((r) => {
        if (r.status === 404) { setNotFound(true); return null; }
        if (!r.ok) throw new Error(`HTTP ${r.status}`);
        return r.json() as Promise<PublicProfile>;
      })
      .then((data) => { if (data) setProfile(data); })
      .catch(() => setNotFound(true));

    const agentsReq = fetch(`${REGISTRY}/api/v1/users/${encodeURIComponent(username)}/agents?limit=50`)
      .then((r) => r.ok ? r.json() : null)
      .then((data) => { if (data?.agents) setAgents(data.agents); })
      .catch(() => {});

    Promise.all([profileReq, agentsReq]).finally(() => setLoading(false));
  }, [username]);

  if (loading) {
    return (
      <div className="max-w-2xl mx-auto mt-16 text-center text-gray-400 text-sm">Loading…</div>
    );
  }

  if (notFound || !profile) {
    return (
      <div className="max-w-2xl mx-auto mt-16 text-center">
        <p className="text-2xl font-bold text-gray-800 mb-2">User not found</p>
        <p className="text-gray-500 text-sm mb-6">This profile is private or does not exist.</p>
        <a href="/agents" className="text-sm text-nexus-500 hover:underline">
          ← Back to agent directory
        </a>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-2xl space-y-6">

      {/* Back nav */}
      <a
        href="/agents"
        className="inline-flex items-center gap-1 text-sm text-gray-400 hover:text-gray-600 transition-colors"
      >
        ← Agent directory
      </a>

      {/* Profile header */}
      <div className="rounded-xl border border-gray-200 bg-white p-6">
        <div className="flex items-start gap-5">
          <Avatar name={profile.display_name || profile.username} avatarURL={profile.avatar_url} />
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-3 flex-wrap">
              <h1 className="text-xl font-bold text-gray-900">
                {profile.display_name || profile.username}
              </h1>
              {profile.email_verified && (
                <span className="rounded-full bg-green-50 border border-green-200 px-2.5 py-0.5 text-xs font-medium text-green-700">
                  verified
                </span>
              )}
            </div>
            <p className="text-sm text-gray-400 mt-0.5">@{profile.username}</p>
            {profile.bio && (
              <p className="mt-2 text-sm text-gray-600">{profile.bio}</p>
            )}
            {profile.website_url && (
              <a
                href={profile.website_url}
                target="_blank"
                rel="noreferrer"
                className="mt-1 inline-block text-xs text-nexus-500 hover:underline break-all"
              >
                {profile.website_url}
              </a>
            )}
          </div>
        </div>

        {/* Stats row */}
        <div className="mt-5 flex gap-6 border-t border-gray-100 pt-4">
          <div className="text-center">
            <p className="text-xl font-bold text-gray-900">{profile.agent_count}</p>
            <p className="text-xs text-gray-400">Active agents</p>
          </div>
          {profile.verified_domains.length > 0 && (
            <div className="text-center">
              <p className="text-xl font-bold text-gray-900">{profile.verified_domains.length}</p>
              <p className="text-xs text-gray-400">Verified domain{profile.verified_domains.length !== 1 ? "s" : ""}</p>
            </div>
          )}
          <div className="text-center">
            <p className="text-sm font-semibold text-gray-700">
              {new Date(profile.member_since).toLocaleDateString(undefined, { month: "short", year: "numeric" })}
            </p>
            <p className="text-xs text-gray-400">Member since</p>
          </div>
        </div>

        {/* Verified domains */}
        {profile.verified_domains.length > 0 && (
          <div className="mt-4 flex flex-wrap gap-2">
            {profile.verified_domains.map((d) => (
              <span
                key={d}
                className="rounded-full border border-indigo-200 bg-indigo-50 px-3 py-0.5 text-xs font-mono text-indigo-700"
              >
                {d}
              </span>
            ))}
          </div>
        )}
      </div>

      {/* Agent list */}
      {agents.length > 0 && (
        <div>
          <h2 className="text-base font-semibold text-gray-900 mb-3">
            Active Agents
            <span className="ml-2 text-xs font-normal text-gray-400">{agents.length}</span>
          </h2>
          <div className="rounded-xl border border-gray-200 bg-white overflow-hidden divide-y divide-gray-100">
            {agents.map((agent) => {
              const uri = agentURI(agent);
              return (
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
                    <code className="mt-0.5 block text-xs text-nexus-500 font-mono truncate">{uri}</code>
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
              );
            })}
          </div>
        </div>
      )}

      {agents.length === 0 && (
        <p className="text-sm text-gray-400 text-center py-6">
          No active agents registered by this user.
        </p>
      )}

    </div>
  );
}
