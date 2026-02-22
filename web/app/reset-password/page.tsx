"use client";

import { Suspense, useState } from "react";
import { useSearchParams } from "next/navigation";

const REGISTRY = process.env.NEXT_PUBLIC_REGISTRY_URL ?? "http://localhost:8080";

function ResetPasswordForm() {
  const searchParams = useSearchParams();
  const token = searchParams.get("token") ?? "";

  const [password, setPassword]   = useState("");
  const [confirm, setConfirm]     = useState("");
  const [done, setDone]           = useState(false);
  const [loading, setLoading]     = useState(false);
  const [error, setError]         = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (password !== confirm) {
      setError("Passwords do not match.");
      return;
    }
    if (password.length < 8) {
      setError("Password must be at least 8 characters.");
      return;
    }

    setLoading(true);
    try {
      const resp = await fetch(`${REGISTRY}/api/v1/auth/reset-password`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ token, password }),
      });
      const body = await resp.json().catch(() => ({})) as { error?: string };
      if (!resp.ok) {
        setError(body.error ?? `HTTP ${resp.status}`);
        return;
      }
      setDone(true);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  };

  if (!token) {
    return (
      <div className="max-w-md mx-auto mt-16">
        <h1 className="mb-4 text-3xl font-bold">Reset password</h1>
        <div className="rounded-lg bg-red-50 p-4 text-red-600 text-sm">
          No reset token found. Please use the link from your email.
        </div>
        <p className="mt-6 text-sm text-gray-500">
          <a href="/forgot-password" className="text-nexus-500 hover:underline">
            Request a new reset link
          </a>
        </p>
      </div>
    );
  }

  if (done) {
    return (
      <div className="max-w-md mx-auto mt-16">
        <h1 className="mb-4 text-3xl font-bold">Password updated</h1>
        <div className="rounded-xl border border-green-200 bg-green-50 p-6">
          <p className="text-green-800 font-medium mb-1">All done</p>
          <p className="text-green-700 text-sm">
            Your password has been changed. You can now log in with your new password.
          </p>
        </div>
        <p className="mt-6 text-center text-sm text-gray-500">
          <a href="/login" className="text-nexus-500 hover:underline font-medium">
            Go to log in
          </a>
        </p>
      </div>
    );
  }

  return (
    <div className="max-w-md mx-auto mt-16">
      <h1 className="mb-2 text-3xl font-bold">Reset password</h1>
      <p className="mb-8 text-gray-500 text-sm">Choose a new password for your account.</p>

      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            New password <span className="text-red-500">*</span>
          </label>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            minLength={8}
            autoFocus
            placeholder="At least 8 characters"
            className="w-full rounded-lg border border-gray-300 px-4 py-3 text-sm focus:border-nexus-500 focus:outline-none"
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Confirm password <span className="text-red-500">*</span>
          </label>
          <input
            type="password"
            value={confirm}
            onChange={(e) => setConfirm(e.target.value)}
            required
            placeholder="Repeat your new password"
            className="w-full rounded-lg border border-gray-300 px-4 py-3 text-sm focus:border-nexus-500 focus:outline-none"
          />
        </div>

        {error && (
          <div className="rounded-lg bg-red-50 p-3 text-sm text-red-600">{error}</div>
        )}

        <button
          type="submit"
          disabled={loading || !password || !confirm}
          className="w-full rounded-lg bg-nexus-500 px-6 py-3 text-white font-semibold hover:bg-indigo-600 disabled:opacity-50 transition-colors"
        >
          {loading ? "Updating…" : "Set new password"}
        </button>
      </form>

      <p className="mt-8 text-center text-sm text-gray-500">
        <a href="/forgot-password" className="text-nexus-500 hover:underline">
          Request a new reset link
        </a>
      </p>
    </div>
  );
}

export default function ResetPasswordPage() {
  return (
    <Suspense fallback={<div className="max-w-md mx-auto mt-16 text-gray-400">Loading…</div>}>
      <ResetPasswordForm />
    </Suspense>
  );
}
