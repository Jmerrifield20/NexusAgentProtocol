package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/jmerrifield20/NexusAgentProtocol/pkg/agentcard"
	"github.com/jmerrifield20/NexusAgentProtocol/pkg/client"
	"github.com/jmerrifield20/NexusAgentProtocol/pkg/uri"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// version is overridden by goreleaser via -ldflags "-X main.version=...".
var version = "dev"

var (
	registryURL string
	cfgFile     string
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "nap",
	Short: "Nexus Agent Protocol CLI",
	Long: `nap is the command-line interface for the Nexus Agent Protocol.

It allows you to register agents, resolve agent:// URIs, and manage
your agent registrations on a Nexus registry.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if cfgFile != "" {
			viper.SetConfigFile(cfgFile)
		} else {
			home, _ := os.UserHomeDir()
			viper.AddConfigPath(home + "/.nap")
			viper.SetConfigName("config")
			viper.SetConfigType("yaml")
		}
		viper.AutomaticEnv()
		_ = viper.ReadInConfig()

		if registryURL == "" {
			registryURL = viper.GetString("registry_url")
		}
		if registryURL == "" {
			registryURL = "https://registry.nexusagentprotocol.com"
		}
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default ~/.nap/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&registryURL, "registry", "", "Nexus registry URL (default https://registry.nexusagentprotocol.com)")

	rootCmd.AddCommand(resolveCmd)
	rootCmd.AddCommand(registerCmd)
	rootCmd.AddCommand(claimCmd)
	rootCmd.AddCommand(revokeCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(dnsChallengeCmd)
}

// ── resolve ──────────────────────────────────────────────────────────────────

// resolveRow holds the outcome of a single URI resolution attempt.
type resolveRow struct {
	uri    string
	result *client.ResolveResult
	err    error
}

var (
	resolveFormat   string
	resolverURL     string
	resolveCacheTTL time.Duration
	resolveInsecure bool
)

var resolveCmd = &cobra.Command{
	Use:   "resolve <agent://...> [agent://...] ...",
	Short: "Resolve one or more agent:// URIs to their transport endpoints",
	Long: `Resolve translates agent:// URIs into transport endpoints.

By default it queries the registry directly. Use --resolver to target the
dedicated resolver service, which maintains a warm endpoint cache:

  nap resolve --resolver http://localhost:9091 agent://nexusagentprotocol.com/finance/taxes/agent_abc

Multiple URIs are resolved concurrently and displayed as a table:

  nap resolve agent://nexusagentprotocol.com/a/agent_1 agent://nexusagentprotocol.com/b/agent_2`,
	Args: cobra.MinimumNArgs(1),
	RunE: runResolve,
}

func init() {
	resolveCmd.Flags().StringVar(&resolveFormat, "format", "text", "Output format: text or json")
	resolveCmd.Flags().StringVar(&resolverURL, "resolver", "", "Resolver service base URL (e.g. http://localhost:9091); uses registry when empty")
	resolveCmd.Flags().DurationVar(&resolveCacheTTL, "cache-ttl", 0, "Cache TTL for results (e.g. 60s); 0 disables caching")
	resolveCmd.Flags().BoolVar(&resolveInsecure, "insecure", false, "Skip TLS certificate verification (development only)")
}

func runResolve(cmd *cobra.Command, args []string) error {
	// Validate all URIs up-front.
	for _, agentURI := range args {
		if _, err := uri.Parse(agentURI); err != nil {
			return fmt.Errorf("invalid URI %q: %w", agentURI, err)
		}
	}

	opts := []client.Option{}
	if resolveCacheTTL > 0 {
		opts = append(opts, client.WithCacheTTL(resolveCacheTTL))
	}
	if resolveInsecure {
		opts = append(opts, client.WithInsecureSkipVerify())
	}

	c, err := client.New(registryURL, opts...)
	if err != nil {
		return err
	}

	ctx := context.Background()

	// Resolve all URIs concurrently.
	resultsCh := make(chan resolveRow, len(args))
	for _, agentURI := range args {
		agentURI := agentURI
		go func() {
			var r *client.ResolveResult
			var err error
			if resolverURL != "" {
				r, err = c.ResolveViaService(ctx, resolverURL, agentURI)
			} else {
				r, err = c.Resolve(ctx, agentURI)
			}
			resultsCh <- resolveRow{uri: agentURI, result: r, err: err}
		}()
	}

	// Collect in input order.
	ordered := make([]resolveRow, len(args))
	byURI := make(map[string]resolveRow, len(args))
	for range args {
		r := <-resultsCh
		byURI[r.uri] = r
	}
	for i, agentURI := range args {
		ordered[i] = byURI[agentURI]
	}

	// Output.
	switch resolveFormat {
	case "json":
		return printResolveJSON(ordered)
	default:
		return printResolveText(ordered)
	}
}

