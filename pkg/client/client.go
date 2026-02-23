// Package client provides the Nexus Go SDK for resolving agent:// URIs
// and communicating with the Nexus registry and resolver services.
package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jmerrifield20/NexusAgentProtocol/pkg/uri"
)

// ErrVerificationPending is returned by VerifyDNSChallenge when the DNS TXT
// record has not yet been published or propagated.
var ErrVerificationPending = errors.New("DNS TXT record not yet published or propagated")

// ResolveResult contains the endpoint information returned by a resolve call.
type ResolveResult struct {
	ID         string `json:"id,omitempty"`
	URI        string `json:"uri"`
	Endpoint   string `json:"endpoint"`
	Status     string `json:"status"`
	CertSerial string `json:"cert_serial,omitempty"`
}

// DNSChallengeResult holds the TXT record details returned by StartDNSChallenge.
type DNSChallengeResult struct {
	ID        string    `json:"id"`
	Domain    string    `json:"domain"`
	TXTHost   string    `json:"txt_host"`
	TXTRecord string    `json:"txt_record"`
	ExpiresAt time.Time `json:"expires_at"`
}

// RegisterAgentRequest is the payload for RegisterAgent.
type RegisterAgentRequest struct {
	TrustRoot      string            `json:"trust_root"`
	CapabilityNode string            `json:"capability_node"`
	DisplayName    string            `json:"display_name"`
	Description    string            `json:"description,omitempty"`
	Endpoint       string            `json:"endpoint"`
	OwnerDomain    string            `json:"owner_domain"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// AgentResult holds the ID and URI of a newly registered agent.
type AgentResult struct {
	ID  string `json:"id"`
	URI string `json:"uri"`
}

// ActivateResult holds the cert bundle returned by ActivateAgent.
type ActivateResult struct {
	URI           string `json:"uri"`
	CertPEM       string `json:"cert_pem"`
	PrivateKeyPEM string `json:"private_key_pem"`
	CAPEM         string `json:"ca_pem"`
}

// AgentDetail holds the full agent record returned by GET /api/v1/agents/:id.
type AgentDetail struct {
	ID             string            `json:"id"`
	TrustRoot      string            `json:"trust_root"`
	CapabilityNode string            `json:"capability_node"`
	AgentID        string            `json:"agent_id"`
	DisplayName    string            `json:"display_name"`
	Description    string            `json:"description,omitempty"`
	Endpoint       string            `json:"endpoint"`
	OwnerDomain    string            `json:"owner_domain"`
	Status         string            `json:"status"`
	CertSerial     string            `json:"cert_serial,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// Client is the Nexus SDK entry point.
type Client struct {
	registryBase string
	httpClient   *http.Client
	cache        *resolverCache

	// token state — guarded by mu
	mu          sync.Mutex
	bearerToken string
	tokenExpiry time.Time // zero = token was set manually (no auto-refresh)
}

// Option is a functional option for configuring a Client.
type Option func(*Client) error

// WithHTTPClient sets a custom http.Client, overriding any TLS options.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) error {
		c.httpClient = hc
		return nil
	}
}

// WithCacheTTL enables in-memory result caching with the given TTL.
func WithCacheTTL(ttl time.Duration) Option {
	return func(c *Client) error {
		c.cache = newResolverCache(ttl)
		return nil
	}
}

// WithBearerToken attaches a pre-obtained Task Token to every request.
// The token is treated as long-lived and will not be auto-refreshed.
func WithBearerToken(token string) Option {
	return func(c *Client) error {
		c.bearerToken = token
		c.tokenExpiry = time.Time{} // zero = manual, never auto-refresh
		return nil
	}
}

