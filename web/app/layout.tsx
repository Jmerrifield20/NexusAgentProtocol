import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Nexus Agentic Protocol — Developer Portal",
  description:
    "Register, resolve, and manage agents on the Nexus Agentic Protocol registry.",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body className="bg-gray-50 text-gray-900 antialiased">
        <nav className="border-b border-gray-200 bg-white px-6 py-4">
          <div className="mx-auto flex max-w-7xl items-center justify-between">
            <a href="/" className="text-xl font-bold text-nexus-500">
              Nexus <span className="font-light text-gray-500">Registry</span>
            </a>
            <div className="flex items-center gap-6 text-sm">
              <a href="/agents" className="hover:text-nexus-500">
                Browse Agents
              </a>
              <a href="/register" className="hover:text-nexus-500">
                Register
              </a>
              <a href="/resolve" className="hover:text-nexus-500">
                Resolve URI
              </a>
              <a href="/developers" className="hover:text-nexus-500">
                Developers
              </a>
              <a
                href="https://github.com/nexus-protocol/nexus"
                className="rounded-md bg-nexus-500 px-4 py-2 text-white hover:bg-indigo-600"
                target="_blank"
                rel="noreferrer"
              >
                GitHub
              </a>
            </div>
          </div>
        </nav>
        <main className="mx-auto max-w-7xl px-6 py-10">{children}</main>
        <footer className="mt-20 border-t border-gray-200 py-8 text-center text-sm text-gray-400">
          Nexus Agentic Protocol — Apache 2.0
        </footer>
      </body>
    </html>
  );
}