func printResolveJSON(results []resolveRow) error {
	type jsonRow struct {
		URI        string `json:"uri"`
		Endpoint   string `json:"endpoint,omitempty"`
		Status     string `json:"status,omitempty"`
		CertSerial string `json:"cert_serial,omitempty"`
		Error      string `json:"error,omitempty"`
	}
	rows := make([]jsonRow, len(results))
	for i, r := range results {
		if r.err != nil {
			rows[i] = jsonRow{URI: r.uri, Error: r.err.Error()}
		} else {
			rows[i] = jsonRow{
				URI:        r.result.URI,
				Endpoint:   r.result.Endpoint,
				Status:     r.result.Status,
				CertSerial: r.result.CertSerial,
			}
		}
	}
	// Single result: unwrap from array for convenience.
	var v any = rows
	if len(rows) == 1 {
		v = rows[0]
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func printResolveText(results []resolveRow) error {
	if len(results) == 1 {
		r := results[0]
		if r.err != nil {
			return fmt.Errorf("resolve %q: %w", r.uri, r.err)
		}
		fmt.Printf("URI:         %s\n", r.result.URI)
		fmt.Printf("Endpoint:    %s\n", r.result.Endpoint)
		fmt.Printf("Status:      %s\n", r.result.Status)
		if r.result.CertSerial != "" {
			fmt.Printf("Cert Serial: %s\n", r.result.CertSerial)
		}
		return nil
	}

	// Multiple results: tabulated.
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "URI\tENDPOINT\tSTATUS\tSERIAL\tERROR")
	for _, r := range results {
		if r.err != nil {
			fmt.Fprintf(w, "%s\t\t\t\t%s\n", r.uri, r.err.Error())
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t\n",
				r.result.URI, r.result.Endpoint, r.result.Status, r.result.CertSerial)
		}
	}
	return w.Flush()
}

// ── register ─────────────────────────────────────────────────────────────────

var (
	regTrustRoot   string
	regCapNode     string
	regDisplayName string
	regDescription string
	regEndpoint    string
	regOwnerDomain string
)

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Register a new agent with the Nexus registry",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New(registryURL)
		if err != nil {
			return err
		}

		result, err := c.RegisterAgent(context.Background(), client.RegisterAgentRequest{
			TrustRoot:      regTrustRoot,
			CapabilityNode: regCapNode,
			DisplayName:    regDisplayName,
			Description:    regDescription,
			Endpoint:       regEndpoint,
			OwnerDomain:    regOwnerDomain,
		})
		if err != nil {
			return fmt.Errorf("register agent: %w", err)
		}

		fmt.Printf("✓ Agent registered\n\n")
		fmt.Printf("  ID:  %s\n", result.ID)
		fmt.Printf("  URI: %s\n\n", result.URI)
		fmt.Println("Next: nap claim <domain> to complete DNS verification and activate")
		return nil
	},
}

