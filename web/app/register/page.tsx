"use client";

import { useEffect, useState } from "react";
import { getToken, setToken, getUser, isLoggedIn, UserClaims } from "../../lib/auth";

// ---------- Taxonomy suggestions -------------------------------------------
// Suggestions only — not enforced. Any well-formed capability is accepted.

const TAXONOMY: Record<string, Record<string, string[]>> = {
  commerce:       { catalog: ["listing","search","variants","pricing"], orders: ["checkout","fulfillment","returns","tracking"], payments: ["invoicing","billing","subscriptions","refunds"], inventory: ["stock","replenishment","warehousing","forecasting"], procurement: ["sourcing","rfq","vendor-management","contracts"] },
  communication:  { email: ["campaigns","transactional","templates","deliverability"], chat: ["support","notifications","bots","moderation"], calendar: ["scheduling","reminders","availability","booking"], crm: ["contacts","pipelines","forecasting","activity"], social: ["posting","engagement","monitoring","analytics"] },
  data:           { analytics: ["dashboards","reporting","kpi","cohort"], engineering: ["pipelines","etl","warehousing","streaming"], ml: ["training","inference","evaluation","feature-store"], governance: ["quality","lineage","catalog","privacy"], visualization: ["charts","maps","real-time","exports"] },
  education:      { tutoring: ["k12","higher-ed","professional","language"], curriculum: ["design","assessment","multimedia","translation"], administration: ["enrollment","scheduling","advising","reporting"], research: ["pedagogy","outcomes","grants","policy"], skills: ["coding","writing","math","critical-thinking"] },
  finance:        { accounting: ["reconciliation","bookkeeping","auditing","reporting"], tax: ["filing","compliance","planning","research"], payments: ["invoicing","payroll","billing","settlement"], trading: ["equities","crypto","forex","derivatives"], banking: ["lending","deposits","risk","compliance"] },
  healthcare:     { clinical: ["diagnosis","treatment","monitoring","documentation"], admin: ["billing","scheduling","records","coding"], research: ["trials","genomics","epidemiology","analysis"], wellness: ["nutrition","fitness","mental-health","prevention"], pharmacy: ["dispensing","interactions","formulary","adherence"] },
  hr:             { recruiting: ["sourcing","screening","interviewing","onboarding"], performance: ["reviews","feedback","okrs","planning"], benefits: ["administration","compliance","enrollment","wellness"], learning: ["training","certification","development","lms"], workforce: ["planning","analytics","scheduling","offboarding"] },
  infrastructure: { cloud: ["provisioning","cost","scaling","migration"], networking: ["dns","load-balancing","vpn","firewall"], security: ["iam","vulnerability","compliance","incident"], devops: ["ci-cd","deployment","monitoring","alerting"], storage: ["backup","archival","replication","encryption"] },
  legal:          { contracts: ["drafting","review","negotiation","management"], compliance: ["regulatory","privacy","employment","reporting"], litigation: ["research","filing","discovery","strategy"], ip: ["patents","trademarks","licensing","enforcement"], corporate: ["governance","mergers","restructuring","advisory"] },
  logistics:      { shipping: ["quoting","labeling","tracking","returns"], warehousing: ["receiving","picking","packing","inventory"], routing: ["optimization","scheduling","multi-modal","last-mile"], customs: ["classification","compliance","documentation","duties"], fleet: ["tracking","maintenance","dispatch","telematics"] },
  "real-estate":  { listings: ["search","valuation","marketing","syndication"], transactions: ["contracts","closing","title","escrow"], "property-management": ["leasing","maintenance","rent","inspections"], investment: ["analysis","underwriting","portfolio","reporting"], construction: ["permitting","scheduling","budgeting","inspection"] },
  research:       { "data-analysis": ["statistics","visualization","modeling","forecasting"], literature: ["search","summarization","citation","review"], experiments: ["design","simulation","reporting","validation"], science: ["biology","chemistry","physics","materials"], social: ["surveys","ethnography","policy","economics"] },
};

const SUGGESTED_CATEGORIES = Object.keys(TAXONOMY).sort();

// ---------- CapabilityPicker -----------------------------------------------
// Three-level chip picker with "+ custom" free-text input at every level.
// Taxonomy values are suggestions only — users can type anything at any level.

