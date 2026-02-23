package federation

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// remoteAgentResponse is the subset of the agent API response needed for resolution.
type remoteAgentResponse struct {
	AgentID        string `json:"agent_id"`
	TrustRoot      string `json:"trust_root"`
	CapabilityNode string `json:"capability_node"`
	DisplayName    string `json:"display_name"`
	Description    string `json:"description"`
	Endpoint       string `json:"endpoint"`
	Status         string `json:"status"`
	Version        string `json:"version"`
}

// RegistryClient is a lightweight HTTP client for querying a remote NAP registry.
type RegistryClient struct {
	baseURL string
	http    *http.Client
}

// NewRegistryClient creates a RegistryClient targeting baseURL.
func NewRegistryClient(baseURL string, timeout time.Duration) *RegistryClient {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return &RegistryClient{
		baseURL: baseURL,
		http:    &http.Client{Timeout: timeout},
	}
}

// Resolve queries /api/v1/resolve on the remote registry and returns the agent JSON.
func (c *RegistryClient) Resolve(ctx context.Context, trustRoot, capNode, agentID string) (*remoteAgentResponse, error) {
	u, err := url.Parse(c.baseURL + "/api/v1/resolve")
	if err != nil {
		return nil, fmt.Errorf("build resolve URL: %w", err)
	}
	q := u.Query()
	q.Set("trust_root", trustRoot)
	q.Set("cap_node", capNode)
	q.Set("agent_id", agentID)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build resolve request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("resolve request to %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remote registry returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read resolve response: %w", err)
	}

	var agent remoteAgentResponse
	if err := json.Unmarshal(body, &agent); err != nil {
		return nil, fmt.Errorf("decode resolve response: %w", err)
	}
	return &agent, nil
}