func init() {
	registerCmd.Flags().StringVar(&regTrustRoot, "trust-root", "nexusagentprotocol.com", "Trust root (registry hostname)")
	registerCmd.Flags().StringVar(&regCapNode, "capability", "", "Capability node path (e.g. finance/taxes)")
	registerCmd.Flags().StringVar(&regDisplayName, "name", "", "Display name for the agent")
	registerCmd.Flags().StringVar(&regDescription, "description", "", "Agent description")
	registerCmd.Flags().StringVar(&regEndpoint, "endpoint", "", "Agent transport endpoint URL")
	registerCmd.Flags().StringVar(&regOwnerDomain, "domain", "", "Owner domain (must pass DNS-01 challenge)")

	_ = registerCmd.MarkFlagRequired("capability")
	_ = registerCmd.MarkFlagRequired("name")
	_ = registerCmd.MarkFlagRequired("endpoint")
	_ = registerCmd.MarkFlagRequired("domain")
}

// ── claim ─────────────────────────────────────────────────────────────────────

var (
	claimCapability  string
	claimName        string
	claimDescription string
	claimEndpoint    string
	claimTrustRoot   string
	claimOutputDir   string
	claimTimeoutMin  int
	claimInsecure    bool
)

// agentSpec collects the registration fields for a single agent.
type agentSpec struct {
	TrustRoot      string
	CapabilityNode string
	DisplayName    string
	Description    string
	Endpoint       string
}

var claimCmd = &cobra.Command{
	Use:   "claim <domain>",
	Short: "Register a domain as an agent via DNS-01 verification",
	Long: `claim guides you through the complete DNS-01 → register → activate flow.

It optionally reads agent-card.json from your domain to pre-populate fields.
On success the agent X.509 cert bundle is written to ~/.nap/certs/<domain>/.`,
	Args: cobra.ExactArgs(1),
	RunE: runClaim,
}

func init() {
	claimCmd.Flags().StringVar(&claimCapability, "capability", "", "Capability node path (e.g. ecommerce/retail)")
	claimCmd.Flags().StringVar(&claimName, "name", "", "Display name for the agent")
	claimCmd.Flags().StringVar(&claimDescription, "description", "", "Agent description")
	claimCmd.Flags().StringVar(&claimEndpoint, "endpoint", "", "Agent transport endpoint URL")
	claimCmd.Flags().StringVar(&claimTrustRoot, "trust-root", "nexusagentprotocol.com", "Trust root hostname")
	claimCmd.Flags().StringVar(&claimOutputDir, "output", "", "Certificate output directory (default ~/.nap/certs/<domain>/)")
	claimCmd.Flags().IntVar(&claimTimeoutMin, "timeout", 10, "DNS polling timeout in minutes")
	claimCmd.Flags().BoolVar(&claimInsecure, "insecure", false, "Skip TLS certificate verification (development only)")
}

