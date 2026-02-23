import type { Metadata } from "next";
import "./globals.css";
import NavBar from "./components/NavBar";

export const metadata: Metadata = {
  title: "Nexus Agent Protocol — Developer Portal",
  description:
    "Register, resolve, and manage agents on the Nexus Agent Protocol registry.",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body className="bg-gray-50 text-gray-900 antialiased">
        <NavBar />
        <main className="mx-auto max-w-7xl px-6 py-10">{children}</main>
        <footer className="mt-20 border-t border-gray-200 bg-white">
          <div className="mx-auto max-w-7xl px-6 py-12">
            <div className="flex flex-col items-center gap-8 md:flex-row md:justify-between">
              {/* Brand */}
              <div>
                <p className="text-sm font-semibold text-gray-900">Nexus Agent Protocol</p>
                <p className="mt-1 text-xs text-gray-400">Open protocol for agent identity &amp; discovery.</p>
                <p className="mt-1 text-xs text-gray-400">Released under the Apache 2.0 licence.</p>
              </div>

              {/* Links */}
              <nav className="flex flex-wrap justify-center gap-x-8 gap-y-3 text-sm text-gray-500">
                <a href="/docs" className="hover:text-indigo-600">Docs</a>
                <a
                  href="https://github.com/nexus-protocol/nexus"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="hover:text-indigo-600"
                >
                  GitHub
                </a>
                <a href="mailto:jack@simkura.com" className="hover:text-indigo-600">
                  jack@simkura.com
                </a>
                <a href="/terms" className="hover:text-indigo-600">Terms of Service</a>
                <a href="/privacy" className="hover:text-indigo-600">Privacy Policy</a>
              </nav>
            </div>
            <p className="mt-8 text-center text-xs text-gray-400">
              © {new Date().getFullYear()} Nexus Agent Protocol. All rights reserved.
            </p>
          </div>
        </footer>
      </body>
    </html>
  );
}