// WithMTLS configures the client for mutual TLS authentication using the
// provided PEM-encoded client certificate, private key, and CA certificate.
//
//	certPEM — the agent's X.509 certificate (from the activate response)
//	keyPEM  — the agent's RSA private key (from the activate response)
//	caPEM   — the Nexus CA certificate (download from GET /api/v1/ca.crt)
func WithMTLS(certPEM, keyPEM, caPEM string) Option {
	return func(c *Client) error {
		clientCert, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
		if err != nil {
			return fmt.Errorf("parse mTLS cert/key: %w", err)
		}

		pool := x509.NewCertPool()
		if caPEM != "" {
			if !pool.AppendCertsFromPEM([]byte(caPEM)) {
				return fmt.Errorf("failed to parse CA certificate PEM")
			}
		}

		tlsCfg := &tls.Config{
			Certificates: []tls.Certificate{clientCert},
			RootCAs:      pool,
			MinVersion:   tls.VersionTLS13,
		}

		c.httpClient = &http.Client{
			Transport: &http.Transport{TLSClientConfig: tlsCfg},
			Timeout:   10 * time.Second,
		}
		return nil
	}
}

// WithInsecureSkipVerify disables TLS certificate verification.
// Only use this in development against a locally-generated CA.
func WithInsecureSkipVerify() Option {
	return func(c *Client) error {
		c.httpClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
			},
			Timeout: 10 * time.Second,
		}
		return nil
	}
}

// New creates a new Nexus SDK Client connected to registryBase.
//
//	c, err := client.New("https://localhost:8443",
//	    client.WithMTLS(certPEM, keyPEM, caPEM),
//	    client.WithCacheTTL(60*time.Second),
//	)
func New(registryBase string, opts ...Option) (*Client, error) {
	c := &Client{
		registryBase: registryBase,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
	}
	for _, o := range opts {
		if err := o(c); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// MustNew is like New but panics on error. Useful in tests and program init.
func MustNew(registryBase string, opts ...Option) *Client {
	c, err := New(registryBase, opts...)
	if err != nil {
		panic(err)
	}
	return c
}

// Resolve translates an agent:// URI string into its transport endpoint.
func (c *Client) Resolve(ctx context.Context, agentURI string) (*ResolveResult, error) {
	parsed, err := uri.Parse(agentURI)
	if err != nil {
		return nil, fmt.Errorf("parse URI: %w", err)
	}

	if c.cache != nil {
		if result, ok := c.cache.get(agentURI); ok {
			return result, nil
		}
	}

	result, err := c.resolve(ctx, parsed)
	if err != nil {
		return nil, err
	}

	if c.cache != nil {
		c.cache.set(agentURI, result)
	}
	return result, nil
}

// FetchToken exchanges the client's mTLS certificate for a JWT Task Token,
// caches it, and returns it. Requires WithMTLS or WithCertDir.
// Subsequent calls reuse the cached token until it approaches expiry.
func (c *Client) FetchToken(ctx context.Context) (string, error) {
	token, expiry, err := c.fetchTokenRaw(ctx)
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	c.bearerToken = token
	c.tokenExpiry = expiry
	c.mu.Unlock()
	return token, nil
}

// fetchTokenRaw fetches a fresh token from the registry without touching
// cached state. It uses the raw httpClient (not c.do) so it does not
// attach any existing bearer token to the token-exchange request.
func (c *Client) fetchTokenRaw(ctx context.Context) (token string, expiry time.Time, err error) {
	url := c.registryBase + "/api/v1/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	// Use httpClient directly — the token endpoint authenticates via mTLS,
	// not via an existing Bearer token.
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("read token response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return "", time.Time{}, fmt.Errorf("token endpoint error %d: %s", resp.StatusCode, string(body))
	}

	var payload struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", time.Time{}, fmt.Errorf("decode token response: %w", err)
	}
	if payload.Error != "" {
		return "", time.Time{}, fmt.Errorf("token endpoint error: %s", payload.Error)
	}

	// Refresh 60 s before actual expiry to avoid clock-skew failures.
	const refreshBuffer = 60 * time.Second
	exp := time.Now().Add(time.Duration(payload.ExpiresIn)*time.Second - refreshBuffer)
	return payload.AccessToken, exp, nil
}

// ensureToken returns a valid bearer token, fetching a new one if the cached
// token is absent or approaching expiry. Thread-safe.
func (c *Client) ensureToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// tokenExpiry.IsZero() means the token was set manually via WithBearerToken
	// and should never be auto-refreshed.
	if c.bearerToken != "" && (c.tokenExpiry.IsZero() || time.Now().Before(c.tokenExpiry)) {
		return c.bearerToken, nil
	}

	token, expiry, err := c.fetchTokenRaw(ctx)
	if err != nil {
		return "", err
	}
	c.bearerToken = token
	c.tokenExpiry = expiry
	return token, nil
}

