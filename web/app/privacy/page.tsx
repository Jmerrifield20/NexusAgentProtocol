export default function PrivacyPage() {
  return (
    <div className="mx-auto max-w-3xl py-12">
      <h1 className="text-4xl font-extrabold tracking-tight text-gray-900">Privacy Policy</h1>
      <p className="mt-3 text-sm text-gray-400">Last updated: February 2026</p>

      <div className="mt-10 space-y-10 text-gray-700 leading-relaxed">

        <section>
          <h2 className="text-xl font-semibold text-gray-900">1. Overview</h2>
          <p className="mt-3">
            Nexus Agent Protocol ("NAP", "we", "us") operates the registry at{" "}
            <code className="bg-gray-100 px-1 rounded text-sm">registry.nexusagentprotocol.com</code>.
            This Privacy Policy explains what data we collect, how we use it, and your rights
            regarding that data.
          </p>
        </section>

        <section>
          <h2 className="text-xl font-semibold text-gray-900">2. Data we collect</h2>

          <h3 className="mt-5 font-medium text-gray-800">Account data</h3>
          <p className="mt-2">
            When you create an account we collect your email address, a display name, and a hashed
            password (we never store plaintext passwords). If you sign in via OAuth (GitHub or Google)
            we receive your email and public profile from that provider.
          </p>

          <h3 className="mt-5 font-medium text-gray-800">Agent registration data</h3>
          <p className="mt-2">
            Agent registrations are <strong>public by design</strong> — the registry is a discovery
            service. Data you submit when registering an agent (display name, description, capability,
            endpoint URL, domain) will be publicly queryable via the API and the web portal.
          </p>

          <h3 className="mt-5 font-medium text-gray-800">Usage and log data</h3>
          <p className="mt-2">
            We log API requests (timestamp, endpoint, HTTP status, IP address) for security monitoring
            and rate limiting. Logs are retained for 30 days and are not sold or shared with third parties.
          </p>

          <h3 className="mt-5 font-medium text-gray-800">Cookies and local storage</h3>
          <p className="mt-2">
            The web portal stores your authentication token in <code className="bg-gray-100 px-1 rounded text-sm">localStorage</code> to
            keep you signed in. We do not use tracking cookies or third-party analytics.
          </p>
        </section>

        <section>
          <h2 className="text-xl font-semibold text-gray-900">3. How we use your data</h2>
          <ul className="mt-3 list-disc pl-6 space-y-2">
            <li>To operate your account and authenticate API requests.</li>
            <li>To send transactional emails (email verification, password reset). We do not send marketing email without your explicit consent.</li>
            <li>To enforce rate limits and detect abuse.</li>
            <li>To maintain the public agent registry.</li>
          </ul>
        </section>

        <section>
          <h2 className="text-xl font-semibold text-gray-900">4. Public nature of agent records</h2>
          <p className="mt-3">
            The purpose of NAP is to make agents <em>discoverable</em>. Any data you include in an
            agent registration — name, description, capability, endpoint URL — is intentionally public
            and indexed. Do not include private or sensitive information in agent registration fields.
          </p>
        </section>

        <section>
          <h2 className="text-xl font-semibold text-gray-900">5. Data sharing</h2>
          <p className="mt-3">
            We do not sell your personal data. We may share data with:
          </p>
          <ul className="mt-3 list-disc pl-6 space-y-2">
            <li><strong>Infrastructure providers</strong> — cloud hosting and database providers who process data on our behalf under data processing agreements.</li>
            <li><strong>Law enforcement</strong> — if required by law or to protect the rights, property, or safety of NAP, our users, or the public.</li>
          </ul>
        </section>

        <section>
          <h2 className="text-xl font-semibold text-gray-900">6. Data retention</h2>
          <p className="mt-3">
            Account data is retained for the lifetime of your account plus 30 days after deletion.
            Agent registration records (excluding personal data) may be retained indefinitely as part
            of the trust ledger — an append-only audit log of registry events.
            API logs are retained for 30 days.
          </p>
        </section>

        <section>
          <h2 className="text-xl font-semibold text-gray-900">7. Your rights</h2>
          <p className="mt-3">Depending on your jurisdiction, you may have the right to:</p>
          <ul className="mt-3 list-disc pl-6 space-y-2">
            <li>Access the personal data we hold about you.</li>
            <li>Correct inaccurate data.</li>
            <li>Delete your account and associated personal data.</li>
            <li>Export your data in a machine-readable format.</li>
          </ul>
          <p className="mt-3">
            To exercise any of these rights, email{" "}
            <a href="mailto:jack@simkura.com" className="text-indigo-600 hover:underline">
              jack@simkura.com
            </a>.
            We will respond within 30 days.
          </p>
        </section>

        <section>
          <h2 className="text-xl font-semibold text-gray-900">8. Security</h2>
          <p className="mt-3">
            Passwords are hashed using bcrypt. API tokens are RS256 JWTs with short expiry windows.
            All data in transit is encrypted with TLS. Domain-verified agents receive mTLS client
            certificates issued by the NAP certificate authority. We follow responsible disclosure
            practices — if you discover a security vulnerability, please email{" "}
            <a href="mailto:jack@simkura.com" className="text-indigo-600 hover:underline">
              jack@simkura.com
            </a>.
          </p>
        </section>

        <section>
          <h2 className="text-xl font-semibold text-gray-900">9. Children</h2>
          <p className="mt-3">
            The Service is not directed at children under 13. We do not knowingly collect personal
            data from children. If you believe a child has provided us with personal data, please
            contact us and we will delete it promptly.
          </p>
        </section>

        <section>
          <h2 className="text-xl font-semibold text-gray-900">10. Changes to this policy</h2>
          <p className="mt-3">
            We may update this Privacy Policy. Material changes will be communicated via email or
            a prominent notice on the site at least 14 days before they take effect.
          </p>
        </section>

        <section>
          <h2 className="text-xl font-semibold text-gray-900">11. Contact</h2>
          <p className="mt-3">
            Privacy questions or requests:{" "}
            <a href="mailto:jack@simkura.com" className="text-indigo-600 hover:underline">
              jack@simkura.com
            </a>
          </p>
        </section>

      </div>
    </div>
  );
}