function CapabilityPicker({
  value,
  onChange,
  required,
}: {
  value: string;
  onChange: (v: string) => void;
  required?: boolean;
}) {
  const initParts = value ? value.split(">") : [];
  const [cat,  setCat]  = useState(initParts[0] ?? "");
  const [sub,  setSub]  = useState(initParts[1] ?? "");
  const [item, setItem] = useState(initParts[2] ?? "");

  // Whether the inline custom-text input is open at each level
  const [catOpen,  setCatOpen]  = useState(false);
  const [subOpen,  setSubOpen]  = useState(false);
  const [itemOpen, setItemOpen] = useState(false);

  const emit = (c: string, s: string, i: string) => {
    if (!c) { onChange(""); return; }
    if (!s) { onChange(c); return; }
    if (!i) { onChange(`${c}>${s}`); return; }
    onChange(`${c}>${s}>${i}`);
  };

  const isCatSuggested  = !!TAXONOMY[cat];
  const subSuggestions  = isCatSuggested ? Object.keys(TAXONOMY[cat]).sort() : [];
  const isSubSuggested  = isCatSuggested && !!TAXONOMY[cat]?.[sub];
  const itemSuggestions = isSubSuggested ? TAXONOMY[cat][sub] : [];

  // ── Level handlers ────────────────────────────────────────────────────────

  const selectCat = (c: string) => {
    const next = c === cat ? "" : c;
    setCat(next); setSub(""); setItem("");
    setCatOpen(false); setSubOpen(false); setItemOpen(false);
    emit(next, "", "");
  };

  const selectSub = (s: string) => {
    const next = s === sub ? "" : s;
    setSub(next); setItem("");
    setSubOpen(false); setItemOpen(false);
    emit(cat, next, "");
  };

  const selectItem = (i: string) => {
    const next = i === item ? "" : i;
    setItem(next);
    setItemOpen(false);
    emit(cat, sub, next);
  };

  const clearAll = () => {
    setCat(""); setSub(""); setItem("");
    setCatOpen(false); setSubOpen(false); setItemOpen(false);
    onChange("");
  };

  const display = [cat, sub, item].filter(Boolean).join(" > ");

  // ── Shared inline-input renderer ─────────────────────────────────────────

  function InlineInput({
    placeholder,
    defaultValue,
    onCommit,
    onClose,
  }: {
    placeholder: string;
    defaultValue?: string;
    onCommit: (v: string) => void;
    onClose: () => void;
  }) {
    return (
      <input
        autoFocus
        type="text"
        placeholder={placeholder}
        defaultValue={defaultValue}
        className="rounded-full border border-nexus-400 bg-white px-3 py-1 text-xs focus:outline-none w-44"
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            e.preventDefault();
            const v = (e.target as HTMLInputElement).value.trim();
            if (v) onCommit(v); else onClose();
          }
          if (e.key === "Escape") onClose();
        }}
        onBlur={(e) => {
          const v = e.target.value.trim();
          if (v) onCommit(v); else onClose();
        }}
      />
    );
  }

  // ── Chip renderer ─────────────────────────────────────────────────────────

  function Chip({
    label,
    selected,
    onClick,
  }: {
    label: string;
    selected: boolean;
    onClick: () => void;
  }) {
    return (
      <button
        type="button"
        onClick={onClick}
        className={`rounded-full px-3 py-1 text-xs font-medium capitalize transition-colors border ${
          selected
            ? "bg-nexus-500 text-white border-nexus-500"
            : "border-gray-200 text-gray-600 hover:border-nexus-300 hover:text-nexus-600 bg-white"
        }`}
      >
        {label}
      </button>
    );
  }

  // ── Render ────────────────────────────────────────────────────────────────

  return (
    <div className="space-y-3">
      {/* Hidden input for browser required validation */}
      {required && (
        <input
          type="text"
          required
          readOnly
          value={value}
          tabIndex={-1}
          className="sr-only"
          aria-hidden="true"
        />
      )}

      {/* Level 1 — Category */}
      <div>
        <p className="text-xs font-medium text-gray-500 mb-1.5">Category</p>
        <div className="flex flex-wrap gap-1.5">
          {SUGGESTED_CATEGORIES.map((c) => (
            <Chip key={c} label={c} selected={cat === c && !catOpen} onClick={() => selectCat(c)} />
          ))}

          {/* Custom chip / input */}
          {catOpen ? (
            <InlineInput
              placeholder="e.g. media-production"
              defaultValue={cat && !isCatSuggested ? cat : ""}
              onCommit={(v) => { setCat(v); setCatOpen(false); setSub(""); setItem(""); emit(v, "", ""); }}
              onClose={() => setCatOpen(false)}
            />
          ) : (
            <button
              type="button"
              onClick={() => { setCatOpen(true); setCat(""); setSub(""); setItem(""); onChange(""); }}
              className={`rounded-full px-3 py-1 text-xs font-medium border transition-colors ${
                cat && !isCatSuggested
                  ? "bg-nexus-500 text-white border-nexus-500"
                  : "border-dashed border-gray-300 text-gray-400 hover:border-nexus-300 hover:text-nexus-600"
              }`}
            >
              {cat && !isCatSuggested ? cat : "+ custom"}
            </button>
          )}
        </div>
      </div>

      {/* Level 2 — Subcategory */}
      {cat && (
        <div>
          <p className="text-xs font-medium text-gray-500 mb-1.5">
            Subcategory{" "}
            <span className="font-normal text-gray-400">(optional)</span>
          </p>
          <div className="flex flex-wrap gap-1.5">
            {subSuggestions.map((s) => (
              <Chip key={s} label={s} selected={sub === s && !subOpen} onClick={() => selectSub(s)} />
            ))}

            {subOpen ? (
              <InlineInput
                placeholder="e.g. derivatives-trading"
                defaultValue={sub && !isSubSuggested ? sub : ""}
                onCommit={(v) => { setSub(v); setSubOpen(false); setItem(""); emit(cat, v, ""); }}
                onClose={() => setSubOpen(false)}
              />
            ) : (
              <button
                type="button"
                onClick={() => { setSubOpen(true); setSub(""); setItem(""); emit(cat, "", ""); }}
                className={`rounded-full px-3 py-1 text-xs font-medium border transition-colors ${
                  sub && !isSubSuggested
                    ? "bg-nexus-500 text-white border-nexus-500"
                    : "border-dashed border-gray-300 text-gray-400 hover:border-nexus-300 hover:text-nexus-600"
                }`}
              >
                {sub && !isSubSuggested ? sub : "+ custom"}
              </button>
            )}
          </div>
        </div>
      )}

      {/* Level 3 — Specialisation */}
      {cat && sub && (
        <div>
          <p className="text-xs font-medium text-gray-500 mb-1.5">
            Specialisation{" "}
            <span className="font-normal text-gray-400">(optional)</span>
          </p>
          <div className="flex flex-wrap gap-1.5">
            {itemSuggestions.map((i) => (
              <Chip key={i} label={i} selected={item === i && !itemOpen} onClick={() => selectItem(i)} />
            ))}

            {itemOpen ? (
              <InlineInput
                placeholder="e.g. high-frequency"
                defaultValue={item && !itemSuggestions.includes(item) ? item : ""}
                onCommit={(v) => { setItem(v); setItemOpen(false); emit(cat, sub, v); }}
                onClose={() => setItemOpen(false)}
              />
            ) : (
              <button
                type="button"
                onClick={() => { setItemOpen(true); setItem(""); emit(cat, sub, ""); }}
                className={`rounded-full px-3 py-1 text-xs font-medium border transition-colors ${
                  item && !itemSuggestions.includes(item)
                    ? "bg-nexus-500 text-white border-nexus-500"
                    : "border-dashed border-gray-300 text-gray-400 hover:border-nexus-300 hover:text-nexus-600"
                }`}
              >
                {item && !itemSuggestions.includes(item) ? item : "+ custom"}
              </button>
            )}
          </div>
        </div>
      )}

      {/* Preview */}
      {cat && (
        <div className="flex items-center gap-3 pt-0.5">
          <p className="text-xs text-gray-400">
            Capability:{" "}
            <code className="font-mono text-nexus-500">{display}</code>
          </p>
          <button
            type="button"
            onClick={clearAll}
            className="text-xs text-gray-300 hover:text-gray-500 transition-colors"
          >
            Clear
          </button>
        </div>
      )}
    </div>
  );
}

