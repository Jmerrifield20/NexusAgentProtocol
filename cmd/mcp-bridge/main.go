// nap-mcp-bridge exposes the Nexus Agentic Protocol as MCP tools, allowing
// Claude Desktop and any MCP-compatible AI host to discover and call NAP agents.
//
// Add to Claude Desktop (~/.claude/claude_desktop_config.json):
//
//	{
//	  "mcpServers": {
//	    "nap": {
//	      "command": "/path/to/nap-mcp-bridge",
//	      "args": ["--registry", "https://registry.nexus.io"]
//	    }
//	  }
//	}
//
// To also enable agent-calling (requires a cert from 'nap claim'):
//
//	"args": [
//	  "--registry", "https://registry.nexus.io",
//	  "--cert-dir", "/Users/you/.nap/certs/example.com"
//	]
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nexus-protocol/nexus/internal/mcpbridge"
	"github.com/nexus-protocol/nexus/pkg/client"
	"github.com/spf13/cobra"
)

var (
	registryURL  string
	certDir      string
	insecure     bool
	cacheTTLSec  int
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "nap-mcp-bridge",
	Short: "MCP bridge for the Nexus Agentic Protocol",
	Long: `nap-mcp-bridge is a stdio MCP server that exposes four NAP tools to any
MCP-compatible AI host (Claude Desktop, Claude API, etc.):

  resolve_agent    — translate an agent:// URI to its transport endpoint
  list_agents      — search the registry by capability
  fetch_agent_card — read a domain's /.well-known/agent-card.json
  call_agent       — resolve + authenticate + call an agent endpoint

The bridge runs in stdio mode (the MCP standard for local servers).
All logging goes to stderr so it does not interfere with the protocol.`,
	SilenceUsage: true,
	RunE:         run,
}

func init() {
	rootCmd.Flags().StringVar(&registryURL, "registry", "https://registry.nexus.io", "Nexus registry URL")
	rootCmd.Flags().StringVar(&certDir, "cert-dir", "", "Certificate directory written by 'nap claim' (enables mTLS + agent calling)")
	rootCmd.Flags().BoolVar(&insecure, "insecure", false, "Skip TLS certificate verification (development only)")
	rootCmd.Flags().IntVar(&cacheTTLSec, "cache-ttl", 60, "URI resolution cache TTL in seconds (0 = disabled)")
}

func run(cmd *cobra.Command, _ []string) error {
	logger := log.New(os.Stderr, "[nap-mcp] ", log.LstdFlags)

	// Build NAP client options.
	opts := []client.Option{}

	if certDir != "" {
		opts = append(opts, client.WithCertDir(certDir))
		logger.Printf("mTLS enabled (cert-dir: %s)", certDir)
	} else {
		logger.Printf("no --cert-dir provided; call_agent tool will be unavailable")
	}

	if insecure {
		opts = append(opts, client.WithInsecureSkipVerify())
		logger.Printf("WARNING: TLS verification disabled — do not use in production")
	}

	if cacheTTLSec > 0 {
		opts = append(opts, client.WithCacheTTL(time.Duration(cacheTTLSec)*time.Second))
		logger.Printf("resolution cache TTL: %ds", cacheTTLSec)
	}

	c, err := client.New(registryURL, opts...)
	if err != nil {
		return fmt.Errorf("create NAP client: %w", err)
	}

	tools := mcpbridge.NewToolRegistry(c)
	server := mcpbridge.NewServer(os.Stdout, tools, logger)

	logger.Printf("NAP MCP bridge ready — registry: %s", registryURL)
	logger.Printf("tools: resolve_agent, list_agents, fetch_agent_card, call_agent")

	return server.Serve(cmd.Context(), os.Stdin)
}
