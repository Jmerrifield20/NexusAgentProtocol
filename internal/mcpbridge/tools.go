package mcpbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nexus-protocol/nexus/pkg/agentcard"
	"github.com/nexus-protocol/nexus/pkg/client"
)

// ToolDefinition is the MCP tool descriptor sent in tools/list responses.
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// toolResult holds the output of a tool call.
type toolResult struct {
	text  string
	isErr bool
}

func ok(text string) (string, bool)  { return text, false }
func fail(text string) (string, bool) { return text, true }
func failf(format string, a ...any) (string, bool) {
	return fmt.Sprintf(format, a...), true
}

// ToolRegistry holds the NAP client and the definitions/handlers for all tools.
type ToolRegistry struct {
	c    *client.Client
	defs []ToolDefinition
}

// NewToolRegistry creates a ToolRegistry backed by the given NAP client.
func NewToolRegistry(c *client.Client) *ToolRegistry {
	r := &ToolRegistry{c: c}
	r.defs = []ToolDefinition{
		{
			Name: "resolve_agent",
			Description: "Resolve an agent:// URI to its current transport endpoint. " +
				"Use this to discover where a specific NAP agent is running before calling it.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"uri": map[string]any{
						"type":        "string",
						"description": "The agent:// URI to resolve, e.g. agent://nexus.io/finance/billing/agent_abc",
					},
				},
				"required": []string{"uri"},
			},
		},
		{
			Name: "list_agents",
			Description: "List agents registered in the Nexus registry. " +
				"Filter by trust_root and/or capability_node to find agents for a specific domain or capability. " +
				"Returns agent URIs, endpoints, display names, and status.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"trust_root": map[string]any{
						"type":        "string",
						"description": "Registry hostname to filter by (e.g. nexus.io). Leave empty for all.",
					},
					"capability_node": map[string]any{
						"type":        "string",
						"description": "Capability path prefix to filter by (e.g. finance or ecommerce/retail). Leave empty for all.",
					},
				},
			},
		},
		{
			Name: "fetch_agent_card",
			Description: "Fetch the /.well-known/agent-card.json from a domain, listing all NAP agents " +
				"that domain has registered. Use this to discover what agents a specific company exposes.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"domain": map[string]any{
						"type":        "string",
						"description": "The domain to fetch the agent card from, e.g. example.com",
					},
				},
				"required": []string{"domain"},
			},
		},
		{
			Name: "call_agent",
			Description: "Call a NAP agent endpoint directly. Resolves the agent:// URI, obtains a scoped " +
				"Task Token, and makes an authenticated HTTP request to the agent. " +
				"Requires the bridge to be running with --cert-dir (your agent identity). " +
				"Use this to transact with other agents: place orders, query live data, trigger actions.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"uri": map[string]any{
						"type":        "string",
						"description": "The agent:// URI of the agent to call",
					},
					"method": map[string]any{
						"type":        "string",
						"description": "HTTP method. Defaults to GET.",
						"enum":        []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
					},
					"path": map[string]any{
						"type":        "string",
						"description": "Path to append to the agent's endpoint, e.g. /v1/invoice",
					},
					"body": map[string]any{
						"type":        "string",
						"description": "JSON request body as a string, for POST/PUT/PATCH requests",
					},
				},
				"required": []string{"uri"},
			},
		},
	}
	return r
}

// Definitions returns the list of tool definitions for tools/list responses.
func (r *ToolRegistry) Definitions() []ToolDefinition {
	return r.defs
}

// Call dispatches a tool call by name and returns (output text, isError).
func (r *ToolRegistry) Call(ctx context.Context, name string, args json.RawMessage) (string, bool) {
	switch name {
	case "resolve_agent":
		return r.resolveAgent(ctx, args)
	case "list_agents":
		return r.listAgents(ctx, args)
	case "fetch_agent_card":
		return r.fetchAgentCard(args)
	case "call_agent":
		return r.callAgent(ctx, args)
	default:
		return failf("unknown tool: %q", name)
	}
}

// ── tool handlers ────────────────────────────────────────────────────────────

func (r *ToolRegistry) resolveAgent(ctx context.Context, args json.RawMessage) (string, bool) {
	var in struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(args, &in); err != nil || in.URI == "" {
		return fail("uri is required")
	}

	result, err := r.c.Resolve(ctx, in.URI)
	if err != nil {
		return failf("resolve failed: %v", err)
	}

	out, _ := json.MarshalIndent(result, "", "  ")
	return ok(string(out))
}

func (r *ToolRegistry) listAgents(ctx context.Context, args json.RawMessage) (string, bool) {
	var in struct {
		TrustRoot      string `json:"trust_root"`
		CapabilityNode string `json:"capability_node"`
	}
	_ = json.Unmarshal(args, &in)

	agents, err := r.c.ListAgents(ctx, in.TrustRoot, in.CapabilityNode)
	if err != nil {
		return failf("list agents failed: %v", err)
	}
	if len(agents) == 0 {
		return ok("No agents found matching the given filters.")
	}

	out, _ := json.MarshalIndent(agents, "", "  ")
	return ok(string(out))
}

func (r *ToolRegistry) fetchAgentCard(args json.RawMessage) (string, bool) {
	var in struct {
		Domain string `json:"domain"`
	}
	if err := json.Unmarshal(args, &in); err != nil || in.Domain == "" {
		return fail("domain is required")
	}

	card, err := agentcard.Fetch(in.Domain)
	if err != nil {
		return failf("fetch agent card failed: %v", err)
	}

	out, _ := json.MarshalIndent(card, "", "  ")
	return ok(string(out))
}

func (r *ToolRegistry) callAgent(ctx context.Context, args json.RawMessage) (string, bool) {
	var in struct {
		URI    string `json:"uri"`
		Method string `json:"method"`
		Path   string `json:"path"`
		Body   string `json:"body"` // JSON string provided by the model
	}
	if err := json.Unmarshal(args, &in); err != nil || in.URI == "" {
		return fail("uri is required")
	}
	if in.Method == "" {
		in.Method = http.MethodGet
	}

	// Convert the body string (if provided) into raw JSON bytes.
	var rawBody json.RawMessage
	if in.Body != "" {
		rawBody = json.RawMessage(in.Body)
	}

	result, err := r.c.CallAgentRaw(ctx, in.URI, in.Method, in.Path, rawBody)
	if err != nil {
		return failf("call agent failed: %v", err)
	}

	// Pretty-print the JSON response for readability.
	var v any
	if jsonErr := json.Unmarshal(result, &v); jsonErr == nil {
		pretty, _ := json.MarshalIndent(v, "", "  ")
		return ok(string(pretty))
	}
	return ok(string(result))
}