// ---------- LocalTunnelTip ------------------------------------------------
// Collapsible help panel shown next to any endpoint URL field.

function LocalTunnelTip() {
  const [open, setOpen] = useState(false);

  return (
    <div className="mt-2 rounded-lg border border-gray-200 bg-gray-50 text-sm">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        className="flex w-full items-center justify-between px-4 py-2.5 text-left text-gray-500 hover:text-gray-700"
      >
        <span className="font-medium">Running your bot locally? Here&apos;s how to get a public URL.</span>
        <span className="text-gray-400 text-xs">{open ? "▲" : "▼"}</span>
      </button>

      {open && (
        <div className="border-t border-gray-200 px-4 py-4 space-y-5 text-gray-600">
          <p className="text-xs text-gray-400 leading-relaxed">
            Your bot needs a public HTTPS URL so other agents can reach it. Your router blocks
            direct connections to <code className="font-mono">localhost</code>, but these free
            tools create a secure tunnel in seconds — no router config or domain required.
          </p>

          {/* ngrok */}
          <div>
            <div className="flex items-center gap-2 mb-1">
              <span className="text-xs font-semibold uppercase tracking-wide text-gray-700">ngrok</span>
              <span className="rounded-full bg-green-100 text-green-700 text-xs px-2 py-0.5 font-medium">Easiest</span>
            </div>
            <p className="text-xs text-gray-500 mb-2">
              Free, no account needed to start. Gives you a random <code className="font-mono">https://abc123.ngrok-free.app</code> URL.
              URL changes each restart on the free plan; sign up for a free static domain.
            </p>
            <ol className="text-xs text-gray-500 list-decimal list-inside space-y-1">
              <li>Install: <a href="https://ngrok.com/download" target="_blank" rel="noopener noreferrer" className="text-nexus-500 hover:underline">ngrok.com/download</a></li>
              <li>Run your bot on a local port, e.g. <code className="font-mono bg-gray-100 px-1 rounded">8000</code></li>
              <li>In a second terminal: <code className="font-mono bg-gray-100 px-1 rounded">ngrok http 8000</code></li>
              <li>Copy the <code className="font-mono">Forwarding</code> URL into the field above</li>
            </ol>
          </div>

          {/* Cloudflare Tunnel */}
          <div>
            <div className="flex items-center gap-2 mb-1">
              <span className="text-xs font-semibold uppercase tracking-wide text-gray-700">Cloudflare Tunnel</span>
              <span className="rounded-full bg-blue-100 text-blue-700 text-xs px-2 py-0.5 font-medium">Best long-term</span>
            </div>
            <p className="text-xs text-gray-500 mb-2">
              100% free. Permanent URL. Works even with a cheap custom domain (~$10/yr).
              Cloudflare handles HTTPS/TLS for you automatically — no certificate setup needed.
            </p>
            <ol className="text-xs text-gray-500 list-decimal list-inside space-y-1">
              <li>Install <code className="font-mono bg-gray-100 px-1 rounded">cloudflared</code>: <a href="https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/" target="_blank" rel="noopener noreferrer" className="text-nexus-500 hover:underline">docs</a></li>
              <li>Quick try (no account): <code className="font-mono bg-gray-100 px-1 rounded">cloudflared tunnel --url http://localhost:8000</code></li>
              <li>For a permanent URL: create a free Cloudflare account and follow the{" "}
                <a href="https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/get-started/" target="_blank" rel="noopener noreferrer" className="text-nexus-500 hover:underline">Quick Tunnel guide</a>
              </li>
            </ol>
          </div>

          {/* Tailscale Funnel */}
          <div>
            <div className="flex items-center gap-2 mb-1">
              <span className="text-xs font-semibold uppercase tracking-wide text-gray-700">Tailscale Funnel</span>
              <span className="rounded-full bg-gray-100 text-gray-600 text-xs px-2 py-0.5 font-medium">If you use Tailscale</span>
            </div>
            <p className="text-xs text-gray-500 mb-1">
              Free with a Tailscale account. Stable <code className="font-mono">https://your-machine.ts.net</code> URL.
            </p>
            <code className="block text-xs bg-gray-100 rounded px-2 py-1 font-mono text-gray-600">
              tailscale funnel 8000
            </code>
          </div>

          {/* HTTP vs HTTPS */}
          <div>
            <p className="text-xs font-semibold uppercase tracking-wide text-gray-700 mb-2">HTTP vs HTTPS — what you need to know</p>
            <div className="space-y-2 text-xs text-gray-500">
              <p>
                <strong className="text-gray-700">Your local bot runs plain HTTP</strong> — e.g.{" "}
                <code className="font-mono bg-gray-100 px-1 rounded">http://localhost:8000</code>. That&apos;s fine.
                The tunnel sits in front and terminates HTTPS for you, so the outside world sees a secure connection
                even though traffic inside your machine is unencrypted.
              </p>
              <p>
                <strong className="text-gray-700">Always paste the tunnel&apos;s HTTPS URL</strong> into the endpoint
                field — not your localhost address. For example, ngrok shows you something like{" "}
                <code className="font-mono bg-gray-100 px-1 rounded">https://abc123.ngrok-free.app</code>.
                That&apos;s the URL to register.
              </p>
              <p>
                <strong className="text-gray-700">Why does HTTPS matter?</strong> Other agents and callers verify
                that your endpoint is encrypted before sending data. An{" "}
                <code className="font-mono bg-gray-100 px-1 rounded">http://</code> endpoint will be rejected
                by cautious callers. All three tunnel tools above give you HTTPS automatically — no certificate
                setup required on your end.
              </p>
              <p>
                <strong className="text-gray-700">When HTTP is acceptable:</strong> only on a local or private
                registry where you control all callers and security isn&apos;t a concern — for example, running
                everything on the same machine during development.
              </p>
            </div>
          </div>

          <div className="rounded-md bg-amber-50 border border-amber-200 px-3 py-2 text-xs text-amber-700">
            <strong>Remember:</strong> Register the public HTTPS tunnel URL, not{" "}
            <code className="font-mono bg-amber-100 px-1 rounded">localhost</code>. Your bot
            only needs to speak HTTP internally — the tunnel and NAP handle the rest.
          </div>
        </div>
      )}
    </div>
  );
}

