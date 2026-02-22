"use client";

import { useState } from "react";

const REGISTRY = process.env.NEXT_PUBLIC_REGISTRY_URL ?? "http://localhost:8080";

export default function ForgotPasswordPage() {
  const [email, setEmail]       = useState("");
  const [submitted, setSubmitted] = useState(false);
  const [loading, setLoading]   = useState(false);
  const [error, setError]       = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setLoading(true);

    try {
      const resp = await fetch(`${REGISTRY}/api/v1/auth/forgot-password`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email }),
      });
      if (!resp.ok) {
        const body = await resp.json();
        setError(body.error ?? `HTTP ${resp.status}`);
        return;
      }
      setSubmitted(true);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="max-w-md mx-auto mt-16">
      <h1 className="mb-2 text-3xl font-bold">Forgot password</h1>

      {submitted ? (
        <div className="mt-6 rounded-xl border border-green-200 bg-green-50 p-6">
          <p className="text-green-800 font-medium mb-1">Check your inbox</p>
          <p className="text-green-700 text-sm">
            If an account exists for <strong>{email}</strong>, we&apos;ve sent a password
            reset link. It expires in 1 hour.
          </p>
          <p className="mt-4 text-xs text-green-600">
            Didn&apos;t receive it? Check your spam folder, or{" "}
            <button
              onClick={() => { setSubmitted(false); setEmail(""); }}
              className="underline hover:text-green-800"
            >
              try again
            </button>.
          </p>
        </div>
      ) : (
        <>
          <p className="mb-8 text-gray-500 text-sm">
            Enter the email address on your account and we&apos;ll send you a reset link.
          </p>

          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Email <span className="text-red-500">*</span>
              </label>
              <input
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                autoFocus
                placeholder="you@example.com"
                className="w-full rounded-lg border border-gray-300 px-4 py-3 text-sm focus:border-nexus-500 focus:outline-none"
              />
            </div>

            {error && (
              <div className="rounded-lg bg-red-50 p-3 text-sm text-red-600">{error}</div>
            )}

            <button
              type="submit"
              disabled={loading || !email}
              className="w-full rounded-lg bg-nexus-500 px-6 py-3 text-white font-semibold hover:bg-indigo-600 disabled:opacity-50 transition-colors"
            >
              {loading ? "Sending…" : "Send reset link"}
            </button>
          </form>
        </>
      )}

      <p className="mt-8 text-center text-sm text-gray-500">
        Remembered it?{" "}
        <a href="/login" className="text-nexus-500 hover:underline font-medium">
          Back to log in
        </a>
      </p>
    </div>
  );
}
