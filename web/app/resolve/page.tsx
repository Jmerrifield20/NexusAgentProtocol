"use client";

import { useState } from "react";

interface ResolveResult {
  uri: string;
  endpoint: string;
  status: string;
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

export default function ResolvePage() {
  const [trustRoot, setTrustRoot] = useState("");
  const [capabilityNode, setCapabilityNode] = useState("");
  const [capabilityMode, setCapabilityMode] = useState<"select" | "custom">("select");
  const [agentId, setAgentId] = useState("");
  const [result, setResult] = useState<ResolveResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const previewUri =
    trustRoot && capabilityNode && agentId
      ? `agent://${trustRoot}/${capabilityNode}/${agentId}`
      : null;

  const handleCapabilitySelect = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const val = e.target.value;
    if (val === "custom") {
      setCapabilityMode("custom");
      setCapabilityNode("");
    } else {
      setCapabilityMode("select");
      setCapabilityNode(val);
    }
  };

  const handleCustomCapability = (e: React.ChangeEvent<HTMLInputElement>) => {
    setCapabilityNode(e.target.value.replace(/\//g, ""));
  };

  const handleResolve = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setResult(null);
    setLoading(true);

    try {
      const base = process.env.NEXT_PUBLIC_REGISTRY_URL ?? "http://localhost:8080";
      const resp = await fetch(
        `${base}/api/v1/resolve?trust_root=${encodeURIComponent(trustRoot)}&capability_node=${encodeURIComponent(capabilityNode)}&agent_id=${encodeURIComponent(agentId)}`
      );

      if (!resp.ok) {
        const body = await resp.json();
        throw new Error(body.error ?? `HTTP ${resp.status}`);
      }

      setResult(await resp.json());
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="max-w-2xl">
      <h1 className="mb-2 text-3xl font-bold">Resolve Agent URI</h1>
      <p className="mb-6 text-gray-500">
        Look up a registered agent by its address and get back the live endpoint to call.
      </p>

      {/* URI anatomy */}
      <div className="mb-8 rounded-xl border border-gray-200 bg-gray-50 p-5">
        <p className="mb-3 text-xs font-semibold uppercase tracking-wide text-gray-400">
          URI anatomy
        </p>
        <code className="block text-sm font-mono text-gray-700 mb-4">
          agent://
          <span className="text-indigo-600">acme.com</span>/
          <span className="text-emerald-600">finance</span>/
          <span className="text-violet-600">billing</span>/
          <span className="text-amber-600">agent_7x2v9q</span>
        </code>
        <div className="grid grid-cols-1 gap-2 text-xs sm:grid-cols-4">
          <div className="flex items-start gap-2">
            <span className="mt-0.5 h-2.5 w-2.5 shrink-0 rounded-full bg-indigo-400" />
            <div>
              <p className="font-semibold text-gray-700">Trust Root</p>
              <p className="text-gray-500">The domain (or <code className="font-mono">nap</code>) verified at registration.</p>
            </div>
          </div>
          <div className="flex items-start gap-2">
            <span className="mt-0.5 h-2.5 w-2.5 shrink-0 rounded-full bg-emerald-400" />
            <div>
              <p className="font-semibold text-gray-700">Category</p>
              <p className="text-gray-500">Top-level capability (e.g. finance).</p>
            </div>
          </div>
          <div className="flex items-start gap-2">
            <span className="mt-0.5 h-2.5 w-2.5 shrink-0 rounded-full bg-violet-400" />
            <div>
              <p className="font-semibold text-gray-700">Primary Skill</p>
              <p className="text-gray-500">Specific skill (e.g. billing). Omitted for top-level-only agents.</p>
            </div>
          </div>
          <div className="flex items-start gap-2">
            <span className="mt-0.5 h-2.5 w-2.5 shrink-0 rounded-full bg-amber-400" />
            <div>
              <p className="font-semibold text-gray-700">Agent ID</p>
              <p className="text-gray-500">Unique ID assigned at registration.</p>
            </div>
          </div>
        </div>
      </div>

      <form onSubmit={handleResolve} className="space-y-5">

        {/* Owner Domain */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Owner Domain <span className="text-red-500">*</span>
          </label>
          <input
            type="text"
            value={trustRoot}
            onChange={(e) => setTrustRoot(e.target.value)}
            placeholder="acme.com"
            required
            className="w-full rounded-lg border border-gray-300 px-4 py-3 text-sm focus:border-nexus-500 focus:outline-none"
          />
          <p className="mt-1 text-xs text-gray-400">The domain the agent was registered under.</p>
        </div>

        {/* Capability Node */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Capability Node <span className="text-red-500">*</span>
          </label>
          <select
            value={capabilityMode === "custom" ? "custom" : capabilityNode}
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
              value={capabilityNode}
              onChange={handleCustomCapability}
              placeholder="e.g. compliance-review"
              required
              className="mt-2 w-full rounded-lg border border-gray-300 px-4 py-3 text-sm focus:border-nexus-500 focus:outline-none"
            />
          )}
          <p className="mt-1 text-xs text-gray-400">The category the agent was registered under.</p>
        </div>

        {/* Agent ID */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Agent ID <span className="text-red-500">*</span>
          </label>
          <input
            type="text"
            value={agentId}
            onChange={(e) => setAgentId(e.target.value)}
            placeholder="agent_7x2v9q"
            required
            className="w-full rounded-lg border border-gray-300 font-mono px-4 py-3 text-sm focus:border-nexus-500 focus:outline-none"
          />
          <p className="mt-1 text-xs text-gray-400">The short ID returned when the agent was registered.</p>
        </div>

        {/* Live URI preview */}
        {previewUri && (
          <div className="rounded-lg bg-gray-50 border border-gray-200 px-4 py-3">
            <p className="text-xs text-gray-400 mb-1">URI preview</p>
            <code className="text-sm font-mono text-nexus-500">{previewUri}</code>
          </div>
        )}

        <button
          type="submit"
          disabled={loading}
          className="w-full rounded-lg bg-nexus-500 px-6 py-3 text-white font-semibold hover:bg-indigo-600 disabled:opacity-50"
        >
          {loading ? "Resolving..." : "Resolve"}
        </button>
      </form>

      {error && (
        <div className="mt-6 rounded-lg bg-red-50 p-4 text-red-600">
          <strong>Error:</strong> {error}
        </div>
      )}

      {result && (
        <div className="mt-6 rounded-xl border border-gray-200 bg-white p-6 shadow-sm space-y-4">
          <div>
            <h2 className="font-semibold text-lg">Resolution Result</h2>
            <p className="mt-1 text-sm text-gray-500">
              The registry found a matching agent. Use the endpoint below to
              send requests directly to it.
            </p>
          </div>
          <div className="divide-y divide-gray-100 text-sm">
            <div className="py-3">
              <p className="text-xs font-medium uppercase tracking-wide text-gray-400 mb-0.5">
                Logical URI
              </p>
              <code className="font-mono text-nexus-500">{result.uri}</code>
              <p className="mt-1 text-xs text-gray-400">
                The stable address — use this in your code, not the endpoint.
              </p>
            </div>
            <div className="py-3">
              <p className="text-xs font-medium uppercase tracking-wide text-gray-400 mb-0.5">
                Transport Endpoint
              </p>
              <a
                href={result.endpoint}
                className="font-mono text-nexus-500 hover:underline break-all"
                target="_blank"
                rel="noreferrer"
              >
                {result.endpoint}
              </a>
              <p className="mt-1 text-xs text-gray-400">
                The live HTTPS URL your HTTP client should POST to.
              </p>
            </div>
            <div className="py-3">
              <p className="text-xs font-medium uppercase tracking-wide text-gray-400 mb-0.5">
                Status
              </p>
              <span
                className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-semibold ${
                  result.status === "active"
                    ? "bg-green-100 text-green-700"
                    : result.status === "pending"
                    ? "bg-yellow-100 text-yellow-700"
                    : "bg-red-100 text-red-700"
                }`}
              >
                {result.status}
              </span>
              <p className="mt-1 text-xs text-gray-400">
                Only <strong>active</strong> agents can receive calls. Pending
                agents must complete DNS verification first.
              </p>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
