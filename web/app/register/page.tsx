"use client";

import { useState } from "react";

interface FormData {
  trust_root: string;
  capability_node: string;
  display_name: string;
  description: string;
  endpoint: string;
  owner_domain: string;
}

export default function RegisterPage() {
  const [form, setForm] = useState<FormData>({
    trust_root: "nexus.io",
    capability_node: "",
    display_name: "",
    description: "",
    endpoint: "",
    owner_domain: "",
  });
  const [result, setResult] = useState<unknown | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const handleChange = (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    setForm((prev) => ({ ...prev, [e.target.name]: e.target.value }));
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

  const fields: { name: keyof FormData; label: string; placeholder: string; required?: boolean }[] = [
    { name: "trust_root", label: "Trust Root", placeholder: "nexus.io", required: true },
    { name: "capability_node", label: "Capability Node", placeholder: "finance/taxes", required: true },
    { name: "display_name", label: "Display Name", placeholder: "My Tax Agent", required: true },
    { name: "description", label: "Description", placeholder: "Handles tax filing queries" },
    { name: "endpoint", label: "Endpoint URL", placeholder: "https://my-agent.example.com", required: true },
    { name: "owner_domain", label: "Owner Domain", placeholder: "example.com", required: true },
  ];

  return (
    <div className="max-w-2xl">
      <h1 className="mb-2 text-3xl font-bold">Register an Agent</h1>
      <p className="mb-8 text-gray-500">
        After submission, your agent will be in <strong>pending</strong> status
        until you complete the DNS-01 domain ownership challenge.
      </p>

      <form onSubmit={handleSubmit} className="space-y-5">
        {fields.map((f) => (
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
