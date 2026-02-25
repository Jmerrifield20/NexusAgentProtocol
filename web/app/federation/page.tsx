export default function FederationPage() {
  return (
    <div className="space-y-16">
      {/* Hero */}
      <section className="py-16 text-center">
        <div className="mx-auto max-w-3xl">
          <span className="inline-block rounded-full bg-nexus-50 px-4 py-1.5 text-sm font-medium text-nexus-500 mb-6">
            NAP Federation
          </span>
          <h1 className="text-5xl font-extrabold tracking-tight text-nexus-900">
            Run Your Own Registry
          </h1>
          <p className="mx-auto mt-6 max-w-2xl text-xl text-gray-500 leading-relaxed">
            Join the NAP federation to issue verified identities under your own domain —
            for your organisation, your platform, or your government.
          </p>
          <div className="mt-10">
            <a
              href="mailto:jack@simkura.com?subject=NAP Federation Request"
              className="rounded-lg bg-nexus-500 px-8 py-3.5 text-white font-semibold shadow hover:bg-indigo-600 inline-block"
            >
              Request to Join →
            </a>
          </div>
        </div>
      </section>

      {/* What you get */}
      <section>
        <h2 className="mb-8 text-2xl font-bold text-gray-900 text-center">
          What you get as a federated registry
        </h2>
        <div className="grid gap-6 md:grid-cols-3">
          <div className="rounded-xl border border-gray-200 bg-white p-6 shadow-sm">
            <div className="mb-4 inline-flex h-10 w-10 items-center justify-center rounded-full bg-nexus-50">
              <svg className="h-5 w-5 text-nexus-500" fill="none" viewBox="0 0 24 24" strokeWidth={1.8} stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75L11.25 15 15 9.75m-3-7.036A11.959 11.959 0 013.598 6 11.99 11.99 0 003 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285z" />
              </svg>
            </div>
            <h3 className="mb-2 font-semibold text-gray-900">Your own intermediate CA</h3>
            <p className="text-sm text-gray-500 leading-relaxed">
              NAP issues you an X.509 intermediate certificate authority. Agents you register
              carry certificates signed by your CA, chaining up to the NAP root.
            </p>
          </div>

          <div className="rounded-xl border border-gray-200 bg-white p-6 shadow-sm">
            <div className="mb-4 inline-flex h-10 w-10 items-center justify-center rounded-full bg-nexus-50">
              <svg className="h-5 w-5 text-nexus-500" fill="none" viewBox="0 0 24 24" strokeWidth={1.8} stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 21a9.004 9.004 0 008.716-6.747M12 21a9.004 9.004 0 01-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9S14.485 3 12 3m0 18c-2.485 0-4.5-4.03-4.5-9S9.515 3 12 3m0 0a8.997 8.997 0 017.843 4.582M12 3a8.997 8.997 0 00-7.843 4.582m15.686 0A11.953 11.953 0 0112 10.5c-2.998 0-5.74-1.1-7.843-2.918m15.686 0A8.959 8.959 0 0121 12c0 .778-.099 1.533-.284 2.253m0 0A17.919 17.919 0 0112 16.5c-3.162 0-6.133-.815-8.716-2.247m0 0A9.015 9.015 0 013 12c0-1.605.42-3.113 1.157-4.418" />
              </svg>
            </div>
            <h3 className="mb-2 font-semibold text-gray-900">Domain-verified agent URIs</h3>
            <p className="text-sm text-gray-500 leading-relaxed">
              Agents registered through your registry get URIs rooted at your domain —
              <code className="mx-1 rounded bg-gray-100 px-1.5 py-0.5 text-xs font-mono text-nexus-500">agent://acme.com/...</code>
              — verifiable by anyone on the internet.
            </p>
          </div>

          <div className="rounded-xl border border-gray-200 bg-white p-6 shadow-sm">
            <div className="mb-4 inline-flex h-10 w-10 items-center justify-center rounded-full bg-nexus-50">
              <svg className="h-5 w-5 text-nexus-500" fill="none" viewBox="0 0 24 24" strokeWidth={1.8} stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" d="M7.5 21L3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5" />
              </svg>
            </div>
            <h3 className="mb-2 font-semibold text-gray-900">Cross-registry resolution</h3>
            <p className="text-sm text-gray-500 leading-relaxed">
              Any agent on the NAP network can resolve and authenticate agents on your
              registry automatically, and vice versa — no manual peering required.
            </p>
          </div>

          <div className="rounded-xl border border-gray-200 bg-white p-6 shadow-sm">
            <div className="mb-4 inline-flex h-10 w-10 items-center justify-center rounded-full bg-nexus-50">
              <svg className="h-5 w-5 text-nexus-500" fill="none" viewBox="0 0 24 24" strokeWidth={1.8} stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" d="M9 17.25v1.007a3 3 0 01-.879 2.122L7.5 21h9l-.621-.621A3 3 0 0115 18.257V17.25m6-12V15a2.25 2.25 0 01-2.25 2.25H5.25A2.25 2.25 0 013 15V5.25m18 0A2.25 2.25 0 0018.75 3H5.25A2.25 2.25 0 003 5.25m18 0H3" />
              </svg>
            </div>
            <h3 className="mb-2 font-semibold text-gray-900">Your own infrastructure</h3>
            <p className="text-sm text-gray-500 leading-relaxed">
              Run the open-source NAP registry on your own servers. Full control over your
              data, your policies, and your agent lifecycle management.
            </p>
          </div>

          <div className="rounded-xl border border-gray-200 bg-white p-6 shadow-sm">
            <div className="mb-4 inline-flex h-10 w-10 items-center justify-center rounded-full bg-nexus-50">
              <svg className="h-5 w-5 text-nexus-500" fill="none" viewBox="0 0 24 24" strokeWidth={1.8} stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" d="M3.75 13.5l10.5-11.25L12 10.5h8.25L9.75 21.75 12 13.5H3.75z" />
              </svg>
            </div>
            <h3 className="mb-2 font-semibold text-gray-900">mTLS &amp; JWT out of the box</h3>
            <p className="text-sm text-gray-500 leading-relaxed">
              Agent-to-agent authentication via mutual TLS and scoped RS256 Task Tokens
              works across registries automatically — no extra integration work.
            </p>
          </div>

          <div className="rounded-xl border border-gray-200 bg-white p-6 shadow-sm">
            <div className="mb-4 inline-flex h-10 w-10 items-center justify-center rounded-full bg-nexus-50">
              <svg className="h-5 w-5 text-nexus-500" fill="none" viewBox="0 0 24 24" strokeWidth={1.8} stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" d="M18 18.72a9.094 9.094 0 003.741-.479 3 3 0 00-4.682-2.72m.94 3.198l.001.031c0 .225-.012.447-.037.666A11.944 11.944 0 0112 21c-2.17 0-4.207-.576-5.963-1.584A6.062 6.062 0 016 18.719m12 0a5.971 5.971 0 00-.941-3.197m0 0A5.995 5.995 0 0012 12.75a5.995 5.995 0 00-5.058 2.772m0 0a3 3 0 00-4.681 2.72 8.986 8.986 0 003.74.477m.94-3.197a5.971 5.971 0 00-.94 3.197M15 6.75a3 3 0 11-6 0 3 3 0 016 0zm6 3a2.25 2.25 0 11-4.5 0 2.25 2.25 0 014.5 0zm-13.5 0a2.25 2.25 0 11-4.5 0 2.25 2.25 0 014.5 0z" />
              </svg>
            </div>
            <h3 className="mb-2 font-semibold text-gray-900">Built for organisations</h3>
            <p className="text-sm text-gray-500 leading-relaxed">
              Suitable for enterprises, platforms, government agencies, and research
              institutions — anyone who needs to operate agent identity at scale.
            </p>
          </div>
        </div>
      </section>

      {/* How it works */}
      <section className="rounded-xl border border-gray-200 bg-white p-10 shadow-sm">
        <h2 className="mb-8 text-2xl font-bold text-gray-900">How federation works</h2>
        <div className="grid gap-8 md:grid-cols-4">
          {[
            {
              step: "1",
              title: "Get in touch",
              body: "Email us with your domain and a brief description of your use case. We'll confirm your organisation is a good fit.",
            },
            {
              step: "2",
              title: "Domain verification",
              body: "You'll verify ownership of your domain via DNS. This anchors your registry's trust root and prevents impersonation.",
            },
            {
              step: "3",
              title: "Intermediate CA issued",
              body: "NAP issues you an intermediate certificate authority. You install it on your registry server — private key never leaves your hands.",
            },
            {
              step: "4",
              title: "Start registering agents",
              body: "Your federated registry is live. Agents you issue are instantly resolvable and verifiable across the entire NAP network.",
            },
          ].map(({ step, title, body }) => (
            <div key={step}>
              <div className="mb-3 inline-flex h-8 w-8 items-center justify-center rounded-full bg-nexus-500 text-sm font-bold text-white">
                {step}
              </div>
              <h3 className="mb-1 font-semibold text-gray-900">{title}</h3>
              <p className="text-sm text-gray-500 leading-relaxed">{body}</p>
            </div>
          ))}
        </div>
      </section>

      {/* Requirements */}
      <section>
        <h2 className="mb-6 text-2xl font-bold text-gray-900">What you need</h2>
        <div className="grid gap-4 md:grid-cols-2">
          {[
            { label: "A domain you control", detail: "You must be able to add DNS TXT records to verify ownership." },
            { label: "A server to run the registry", detail: "The NAP registry is open source. Runs on any Linux VM — a small cloud instance is sufficient to start." },
            { label: "HTTPS endpoint", detail: "Your registry must be reachable over HTTPS with a valid TLS certificate." },
            { label: "A use case", detail: "Tell us who will be registering agents and why. Federation is designed for organisations, not individual use." },
          ].map(({ label, detail }) => (
            <div key={label} className="flex gap-4 rounded-xl border border-gray-200 bg-white p-5 shadow-sm">
              <svg className="mt-0.5 h-5 w-5 shrink-0 text-nexus-500" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" />
              </svg>
              <div>
                <p className="font-semibold text-gray-900 text-sm">{label}</p>
                <p className="mt-0.5 text-sm text-gray-500">{detail}</p>
              </div>
            </div>
          ))}
        </div>
      </section>

      {/* CTA */}
      <section className="rounded-xl bg-nexus-900 p-12 text-center text-white">
        <h2 className="text-3xl font-bold mb-4">Ready to join the federation?</h2>
        <p className="mx-auto max-w-xl text-indigo-200 mb-8 leading-relaxed">
          Send us a brief email with your domain, your organisation name, and what you're
          building. We'll get back to you within a couple of days.
        </p>
        <a
          href="mailto:jack@simkura.com?subject=NAP Federation Request"
          className="inline-block rounded-lg bg-white px-8 py-3.5 font-semibold text-nexus-900 shadow hover:bg-indigo-50"
        >
          Email jack@simkura.com
        </a>
        <p className="mt-4 text-sm text-indigo-300">
          Or read the{" "}
          <a
            href="https://github.com/jmerrifield20/NexusAgentProtocol/blob/main/docs/self-hosting.md"
            target="_blank"
            rel="noreferrer"
            className="underline hover:text-white"
          >
            self-hosting guide
          </a>{" "}
          to understand the setup before reaching out.
        </p>
      </section>
    </div>
  );
}
