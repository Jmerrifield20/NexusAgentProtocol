// Package mcpmanifest defines types for MCP (Model Context Protocol) manifests.
//
// Agents may declare their MCP tool and resource definitions at registration time;
// the registry generates a manifest served at a stable URL:
//
//	GET /api/v1/agents/:id/mcp-manifest.json
//
// NAP extension fields (prefixed "nap:") are ignored by plain MCP clients.
package mcpmanifest

import "encoding/json"

// MCPTool describes a tool an MCP-compatible agent exposes.
type MCPTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"` // JSON Schema object
}

// MCPResource describes a resource an MCP-compatible agent exposes.
type MCPResource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MimeType    string `json:"mimeType,omitempty"`
}

// MCPManifest is the MCP server manifest for an agent.
// See https://spec.modelcontextprotocol.io/specification/ for the base schema.
type MCPManifest struct {
	SchemaVersion string        `json:"schemaVersion"` // "2024-11-05"
	Name          string        `json:"name"`
	Version       string        `json:"version"`
	Description   string        `json:"description"`
	Tools         []MCPTool     `json:"tools,omitempty"`
	Resources     []MCPResource `json:"resources,omitempty"`

	// NAP extension fields — ignored by plain MCP clients per JSON extensibility rules.
	NAPURI       string `json:"nap:uri"`
	NAPTrustTier string `json:"nap:trustTier"`
	NAPRegistry  string `json:"nap:registry"`
}
