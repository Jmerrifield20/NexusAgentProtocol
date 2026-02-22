export default function HomePage() {
  return (
    <div className="space-y-16">

      {/* Hero */}
      <section className="py-20 text-center">
        <h1 className="text-5xl font-extrabold tracking-tight text-nexus-900">
          Nexus Agent Protocol
        </h1>
        <p className="mx-auto mt-6 max-w-2xl text-xl text-gray-500">
          A public registry that gives AI agents a permanent, verifiable address
          on the internet — so any system can find and talk to them.
        </p>
        <div className="mt-10 flex justify-center gap-4">
          <a
            href="/register"
            className="rounded-lg bg-nexus-500 px-6 py-3 text-white font-semibold shadow hover:bg-indigo-600"
          >
            Register an Agent
          </a>
          <a
            href="/agents"
            className="rounded-lg border border-gray-300 px-6 py-3 font-semibold text-gray-700 hover:bg-gray-50"
          >
            Find Agents
          </a>
        </div>
      </section>

      {/* Plain-English explainer */}
      <section className="rounded-xl border border-gray-200 bg-white p-10 shadow-sm space-y-6">
        <h2 className="text-2xl font-bold text-gray-900">What is this?</h2>
        <p className="text-gray-600 leading-relaxed">
          Think of Nexus like DNS — but for AI agents instead of websites.
        </p>
        <p className="text-gray-600 leading-relaxed">
          When you build an AI agent (a program that can autonomously perform tasks, answer questions,
          or take actions on your behalf), you deploy it at some URL like{" "}
          <code className="rounded bg-gray-100 px-1.5 py-0.5 text-sm font-mono text-nexus-500">
            https://api.acme.com/agents/tax-bot
          </code>
          . That URL can change. It can move servers, get load balanced, or be replaced entirely.
          Any system that was hardcoded to that URL breaks.
        </p>
        <p className="text-gray-600 leading-relaxed">
          Nexus solves this by giving your agent a <strong>stable logical address</strong> — an{" "}
          <code className="rounded bg-gray-100 px-1.5 py-0.5 text-sm font-mono text-nexus-500">
            agent://
          </code>{" "}
          URI — that never changes even if the underlying server does. Other systems
          resolve that address through this registry to find the current endpoint, just like a
          browser resolves a domain name to an IP address.
        </p>
        <p className="text-gray-600 leading-relaxed">
          Before an agent gets an address, Nexus verifies your identity. Domain-verified agents prove
          they own the domain via a DNS challenge — the same mechanism Let&apos;s Encrypt uses. Free-hosted
          agents are verified by email. Either way, every{" "}
          <code className="rounded bg-gray-100 px-1.5 py-0.5 text-sm font-mono text-nexus-500">
            agent://
          </code>{" "}
          URI is backed by a confirmed identity — not just a string anyone could make up.
        </p>
      </section>

      {/* Diagrams */}
      <section>
        <h2 className="mb-2 text-2xl font-bold text-gray-900">How two agents find each other</h2>
        <p className="mb-8 text-gray-500 text-sm">
          The Nexus Registry acts as the neutral lookup authority. Agents register once, then any other agent can discover and call them by URI — without hardcoding URLs.
        </p>

        {/* Diagram 1 — Registration */}
        <div className="mb-4 rounded-2xl border border-gray-100 bg-white p-6 shadow-sm">
          <p className="mb-5 text-xs font-semibold uppercase tracking-widest text-gray-400">
            Phase 1 — Registration &amp; Verification
          </p>
          <svg viewBox="0 0 780 195" className="w-full" aria-label="Registration diagram">
            <defs>
              <filter id="f1s" x="-20%" y="-20%" width="140%" height="140%">
                <feDropShadow dx="0" dy="2" stdDeviation="4" floodColor="#0f172a" floodOpacity="0.07"/>
              </filter>
              <filter id="f1g" x="-30%" y="-30%" width="160%" height="160%">
                <feDropShadow dx="0" dy="4" stdDeviation="12" floodColor="#4f46e5" floodOpacity="0.22"/>
              </filter>
              <marker id="m1i" markerWidth="9" markerHeight="9" refX="7" refY="4" orient="auto">
                <path d="M0,0 L0,8 L9,4 z" fill="#4f46e5"/>
              </marker>
              <marker id="m1g" markerWidth="9" markerHeight="9" refX="7" refY="4" orient="auto">
                <path d="M0,0 L0,8 L9,4 z" fill="#16a34a"/>
              </marker>
            </defs>

            {/* Background */}
            <rect width="780" height="195" rx="14" fill="#f8fafc"/>

            {/* Agent A */}
            <rect x="28" y="72" width="158" height="72" rx="10" fill="white" stroke="#e0e7ff" strokeWidth="1.5" filter="url(#f1s)"/>
            <text x="107" y="104" textAnchor="middle" fontFamily="system-ui, sans-serif" fontSize="13" fontWeight="600" fill="#1e1b4b">Agent A</text>
            <text x="107" y="122" textAnchor="middle" fontFamily="system-ui, sans-serif" fontSize="10" fill="#94a3b8">agent://acme.com/finance/…</text>

            {/* Nexus Registry */}
            <rect x="311" y="52" width="158" height="112" rx="10" fill="#1e1b4b" filter="url(#f1g)"/>
            <text x="390" y="100" textAnchor="middle" fontFamily="system-ui, sans-serif" fontSize="13" fontWeight="700" fill="white">Nexus Registry</text>
            <text x="390" y="120" textAnchor="middle" fontFamily="system-ui, sans-serif" fontSize="10" fill="#818cf8">Trust Registry</text>
            <rect x="338" y="133" width="104" height="18" rx="9" fill="#2d2a5e"/>
            <text x="390" y="146" textAnchor="middle" fontFamily="system-ui, sans-serif" fontSize="9" fill="#a5b4fc">DNS-01 · Email · Ledger</text>

            {/* Agent B */}
            <rect x="594" y="72" width="158" height="72" rx="10" fill="white" stroke="#e0e7ff" strokeWidth="1.5" filter="url(#f1s)"/>
            <text x="673" y="104" textAnchor="middle" fontFamily="system-ui, sans-serif" fontSize="13" fontWeight="600" fill="#1e1b4b">Agent B</text>
            <text x="673" y="122" textAnchor="middle" fontFamily="system-ui, sans-serif" fontSize="10" fill="#94a3b8">agent://nexus.io/finance/…</text>

            {/* A → Registry: register */}
            <line x1="188" y1="100" x2="309" y2="100" stroke="#4f46e5" strokeWidth="1.5" strokeDasharray="5,3" markerEnd="url(#m1i)"/>
            <rect x="196" y="84" width="106" height="16" rx="8" fill="white" stroke="#e0e7ff" strokeWidth="1"/>
            <text x="249" y="96" textAnchor="middle" fontFamily="system-ui, sans-serif" fontSize="9.5" fill="#4f46e5">Register + verify</text>

            {/* Registry → A: endorsement */}
            <line x1="309" y1="126" x2="188" y2="126" stroke="#16a34a" strokeWidth="1.5" markerEnd="url(#m1g)"/>
            <rect x="196" y="130" width="106" height="16" rx="8" fill="white" stroke="#dcfce7" strokeWidth="1"/>
            <text x="249" y="142" textAnchor="middle" fontFamily="system-ui, sans-serif" fontSize="9.5" fill="#16a34a">NAP Endorsement</text>

            {/* B → Registry: register */}
            <line x1="592" y1="100" x2="471" y2="100" stroke="#4f46e5" strokeWidth="1.5" strokeDasharray="5,3" markerEnd="url(#m1i)"/>
            <rect x="478" y="84" width="106" height="16" rx="8" fill="white" stroke="#e0e7ff" strokeWidth="1"/>
            <text x="531" y="96" textAnchor="middle" fontFamily="system-ui, sans-serif" fontSize="9.5" fill="#4f46e5">Register + verify</text>

            {/* Registry → B: endorsement */}
            <line x1="471" y1="126" x2="592" y2="126" stroke="#16a34a" strokeWidth="1.5" markerEnd="url(#m1g)"/>
            <rect x="478" y="130" width="106" height="16" rx="8" fill="white" stroke="#dcfce7" strokeWidth="1"/>
            <text x="531" y="142" textAnchor="middle" fontFamily="system-ui, sans-serif" fontSize="9.5" fill="#16a34a">NAP Endorsement</text>
          </svg>
        </div>

        {/* Diagram 2 — Discovery & Direct Call */}
        <div className="rounded-2xl border border-gray-100 bg-white p-6 shadow-sm">
          <p className="mb-5 text-xs font-semibold uppercase tracking-widest text-gray-400">
            Phase 2 — Discovery &amp; Direct Communication
          </p>
          <p className="mb-4 text-xs text-gray-400">
            Note: Agent A does not need to be registered with NAP to look up Agent B. Any HTTP client can resolve an <code className="font-mono">agent://</code> URI — registration is only required to be <em>discoverable</em>, not to discover others.
          </p>
          <svg viewBox="0 0 780 235" className="w-full" aria-label="Discovery and call diagram">
            <defs>
              <filter id="f2s" x="-20%" y="-20%" width="140%" height="140%">
                <feDropShadow dx="0" dy="2" stdDeviation="4" floodColor="#0f172a" floodOpacity="0.07"/>
              </filter>
              <filter id="f2g" x="-30%" y="-30%" width="160%" height="160%">
                <feDropShadow dx="0" dy="4" stdDeviation="12" floodColor="#4f46e5" floodOpacity="0.22"/>
              </filter>
              <marker id="m2i" markerWidth="9" markerHeight="9" refX="7" refY="4" orient="auto">
                <path d="M0,0 L0,8 L9,4 z" fill="#4f46e5"/>
              </marker>
              <marker id="m2a" markerWidth="9" markerHeight="9" refX="7" refY="4" orient="auto">
                <path d="M0,0 L0,8 L9,4 z" fill="#d97706"/>
              </marker>
              <marker id="m2g" markerWidth="9" markerHeight="9" refX="7" refY="4" orient="auto">
                <path d="M0,0 L0,8 L9,4 z" fill="#16a34a"/>
              </marker>
            </defs>

            {/* Background */}
            <rect width="780" height="235" rx="14" fill="#f8fafc"/>

            {/* Agent A */}
            <rect x="28" y="72" width="158" height="72" rx="10" fill="white" stroke="#e0e7ff" strokeWidth="1.5" filter="url(#f2s)"/>
            <text x="107" y="104" textAnchor="middle" fontFamily="system-ui, sans-serif" fontSize="13" fontWeight="600" fill="#1e1b4b">Agent A</text>
            <text x="107" y="122" textAnchor="middle" fontFamily="system-ui, sans-serif" fontSize="10" fill="#94a3b8">agent://acme.com/finance/…</text>

            {/* Nexus Registry */}
            <rect x="311" y="52" width="158" height="112" rx="10" fill="#1e1b4b" filter="url(#f2g)"/>
            <text x="390" y="100" textAnchor="middle" fontFamily="system-ui, sans-serif" fontSize="13" fontWeight="700" fill="white">Nexus Registry</text>
            <text x="390" y="120" textAnchor="middle" fontFamily="system-ui, sans-serif" fontSize="10" fill="#818cf8">Trust Registry</text>
            <rect x="338" y="133" width="104" height="18" rx="9" fill="#2d2a5e"/>
            <text x="390" y="146" textAnchor="middle" fontFamily="system-ui, sans-serif" fontSize="9" fill="#a5b4fc">DNS-01 · Email · Ledger</text>

            {/* Agent B */}
            <rect x="594" y="72" width="158" height="72" rx="10" fill="white" stroke="#e0e7ff" strokeWidth="1.5" filter="url(#f2s)"/>
            <text x="673" y="104" textAnchor="middle" fontFamily="system-ui, sans-serif" fontSize="13" fontWeight="600" fill="#1e1b4b">Agent B</text>
            <text x="673" y="122" textAnchor="middle" fontFamily="system-ui, sans-serif" fontSize="10" fill="#94a3b8">agent://nexus.io/finance/…</text>

            {/* ① A → Registry: resolve */}
            <line x1="188" y1="100" x2="309" y2="100" stroke="#4f46e5" strokeWidth="1.5" markerEnd="url(#m2i)"/>
            <rect x="191" y="84" width="115" height="16" rx="8" fill="white" stroke="#e0e7ff" strokeWidth="1"/>
            <text x="248" y="96" textAnchor="middle" fontFamily="system-ui, sans-serif" fontSize="9.5" fill="#4f46e5">① Resolve URI lookup</text>

            {/* ② Registry → A: endpoint returned */}
            <line x1="309" y1="126" x2="188" y2="126" stroke="#d97706" strokeWidth="1.5" markerEnd="url(#m2a)"/>
            <rect x="191" y="130" width="115" height="16" rx="8" fill="white" stroke="#fef3c7" strokeWidth="1"/>
            <text x="248" y="142" textAnchor="middle" fontFamily="system-ui, sans-serif" fontSize="9.5" fill="#d97706">② Endpoint returned</text>

            {/* ③ A → B: direct call (curved) */}
            <path d="M 107,146 C 107,205 673,205 673,146" fill="none" stroke="#16a34a" strokeWidth="2"/>
            <rect x="313" y="196" width="154" height="18" rx="9" fill="white" stroke="#dcfce7" strokeWidth="1"/>
            <text x="390" y="209" textAnchor="middle" fontFamily="system-ui, sans-serif" fontSize="9.5" fill="#16a34a" fontWeight="500">③ Direct call — registry not involved</text>
          </svg>
        </div>
      </section>

      {/* How it works — step by step */}
      <section>
        <h2 className="mb-6 text-2xl font-bold text-gray-900">How it works</h2>
        <div className="grid gap-4 md:grid-cols-4">
          {[
            {
              step: "1",
              title: "Register",
              desc: "Submit your agent's details — name, capability, and the URL where it runs. Choose free-hosted (email account) or domain-verified (your own domain).",
            },
            {
              step: "2",
              title: "Verify",
              desc: "Free-hosted: verify your email address. Domain-verified: add a DNS TXT record to prove domain ownership — the same mechanism used by Let's Encrypt.",
            },
            {
              step: "3",
              title: "Activate",
              desc: "Once verified, your agent goes live and receives a NAP Endorsement — a registry-signed certificate of trust. Domain-verified agents also receive a full X.509 identity certificate.",
            },
            {
              step: "4",
              title: "Discover",
              desc: "Anyone can search your domain to find your agents, or resolve a full agent:// URI to get the live endpoint.",
            },
          ].map((s) => (
            <div key={s.step} className="rounded-xl border border-gray-200 bg-white p-6 shadow-sm">
              <div className="mb-3 inline-flex h-8 w-8 items-center justify-center rounded-full bg-nexus-500 text-sm font-bold text-white">
                {s.step}
              </div>
              <h3 className="mb-1 font-semibold text-gray-900">{s.title}</h3>
              <p className="text-sm text-gray-500 leading-relaxed">{s.desc}</p>
            </div>
          ))}
        </div>
      </section>

      {/* URI anatomy */}
      <section className="rounded-xl border border-gray-200 bg-gray-50 p-8">
        <h2 className="mb-2 text-2xl font-bold text-gray-900">What does an agent address look like?</h2>
        <p className="mb-6 text-gray-500 text-sm">
          Every registered agent gets a URI with three segments:
        </p>
        <code className="block text-lg font-mono text-gray-800 mb-6">
          agent://
          <span className="text-indigo-600">acme.com</span>/
          <span className="text-emerald-600">finance</span>/
          <span className="text-amber-600">agent_7x2v9q</span>
        </code>
        <div className="grid gap-4 sm:grid-cols-3 text-sm">
          <div className="rounded-lg border border-indigo-100 bg-white p-4">
            <p className="font-semibold text-indigo-600 mb-1">acme.com <span className="text-xs font-normal text-gray-400">(or <code className="font-mono">nap</code>)</span></p>
            <p className="text-gray-500">
              The org — who owns this agent. For <strong>domain-verified</strong> agents this is the
              full verified domain, proven by DNS-01 challenge. Only the owner of <code className="font-mono text-xs">acme.com</code> can
              claim <code className="font-mono text-xs">acme.com</code> — <code className="font-mono text-xs">acme.io</code> gets a different address.
              For <strong>free-hosted</strong> agents this is always <code className="font-mono text-xs">nap</code>
              — a registry-controlled namespace that prevents impersonation.
            </p>
          </div>
          <div className="rounded-lg border border-emerald-100 bg-white p-4">
            <p className="font-semibold text-emerald-600 mb-1">finance</p>
            <p className="text-gray-500">The top-level capability category from the NAP taxonomy. Tells callers what kind of task this agent handles. Sub-categories (e.g. <code className="font-mono text-xs">finance &gt; accounting</code>) are searchable but don&apos;t appear in the URI — keeping addresses stable as capability paths evolve.</p>
          </div>
          <div className="rounded-lg border border-amber-100 bg-white p-4">
            <p className="font-semibold text-amber-600 mb-1">agent_7x2v9q</p>
            <p className="text-gray-500">A unique ID assigned at registration. Never changes — even if the agent moves servers, gets a new domain, or changes capability sub-categories.</p>
          </div>
        </div>
      </section>

      {/* Feature cards */}
      <section>
        <h2 className="mb-6 text-2xl font-bold text-gray-900">What the registry provides</h2>
        <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-4">
          {[
            {
              title: "Verified Identity",
              desc: "Domain-verified agents prove DNS ownership via the same mechanism as Let's Encrypt. Free-hosted agents are email-verified. Trust tier tells callers exactly how much verification has been performed.",
            },
            {
              title: "Stable Addressing",
              desc: "The agent:// URI never changes. Update your server URL in the registry and all callers automatically get the new endpoint — no code changes needed.",
            },
            {
              title: "A2A-Compatible Agent Cards",
              desc: "Activation produces a ready-to-deploy agent card you host on your own domain at /.well-known/agent.json. Any A2A client can read it natively. NAP-aware clients additionally verify the embedded endorsement JWT.",
            },
            {
              title: "Tamper-Proof Audit Log",
              desc: "Every registration and activation is recorded in an append-only hash chain. You can independently verify the full history of any agent's identity.",
            },
          ].map((f) => (
            <div key={f.title} className="rounded-xl border border-gray-200 bg-white p-6 shadow-sm">
              <h3 className="mb-2 font-semibold text-gray-900">{f.title}</h3>
              <p className="text-sm text-gray-500 leading-relaxed">{f.desc}</p>
            </div>
          ))}
        </div>
      </section>

      {/* Quick-start */}
      <section className="rounded-xl bg-nexus-900 p-10 text-white">
        <h2 className="mb-2 text-2xl font-bold">Quick Start</h2>
        <p className="mb-6 text-sm text-white/60">Resolve an agent address in your Go code:</p>
        <pre className="overflow-x-auto rounded-lg bg-black/30 p-6 text-sm font-mono leading-relaxed">
          {`go get github.com/nexus-protocol/nexus/pkg/client

c, _ := client.New("https://registry.nexusagentprotocol.com")
result, err := c.Resolve(ctx, "agent://acme.com/finance/agent_7x2v9q")
fmt.Println(result.Endpoint) // https://api.acme.com/agents/tax-bot`}
        </pre>
      </section>

    </div>
  );
}
