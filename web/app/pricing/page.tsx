// Pricing is a server component — no "use client" needed.

const PLANS = [
  {
    name: "Community",
    price: "Free",
    priceSub: "forever",
    highlight: true,
    badge: "Current plan",
    description: "Everything you need to get started — hosted agents, domain registration, and full API access.",
    cta: { label: "Get started", href: "/signup" },
    features: [
      "Unlimited hosted agents",
      "Unlimited domain-verified agents",
      "agent:// URI per agent",
      "Trust ledger entries",
      "mTLS certificate per agent",
      "DNS-01 domain verification",
      "Full REST API access",
      "Community support",
    ],
  },
  {
    name: "Pro",
    price: "Coming soon",
    priceSub: "per domain / month",
    highlight: false,
    badge: "Coming soon",
    description: "For teams running production agents under their own domain with SLA guarantees.",
    cta: { label: "Join waitlist", href: "#waitlist" },
    features: [
      "Everything in Community",
      "Priority DNS verification",
      "Uptime SLA",
      "Agent health monitoring dashboard",
      "Webhook alerts on agent status changes",
      "Team access & roles",
      "Email support",
    ],
  },
  {
    name: "Enterprise",
    price: "Coming soon",
    priceSub: "per domain / month",
    highlight: false,
    badge: "Coming soon",
    description: "Private registry deployment, custom trust roots, and dedicated support.",
    cta: { label: "Contact us", href: "mailto:hello@nexusagentprotocol.com" },
    features: [
      "Everything in Pro",
      "Self-hosted or private cloud registry",
      "Custom trust root (your own CA)",
      "Custom capability taxonomy",
      "SSO / SAML",
      "Dedicated onboarding",
      "SLA with credits",
    ],
  },
];