func runClaim(cmd *cobra.Command, args []string) error {
	domain := args[0]
	ctx := context.Background()
	stdin := bufio.NewReader(os.Stdin)

	// 1. Try fetching agent-card.json from the domain.
	var specs []agentSpec
	card, fetchErr := agentcard.Fetch(domain)
	if fetchErr == nil && len(card.Agents) > 0 {
		fmt.Printf("Found agent-card.json with %d agent(s) on %s\n\n", len(card.Agents), domain)
		for _, e := range card.Agents {
			trustRoot := card.TrustRoot
			if claimTrustRoot != "nexusagentprotocol.com" { // user explicitly overrode it
				trustRoot = claimTrustRoot
			}
			specs = append(specs, agentSpec{
				TrustRoot:      trustRoot,
				CapabilityNode: e.CapabilityNode,
				DisplayName:    e.DisplayName,
				Description:    e.Description,
				Endpoint:       e.Endpoint,
			})
		}
	} else {
		// No agent-card.json — flags are required.
		if claimCapability == "" || claimName == "" || claimEndpoint == "" {
			return fmt.Errorf("--capability, --name, and --endpoint are required when agent-card.json is not found on the domain")
		}
		specs = []agentSpec{{
			TrustRoot:      claimTrustRoot,
			CapabilityNode: claimCapability,
			DisplayName:    claimName,
			Description:    claimDescription,
			Endpoint:       claimEndpoint,
		}}
	}

	// 2. Print summary and prompt for confirmation.
	fmt.Printf("Will register %d agent(s) for domain: %s\n\n", len(specs), domain)
	for i, s := range specs {
		fmt.Printf("  [%d] %s  (%s)\n", i+1, s.DisplayName, s.CapabilityNode)
		fmt.Printf("       Endpoint: %s\n", s.Endpoint)
	}
	fmt.Printf("\nRegistry: %s\n\n", registryURL)

	fmt.Print("Proceed? [Y/n]: ")
	answer, _ := stdin.ReadString('\n')
	answer = strings.TrimSpace(answer)
	if answer != "" && strings.ToLower(answer) != "y" {
		fmt.Println("Aborted.")
		return nil
	}

	// Build client.
	opts := []client.Option{}
	if claimInsecure {
		opts = append(opts, client.WithInsecureSkipVerify())
	}
	c, err := client.New(registryURL, opts...)
	if err != nil {
		return err
	}

	// 3. Start DNS-01 challenge.
	fmt.Printf("\nStarting DNS-01 challenge for %s...\n", domain)
	challenge, err := c.StartDNSChallenge(ctx, domain)
	if err != nil {
		return fmt.Errorf("start DNS challenge: %w", err)
	}

	// 4. Print the DNS TXT record box.
	fmt.Println()
	fmt.Println("┌─────────────────────────────────────────────────────────────┐")
	fmt.Println("│  Add this DNS TXT record to your domain:                    │")
	fmt.Println("│                                                             │")
	fmt.Printf("│  Host:  %-51s│\n", challenge.TXTHost)
	fmt.Println("│  Type:  TXT                                                 │")
	fmt.Printf("│  Value: %-51s│\n", challenge.TXTRecord)
	fmt.Println("│                                                             │")
	fmt.Println("│  Press Enter when published (TTL ~60s to propagate)         │")
	fmt.Println("└─────────────────────────────────────────────────────────────┘")
	fmt.Println()

	// 5. Wait for the user to press Enter.
	stdin.ReadString('\n') //nolint:errcheck

	// 6. Poll VerifyDNSChallenge until verified or timed out.
	timeout := time.Duration(claimTimeoutMin) * time.Minute
	deadline := time.Now().Add(timeout)
	spinner := []string{"|", "/", "-", "\\"}
	spinIdx := 0
	verified := false

	for time.Now().Before(deadline) {
		ok, verifyErr := c.VerifyDNSChallenge(ctx, challenge.ID)
		if ok {
			verified = true
			break
		}
		if verifyErr != nil && !errors.Is(verifyErr, client.ErrVerificationPending) {
			fmt.Println()
			return fmt.Errorf("verify DNS challenge: %w", verifyErr)
		}
		fmt.Printf("\rVerifying DNS record... %s ", spinner[spinIdx%len(spinner)])
		spinIdx++
		time.Sleep(15 * time.Second)
	}
	fmt.Println()

	if !verified {
		return fmt.Errorf(
			"DNS verification timed out after %d minute(s)\n\nEnsure the TXT record is published:\n  Host:  %s\n  Value: %s",
			claimTimeoutMin, challenge.TXTHost, challenge.TXTRecord,
		)
	}
	fmt.Println("✓ Domain ownership verified")

	// 7–8. For each agent spec: register, activate, save certs, print success.
	for _, spec := range specs {
		fmt.Printf("\nRegistering agent: %s (%s)...\n", spec.DisplayName, spec.CapabilityNode)

		agentResult, err := c.RegisterAgent(ctx, client.RegisterAgentRequest{
			TrustRoot:      spec.TrustRoot,
			CapabilityNode: spec.CapabilityNode,
			DisplayName:    spec.DisplayName,
			Description:    spec.Description,
			Endpoint:       spec.Endpoint,
			OwnerDomain:    domain,
		})
		if err != nil {
			return fmt.Errorf("register agent %q: %w", spec.DisplayName, err)
		}
		fmt.Printf("✓ Agent registered: %s (pending)\n", agentResult.URI)

		activateResult, err := c.ActivateAgent(ctx, agentResult.ID)
		if err != nil {
			return fmt.Errorf("activate agent %s: %w", agentResult.ID, err)
		}
		fmt.Println("✓ Agent activated")

		// Determine output directory.
		outputDir := claimOutputDir
		if outputDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("get home dir: %w", err)
			}
			outputDir = filepath.Join(home, ".nap", "certs", domain)
		}

		if err := saveCertBundle(outputDir, activateResult); err != nil {
			return fmt.Errorf("save certs: %w", err)
		}

		// Print success summary.
		fmt.Printf("\n✓ Agent registered successfully!\n\n")
		fmt.Printf("  URI:      %s\n", activateResult.URI)
		fmt.Printf("  Endpoint: %s\n", spec.Endpoint)
		fmt.Printf("  Certs:    %s\n\n", outputDir)
		fmt.Println("Next steps:")
		fmt.Printf("  nap resolve %s\n", activateResult.URI)
	}

	return nil
}

