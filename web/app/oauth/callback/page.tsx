"use client";

import { useEffect, useState } from "react";
import { setToken } from "../../../lib/auth";

export default function OAuthCallbackPage() {
  const [status, setStatus] = useState<"loading" | "success" | "error">("loading");
  const [message, setMessage] = useState("");

  useEffect(() => {
    // Token is delivered in the URL fragment (e.g. #token=xxx) so it never
    // leaves the client. window.location.hash is not sent to the server.
    const hash = window.location.hash.slice(1); // strip leading '#'
    const params = new URLSearchParams(hash);
    const token = params.get("token");
    const errorMsg = params.get("error");

    if (errorMsg) {
      setStatus("error");
      setMessage(errorMsg);
      return;
    }

    if (!token) {
      setStatus("error");
      setMessage("No token received from OAuth provider.");
      return;
    }

    setToken(token);
    setStatus("success");

    // Brief pause so the user sees the success message, then redirect.
    setTimeout(() => {
      window.location.href = "/account";
    }, 1200);
  }, []);

  if (status === "loading") {
    return (
      <div className="max-w-md mx-auto mt-24 text-center">
        <div className="text-gray-500 text-lg">Completing sign-in…</div>
        <div className="mt-4 animate-pulse text-nexus-500">Please wait</div>
      </div>
    );
  }

  if (status === "success") {
    return (
      <div className="max-w-md mx-auto mt-24 text-center">
        <div className="rounded-xl border border-green-200 bg-green-50 p-8">
          <h2 className="text-xl font-semibold text-green-800 mb-2">Signed in!</h2>
          <p className="text-green-700 text-sm">Redirecting you to your account…</p>
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-md mx-auto mt-24 text-center">
      <div className="rounded-xl border border-red-200 bg-red-50 p-8">
        <h2 className="text-xl font-semibold text-red-800 mb-2">Sign-in failed</h2>
        <p className="text-red-700 text-sm mb-4">{message}</p>
        <a href="/login" className="text-nexus-500 hover:underline text-sm">
          Back to Log In
        </a>
      </div>
    </div>
  );
}
