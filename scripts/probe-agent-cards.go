//go:build ignore

// probe-agent-cards.go checks a list of domains for /.well-known/agent-card.json
// and related agent discovery endpoints.
//
// Run with: go run scripts/probe-agent-cards.go
package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// Domains to probe — AI companies, agent platforms, major tech, and early NAP targets.
var domains = []string{
	// AI labs & model providers
	"openai.com", "anthropic.com", "google.com", "microsoft.com",
	"mistral.ai", "cohere.com", "ai21.com", "aleph-alpha.com",
	"inflection.ai", "xai.com", "meta.com", "deepmind.google",
	"stability.ai", "huggingface.co", "replicate.com",

	// AI agent platforms & frameworks
	"langchain.com", "crewai.com", "agentops.ai", "fixie.ai",
	"adept.ai", "cognitivecompute.com", "autogen.microsoft.com",
	"dust.tt", "superagent.sh", "relevanceai.com",
	"taskade.com", "gumloop.com", "lindy.ai",

	// API / developer infrastructure
	"stripe.com", "twilio.com", "sendgrid.com", "braintree.com",
	"plaid.com", "ocrolus.com", "finix.com",

	// Commerce & retail
	"shopify.com", "bigcommerce.com", "woocommerce.com",

	// Enterprise SaaS
	"salesforce.com", "hubspot.com", "zendesk.com",
	"atlassian.com", "notion.so", "airtable.com",
	"zapier.com", "make.com", "n8n.io",

	// Cloud providers
	"aws.amazon.com", "cloud.google.com", "azure.microsoft.com",

	// Emerging agent-native startups (2025/2026)
	"perplexity.ai", "you.com", "phind.com", "poe.com",
	"character.ai", "inflection.ai", "imbue.com",
	"aisera.com", "moveworks.com", "cognigy.com",

	// Well-known .well-known implementors (for baseline)
	"cloudflare.com", "github.com", "gitlab.com",
}

// Also probe these alternative discovery paths used by competing protocols.
var altPaths = []string{
	"/.well-known/agent-card.json",  // NAP (ours)
	"/.well-known/ai-plugin.json",   // OpenAI plugin manifest (legacy)
	"/.well-known/mcp.json",         // Potential MCP discovery
	"/.well-known/agents.json",      // Generic alternative
	"/agent-card.json",              // Root-level fallback
}

type result struct {
	domain   string
	path     string
	status   int
	bodySnip string // first 200 chars
	err      string
	latency  time.Duration
}

func probe(domain, path string, client *http.Client) result {
	url := "https://" + domain + path
	start := time.Now()

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return result{domain: domain, path: path, err: err.Error()}
	}
	req.Header.Set("User-Agent", "NAP-Probe/0.1 (agent-card discovery; +https://nexusagentprotocol.com)")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	latency := time.Since(start)
	if err != nil {
		// Simplify network errors for display
		msg := err.Error()
		if len(msg) > 60 {
			msg = msg[:60] + "..."
		}
		return result{domain: domain, path: path, err: msg, latency: latency}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	snip := strings.TrimSpace(string(body))
	if len(snip) > 200 {
		snip = snip[:200] + "…"
	}

	return result{
		domain:   domain,
		path:     path,
		status:   resp.StatusCode,
		bodySnip: snip,
		latency:  latency,
	}
}

func isJSON(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[")
}

func main() {
	httpClient := &http.Client{
		Timeout: 8 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: false}, //nolint:gosec
			MaxIdleConnsPerHost: 4,
			DisableKeepAlives:   false,
		},
	}

	type job struct {
		domain, path string
	}

	jobs := make(chan job, len(domains)*len(altPaths))
	results := make(chan result, len(domains)*len(altPaths))

	// Worker pool — 20 concurrent probes
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				results <- probe(j.domain, j.path, httpClient)
			}
		}()
	}

	total := 0
	for _, d := range domains {
		for _, p := range altPaths {
			jobs <- job{d, p}
			total++
		}
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect
	var hits []result
	var jsonHits []result
	checked := 0
	for r := range results {
		checked++
		fmt.Printf("\r  probing... %d/%d", checked, total)

		if r.status == 200 {
			hits = append(hits, r)
			if isJSON(r.bodySnip) {
				jsonHits = append(jsonHits, r)
			}
		}
	}
	fmt.Printf("\r  done — %d endpoints probed\n\n", total)

	// Sort hits by domain for readability
	sort.Slice(hits, func(i, j int) bool {
		return hits[i].domain < hits[j].domain
	})

	// ── Report ────────────────────────────────────────────────────────────────
	fmt.Printf("══════════════════════════════════════════════════════\n")
	fmt.Printf("  NAP / Agent Discovery Probe Results\n")
	fmt.Printf("  Domains checked: %d  |  Paths per domain: %d\n", len(domains), len(altPaths))
	fmt.Printf("══════════════════════════════════════════════════════\n\n")

	if len(hits) == 0 {
		fmt.Println("  No 200 responses found on any discovery path.")
		fmt.Println("  The agent-card.json standard is not yet adopted in the wild.")
		fmt.Println("  NAP has first-mover advantage on this discovery mechanism.")
		return
	}

	fmt.Printf("  200 OK responses: %d\n", len(hits))
	fmt.Printf("  JSON responses:   %d\n\n", len(jsonHits))

	if len(jsonHits) > 0 {
		fmt.Println("── JSON hits (most likely intentional agent cards) ──")
		for _, r := range jsonHits {
			fmt.Printf("\n  ✦ https://%s%s  (%dms)\n", r.domain, r.path, r.latency.Milliseconds())
			// Try to pretty-print the JSON
			var v any
			if err := json.Unmarshal([]byte(r.bodySnip), &v); err == nil {
				b, _ := json.MarshalIndent(v, "    ", "  ")
				fmt.Printf("    %s\n", string(b))
			} else {
				fmt.Printf("    %s\n", r.bodySnip)
			}
		}
		fmt.Println()
	}

	nonJSON := []result{}
	for _, r := range hits {
		if !isJSON(r.bodySnip) {
			nonJSON = append(nonJSON, r)
		}
	}
	if len(nonJSON) > 0 {
		fmt.Println("── 200 OK but non-JSON (HTML/redirect/placeholder) ──")
		for _, r := range nonJSON {
			fmt.Printf("  • https://%s%s  (%dms)\n", r.domain, r.path, r.latency.Milliseconds())
		}
		fmt.Println()
	}

	fmt.Println("══════════════════════════════════════════════════════")
}