// saveCertBundle writes cert.pem, key.pem (chmod 600), and ca.pem to dir.
func saveCertBundle(dir string, result *client.ActivateResult) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create cert dir: %w", err)
	}

	type certFile struct {
		name string
		data string
		mode os.FileMode
	}
	files := []certFile{
		{"cert.pem", result.CertPEM, 0o644},
		{"key.pem", result.PrivateKeyPEM, 0o600},
		{"ca.pem", result.CAPEM, 0o644},
	}
	for _, f := range files {
		if f.data == "" {
			continue
		}
		path := filepath.Join(dir, f.name)
		if err := os.WriteFile(path, []byte(f.data), f.mode); err != nil {
			return fmt.Errorf("write %s: %w", f.name, err)
		}
	}
	return nil
}

// ── revoke ───────────────────────────────────────────────────────────────────

var (
	revokeCert     string
	revokeKey      string
	revokeCA       string
	revokeToken    string
	revokeForce    bool
	revokeInsecure bool
)

var revokeCmd = &cobra.Command{
	Use:   "revoke <agent-uuid | agent://...>",
	Short: "Revoke an agent registration",
	Long: `Revoke marks an agent as revoked in the Nexus registry.

Authentication is required. Present a JWT token (--token) or your agent's
mTLS certificate (--cert/--key). If neither is supplied, credentials are
auto-discovered from ~/.nap/certs/<owner-domain>/.

The token must belong to the target agent's URI, or carry the nexus:admin scope.

Examples:

  # Revoke by UUID — certs auto-discovered from ~/.nap/certs/<domain>/
  nap revoke 550e8400-e29b-41d4-a716-446655440000

  # Revoke by agent:// URI with an explicit JWT
  nap revoke --token eyJhbG... agent://nexusagentprotocol.com/finance/taxes/agent_abc

  # Revoke with explicit cert files, skip confirmation
  nap revoke --cert cert.pem --key key.pem --ca ca.pem --force <uuid>`,
	Args: cobra.ExactArgs(1),
	RunE: runRevoke,
}

func init() {
	revokeCmd.Flags().StringVar(&revokeCert, "cert", "", "Client certificate PEM file (for mTLS → token exchange)")
	revokeCmd.Flags().StringVar(&revokeKey, "key", "", "Client private key PEM file")
	revokeCmd.Flags().StringVar(&revokeCA, "ca", "", "CA certificate PEM file (validates registry TLS)")
	revokeCmd.Flags().StringVar(&revokeToken, "token", "", "JWT Bearer token (skips mTLS token exchange)")
	revokeCmd.Flags().BoolVar(&revokeForce, "force", false, "Skip confirmation prompt")
	revokeCmd.Flags().BoolVar(&revokeInsecure, "insecure", false, "Skip TLS certificate verification (development only)")
}

