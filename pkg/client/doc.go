// Package client is the Nexus Agentic Protocol (NAP) Go SDK.
//
// It provides everything a developer needs to build NAP-compliant agents:
// discovering other agents, authenticating with the Nexus registry, obtaining
// scoped Task Tokens, and calling agent endpoints — all in one coherent API.
//
// # Connecting as an existing agent (most common case)
//
// After running 'nap claim', your certificates live in ~/.nap/certs/<domain>/.
// Load them in one call:
//
//	c, err := client.NewFromCertDir(
//	    "https://registry.nexus.io",
//	    os.ExpandEnv("$HOME/.nap/certs/example.com"),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// # Calling another agent
//
// CallAgent resolves the URI, obtains a Task Token (auto-refreshed), and
// makes the authenticated HTTP call in one step:
//
//	var reply InvoiceResponse
//	err = c.CallAgent(ctx,
//	    "agent://nexus.io/finance/billing/agent_7x2v9q",
//	    http.MethodPost, "/v1/invoice",
//	    &InvoiceRequest{Amount: 100, Currency: "USD"},
//	    &reply,
//	)
//
// reqBody and respBody are JSON-encoded/decoded automatically. Pass nil for
// either when not needed (e.g. GET requests or when the response is ignored).
//
// # URI resolution only
//
// When you only need the transport endpoint without making a call:
//
//	result, err := c.Resolve(ctx, "agent://nexus.io/finance/billing/agent_7x2v9q")
//	fmt.Println(result.Endpoint) // https://billing.example.com
//
// Add result caching with WithCacheTTL to avoid repeated registry lookups:
//
//	c, _ := client.NewFromCertDir(registryURL, certDir,
//	    client.WithCacheTTL(60*time.Second),
//	)
//
// # Unauthenticated resolution (read-only clients)
//
// Resolution is public — no certificate is required:
//
//	c, _ := client.New("https://registry.nexus.io")
//	result, err := c.Resolve(ctx, agentURI)
//
// # Token management
//
// Tokens are fetched automatically by CallAgent and cached until 60 seconds
// before expiry. For manual control:
//
//	token, err := c.FetchToken(ctx) // exchanges mTLS cert for JWT
//
// # Registering a new agent programmatically
//
// For scripted or server-side registration (the CLI 'nap claim' covers the
// interactive case):
//
//	challenge, _ := c.StartDNSChallenge(ctx, "example.com")
//	// ... publish challenge.TXTHost / challenge.TXTRecord ...
//	c.VerifyDNSChallenge(ctx, challenge.ID)
//	agent, _ := c.RegisterAgent(ctx, client.RegisterAgentRequest{
//	    TrustRoot:      "nexus.io",
//	    CapabilityNode: "finance/billing",
//	    DisplayName:    "Acme Billing",
//	    Endpoint:       "https://billing.example.com",
//	    OwnerDomain:    "example.com",
//	})
//	certs, _ := c.ActivateAgent(ctx, agent.ID)
//	// Store certs.PrivateKeyPEM securely — it is not persisted by the registry.
package client
