"use client";

import { useEffect, useState } from "react";
import { getToken, getUser, clearToken, isLoggedIn, UserClaims } from "../../lib/auth";
import { Agent, agentURI, TIER_STYLES, STATUS_STYLES } from "../../lib/agent";
import { useRouter } from "next/navigation";

// ── Edit profile state ────────────────────────────────────────────────────────

interface ProfileForm {
  bio:         string;
  avatar_url:  string;
  website_url: string;
}

const REGISTRY = process.env.NEXT_PUBLIC_REGISTRY_URL ?? "http://localhost:8080";

// ── Badges ───────────────────────────────────────────────────────────────────

function TierBadge({ tier }: { tier: string }) {
  return (
    <span className={`rounded-full px-2 py-0.5 text-xs font-semibold capitalize ${TIER_STYLES[tier] ?? TIER_STYLES.unverified}`}>
      {tier}
    </span>
  );
}

function StatusBadge({ status }: { status: string }) {
  return (
    <span className={`rounded-full px-2 py-0.5 text-xs font-semibold capitalize ${STATUS_STYLES[status] ?? "bg-gray-100 text-gray-500"}`}>
      {status}
    </span>
  );
}

// ── Avatar ───────────────────────────────────────────────────────────────────

function Avatar({ username }: { username: string }) {
  return (
    <div className="h-14 w-14 rounded-full bg-nexus-500 flex items-center justify-center text-white text-xl font-bold select-none">
      {username.slice(0, 2).toUpperCase()}
    </div>
  );
}

// ── Agent list row ────────────────────────────────────────────────────────────