func runRevoke(cmd *cobra.Command, args []string) error {
	arg := args[0]
	ctx := context.Background()

	// Build an unauthenticated client for lookups.
	var lookupOpts []client.Option
	if revokeInsecure {
		lookupOpts = append(lookupOpts, client.WithInsecureSkipVerify())
	}
	c, err := client.New(registryURL, lookupOpts...)
	if err != nil {
		return err
	}

	// Resolve argument to UUID + full agent details.
	agentUUID, agent, err := resolveAgentArg(ctx, c, arg)
	if err != nil {
		return err
	}

	agentURIStr := "agent://" + agent.TrustRoot + "/" + agent.CapabilityNode + "/" + agent.AgentID

	// Show agent details and prompt for confirmation.
	fmt.Printf("\nAgent to revoke:\n\n")
	fmt.Printf("  URI:      %s\n", agentURIStr)
	fmt.Printf("  Name:     %s\n", agent.DisplayName)
	fmt.Printf("  Domain:   %s\n", agent.OwnerDomain)
	fmt.Printf("  Endpoint: %s\n", agent.Endpoint)
	fmt.Printf("  Status:   %s\n\n", agent.Status)

	if !revokeForce {
		fmt.Print("This action cannot be undone. Confirm revocation? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(answer)) != "y" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	// Obtain a JWT token for authentication.
	token := revokeToken
	if token == "" {
		certPEM, keyPEM, caPEM, loadErr := loadOrDiscoverCerts(revokeCert, revokeKey, revokeCA, agent.OwnerDomain)
		if loadErr != nil {
			return fmt.Errorf("no credentials for revocation: %w\n\nUse --token, or --cert/--key, or run 'nap claim %s' first", loadErr, agent.OwnerDomain)
		}
		authClient, err := client.New(registryURL, client.WithMTLS(certPEM, keyPEM, caPEM))
		if err != nil {
			return fmt.Errorf("build mTLS client: %w", err)
		}
		token, err = authClient.FetchToken(ctx)
		if err != nil {
			return fmt.Errorf("fetch auth token via mTLS: %w", err)
		}
	}

	// Revoke with Bearer token.
	authC, err := client.New(registryURL, append(lookupOpts, client.WithBearerToken(token))...)
	if err != nil {
		return err
	}
	if err := authC.RevokeAgent(ctx, agentUUID); err != nil {
		return fmt.Errorf("revoke failed: %w", err)
	}

	fmt.Printf("✓ Agent revoked: %s\n", agentURIStr)
	return nil
}

// resolveAgentArg accepts either an agent:// URI or a UUID string and returns
// the agent's UUID and full AgentDetail from the registry.
func resolveAgentArg(ctx context.Context, c *client.Client, arg string) (string, *client.AgentDetail, error) {
	if strings.HasPrefix(arg, "agent://") {
		result, err := c.Resolve(ctx, arg)
		if err != nil {
			return "", nil, fmt.Errorf("resolve %q: %w", arg, err)
		}
		if result.ID == "" {
			return "", nil, fmt.Errorf("registry did not return an agent ID for %q; use the UUID directly", arg)
		}
		agent, err := c.GetAgent(ctx, result.ID)
		if err != nil {
			return "", nil, fmt.Errorf("get agent details: %w", err)
		}
		return result.ID, agent, nil
	}
	agent, err := c.GetAgent(ctx, arg)
	if err != nil {
		return "", nil, fmt.Errorf("get agent %q: %w", arg, err)
	}
	return arg, agent, nil
}

// loadOrDiscoverCerts returns (certPEM, keyPEM, caPEM). It reads from the given
// file paths when provided, or auto-discovers from ~/.nap/certs/<ownerDomain>/.
func loadOrDiscoverCerts(certPath, keyPath, caPath, ownerDomain string) (certPEM, keyPEM, caPEM string, err error) {
	if certPath != "" || keyPath != "" {
		certPEM, err = readFile(certPath)
		if err != nil {
			return "", "", "", fmt.Errorf("read cert %q: %w", certPath, err)
		}
		keyPEM, err = readFile(keyPath)
		if err != nil {
			return "", "", "", fmt.Errorf("read key %q: %w", keyPath, err)
		}
		if caPath != "" {
			caPEM, err = readFile(caPath)
			if err != nil {
				return "", "", "", fmt.Errorf("read CA %q: %w", caPath, err)
			}
		}
		return certPEM, keyPEM, caPEM, nil
	}

	// Auto-discover from ~/.nap/certs/<ownerDomain>/.
	if ownerDomain == "" {
		return "", "", "", fmt.Errorf("no cert paths provided and owner domain is unknown")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", "", fmt.Errorf("get home dir: %w", err)
	}
	dir := filepath.Join(home, ".nap", "certs", ownerDomain)

	certPEM, err = readFile(filepath.Join(dir, "cert.pem"))
	if err != nil {
		return "", "", "", fmt.Errorf("cert not found in %s (run 'nap claim %s' first): %w", dir, ownerDomain, err)
	}
	keyPEM, err = readFile(filepath.Join(dir, "key.pem"))
	if err != nil {
		return "", "", "", fmt.Errorf("key not found in %s: %w", dir, err)
	}
	caPEM, _ = readFile(filepath.Join(dir, "ca.pem")) // best-effort; may be empty
	return certPEM, keyPEM, caPEM, nil
}

// readFile reads the contents of a file and returns them as a string.
func readFile(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is empty")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ── version ──────────────────────────────────────────────────────────────────

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the nap CLI version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("nap %s (Nexus Agent Protocol)\n", version)
	},
}

// ── dns-challenge ─────────────────────────────────────────────────────────────

var dnsChallengeCmd = &cobra.Command{
	Use:   "dns-challenge",
	Short: "Manage DNS-01 domain ownership challenges",
	Long: `dns-challenge provides fine-grained control over the DNS-01 verification flow.

For the full guided flow (challenge → verify → register → activate), use 'nap claim'.
Use these subcommands to start or verify challenges independently, e.g. when
pre-verifying a domain before registering multiple agents.`,
}

var dnsStartCmd = &cobra.Command{
	Use:   "start <domain>",
	Short: "Start a DNS-01 challenge for a domain",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		domain := args[0]
		c, err := client.New(registryURL)
		if err != nil {
			return err
		}

		challenge, err := c.StartDNSChallenge(context.Background(), domain)
		if err != nil {
			return fmt.Errorf("start challenge: %w", err)
		}

		fmt.Printf("Challenge ID: %s\n\n", challenge.ID)
		fmt.Println("Add this DNS TXT record to your domain:")
		fmt.Printf("  Host:  %s\n", challenge.TXTHost)
		fmt.Printf("  Type:  TXT\n")
		fmt.Printf("  Value: %s\n\n", challenge.TXTRecord)
		fmt.Printf("Expires: %s\n\n", challenge.ExpiresAt.Format(time.RFC3339))
		fmt.Printf("When published, run:\n  nap dns-challenge verify %s\n", challenge.ID)
		return nil
	},
}