function CheckIcon() {
  return (
    <svg className="h-4 w-4 text-emerald-500 shrink-0 mt-0.5" viewBox="0 0 16 16" fill="none">
      <circle cx="8" cy="8" r="7.5" stroke="currentColor" strokeOpacity="0.3" />
      <path d="M5 8l2.5 2.5L11 5.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export default function PricingPage() {
  return (
    <div className="max-w-5xl mx-auto space-y-12">

      {/* Header */}
      <div className="text-center space-y-3">
        <h1 className="text-4xl font-bold text-gray-900">Simple, transparent pricing</h1>
        <p className="text-lg text-gray-500 max-w-xl mx-auto">
          Everything is free while we&rsquo;re in beta. Paid plans will be priced per domain —
          not per agent — so your costs stay predictable as you scale.
        </p>
        <div className="inline-flex items-center gap-2 rounded-full bg-emerald-50 border border-emerald-200 px-4 py-1.5">
          <span className="h-2 w-2 rounded-full bg-emerald-400 animate-pulse" />
          <span className="text-sm font-medium text-emerald-700">All features free during beta</span>
        </div>
      </div>

      {/* Plan cards */}
      <div className="grid md:grid-cols-3 gap-6 items-start">
        {PLANS.map((plan) => (
          <div
            key={plan.name}
            className={`rounded-2xl border p-6 flex flex-col gap-5 ${
              plan.highlight
                ? "border-nexus-400 bg-white shadow-md shadow-nexus-100 ring-1 ring-nexus-300"
                : "border-gray-200 bg-white"
            }`}
          >
            {/* Plan header */}
            <div>
              <div className="flex items-center justify-between mb-2">
                <span className="text-sm font-semibold text-gray-900">{plan.name}</span>
                <span
                  className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${
                    plan.highlight
                      ? "bg-nexus-100 text-nexus-700"
                      : "bg-gray-100 text-gray-500"
                  }`}
                >
                  {plan.badge}
                </span>
              </div>

              <div className="mb-3">
                <span className={`font-bold ${plan.price === "Free" ? "text-3xl text-gray-900" : "text-lg text-gray-400"}`}>
                  {plan.price}
                </span>
                {plan.price !== "Free" && (
                  <span className="text-xs text-gray-400 ml-1">{plan.priceSub}</span>
                )}
                {plan.price === "Free" && (
                  <span className="text-sm text-gray-400 ml-1">{plan.priceSub}</span>
                )}
              </div>

              <p className="text-xs text-gray-500 leading-relaxed">{plan.description}</p>
            </div>

            {/* CTA */}
            <a
              href={plan.cta.href}
              className={`block w-full rounded-lg px-4 py-2.5 text-sm font-semibold text-center transition-colors ${
                plan.highlight
                  ? "bg-nexus-500 text-white hover:bg-indigo-600"
                  : "border border-gray-200 text-gray-600 hover:bg-gray-50"
              }`}
            >
              {plan.cta.label}
            </a>

            {/* Features */}
            <ul className="space-y-2">
              {plan.features.map((f) => (
                <li key={f} className="flex items-start gap-2 text-xs text-gray-600">
                  <CheckIcon />
                  {f}
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>

      {/* Pricing model explainer */}
      <div className="rounded-2xl border border-gray-200 bg-white p-8 space-y-6">
        <h2 className="text-xl font-bold text-gray-900">Why per-domain pricing?</h2>
        <div className="grid sm:grid-cols-3 gap-6 text-sm text-gray-600">
          <div className="space-y-2">
            <p className="font-semibold text-gray-800">Predictable costs</p>
            <p>
              One domain, one bill — no matter how many agents you run under it.
              Scale from 1 to 1,000 agents without your registry costs changing.
            </p>
          </div>
          <div className="space-y-2">
            <p className="font-semibold text-gray-800">Matches how NAP works</p>
            <p>
              Trust in NAP is anchored to domains. The domain is the identity unit,
              so it makes sense for pricing to reflect that too.
            </p>
          </div>
          <div className="space-y-2">
            <p className="font-semibold text-gray-800">Fair for teams</p>
            <p>
              A startup with 5 agents and an enterprise with 500 agents sharing one
              domain pay the same rate — the value is in the domain, not the count.
            </p>
          </div>
        </div>
      </div>

      {/* FAQ */}
      <div className="space-y-4">
        <h2 className="text-xl font-bold text-gray-900">Frequently asked questions</h2>
        <div className="divide-y divide-gray-100 rounded-xl border border-gray-200 bg-white">
          {[
            {
              q: "When will paid plans be available?",
              a: "We're still in beta and focused on getting the protocol right. Paid plans will be announced with plenty of notice — everyone on the free plan will have time to decide whether to upgrade.",
            },
            {
              q: "Will my existing agents be affected when paid plans launch?",
              a: "No. Agents registered during the beta period will remain active. We'll grandfather existing registrations where possible.",
            },
            {
              q: "What counts as a domain?",
              a: "Each unique owner domain you register agents under counts as one domain. Subdomains under the same root domain (e.g. api.acme.com and agent.acme.com) count as one.",
            },
            {
              q: "Do hosted agents (under the NAP namespace) have a cost?",
              a: "Community-tier hosted agents — those under the shared NAP namespace rather than your own domain — will remain free. They're designed for experimentation and lightweight use.",
            },
            {
              q: "Can I self-host the registry?",
              a: "Yes. The registry is open source under Apache 2.0. Enterprise plans include support for private deployments with custom trust roots.",
            },
          ].map(({ q, a }) => (
            <div key={q} className="px-5 py-4">
              <p className="text-sm font-semibold text-gray-800 mb-1">{q}</p>
              <p className="text-sm text-gray-500">{a}</p>
            </div>
          ))}
        </div>
      </div>

      {/* Waitlist */}
      <div id="waitlist" className="rounded-2xl border border-indigo-100 bg-indigo-50 p-8 text-center space-y-4">
        <h2 className="text-xl font-bold text-gray-900">Get notified when Pro launches</h2>
        <p className="text-sm text-gray-500 max-w-md mx-auto">
          We&rsquo;ll email you before any pricing changes go live and give early beta users priority access.
        </p>
        <p className="text-sm text-indigo-600 font-medium">
          Create a free account — we&rsquo;ll notify all registered users automatically.
        </p>
        <a
          href="/signup"
          className="inline-block rounded-lg bg-nexus-500 px-6 py-2.5 text-sm font-semibold text-white hover:bg-indigo-600 transition-colors"
        >
          Create free account
        </a>
      </div>

    </div>
  );
}
