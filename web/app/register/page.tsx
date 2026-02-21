"use client";

import { useState } from "react";

interface FormData {
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

export default function RegisterPage() {
  const [form, setForm] = useState<FormData>({
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
      const base =
        process.env.NEXT_PUBLIC_REGISTRY_URL ?? "http://localhost:8080";
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

  const otherFields: { name: keyof FormData; label: string; placeholder: string; hint: string; required?: boolean }[] = [
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
    <div className="max-w-2xl">
      <h1 className="mb-2 text-3xl font-bold">Register an Agent</h1>
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

        {/* Other fields */}
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
    </div>
  );
}