var dnsVerifyCmd = &cobra.Command{
	Use:   "verify <challenge-id>",
	Short: "Verify a DNS-01 challenge by triggering a TXT record lookup",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		challengeID := args[0]
		c, err := client.New(registryURL)
		if err != nil {
			return err
		}

		fmt.Printf("Verifying challenge %s...\n", challengeID)
		ok, err := c.VerifyDNSChallenge(context.Background(), challengeID)
		if err != nil {
			if errors.Is(err, client.ErrVerificationPending) {
				fmt.Println("DNS record not yet visible. Check propagation and retry.")
				return nil
			}
			return fmt.Errorf("verify: %w", err)
		}
		if ok {
			fmt.Println("✓ Domain ownership verified")
			fmt.Println("You may now activate agents registered under this domain.")
		}
		return nil
	},
}

var dnsStatusCmd = &cobra.Command{
	Use:   "status <challenge-id>",
	Short: "Show the current state of a DNS-01 challenge",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		challengeID := args[0]
		c, err := client.New(registryURL)
		if err != nil {
			return err
		}

		// GET /api/v1/dns/challenge/:id
		// Use the underlying HTTP client via a simple approach: try to verify
		// idempotently and check the status from the GET endpoint.
		body, err := c.GetDNSChallenge(context.Background(), challengeID)
		if err != nil {
			return fmt.Errorf("get challenge: %w", err)
		}

		out, _ := json.MarshalIndent(body, "", "  ")
		fmt.Println(string(out))
		return nil
	},
}

func init() {
	dnsChallengeCmd.AddCommand(dnsStartCmd)
	dnsChallengeCmd.AddCommand(dnsVerifyCmd)
	dnsChallengeCmd.AddCommand(dnsStatusCmd)
}
