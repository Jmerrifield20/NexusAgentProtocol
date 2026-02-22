"use client";

import { useEffect, useState } from "react";
import { getToken, setToken, getUser, isLoggedIn, UserClaims } from "../../lib/auth";

// ---------- Domain-Verified form ----------

interface DomainFormData {
  capability_node: string;
  display_name: string;
  description: string;
  endpoint: string;
  owner_domain: string;
}

const CAPABILITY_OPTIONS = [
  { value: "finance", label: "Finance" },
  { value: "legal", label: "Legal" },
  { value: "support", label: "Support" },
  { value: "sales", label: "Sales" },
  { value: "marketing", label: "Marketing" },
  { value: "engineering", label: "Engineering" },
  { value: "hr", label: "HR" },
  { value: "data-analysis", label: "Data Analysis" },
  { value: "research", label: "Research" },
  { value: "logistics", label: "Logistics" },
  { value: "healthcare", label: "Healthcare" },
  { value: "education", label: "Education" },
  { value: "custom", label: "Custom — write your own" },
];

function DomainVerifiedTab() {
  const [form, setForm] = useState<DomainFormData>({
    capability_node: "",
    display_name: "",
    description: "",
    endpoint: "",
    owner_domain: "",
  });
  const [capabilityMode, setCapabilityMode] = useState<"select" | "custom">("select");
  const [result, setResult] = useState<unknown | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const handleChange = (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    setForm((prev) => ({ ...prev, [e.target.name]: e.target.value }));
  };

  const handleCapabilitySelect = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const val = e.target.value;
    if (val === "custom") {
      setCapabilityMode("custom");
      setForm((prev) => ({ ...prev, capability_node: "" }));
    } else {
      setCapabilityMode("select");
      setForm((prev) => ({ ...prev, capability_node: val }));
    }
  };

  const handleCustomCapability = (e: React.ChangeEvent<HTMLInputElement>) => {
    const val = e.target.value.replace(/\//g, "");
    setForm((prev) => ({ ...prev, capability_node: val }));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setResult(null);
    setLoading(true);

    try {
      const base = process.env.NEXT_PUBLIC_REGISTRY_URL ?? "http://localhost:8080";
      const resp = await fetch(`${base}/api/v1/agents`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(form),
      });

      const body = await resp.json();
      if (!resp.ok) throw new Error(body.error ?? `HTTP ${resp.status}`);
      setResult(body);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  };

  const otherFields: { name: keyof DomainFormData; label: string; placeholder: string; hint: string; required?: boolean }[] = [
    {
      name: "display_name",
      label: "Display Name",
      placeholder: "My Tax Agent",
      hint: "A human-readable name shown in listings and search results.",
      required: true,
    },
    {
      name: "description",
      label: "Description",
      placeholder: "Handles tax filing queries",
      hint: "Optional. A short summary of what this agent does.",
    },
    {
      name: "endpoint",
      label: "Endpoint URL",
      placeholder: "https://my-agent.example.com",
      hint: "The publicly reachable URL where this agent accepts requests. Must be a full URL including scheme.",
      required: true,
    },
    {
      name: "owner_domain",
      label: "Owner Domain",
      placeholder: "example.com",
      hint: "The domain you own and control. This becomes the trust root for your agent URI (e.g. agent://acme.com/...) and is verified via DNS-01 challenge.",
      required: true,
    },
  ];

  return (
    <>
      <p className="mb-8 text-gray-500">
        After submission, your agent will be in <strong>pending</strong> status
        until you complete the DNS-01 domain ownership challenge.
      </p>

      <form onSubmit={handleSubmit} className="space-y-5">
        {/* Capability Node */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Capability Node <span className="text-red-500">*</span>
          </label>
          <select
            value={capabilityMode === "custom" ? "custom" : form.capability_node}
            onChange={handleCapabilitySelect}
            required={capabilityMode === "select"}
            className="w-full rounded-lg border border-gray-300 px-4 py-3 text-sm focus:border-nexus-500 focus:outline-none bg-white"
          >
            <option value="" disabled>Select a category...</option>
            {CAPABILITY_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>{opt.label}</option>
            ))}
          </select>
          {capabilityMode === "custom" && (
            <input
              type="text"
              name="capability_node"
              value={form.capability_node}
              onChange={handleCustomCapability}
              placeholder="e.g. compliance-review"
              required
              className="mt-2 w-full rounded-lg border border-gray-300 px-4 py-3 text-sm focus:border-nexus-500 focus:outline-none"
            />
          )}
          <p className="mt-1 text-xs text-gray-400">
            The category this agent belongs to. This appears in the agent:// URI — e.g.{" "}
            <code className="font-mono">agent://acme.com/finance/…</code>
          </p>
        </div>

        {otherFields.map((f) => (
          <div key={f.name}>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              {f.label} {f.required && <span className="text-red-500">*</span>}
            </label>
            <input
              type="text"
              name={f.name}
              value={form[f.name]}
              onChange={handleChange}
              placeholder={f.placeholder}
              required={f.required}
              className="w-full rounded-lg border border-gray-300 px-4 py-3 text-sm focus:border-nexus-500 focus:outline-none"
            />
            <p className="mt-1 text-xs text-gray-400">{f.hint}</p>
          </div>
        ))}

        <button
          type="submit"
          disabled={loading}
          className="w-full rounded-lg bg-nexus-500 px-6 py-3 text-white font-semibold hover:bg-indigo-600 disabled:opacity-50"
        >
          {loading ? "Registering..." : "Register Agent"}
        </button>
      </form>

      {error && (
        <div className="mt-6 rounded-lg bg-red-50 p-4 text-red-600">
          <strong>Error:</strong> {error}
        </div>
      )}

      {result && (
        <div className="mt-6 rounded-xl border border-green-200 bg-green-50 p-6">
          <h2 className="font-semibold text-green-800 mb-3">
            Agent Registered Successfully
          </h2>
          <pre className="text-xs font-mono text-green-700 overflow-x-auto">
            {JSON.stringify(result, null, 2)}
          </pre>
        </div>
      )}
    </>
  );
}

