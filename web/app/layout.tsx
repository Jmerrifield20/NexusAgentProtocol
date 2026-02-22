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
        <footer className="mt-20 border-t border-gray-200 py-8 text-center text-sm text-gray-400">
          Nexus Agent Protocol — Apache 2.0
        </footer>
      </body>
    </html>
  );
}
