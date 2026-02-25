"use client";

import { useEffect, useState } from "react";
import { clearToken, getUser, isLoggedIn, UserClaims } from "../../lib/auth";

export default function NavBar() {
  const [user, setUser] = useState<UserClaims | null>(null);

  useEffect(() => {
    if (isLoggedIn()) {
      setUser(getUser());
    }
  }, []);

  const handleLogout = () => {
    clearToken();
    setUser(null);
    window.location.href = "/";
  };

  return (
    <nav className="border-b border-gray-200 bg-white px-6 py-4">
      <div className="mx-auto flex max-w-7xl items-center justify-between">
        <a href="/" className="text-xl font-bold text-nexus-500">
          Nexus <span className="font-light text-gray-500">Registry</span>
        </a>
        <div className="flex items-center gap-6 text-sm">
          <a href="/agents" className="hover:text-nexus-500">
            Find Agents
          </a>
          <a href="/register" className="hover:text-nexus-500">
            Register
          </a>
          <a href="/developers" className="hover:text-nexus-500">
            Developers
          </a>
          <a href="/federation" className="hover:text-nexus-500">
            Federation
          </a>
          <a
            href="https://github.com/jmerrifield20/NexusAgentProtocol"
            className="rounded-md bg-nexus-500 px-4 py-2 text-white hover:bg-indigo-600"
            target="_blank"
            rel="noreferrer"
          >
            GitHub
          </a>

          {user ? (
            <>
              <a href="/account" className="text-gray-700 font-medium hover:text-nexus-500">{user.username}</a>
              <button
                onClick={handleLogout}
                className="rounded-md border border-gray-300 px-3 py-1.5 text-sm hover:bg-gray-50"
              >
                Logout
              </button>
            </>
          ) : (
            <>
              <a href="/signup" className="hover:text-nexus-500">
                Sign Up
              </a>
              <a
                href="/login"
                className="rounded-md border border-gray-300 px-3 py-1.5 hover:bg-gray-50"
              >
                Log In
              </a>
            </>
          )}
        </div>
      </div>
    </nav>
  );
}