// CallAgent resolves an agent:// URI, obtains a Task Token (auto-refreshed),
// and makes an authenticated HTTP call to the agent's endpoint.
//
//	var reply InvoiceResponse
//	err := c.CallAgent(ctx,
//	    "agent://nexusagentprotocol.com/finance/billing/agent_7x2v9q",
//	    http.MethodPost, "/v1/invoice",
//	    &InvoiceRequest{Amount: 100},
//	    &reply,
//	)
//
// reqBody and respBody are JSON-encoded/decoded automatically. Pass nil for
// either when not needed (e.g. GET requests or when the response is ignored).
func (c *Client) CallAgent(ctx context.Context, agentURI, method, path string, reqBody, respBody any) error {
	// 1. Resolve the agent URI to its current transport endpoint.
	resolved, err := c.Resolve(ctx, agentURI)
	if err != nil {
		return fmt.Errorf("resolve %q: %w", agentURI, err)
	}

	// 2. Obtain a valid Task Token (fetched/refreshed automatically).
	token, err := c.ensureToken(ctx)
	if err != nil {
		return fmt.Errorf("obtain task token: %w", err)
	}

	// 3. Build the HTTP request to the agent endpoint.
	target := strings.TrimRight(resolved.Endpoint, "/")
	if path != "" {
		target += "/" + strings.TrimLeft(path, "/")
	}

	var bodyReader io.Reader
	if reqBody != nil {
		b, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, target, bodyReader)
	if err != nil {
		return fmt.Errorf("build agent request: %w", err)
	}
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	// 4. Execute against the agent (not the registry — use httpClient directly).
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call agent: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB limit
	if err != nil {
		return fmt.Errorf("read agent response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("agent returned HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	if respBody != nil && len(respBytes) > 0 {
		if err := json.Unmarshal(respBytes, respBody); err != nil {
			return fmt.Errorf("decode agent response: %w", err)
		}
	}
	return nil
}

// ResolveViaService translates an agent:// URI using the dedicated resolver
// service's grpc-gateway REST endpoint (GET /v1/resolve).
//
// resolverBase is the base URL of the resolver, e.g. "http://localhost:9091".
// Using the resolver service is preferred in production: it maintains a warm
// endpoint cache and is decoupled from the registry database.
func (c *Client) ResolveViaService(ctx context.Context, resolverBase, agentURI string) (*ResolveResult, error) {
	parsed, err := uri.Parse(agentURI)
	if err != nil {
		return nil, fmt.Errorf("parse URI: %w", err)
	}

	if c.cache != nil {
		if result, ok := c.cache.get(agentURI); ok {
			return result, nil
		}
	}

	url := fmt.Sprintf("%s/v1/resolve?trust_root=%s&capability_node=%s&agent_id=%s",
		resolverBase, parsed.OrgName, parsed.Category, parsed.AgentID,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	body, err := c.do(req)
	if err != nil {
		return nil, err
	}

	var result ResolveResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if c.cache != nil {
		c.cache.set(agentURI, &result)
	}
	return &result, nil
}

// resolve performs the actual HTTP resolve call to the registry.
func (c *Client) resolve(ctx context.Context, u *uri.URI) (*ResolveResult, error) {
	url := fmt.Sprintf("%s/api/v1/resolve?trust_root=%s&capability_node=%s&agent_id=%s",
		c.registryBase, u.OrgName, u.Category, u.AgentID,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	body, err := c.do(req)
	if err != nil {
		return nil, err
	}

	var result ResolveResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// GetDNSChallenge fetches the current state of a DNS challenge by ID.
func (c *Client) GetDNSChallenge(ctx context.Context, challengeID string) (map[string]any, error) {
	url := c.registryBase + "/api/v1/dns/challenge/" + challengeID
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	body, err := c.do(req)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result, nil
}

// StartDNSChallenge posts to /api/v1/dns/challenge and returns the TXT record
// that the caller must publish to prove domain ownership.
func (c *Client) StartDNSChallenge(ctx context.Context, domain string) (*DNSChallengeResult, error) {
	payload, _ := json.Marshal(map[string]string{"domain": domain})
	url := c.registryBase + "/api/v1/dns/challenge"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	body, err := c.do(req)
	if err != nil {
		return nil, err
	}

	var result DNSChallengeResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode challenge response: %w", err)
	}
	return &result, nil
}

// VerifyDNSChallenge posts to /api/v1/dns/challenge/:id/verify.
// Returns (true, nil) on success, (false, ErrVerificationPending) when the DNS
// record has not yet propagated, or (false, err) for other errors.
func (c *Client) VerifyDNSChallenge(ctx context.Context, challengeID string) (bool, error) {
	url := c.registryBase + "/api/v1/dns/challenge/" + challengeID + "/verify"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return false, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	status, body, err := c.doStatusBody(req)
	if err != nil {
		return false, err
	}

	switch status {
	case http.StatusOK:
		return true, nil
	case http.StatusUnprocessableEntity:
		return false, ErrVerificationPending
	case http.StatusGone:
		return false, fmt.Errorf("challenge expired")
	case http.StatusNotFound:
		return false, fmt.Errorf("challenge not found")
	default:
		return false, fmt.Errorf("server error %d: %s", status, string(body))
	}
}

// RegisterAgent posts to /api/v1/agents and returns the new agent's ID and URI.
func (c *Client) RegisterAgent(ctx context.Context, reg RegisterAgentRequest) (*AgentResult, error) {
	payload, err := json.Marshal(reg)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	url := c.registryBase + "/api/v1/agents"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	body, err := c.do(req)
	if err != nil {
		return nil, err
	}

	// Registry returns the full agent model; extract the fields we need.
	var agent struct {
		ID             string `json:"id"`
		TrustRoot      string `json:"trust_root"`
		CapabilityNode string `json:"capability_node"`
		AgentID        string `json:"agent_id"`
	}
	if err := json.Unmarshal(body, &agent); err != nil {
		return nil, fmt.Errorf("decode agent response: %w", err)
	}

	agentURI := "agent://" + agent.TrustRoot + "/" + agent.CapabilityNode + "/" + agent.AgentID
	return &AgentResult{ID: agent.ID, URI: agentURI}, nil
}

// ActivateAgent posts to /api/v1/agents/:id/activate and returns the cert bundle.
// The returned PrivateKeyPEM is delivered once — it is not persisted by the registry.
func (c *Client) ActivateAgent(ctx context.Context, agentID string) (*ActivateResult, error) {
	url := c.registryBase + "/api/v1/agents/" + agentID + "/activate"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	body, err := c.do(req)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Agent struct {
			TrustRoot      string `json:"trust_root"`
			CapabilityNode string `json:"capability_node"`
			AgentID        string `json:"agent_id"`
		} `json:"agent"`
		Certificate struct {
			PEM string `json:"pem"`
		} `json:"certificate"`
		PrivateKeyPEM string `json:"private_key_pem"`
		CAPEM         string `json:"ca_pem"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode activate response: %w", err)
	}

	agentURI := "agent://" + resp.Agent.TrustRoot + "/" + resp.Agent.CapabilityNode + "/" + resp.Agent.AgentID
	return &ActivateResult{
		URI:           agentURI,
		CertPEM:       resp.Certificate.PEM,
		PrivateKeyPEM: resp.PrivateKeyPEM,
		CAPEM:         resp.CAPEM,
	}, nil
}

// GetAgent fetches a single agent record by its UUID from GET /api/v1/agents/:id.
func (c *Client) GetAgent(ctx context.Context, id string) (*AgentDetail, error) {
	url := c.registryBase + "/api/v1/agents/" + id
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	body, err := c.do(req)
	if err != nil {
		return nil, err
	}

	var agent AgentDetail
	if err := json.Unmarshal(body, &agent); err != nil {
		return nil, fmt.Errorf("decode agent response: %w", err)
	}
	return &agent, nil
}

// ListAgents returns registered agents, optionally filtered by trust root and
// capability node. Pass empty strings to list all agents.
func (c *Client) ListAgents(ctx context.Context, trustRoot, capabilityNode string) ([]AgentDetail, error) {
	endpoint := fmt.Sprintf("%s/api/v1/agents?trust_root=%s&capability_node=%s",
		c.registryBase, trustRoot, capabilityNode)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	body, err := c.do(req)
	if err != nil {
		return nil, err
	}

	var wrapper struct {
		Agents []AgentDetail `json:"agents"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return wrapper.Agents, nil
}

// CallAgentRaw is like CallAgent but accepts and returns raw JSON bytes.
// Pass nil body for requests with no body (e.g. GET).
func (c *Client) CallAgentRaw(ctx context.Context, agentURI, method, path string, body json.RawMessage) (json.RawMessage, error) {
	resolved, err := c.Resolve(ctx, agentURI)
	if err != nil {
		return nil, fmt.Errorf("resolve %q: %w", agentURI, err)
	}

	token, err := c.ensureToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("obtain task token: %w", err)
	}

	target := strings.TrimRight(resolved.Endpoint, "/")
	if path != "" {
		target += "/" + strings.TrimLeft(path, "/")
	}

	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, target, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("build agent request: %w", err)
	}
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call agent: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read agent response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("agent returned HTTP %d: %s", resp.StatusCode, string(respBytes))
	}
	return respBytes, nil
}

// RevokeAgent posts to POST /api/v1/agents/:id/revoke using the client's Bearer token.
// The client must have been configured with WithBearerToken before calling this.
func (c *Client) RevokeAgent(ctx context.Context, id string) error {
	url := c.registryBase + "/api/v1/agents/" + id + "/revoke"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	_, err = c.do(req)
	return err
}

// do executes an HTTP request, attaching the Bearer token if present.
func (c *Client) do(req *http.Request) ([]byte, error) {
	if c.bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearerToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("not found: %s", req.URL.Path)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("unauthorized: %s", string(body))
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("server error %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// doStatusBody is a lower-level HTTP call that returns (statusCode, body, error)
// without failing on 4xx responses. The caller interprets the status code.
func (c *Client) doStatusBody(req *http.Request) (int, []byte, error) {
	if c.bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearerToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read response: %w", err)
	}
	return resp.StatusCode, body, nil
}

// --- simple in-memory resolver cache ---

type cacheEntry struct {
	result    *ResolveResult
	expiresAt time.Time
}

type resolverCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	ttl     time.Duration
}

func newResolverCache(ttl time.Duration) *resolverCache {
	return &resolverCache{entries: make(map[string]*cacheEntry), ttl: ttl}
}

func (rc *resolverCache) get(key string) (*ResolveResult, bool) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	e, ok := rc.entries[key]
	if !ok || time.Now().After(e.expiresAt) {
		return nil, false
	}
	return e.result, true
}

func (rc *resolverCache) set(key string, result *ResolveResult) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.entries[key] = &cacheEntry{result: result, expiresAt: time.Now().Add(rc.ttl)}
}