// ---------- Free Hosted form ----------

interface HostedResult {
  id?: string;
  trust_root?: string;
  capability_node?: string;
  agent_id?: string;
  endpoint?: string;
  [key: string]: unknown;
}

function FreeHostedTab() {
  const REGISTRY = process.env.NEXT_PUBLIC_REGISTRY_URL ?? "http://localhost:8080";

  // Agent form state
  const [user, setUser] = useState<UserClaims | null>(null);
  const [agentName, setAgentName] = useState("");
  const [description, setDescription] = useState("");
  const [result, setResult] = useState<HostedResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  // Inline auth panel state
  const [showAuth, setShowAuth] = useState(false);
  const [authMode, setAuthMode] = useState<"signup" | "login">("signup");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [authError, setAuthError] = useState<string | null>(null);
  const [authLoading, setAuthLoading] = useState(false);

  useEffect(() => {
    if (isLoggedIn()) setUser(getUser());
  }, []);

  // Core registration call — used both after inline auth and when already logged in.
  const registerAgent = async (token: string): Promise<boolean> => {
    const resp = await fetch(`${REGISTRY}/api/v1/agents`, {
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
      body: JSON.stringify({ display_name: agentName, description, registration_type: "nap_hosted" }),
    });
    const body = await resp.json();
    if (resp.status === 422 || resp.status === 403) {
      setError("Free tier limit of 3 agents reached.");
      return false;
    }
    if (!resp.ok) {
      setError(body.error ?? `HTTP ${resp.status}`);
      return false;
    }
    setResult(body as HostedResult);
    return true;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setResult(null);

    if (!user) {
      // Show inline auth instead of blocking or redirecting.
      setShowAuth(true);
      return;
    }

    setLoading(true);
    try {
      await registerAgent(getToken()!);
    } catch {
      setError("Something went wrong. Please try again.");
    } finally {
      setLoading(false);
    }
  };

  const handleAuth = async (e: React.FormEvent) => {
    e.preventDefault();
    setAuthError(null);
    setAuthLoading(true);

    try {
      const url = authMode === "signup" ? "/api/v1/auth/signup" : "/api/v1/auth/login";
      const resp = await fetch(`${REGISTRY}${url}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, password }),
      });
      const body = await resp.json();

      if (!resp.ok) {
        if (resp.status === 409) setAuthError("Email already in use — log in instead.");
        else if (resp.status === 401) setAuthError("Invalid email or password.");
        else setAuthError(body.error ?? `Error ${resp.status}`);
        return;
      }

      // Store the token and update user state.
      setToken(body.token);
      const claims = getUser();
      setUser(claims);
      setShowAuth(false);

      // Auto-complete the registration.
      setLoading(true);
      await registerAgent(body.token);
    } catch {
      setAuthError("Something went wrong. Please try again.");
    } finally {
      setAuthLoading(false);
      setLoading(false);
    }
  };

  const uriPreview = user
    ? `agent://nexusagentprotocol.com/hosted/${user.username}/…`
    : `agent://nexusagentprotocol.com/hosted/you/…`;

  return (
    <>
      <p className="mb-4 text-gray-500">
        Your agent will be registered under the Nexus domain. We assign both the address and the endpoint — you don&apos;t need to own a domain or run a server to get started.
      </p>
      <div className="mb-8 rounded-lg bg-gray-50 border border-gray-200 px-4 py-3">
        <p className="text-xs text-gray-400 mb-1">Your agent address will look like</p>
        <code className="text-sm font-mono text-nexus-500">{uriPreview}</code>
      </div>

      <form onSubmit={handleSubmit} className="space-y-5">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Agent Name <span className="text-red-500">*</span>
          </label>
          <input
            type="text"
            value={agentName}
            onChange={(e) => setAgentName(e.target.value)}
            required
            placeholder="My Research Agent"
            className="w-full rounded-lg border border-gray-300 px-4 py-3 text-sm focus:border-nexus-500 focus:outline-none"
          />
          <p className="mt-1 text-xs text-gray-400">A human-readable name shown in listings and search results.</p>
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Description</label>
          <textarea
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            rows={3}
            placeholder="A short summary of what this agent does."
            className="w-full rounded-lg border border-gray-300 px-4 py-3 text-sm focus:border-nexus-500 focus:outline-none resize-none"
          />
        </div>

        <button
          type="submit"
          disabled={loading}
          className="w-full rounded-lg bg-nexus-500 px-6 py-3 text-white font-semibold hover:bg-indigo-600 disabled:opacity-50 transition-colors"
        >
          {loading ? "Registering…" : "Register Agent"}
        </button>
      </form>

      {/* Inline auth panel — appears when user submits without being logged in */}
      {showAuth && !result && (
        <div className="mt-6 rounded-xl border border-gray-200 bg-gray-50 p-6">
          <p className="text-sm font-semibold text-gray-900 mb-1">
            {authMode === "signup" ? "Create a free account to register" : "Log in to register"}
          </p>
          <p className="text-xs text-gray-500 mb-5">
            {authMode === "signup"
              ? "Takes 10 seconds. We'll register your agent right after."
              : "Welcome back — we'll register your agent right after."}
          </p>

          <form onSubmit={handleAuth} className="space-y-3">
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              placeholder="Email address"
              className="w-full rounded-lg border border-gray-300 px-4 py-2.5 text-sm focus:border-nexus-500 focus:outline-none bg-white"
            />
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              placeholder="Password"
              minLength={authMode === "signup" ? 8 : 1}
              className="w-full rounded-lg border border-gray-300 px-4 py-2.5 text-sm focus:border-nexus-500 focus:outline-none bg-white"
            />

            {authError && (
              <p className="text-xs text-red-600">{authError}</p>
            )}

            <button
              type="submit"
              disabled={authLoading}
              className="w-full rounded-lg bg-nexus-500 px-4 py-2.5 text-sm font-semibold text-white hover:bg-indigo-600 disabled:opacity-50 transition-colors"
            >
              {authLoading
                ? "Just a moment…"
                : authMode === "signup"
                ? "Create account & register agent"
                : "Log in & register agent"}
            </button>
          </form>

          <p className="mt-4 text-center text-xs text-gray-400">
            {authMode === "signup" ? (
              <>Already have an account?{" "}
                <button onClick={() => { setAuthMode("login"); setAuthError(null); }} className="text-nexus-500 hover:underline">Log in</button>
              </>
            ) : (
              <>New here?{" "}
                <button onClick={() => { setAuthMode("signup"); setAuthError(null); }} className="text-nexus-500 hover:underline">Create a free account</button>
              </>
            )}
          </p>
        </div>
      )}

      {error && (
        <div className="mt-6 rounded-lg bg-red-50 p-4 text-red-600 text-sm">
          <strong>Error:</strong> {error}
        </div>
      )}

      {result && (
        <div className="mt-6 rounded-xl border border-gray-200 bg-white p-6 shadow-sm space-y-4">
          <p className="text-xs font-semibold uppercase tracking-widest text-gray-400">Agent Registered</p>

          {result.trust_root && result.capability_node && result.agent_id && (
            <div>
              <p className="text-xs font-medium text-gray-500 mb-1">Your agent address (URI)</p>
              <code className="block rounded-lg bg-gray-50 border border-gray-100 px-4 py-3 text-sm font-mono text-nexus-500 break-all">
                agent://{result.trust_root}/{result.capability_node}/{result.agent_id}
              </code>
              <p className="mt-1 text-xs text-gray-400">
                This is your permanent address. Share it — other systems use it to find your agent.
              </p>
            </div>
          )}

          <div className="rounded-lg bg-gray-50 border border-gray-200 px-4 py-3 text-sm text-gray-700 space-y-1">
            <p className="font-medium">What happens next</p>
            <ol className="list-decimal list-inside space-y-1 text-gray-500 text-xs">
              <li>Verify your email (check your inbox).</li>
              <li>Activate your agent from your <a href="/account" className="text-nexus-500 hover:underline">account dashboard</a>.</li>
              <li>Set your server URL — the address where your agent accepts requests. You can do this any time from the dashboard.</li>
            </ol>
          </div>

          <p className="text-xs text-gray-400">
            Your <code className="font-mono">agent://</code> URI is permanent from this moment. Your server URL can be updated any time.
          </p>
        </div>
      )}
    </>
  );
}

// ---------- Page ----------

type Tab = "hosted" | "domain";

export default function RegisterPage() {
  const [tab, setTab] = useState<Tab>("hosted");

  return (
    <div className="max-w-2xl">
      <h1 className="mb-6 text-3xl font-bold">Register an Agent</h1>

      {/* Tab switcher */}
      <div className="mb-8 flex rounded-lg border border-gray-200 overflow-hidden text-sm font-medium">
        <button
          onClick={() => setTab("hosted")}
          className={`flex-1 px-4 py-2.5 transition-colors ${
            tab === "hosted"
              ? "bg-nexus-500 text-white"
              : "bg-white text-gray-600 hover:bg-gray-50"
          }`}
        >
          Free Hosted
        </button>
        <button
          onClick={() => setTab("domain")}
          className={`flex-1 px-4 py-2.5 transition-colors border-l border-gray-200 ${
            tab === "domain"
              ? "bg-nexus-500 text-white"
              : "bg-white text-gray-600 hover:bg-gray-50"
          }`}
        >
          Domain-Verified
        </button>
      </div>

      {tab === "hosted" ? <FreeHostedTab /> : <DomainVerifiedTab />}
    </div>
  );
}