// ---------- DnsSetupGuide -------------------------------------------------

function DnsSetupGuide({ domain }: { domain: string }) {
  const [open, setOpen] = useState(false);
  const host = domain || "example.com";
  const subdomain = `agent.${host}`;
  const txtHost = `_nap-challenge.${host}`;

  return (
    <div className="mt-2 rounded-lg border border-gray-200 bg-gray-50 text-sm">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        className="flex w-full items-center justify-between px-4 py-2.5 text-left text-gray-500 hover:text-gray-700"
      >
        <span className="font-medium">How do I set up my DNS records?</span>
        <span className="text-gray-400 text-xs">{open ? "▲" : "▼"}</span>
      </button>

      {open && (
        <div className="border-t border-gray-200 px-4 py-4 space-y-5 text-xs text-gray-600">
          <p className="text-gray-500 leading-relaxed">
            Two DNS records are involved: one to point your domain at your server, and one
            that NAP uses to confirm you actually own the domain. Both are added through
            your domain registrar or DNS provider (Cloudflare, Namecheap, GoDaddy, Route 53, etc.).
          </p>

          {/* Step 1 — A record */}
          <div>
            <p className="font-semibold text-gray-700 mb-1">Step 1 — Point a subdomain at your server (A record)</p>
            <p className="text-gray-500 mb-2">
              An <strong>A record</strong> maps a hostname to an IPv4 address. Create one so your
              agent has a proper domain address instead of a raw IP. Use a subdomain like{" "}
              <code className="font-mono bg-gray-100 px-1 rounded">{subdomain}</code> to keep
              things tidy — your root domain stays separate.
            </p>
            <div className="rounded-md border border-gray-200 bg-white overflow-hidden">
              <table className="w-full text-xs">
                <thead className="bg-gray-50 text-gray-400 uppercase text-left">
                  <tr>
                    <th className="px-3 py-2 font-medium">Type</th>
                    <th className="px-3 py-2 font-medium">Host / Name</th>
                    <th className="px-3 py-2 font-medium">Value</th>
                    <th className="px-3 py-2 font-medium">TTL</th>
                  </tr>
                </thead>
                <tbody className="font-mono">
                  <tr className="border-t border-gray-100">
                    <td className="px-3 py-2 text-indigo-600 font-semibold">A</td>
                    <td className="px-3 py-2">{subdomain}</td>
                    <td className="px-3 py-2 text-gray-400 italic">your-server-ip</td>
                    <td className="px-3 py-2 text-gray-400">3600</td>
                  </tr>
                </tbody>
              </table>
            </div>
            <p className="mt-2 text-gray-400">
              If your cloud provider (AWS, GCP, Fly.io, Railway…) gives you a hostname instead
              of an IP, use a <strong>CNAME</strong> record pointing to that hostname instead.
            </p>
          </div>

          {/* Step 2 — TXT record */}
          <div>
            <p className="font-semibold text-gray-700 mb-1">Step 2 — Prove domain ownership (TXT record)</p>
            <p className="text-gray-500 mb-2">
              After you register below, NAP generates a unique token and gives you a{" "}
              <strong>TXT record</strong> to add. This is a standard technique (also used by Google
              Workspace, Let&apos;s Encrypt, and others) to prove you control the domain without
              any server access required.
            </p>
            <div className="rounded-md border border-gray-200 bg-white overflow-hidden">
              <table className="w-full text-xs">
                <thead className="bg-gray-50 text-gray-400 uppercase text-left">
                  <tr>
                    <th className="px-3 py-2 font-medium">Type</th>
                    <th className="px-3 py-2 font-medium">Host / Name</th>
                    <th className="px-3 py-2 font-medium">Value</th>
                    <th className="px-3 py-2 font-medium">TTL</th>
                  </tr>
                </thead>
                <tbody className="font-mono">
                  <tr className="border-t border-gray-100">
                    <td className="px-3 py-2 text-indigo-600 font-semibold">TXT</td>
                    <td className="px-3 py-2">{txtHost}</td>
                    <td className="px-3 py-2 text-gray-400 italic">token from NAP</td>
                    <td className="px-3 py-2 text-gray-400">300</td>
                  </tr>
                </tbody>
              </table>
            </div>
            <p className="mt-2 text-gray-400">
              Use a low TTL (300 = 5 min) on the TXT record — it only needs to exist long
              enough for verification and you can delete it afterwards.
            </p>
          </div>

          {/* Where to add records */}
          <div>
            <p className="font-semibold text-gray-700 mb-2">Where to add records in common providers</p>
            <div className="space-y-1 text-gray-500">
              <p><strong className="text-gray-600">Cloudflare</strong> — Dashboard → your domain → DNS → Add record</p>
              <p><strong className="text-gray-600">Namecheap</strong> — Dashboard → Manage → Advanced DNS</p>
              <p><strong className="text-gray-600">GoDaddy</strong> — My Products → DNS → Add New Record</p>
              <p><strong className="text-gray-600">AWS Route 53</strong> — Hosted Zones → your zone → Create Record</p>
              <p><strong className="text-gray-600">Google Domains / Squarespace</strong> — DNS → Manage custom records</p>
            </div>
            <p className="mt-2 text-gray-400">
              DNS changes typically propagate in under 5 minutes on Cloudflare, and up to
              an hour on other registrars. NAP polls automatically once you trigger verification.
            </p>
          </div>
        </div>
      )}
    </div>
  );
}

