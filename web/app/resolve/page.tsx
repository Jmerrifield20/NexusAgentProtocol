"use client";

import { useState } from "react";

interface ResolveResult {
  uri: string;
  endpoint: string;
  status: string;
}

export default function ResolvePage() {
  const [uriInput, setUriInput] = useState("agent://nexus.io/finance/taxes/");
  const [result, setResult] = useState<ResolveResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const handleResolve = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setResult(null);
    setLoading(true);

    try {
      // Parse agent://trust_root/cap_node/agent_id
      const withoutScheme = uriInput.replace(/^agent:\/\//, "");
      const parts = withoutScheme.split("/");
      if (parts.length < 3) throw new Error("Invalid agent:// URI format");

      const trustRoot = parts[0];
      const agentId = parts[parts.length - 1];
      const capNode = parts.slice(1, -1).join("/");

      const base =
        process.env.NEXT_PUBLIC_REGISTRY_URL ?? "http://localhost:8080";
      const resp = await fetch(
        `${base}/api/v1/resolve?trust_root=${encodeURIComponent(trustRoot)}&capability_node=${encodeURIComponent(capNode)}&agent_id=${encodeURIComponent(agentId)}`
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
      <h1 className="mb-8 text-3xl font-bold">Resolve Agent URI</h1>

      <form onSubmit={handleResolve} className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            agent:// URI
          </label>
          <input
            type="text"
            value={uriInput}
            onChange={(e) => setUriInput(e.target.value)}
            placeholder="agent://nexus.io/finance/taxes/agent_7x2v9q"
            className="w-full rounded-lg border border-gray-300 px-4 py-3 font-mono text-sm focus:border-nexus-500 focus:outline-none"
          />
        </div>
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
        <div className="mt-6 rounded-xl border border-gray-200 bg-white p-6 shadow-sm space-y-3">
          <h2 className="font-semibold text-lg">Resolution Result</h2>
          <div className="text-sm space-y-2">
            <div>
              <span className="text-gray-500">URI:</span>{" "}
              <code className="font-mono text-nexus-500">{result.uri}</code>
            </div>
            <div>
              <span className="text-gray-500">Endpoint:</span>{" "}
              <a
                href={result.endpoint}
                className="text-nexus-500 hover:underline"
                target="_blank"
                rel="noreferrer"
              >
                {result.endpoint}
              </a>
            </div>
            <div>
              <span className="text-gray-500">Status:</span>{" "}
              <span className="font-medium">{result.status}</span>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
