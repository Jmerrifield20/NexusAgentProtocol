export default function TermsPage() {
  return (
    <div className="mx-auto max-w-3xl py-12">
      <h1 className="text-4xl font-extrabold tracking-tight text-gray-900">Terms of Service</h1>
      <p className="mt-3 text-sm text-gray-400">Last updated: February 2026</p>

      <div className="mt-10 space-y-10 text-gray-700 leading-relaxed">

        <section>
          <h2 className="text-xl font-semibold text-gray-900">1. Acceptance</h2>
          <p className="mt-3">
            By accessing or using the Nexus Agent Protocol registry ("NAP", "the Service"), you agree
            to be bound by these Terms of Service. If you do not agree, do not use the Service.
          </p>
        </section>

        <section>
          <h2 className="text-xl font-semibold text-gray-900">2. What the Service provides</h2>
          <p className="mt-3">
            NAP is a public registry that assigns permanent, verifiable <code className="bg-gray-100 px-1 rounded text-sm">agent://</code> URIs
            to AI agents. We provide agent registration, DNS-01 domain verification, an mTLS certificate
            authority for trusted-tier agents, and a resolution API.
          </p>
        </section>

        <section>
          <h2 className="text-xl font-semibold text-gray-900">3. Accounts and registration</h2>
          <p className="mt-3">
            You must provide accurate information when creating an account. You are responsible for
            maintaining the security of your credentials. Free-tier accounts may register up to three
            agents. We reserve the right to suspend accounts that violate these Terms.
          </p>
        </section>

        <section>
          <h2 className="text-xl font-semibold text-gray-900">4. Acceptable use</h2>
          <p className="mt-3">You agree not to:</p>
          <ul className="mt-3 list-disc pl-6 space-y-2">
            <li>Register agents that impersonate other organisations or individuals.</li>
            <li>Use the Service to distribute malware, spam, or illegal content.</li>
            <li>Attempt to circumvent rate limits, access controls, or the domain verification process.</li>
            <li>Claim a domain namespace you do not own or control.</li>
            <li>Use the Service in any way that violates applicable law or regulation.</li>
          </ul>
        </section>

        <section>
          <h2 className="text-xl font-semibold text-gray-900">5. Namespace ownership</h2>
          <p className="mt-3">
            The <code className="bg-gray-100 px-1 rounded text-sm">nap/</code> namespace is reserved and
            controlled solely by Nexus Agent Protocol. Domain-verified namespaces (e.g.{" "}
            <code className="bg-gray-100 px-1 rounded text-sm">acme.com/</code>) are granted only to
            registrants who successfully complete DNS-01 verification for the corresponding domain.
            We do not transfer or sell namespace rights.
          </p>
        </section>

        <section>
          <h2 className="text-xl font-semibold text-gray-900">6. Agent URIs</h2>
          <p className="mt-3">
            Once assigned, an <code className="bg-gray-100 px-1 rounded text-sm">agent://</code> URI is
            permanent for the lifetime of the registration. We do not reuse URIs from deleted agents.
            You may update the endpoint URL associated with a URI at any time via your account dashboard
            or the API.
          </p>
        </section>

        <section>
          <h2 className="text-xl font-semibold text-gray-900">7. Open-source software</h2>
          <p className="mt-3">
            The NAP registry software is released under the Apache 2.0 licence. You are free to
            self-host an instance. These Terms apply only to the hosted service at{" "}
            <code className="bg-gray-100 px-1 rounded text-sm">registry.nexusagentprotocol.com</code>.
          </p>
        </section>

        <section>
          <h2 className="text-xl font-semibold text-gray-900">8. Availability and SLA</h2>
          <p className="mt-3">
            We aim for high availability but provide the Service on an "as-is" basis for free-tier users
            with no uptime guarantee. Enterprise SLA terms are available under a separate agreement.
            Contact <a href="mailto:jack@simkura.com" className="text-indigo-600 hover:underline">jack@simkura.com</a> for details.
          </p>
        </section>

        <section>
          <h2 className="text-xl font-semibold text-gray-900">9. Termination</h2>
          <p className="mt-3">
            We may suspend or terminate your access for violation of these Terms, extended inactivity,
            or at our discretion with reasonable notice. You may delete your account at any time via
            the dashboard.
          </p>
        </section>

        <section>
          <h2 className="text-xl font-semibold text-gray-900">10. Disclaimer and limitation of liability</h2>
          <p className="mt-3">
            The Service is provided "as is" without warranties of any kind, express or implied. To the
            fullest extent permitted by law, Nexus Agent Protocol shall not be liable for any indirect,
            incidental, special, or consequential damages arising from your use of the Service.
          </p>
        </section>

        <section>
          <h2 className="text-xl font-semibold text-gray-900">11. Changes to these Terms</h2>
          <p className="mt-3">
            We may update these Terms from time to time. Material changes will be announced via email
            or a notice on the site. Continued use of the Service after changes constitutes acceptance.
          </p>
        </section>

        <section>
          <h2 className="text-xl font-semibold text-gray-900">12. Contact</h2>
          <p className="mt-3">
            Questions about these Terms? Email us at{" "}
            <a href="mailto:jack@simkura.com" className="text-indigo-600 hover:underline">
              jack@simkura.com
            </a>.
          </p>
        </section>

      </div>
    </div>
  );
}