// ---------- Domain-Verified form ------------------------------------------

function DomainVerifiedTab() {
  const [capabilityNode, setCapabilityNode] = useState("");
  const [displayName, setDisplayName]       = useState("");
  const [description, setDescription]       = useState("");
  const [endpoint, setEndpoint]             = useState("");
  const [ownerDomain, setOwnerDomain]       = useState("");
  const [result, setResult] = useState<{ agent_id?: string; trust_root?: string; capability_node?: string } | null>(null);
  const [error, setError]   = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setResult(null);
    setLoading(true);

    try {
      const base = process.env.NEXT_PUBLIC_REGISTRY_URL ?? "http://localhost:8080";
      const resp = await fetch(`${base}/api/v1/agents`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          capability: capabilityNode,
          display_name: displayName,
          description,
          endpoint,
          owner_domain: ownerDomain,
        }),
      });
      const body = await resp.json();
      if (!resp.ok) throw new Error(body.error ?? `HTTP ${resp.status}`);
      setResult(body);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  };

  const agentURI = result
    ? `agent://${result.trust_root}/${result.capability_node?.split(">")[0]}/${result.agent_id}`
    : "";

  return (
    <>
      {/* Credibility benefits */}
      <div className="mb-6 rounded-xl border border-indigo-100 bg-indigo-50 p-5 space-y-3">
        <p className="text-sm font-semibold text-indigo-900">Why register under your own domain?</p>
        <ul className="space-y-2 text-sm text-indigo-800">
          <li>
            <strong>Your brand in the URI.</strong> Your agent address becomes{" "}
            <code className="font-mono text-xs bg-indigo-100 px-1 rounded">agent://yourcompany.com/finance/…</code>.
            Callers instantly know who owns it — no shared namespace.
          </li>
          <li>
            <strong>Higher trust tier.</strong> Domain-verified agents display as{" "}
            <em>Verified</em> in listings. Completing the full flow with the{" "}
            <code className="font-mono text-xs bg-indigo-100 px-1 rounded">nap claim</code> CLI
            issues an X.509 certificate and upgrades you to <em>Trusted</em> — the highest tier,
            accepted by enterprise callers who require cryptographic proof of identity.
          </li>
          <li>
            <strong>Your domain&apos;s reputation carries over.</strong> Organisations and automated
            systems that already trust <em>yourcompany.com</em> will extend that trust to your agents
            without any extra configuration.
          </li>
          <li>
            <strong>Unlimited agents.</strong> Register as many agents as you need under the same
            domain — each gets its own URI and certificate. No per-agent quota.
          </li>
        </ul>
      </div>

      <p className="mb-6 text-sm text-gray-500">
        After submission your agent will be <strong>pending</strong> until you complete a quick
        DNS ownership check — the same technique used by Let&apos;s Encrypt and Google Workspace.
        It takes about 5 minutes once your DNS records are in place.
      </p>

      <form onSubmit={handleSubmit} className="space-y-5">
        {/* Capability */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Capability <span className="text-red-500">*</span>
          </label>
          <CapabilityPicker value={capabilityNode} onChange={setCapabilityNode} required />
          <p className="mt-1 text-xs text-gray-400">
            Determines how other agents discover yours —{" "}
            <code className="font-mono">agent://acme.com/finance/…</code>
          </p>
        </div>

        {/* Display name */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Display Name <span className="text-red-500">*</span>
          </label>
          <input type="text" value={displayName} onChange={(e) => setDisplayName(e.target.value)}
            required placeholder="My Tax Agent"
            className="w-full rounded-lg border border-gray-300 px-4 py-3 text-sm focus:border-nexus-500 focus:outline-none" />
          <p className="mt-1 text-xs text-gray-400">A human-readable name shown in listings and search results.</p>
        </div>

        {/* Description */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Description</label>
          <input type="text" value={description} onChange={(e) => setDescription(e.target.value)}
            placeholder="Handles tax filing queries"
            className="w-full rounded-lg border border-gray-300 px-4 py-3 text-sm focus:border-nexus-500 focus:outline-none" />
        </div>

        {/* Endpoint */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Endpoint URL <span className="text-red-500">*</span>
          </label>
          <input type="text" value={endpoint} onChange={(e) => setEndpoint(e.target.value)}
            required placeholder="https://agent.example.com"
            className="w-full rounded-lg border border-gray-300 px-4 py-3 text-sm focus:border-nexus-500 focus:outline-none" />
          <p className="mt-1 text-xs text-gray-400">
            Publicly reachable HTTPS URL where your agent accepts requests. Typically a subdomain
            of your owner domain, e.g.{" "}
            <code className="font-mono">https://agent.example.com</code>.
          </p>
          <LocalTunnelTip />
        </div>

        {/* Owner domain */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Owner Domain <span className="text-red-500">*</span>
          </label>
          <input type="text" value={ownerDomain} onChange={(e) => setOwnerDomain(e.target.value)}
            required placeholder="example.com"
            className="w-full rounded-lg border border-gray-300 px-4 py-3 text-sm focus:border-nexus-500 focus:outline-none" />
          <p className="mt-1 text-xs text-gray-400">
            The domain you own. Becomes your trust root —{" "}
            <code className="font-mono">agent://example.com/capability/…</code> — verified via a DNS TXT record.
          </p>
          <DnsSetupGuide domain={ownerDomain} />
        </div>

        <button type="submit" disabled={loading || !capabilityNode}
          className="w-full rounded-lg bg-nexus-500 px-6 py-3 text-white font-semibold hover:bg-indigo-600 disabled:opacity-50">
          {loading ? "Registering..." : "Register Agent"}
        </button>
      </form>

      {error && (
        <div className="mt-6 rounded-lg bg-red-50 p-4 text-sm text-red-600">
          <strong>Error:</strong> {error}
        </div>
      )}

      {result && (
        <div className="mt-6 rounded-xl border border-green-200 bg-green-50 p-6 space-y-4">
          <p className="text-xs font-semibold uppercase tracking-widest text-green-600">Agent Registered</p>

          <div>
            <p className="text-xs font-medium text-green-700 mb-1">Your agent address (pending activation)</p>
            <code className="block rounded-lg bg-white border border-green-100 px-4 py-3 text-sm font-mono text-nexus-500 break-all">
              {agentURI}
            </code>
          </div>

          <div className="rounded-lg bg-white border border-green-100 px-4 py-3 text-sm text-gray-700 space-y-2">
            <p className="font-medium text-green-800">Complete verification in 3 steps</p>
            <ol className="list-decimal list-inside space-y-2 text-gray-600 text-xs">
              <li>
                <strong>Get your TXT record</strong> — run{" "}
                <code className="font-mono bg-gray-100 px-1 rounded">nap dns-challenge start {ownerDomain || "example.com"}</code>{" "}
                or use the DNS challenge page in your dashboard. NAP will give you a token to add.
              </li>
              <li>
                <strong>Add it to your DNS</strong> — create a TXT record for{" "}
                <code className="font-mono bg-gray-100 px-1 rounded">_nap-challenge.{ownerDomain || "example.com"}</code>{" "}
                with the token value. See the DNS setup guide above for help.
              </li>
              <li>
                <strong>Verify and activate</strong> — run{" "}
                <code className="font-mono bg-gray-100 px-1 rounded">nap claim {ownerDomain || "example.com"}</code>{" "}
                (handles steps 1–3 automatically), or trigger verification from your dashboard.
                Your agent goes live immediately on success.
              </li>
            </ol>
          </div>

          <p className="text-xs text-green-600">
            Your <code className="font-mono">agent://</code> URI is permanent from this moment.
            Your endpoint URL can be updated any time from the dashboard.
          </p>
        </div>
      )}
    </>
  );
}

// ---------- NAP Hosted form ----------

interface HostedResult {
  id?: string;
  trust_root?: string;
  capability_node?: string;
  agent_id?: string;
  endpoint?: string;
  [key: string]: unknown;
}

function FreeHostedTab() {
  const REGISTRY = process.env.NEXT_PUBLIC_REGISTRY_URL ?? "http://localhost:8080";

  // Agent form state
  const [user, setUser] = useState<UserClaims | null>(null);
  const [agentName, setAgentName] = useState("");
  const [description, setDescription] = useState("");
  const [capability, setCapability] = useState("");
  const [result, setResult] = useState<HostedResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  // Inline auth panel state
  const [showAuth, setShowAuth] = useState(false);
  const [authMode, setAuthMode] = useState<"signup" | "login">("signup");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [authError, setAuthError] = useState<string | null>(null);
  const [authLoading, setAuthLoading] = useState(false);

  useEffect(() => {
    if (isLoggedIn()) setUser(getUser());
  }, []);

  // Core registration call — used both after inline auth and when already logged in.
  const registerAgent = async (token: string): Promise<boolean> => {
    const resp = await fetch(`${REGISTRY}/api/v1/agents`, {
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
      body: JSON.stringify({ display_name: agentName, description, capability, registration_type: "nap_hosted" }),
    });
    const body = await resp.json();
    if (resp.status === 422 || resp.status === 403) {
      setError(body.error ?? "Agent limit reached for your current plan.");
      return false;
    }
    if (!resp.ok) {
      setError(body.error ?? `HTTP ${resp.status}`);
      return false;
    }
    setResult(body as HostedResult);
    return true;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setResult(null);

    if (!user) {
      // Show inline auth instead of blocking or redirecting.
      setShowAuth(true);
      return;
    }

    setLoading(true);
    try {
      await registerAgent(getToken()!);
    } catch {
      setError("Something went wrong. Please try again.");
    } finally {
      setLoading(false);
    }
  };

  const handleAuth = async (e: React.FormEvent) => {
    e.preventDefault();
    setAuthError(null);
    setAuthLoading(true);

    try {
      const url = authMode === "signup" ? "/api/v1/auth/signup" : "/api/v1/auth/login";
      const resp = await fetch(`${REGISTRY}${url}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, password }),
      });
      const body = await resp.json();

      if (!resp.ok) {
        if (resp.status === 409) setAuthError("Email already in use — log in instead.");
        else if (resp.status === 401) setAuthError("Invalid email or password.");
        else setAuthError(body.error ?? `Error ${resp.status}`);
        return;
      }

      // Store the token and update user state.
      setToken(body.token);
      const claims = getUser();
      setUser(claims);
      setShowAuth(false);

      // Auto-complete the registration.
      setLoading(true);
      await registerAgent(body.token);
    } catch {
      setAuthError("Something went wrong. Please try again.");
    } finally {
      setAuthLoading(false);
      setLoading(false);
    }
  };

  const username = user?.username ?? "you";
  const topCategory = capability ? capability.split(">")[0] : "{category}";
  const uriPreview = `agent://nap/${topCategory}/…`;

  return (
    <>
      <p className="mb-4 text-gray-500">
        Your agent will be registered under the Nexus domain. We assign both the address and the endpoint — you don&apos;t need to own a domain or run a server to get started.
      </p>
      <div className="mb-8 rounded-lg bg-gray-50 border border-gray-200 px-4 py-3">
        <p className="text-xs text-gray-400 mb-1">Your agent address will look like</p>
        <code className="text-sm font-mono text-nexus-500">{uriPreview}</code>
      </div>

      <form onSubmit={handleSubmit} className="space-y-5">
        {/* Capability */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Capability <span className="text-red-500">*</span>
          </label>
          <CapabilityPicker value={capability} onChange={setCapability} required />
          <p className="mt-1 text-xs text-gray-400">
            What does this agent do? Pick up to three levels — other agents use this to find yours.
          </p>
        </div>

        {/* Agent name */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Agent Name <span className="text-red-500">*</span>
          </label>
          <input
            type="text"
            value={agentName}
            onChange={(e) => setAgentName(e.target.value)}
            required
            placeholder="My Research Agent"
            className="w-full rounded-lg border border-gray-300 px-4 py-3 text-sm focus:border-nexus-500 focus:outline-none"
          />
          <p className="mt-1 text-xs text-gray-400">A human-readable name shown in listings and search results.</p>
        </div>

        {/* Description */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Description</label>
          <textarea
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            rows={3}
            placeholder="A short summary of what this agent does."
            className="w-full rounded-lg border border-gray-300 px-4 py-3 text-sm focus:border-nexus-500 focus:outline-none resize-none"
          />
        </div>

        <button
          type="submit"
          disabled={loading || !capability}
          className="w-full rounded-lg bg-nexus-500 px-6 py-3 text-white font-semibold hover:bg-indigo-600 disabled:opacity-50 transition-colors"
        >
          {loading ? "Registering…" : "Register Agent"}
        </button>
      </form>

      {/* Inline auth panel — appears when user submits without being logged in */}
      {showAuth && !result && (
        <div className="mt-6 rounded-xl border border-gray-200 bg-gray-50 p-6">
          <p className="text-sm font-semibold text-gray-900 mb-1">
            {authMode === "signup" ? "Create a free account to register" : "Log in to register"}
          </p>
          <p className="text-xs text-gray-500 mb-5">
            {authMode === "signup"
              ? "Takes 10 seconds. We'll register your agent right after."
              : "Welcome back — we'll register your agent right after."}
          </p>

          <form onSubmit={handleAuth} className="space-y-3">
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              placeholder="Email address"
              className="w-full rounded-lg border border-gray-300 px-4 py-2.5 text-sm focus:border-nexus-500 focus:outline-none bg-white"
            />
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              placeholder="Password"
              minLength={authMode === "signup" ? 8 : 1}
              className="w-full rounded-lg border border-gray-300 px-4 py-2.5 text-sm focus:border-nexus-500 focus:outline-none bg-white"
            />

            {authError && (
              <p className="text-xs text-red-600">{authError}</p>
            )}

            <button
              type="submit"
              disabled={authLoading}
              className="w-full rounded-lg bg-nexus-500 px-4 py-2.5 text-sm font-semibold text-white hover:bg-indigo-600 disabled:opacity-50 transition-colors"
            >
              {authLoading
                ? "Just a moment…"
                : authMode === "signup"
                ? "Create account & register agent"
                : "Log in & register agent"}
            </button>
          </form>

          <p className="mt-4 text-center text-xs text-gray-400">
            {authMode === "signup" ? (
              <>Already have an account?{" "}
                <button onClick={() => { setAuthMode("login"); setAuthError(null); }} className="text-nexus-500 hover:underline">Log in</button>
              </>
            ) : (
              <>New here?{" "}
                <button onClick={() => { setAuthMode("signup"); setAuthError(null); }} className="text-nexus-500 hover:underline">Create a free account</button>
              </>
            )}
          </p>
        </div>
      )}

      {error && (
        <div className="mt-6 rounded-lg bg-red-50 p-4 text-red-600 text-sm">
          <strong>Error:</strong> {error}
        </div>
      )}

      {result && (
        <div className="mt-6 rounded-xl border border-gray-200 bg-white p-6 shadow-sm space-y-4">
          <p className="text-xs font-semibold uppercase tracking-widest text-gray-400">Agent Registered</p>

          {result.trust_root && result.capability_node && result.agent_id && (
            <div>
              <p className="text-xs font-medium text-gray-500 mb-1">Your agent address (URI)</p>
              <code className="block rounded-lg bg-gray-50 border border-gray-100 px-4 py-3 text-sm font-mono text-nexus-500 break-all">
                agent://{result.trust_root}/{result.capability_node?.split(">")[0]}/{result.agent_id}
              </code>
              <p className="mt-1 text-xs text-gray-400">
                This is your permanent address. Share it — other systems use it to find your agent.
              </p>
            </div>
          )}

          <div className="rounded-lg bg-gray-50 border border-gray-200 px-4 py-3 text-sm text-gray-700 space-y-1">
            <p className="font-medium">What happens next</p>
            <ol className="list-decimal list-inside space-y-1 text-gray-500 text-xs">
              <li>Verify your email (check your inbox).</li>
              <li>Activate your agent from your <a href="/account" className="text-nexus-500 hover:underline">account dashboard</a>.</li>
              <li>Set your server URL — the address where your agent accepts requests. You can do this any time from the dashboard.</li>
            </ol>
          </div>

          <LocalTunnelTip />

          <p className="text-xs text-gray-400">
            Your <code className="font-mono">agent://</code> URI is permanent from this moment. Your server URL can be updated any time.
          </p>
        </div>
      )}
    </>
  );
}

// ---------- Page ----------

type Tab = "hosted" | "domain";

export default function RegisterPage() {
  const [tab, setTab] = useState<Tab>("hosted");

  return (
    <div className="max-w-2xl">
      <h1 className="mb-6 text-3xl font-bold">Register an Agent</h1>

      {/* Tab switcher */}
      <div className="mb-8 flex rounded-lg border border-gray-200 overflow-hidden text-sm font-medium">
        <button
          onClick={() => setTab("hosted")}
          className={`flex-1 px-4 py-2.5 transition-colors ${
            tab === "hosted"
              ? "bg-nexus-500 text-white"
              : "bg-white text-gray-600 hover:bg-gray-50"
          }`}
        >
          NAP Hosted
        </button>
        <button
          onClick={() => setTab("domain")}
          className={`flex-1 px-4 py-2.5 transition-colors border-l border-gray-200 ${
            tab === "domain"
              ? "bg-nexus-500 text-white"
              : "bg-white text-gray-600 hover:bg-gray-50"
          }`}
        >
          Domain-Verified
        </button>
      </div>

      {tab === "hosted" ? <FreeHostedTab /> : <DomainVerifiedTab />}
    </div>
  );
}
