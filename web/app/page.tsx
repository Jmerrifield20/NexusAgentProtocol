export default function HomePage() {
  return (
    <div className="space-y-16">
      {/* Hero */}
      <section className="py-20 text-center">
        <h1 className="text-5xl font-extrabold tracking-tight text-nexus-900">
          Nexus Agentic Protocol
        </h1>
        <p className="mx-auto mt-6 max-w-2xl text-xl text-gray-500">
          A decentralized, open-source authority for registering and resolving
          agents on the internet via the{" "}
          <code className="rounded bg-gray-100 px-2 py-1 text-sm font-mono text-nexus-500">
            agent://
          </code>{" "}
          URI scheme.
        </p>
        <div className="mt-10 flex justify-center gap-4">
          <a
            href="/register"
            className="rounded-lg bg-nexus-500 px-6 py-3 text-white font-semibold shadow hover:bg-indigo-600"
          >
            Register an Agent
          </a>
          <a
            href="/resolve"
            className="rounded-lg border border-gray-300 px-6 py-3 font-semibold text-gray-700 hover:bg-gray-50"
          >
            Resolve a URI
          </a>
        </div>
      </section>

      {/* Feature cards */}
      <section className="grid gap-8 md:grid-cols-3">
        {[
          {
            title: "Identity Minting",
            desc: "Verify domain ownership via DNS-01 challenge and receive an X.509 Agent Identity Certificate.",
            icon: "🔐",
          },
          {
            title: "URI Resolution",
            desc: "Translate agent:// addresses into live HTTPS, gRPC, or WebSocket endpoints in milliseconds.",
            icon: "🔍",
          },
          {
            title: "Trust Ledger",
            desc: "Every registration event is appended to a Merkle-chain audit log for cryptographic non-repudiation.",
            icon: "📜",
          },
        ].map((f) => (
          <div
            key={f.title}
            className="rounded-xl border border-gray-200 bg-white p-8 shadow-sm"
          >
            <div className="mb-4 text-4xl">{f.icon}</div>
            <h3 className="mb-2 text-lg font-semibold">{f.title}</h3>
            <p className="text-gray-500 text-sm leading-relaxed">{f.desc}</p>
          </div>
        ))}
      </section>

      {/* Quick-start */}
      <section className="rounded-xl bg-nexus-900 p-10 text-white">
        <h2 className="mb-6 text-2xl font-bold">Quick Start</h2>
        <pre className="overflow-x-auto rounded-lg bg-black/30 p-6 text-sm font-mono leading-relaxed">
          {`# Install
go get github.com/nexus-protocol/nexus/pkg/client

# Resolve in Go
import "github.com/nexus-protocol/nexus/pkg/client"

c := client.New("https://registry.nexus.io")
result, err := c.Resolve(ctx, "agent://nexus.io/finance/taxes/agent_7x2v9q")
fmt.Println(result.Endpoint)`}
        </pre>
      </section>
    </div>
  );
}