function AgentRow({ agent }: { agent: Agent }) {
  const isHosted = agent.registration_type === "nap_hosted";
  const typeLabel = isHosted ? "Hosted" : (agent.owner_domain || "Domain");
  const typeStyle = isHosted
    ? "bg-blue-50 text-blue-600"
    : "bg-gray-100 text-gray-600";

  return (
    <a
      href={`/account/agents/${agent.id}`}
      className="flex items-center gap-4 px-5 py-4 hover:bg-gray-50 transition-colors group cursor-pointer"
    >
      {/* Name + URI */}
      <div className="flex-1 min-w-0">
        <p className="font-medium text-gray-900 text-sm truncate">{agent.display_name}</p>
        <p className="text-xs text-gray-400 font-mono mt-0.5 truncate">{agentURI(agent)}</p>
      </div>

      {/* Badges */}
      <div className="hidden sm:flex items-center gap-2 shrink-0">
        <StatusBadge status={agent.status} />
        <TierBadge tier={agent.trust_tier} />
        <span className={`rounded px-1.5 py-0.5 text-xs font-medium ${typeStyle}`}>
          {typeLabel}
        </span>
        <span className="text-xs text-gray-300 w-20 text-right">
          {new Date(agent.created_at).toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" })}
        </span>
      </div>

      {/* Mobile badges */}
      <div className="flex sm:hidden items-center gap-2 shrink-0">
        <StatusBadge status={agent.status} />
      </div>

      {/* Arrow */}
      <span className="text-gray-300 group-hover:text-gray-500 transition-colors text-sm select-none">
        →
      </span>
    </a>
  );
}

// ── Page ─────────────────────────────────────────────────────────────────────

export default function AccountPage() {
  const router = useRouter();
  const [user, setUser]       = useState<UserClaims | null>(null);
  const [agents, setAgents]   = useState<Agent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError]     = useState("");

  // Profile edit state
  const [profileForm, setProfileForm]   = useState<ProfileForm>({ bio: "", avatar_url: "", website_url: "" });
  const [profileSaving, setProfileSaving] = useState(false);
  const [profileOk, setProfileOk]       = useState(false);
  const [profileErr, setProfileErr]     = useState("");

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

  const handleProfileSave = async (e: React.FormEvent) => {
    e.preventDefault();
    setProfileSaving(true);
    setProfileOk(false);
    setProfileErr("");
    try {
      const r = await fetch(`${REGISTRY}/api/v1/users/me/profile`, {
        method: "PATCH",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${getToken()}`,
        },
        body: JSON.stringify(profileForm),
      });
      if (!r.ok) {
        const d = await r.json().catch(() => ({})) as { error?: string };
        setProfileErr(d.error ?? `HTTP ${r.status}`);
      } else {
        setProfileOk(true);
        setTimeout(() => setProfileOk(false), 3000);
      }
    } catch {
      setProfileErr("Network error");
    } finally {
      setProfileSaving(false);
    }
  };

  if (!user) return null;

  const active  = agents.filter((a) => a.status === "active").length;
  const pending = agents.filter((a) => a.status === "pending").length;
  const hosted  = agents.filter((a) => a.registration_type === "nap_hosted").length;

  return (
    <div className="mx-auto max-w-3xl space-y-8">

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

      {/* Stats */}
      <div className="grid grid-cols-3 gap-4">
        {[
          { label: "Total agents", value: agents.length },
          { label: "Active",       value: active },
          { label: "Pending",      value: pending },
        ].map((s) => (
          <div key={s.label} className="rounded-xl border border-gray-200 bg-white px-5 py-4 text-center">
            <p className="text-2xl font-bold text-gray-900">{s.value}</p>
            <p className="text-xs text-gray-400 mt-0.5">{s.label}</p>
          </div>
        ))}
      </div>

      {/* Edit Profile */}
      <div className="rounded-xl border border-gray-200 bg-white p-6 space-y-4">
        <h2 className="text-base font-semibold text-gray-900">Edit Profile</h2>
        <form onSubmit={handleProfileSave} className="space-y-4">
          <div>
            <label className="block text-xs font-medium text-gray-500 mb-1">Bio</label>
            <textarea
              rows={2}
              value={profileForm.bio}
              onChange={(e) => setProfileForm((f) => ({ ...f, bio: e.target.value }))}
              placeholder="A short description about yourself"
              className="w-full rounded-lg border border-gray-300 px-4 py-2.5 text-sm focus:border-nexus-500 focus:outline-none resize-none"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-500 mb-1">Avatar URL</label>
            <input
              type="url"
              value={profileForm.avatar_url}
              onChange={(e) => setProfileForm((f) => ({ ...f, avatar_url: e.target.value }))}
              placeholder="https://example.com/avatar.png"
              className="w-full rounded-lg border border-gray-300 px-4 py-2.5 text-sm focus:border-nexus-500 focus:outline-none"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-500 mb-1">Website URL</label>
            <input
              type="url"
              value={profileForm.website_url}
              onChange={(e) => setProfileForm((f) => ({ ...f, website_url: e.target.value }))}
              placeholder="https://yourwebsite.com"
              className="w-full rounded-lg border border-gray-300 px-4 py-2.5 text-sm focus:border-nexus-500 focus:outline-none"
            />
          </div>
          <div className="flex items-center gap-3">
            <button
              type="submit"
              disabled={profileSaving}
              className="rounded-lg bg-nexus-500 px-5 py-2.5 text-sm font-semibold text-white hover:bg-indigo-600 disabled:opacity-50 transition-colors"
            >
              {profileSaving ? "Saving…" : "Save Profile"}
            </button>
            {profileOk && <span className="text-sm text-green-600">Saved!</span>}
            {profileErr && <span className="text-sm text-red-600">{profileErr}</span>}
          </div>
          <p className="text-xs text-gray-400">
            Your profile is public at{" "}
            <a href={`/users/${user.username}`} className="text-nexus-500 hover:underline">
              /users/{user.username}
            </a>
          </p>
        </form>
      </div>

      {/* Agents list */}
      <div>
        <div className="flex items-center justify-between mb-3">
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
          <div className="rounded-xl border border-gray-200 bg-white p-10 text-center text-gray-400 text-sm">
            Loading…
          </div>
        )}

        {error && (
          <div className="rounded-xl border border-red-100 bg-red-50 p-4 text-sm text-red-700">
            {error}
          </div>
        )}

        {!loading && !error && agents.length === 0 && (
          <div className="rounded-xl border border-dashed border-gray-300 bg-white p-12 text-center">
            <p className="text-gray-500 text-sm mb-4">You haven&apos;t registered any agents yet.</p>
            <a
              href="/register"
              className="inline-block rounded-lg bg-nexus-500 px-5 py-2.5 text-sm font-medium text-white hover:bg-indigo-600 transition-colors"
            >
              Register your first agent
            </a>
          </div>
        )}

        {!loading && agents.length > 0 && (
          <div className="rounded-xl border border-gray-200 bg-white overflow-hidden divide-y divide-gray-100">
            {agents.map((agent) => (
              <AgentRow key={agent.id} agent={agent} />
            ))}
          </div>
        )}
      </div>

    </div>
  );
}
