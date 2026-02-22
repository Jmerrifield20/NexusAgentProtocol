"use client";

import { useEffect, useState, Suspense } from "react";
import { useSearchParams } from "next/navigation";

function VerifyEmailContent() {
  const searchParams = useSearchParams();
  const [status, setStatus] = useState<"loading" | "success" | "error">("loading");
  const [message, setMessage] = useState("");

  useEffect(() => {
    const token = searchParams.get("token");
    if (!token) {
      setStatus("error");
      setMessage("No verification token found in the URL.");
      return;
    }

    const base = process.env.NEXT_PUBLIC_REGISTRY_URL ?? "http://localhost:8080";
    fetch(`${base}/api/v1/auth/verify-email`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ token }),
    })
      .then(async (resp) => {
        const body = await resp.json().catch(() => ({}));
        if (resp.ok) {
          setStatus("success");
        } else {
          setStatus("error");
          setMessage(body.error ?? `Verification failed (HTTP ${resp.status})`);
        }
      })
      .catch((e: unknown) => {
        setStatus("error");
        setMessage(e instanceof Error ? e.message : String(e));
      });
  }, [searchParams]);

  if (status === "loading") {
    return (
      <div className="max-w-md mx-auto mt-16 text-center">
        <div className="text-gray-500 text-lg">Verifying your email...</div>
        <div className="mt-4 animate-pulse text-nexus-500">Please wait</div>
      </div>
    );
  }

  if (status === "success") {
    return (
      <div className="max-w-md mx-auto mt-16">
        <div className="rounded-xl border border-green-200 bg-green-50 p-8 text-center">
          <h2 className="text-xl font-semibold text-green-800 mb-2">Email verified!</h2>
          <p className="text-green-700 mb-4">
            You can now register agents on the Nexus Agent Protocol.
          </p>
          <a
            href="/register"
            className="inline-block rounded-lg bg-nexus-500 px-6 py-2.5 text-sm text-white font-semibold hover:bg-indigo-600"
          >
            Register an Agent
          </a>
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-md mx-auto mt-16">
      <div className="rounded-xl border border-red-200 bg-red-50 p-8 text-center">
        <h2 className="text-xl font-semibold text-red-800 mb-2">Verification failed</h2>
        <p className="text-red-700 mb-4">{message}</p>
        <a
          href="/signup"
          className="inline-block text-sm text-nexus-500 hover:underline"
        >
          Back to Sign Up
        </a>
      </div>
    </div>
  );
}

export default function VerifyEmailPage() {
  return (
    <Suspense fallback={
      <div className="max-w-md mx-auto mt-16 text-center text-gray-500">
        Loading...
      </div>
    }>
      <VerifyEmailContent />
    </Suspense>
  );
}
