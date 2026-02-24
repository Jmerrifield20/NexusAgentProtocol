package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/identity"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/registry/model"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/registry/repository"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/registry/service"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/users"
	"github.com/jmerrifield20/NexusAgentProtocol/pkg/uri"
	"go.uber.org/zap"
)

// userLookup is the interface used by AgentHandler to look up agent owners.
type userLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (*users.User, error)
}

// AgentHandler handles HTTP requests for the agent registry.
type AgentHandler struct {
	svc        *service.AgentService
	tokens     *identity.TokenIssuer     // nil = no agent token auth enforcement
	userTokens *identity.UserTokenIssuer // nil = no user token support
	ownerSvc   userLookup               // nil = no owner attribution
	logger     *zap.Logger
}

// SetUserLookup configures the user lookup service used to attach owner info to GetAgent responses.
func (h *AgentHandler) SetUserLookup(ul userLookup) {
	h.ownerSvc = ul
}

// NewAgentHandler creates a new AgentHandler.
// tokens and userTokens may be nil to disable JWT auth on protected routes.
func NewAgentHandler(svc *service.AgentService, tokens *identity.TokenIssuer, logger *zap.Logger) *AgentHandler {
	return &AgentHandler{svc: svc, tokens: tokens, logger: logger}
}

// SetUserTokenIssuer configures user JWT support for ownership checks.
func (h *AgentHandler) SetUserTokenIssuer(ut *identity.UserTokenIssuer) {
	h.userTokens = ut
}

// requireToken returns the RequireToken middleware when agent auth is configured,
// or a no-op middleware for development/open mode.
func (h *AgentHandler) requireToken() gin.HandlerFunc {
	if h.tokens == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return identity.RequireToken(h.tokens)
}

// requireUserToken returns the RequireUserToken middleware when user auth is configured,
// or a no-op middleware when userTokens is nil.
func (h *AgentHandler) requireUserToken() gin.HandlerFunc {
	if h.userTokens == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return identity.RequireUserToken(h.userTokens)
}

// optionalAgentToken tries to parse an agent task JWT from the Authorization header
// and injects it into the context if present and valid. Never aborts.
func (h *AgentHandler) optionalAgentToken() gin.HandlerFunc {
	if h.tokens == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return identity.OptionalToken(h.tokens)
}

// optionalUserToken tries to parse a user JWT from the Authorization header
// and injects it into the context if present and valid. Never aborts.
func (h *AgentHandler) optionalUserToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.userTokens == nil {
			c.Next()
			return
		}
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.Next()
			return
		}
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		if claims, err := h.userTokens.Verify(tokenStr); err == nil {
			c.Set("nexus_user_claims", claims)
		}
		c.Next()
	}
}

// Register registers all agent routes on the given router group.
func (h *AgentHandler) Register(rg *gin.RouterGroup) {
	agents := rg.Group("/agents")
	{
		agents.POST("", h.optionalUserToken(), h.CreateAgent)
		agents.GET("", h.ListAgents)
		agents.GET("/:id", h.GetAgent)
		agents.GET("/:id/agent.json", h.GetAgentCard)
		agents.GET("/:id/mcp-manifest.json", h.GetMCPManifest)
		agents.PATCH("/:id", h.optionalAgentToken(), h.optionalUserToken(), h.UpdateAgent)
		agents.DELETE("/:id", h.optionalAgentToken(), h.optionalUserToken(), h.DeleteAgent)
		agents.POST("/:id/activate", h.optionalAgentToken(), h.optionalUserToken(), h.ActivateAgent)
		agents.POST("/:id/revoke", h.requireToken(), h.optionalUserToken(), h.RevokeAgent)
		agents.POST("/:id/suspend", h.optionalAgentToken(), h.optionalUserToken(), h.SuspendAgent)
		agents.POST("/:id/restore", h.optionalAgentToken(), h.optionalUserToken(), h.RestoreAgent)
		agents.POST("/:id/deprecate", h.optionalAgentToken(), h.optionalUserToken(), h.DeprecateAgent)
		agents.POST("/:id/report-abuse", h.requireUserToken(), h.ReportAbuseProxy)
	}

	rg.GET("/resolve", h.ResolveAgent)
	rg.POST("/resolve/batch", h.BatchResolve)
	rg.GET("/lookup", h.LookupByDomain)
	rg.GET("/capabilities", h.GetCapabilities)
	rg.GET("/crl", h.GetCRL)
	rg.GET("/users/me/agents", h.requireUserToken(), h.ListMyAgents)
}

// userFromCtx is a convenience wrapper around UserClaimsFromCtx.
func userFromCtx(c *gin.Context) *identity.UserTokenClaims {
	return identity.UserClaimsFromCtx(c)
}

// CreateAgent handles POST /agents — registers a new agent.
func (h *AgentHandler) CreateAgent(c *gin.Context) {
	var req model.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// If a user JWT is present and registration_type == nap_hosted, fill in owner info.
	if req.RegistrationType == model.RegistrationTypeNAPHosted {
		userClaims := userFromCtx(c)
		if userClaims == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "user authentication required for nap_hosted registration"})
			return
		}
		uid, err := uuid.Parse(userClaims.UserID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID in token"})
			return
		}
		req.OwnerUserID = &uid
		req.Username = userClaims.Username
	}

	// Threat scoring — runs before any database writes.
	ctx := c.Request.Context()
	threatReport, err := h.svc.ScoreThreat(ctx, &req)
	if err != nil {
		h.logger.Warn("threat scoring failed (non-fatal)", zap.Error(err))
	}
	if threatReport != nil && threatReport.Rejected {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":        "registration rejected: threat score too high",
			"threat_report": threatReport,
		})
		return
	}

	agent, err := h.svc.Register(ctx, &req)
	if err != nil {
		h.logger.Error("register agent", zap.Error(err))
		var valErr *model.ErrValidation
		if errors.As(err, &valErr) {
			c.JSON(http.StatusBadRequest, gin.H{"error": valErr.Msg})
			return
		}
		if isQuotaError(err) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "registration failed"})
		return
	}

	resp := gin.H{
		"agent":     agent,
		"agent_uri": agent.URI(),
	}
	if threatReport != nil {
		resp["threat_report"] = threatReport
	}
	c.JSON(http.StatusCreated, resp)
}

// ListAgents handles GET /agents — returns paginated agent list.
// Optional ?q= performs an inclusive partial-match search across name, description,
// org, capability, agent_id, and tags. ?username= filters by agent owner username.
// Without ?q= or ?username=, returns all agents filtered by the optional trust_root
// and capability_node params.
func (h *AgentHandler) ListAgents(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	trustRoot := c.Query("trust_root")
	capNode := c.Query("capability_node")
	username := strings.TrimSpace(c.Query("username"))

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	var agents []*model.Agent
	var err error

	ctx := c.Request.Context()

	switch {
	case username != "":
		agents, err = h.svc.ListActiveByUsername(ctx, username, limit, offset)
	case q != "":
		agents, err = h.svc.Search(ctx, q, limit, offset)
	default:
		agents, err = h.svc.List(ctx, trustRoot, capNode, limit, offset)
	}
	if err != nil {
		h.logger.Error("list agents", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list agents"})
		return
	}
	if agents == nil {
		agents = []*model.Agent{}
	}

	c.JSON(http.StatusOK, gin.H{"agents": agents, "count": len(agents)})
}

// GetAgent handles GET /agents/:id — retrieves a single agent by UUID.
// Response shape: {"agent": {...}, "owner": {"username": "...", "display_name": "...", "avatar_url": "..."}}
// The "owner" field is null when the agent has no owner account or owner lookup is unavailable.
func (h *AgentHandler) GetAgent(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent ID"})
		return
	}

	ctx := c.Request.Context()

	agent, err := h.svc.Get(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get agent"})
		return
	}

	resp := gin.H{"agent": agent}

	if h.ownerSvc != nil && agent.OwnerUserID != nil {
		if owner, err := h.ownerSvc.GetByID(ctx, *agent.OwnerUserID); err == nil {
			resp["owner"] = gin.H{
				"username":     owner.Username,
				"display_name": owner.DisplayName,
				"avatar_url":   owner.AvatarURL,
			}
		}
	}

	c.JSON(http.StatusOK, resp)
}

// UpdateAgent handles PATCH /agents/:id — updates mutable agent fields.
func (h *AgentHandler) UpdateAgent(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent ID"})
		return
	}

	var req model.UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Ownership authorisation — enforced whenever at least one token issuer is configured.
	// Accepts either an agent task token (for domain-verified agents) or a user JWT
	// (for nap_hosted agents). Anonymous requests are always rejected.
	if h.tokens != nil || h.userTokens != nil {
		agent, err := h.svc.Get(c.Request.Context(), id)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get agent"})
			return
		}

		agentClaims := identity.ClaimsFromCtx(c)
		userClaims := userFromCtx(c)

		authorized := false
		if agentClaims != nil {
			authorized = agentClaims.AgentURI == agent.URI() || identity.HasScope(agentClaims, "nexus:admin")
		}
		if !authorized && userClaims != nil && agent.OwnerUserID != nil {
			uid, _ := uuid.Parse(userClaims.UserID)
			authorized = *agent.OwnerUserID == uid
		}

		if !authorized {
			if agentClaims == nil && userClaims == nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
				return
			}
			c.JSON(http.StatusForbidden, gin.H{"error": "cannot update another agent's registration"})
			return
		}
	}

	agent, err := h.svc.Update(c.Request.Context(), id, &req)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update agent"})
		return
	}

	c.JSON(http.StatusOK, agent)
}

// DeleteAgent handles DELETE /agents/:id — permanently removes an agent.
func (h *AgentHandler) DeleteAgent(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent ID"})
		return
	}

	ctx := c.Request.Context()

	agent, err := h.svc.Get(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get agent"})
		return
	}

	// Authorisation: agent task token OR user token with ownership.
	if h.tokens != nil {
		agentClaims := identity.ClaimsFromCtx(c)
		userClaims := userFromCtx(c)

		authorized := false
		if agentClaims != nil {
			authorized = agentClaims.AgentURI == agent.URI() || identity.HasScope(agentClaims, "nexus:admin")
		}
		if !authorized && userClaims != nil && agent.OwnerUserID != nil {
			uid, _ := uuid.Parse(userClaims.UserID)
			authorized = *agent.OwnerUserID == uid
		}

		if !authorized {
			if agentClaims == nil && userClaims == nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
				return
			}
			c.JSON(http.StatusForbidden, gin.H{"error": "cannot delete another agent's registration"})
			return
		}
	}

	if err := h.svc.Delete(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete agent"})
		return
	}

	c.Status(http.StatusNoContent)
}

// ActivateAgent handles POST /agents/:id/activate.
func (h *AgentHandler) ActivateAgent(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent ID"})
		return
	}

	ctx := c.Request.Context()

	// Ownership check: caller must be the agent (task token) or the owning user.
	agent, err := h.svc.Get(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get agent"})
		return
	}

	agentClaims := identity.ClaimsFromCtx(c)
	userClaims := userFromCtx(c)

	authorized := false
	if agentClaims != nil {
		authorized = agentClaims.AgentURI == agent.URI() || identity.HasScope(agentClaims, "nexus:admin")
	}
	if !authorized && userClaims != nil && agent.OwnerUserID != nil {
		uid, _ := uuid.Parse(userClaims.UserID)
		authorized = *agent.OwnerUserID == uid
	}
	// Domain-verified agents without an owner can be activated by anyone who
	// completes the DNS-01 challenge — the DNS check in svc.Activate is the gate.
	if !authorized && agent.OwnerUserID == nil && agent.RegistrationType == "domain" {
		authorized = true
	}

	if !authorized {
		if agentClaims == nil && userClaims == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "not authorized to activate this agent"})
		return
	}

	result, err := h.svc.Activate(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return
		}
		h.logger.Error("activate agent", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to activate agent"})
		return
	}

	resp := gin.H{
		"status": "activated",
		"agent":  result.Agent,
	}

	if result.CertPEM != "" {
		resp["certificate"] = gin.H{
			"serial":     result.Serial,
			"pem":        result.CertPEM,
			"expires_at": result.ExpiresAt,
		}
		resp["private_key_pem"] = result.KeyPEM
		resp["ca_pem"] = result.CAPEM
		resp["warning"] = "Store private_key_pem securely. It will not be shown again."
	}

	if result.AgentCardJSON != "" {
		resp["agent_card_json"] = result.AgentCardJSON
		resp["agent_card_note"] = "Deploy agent_card_json at https://yourdomain/.well-known/agent.json for A2A client discovery."
	}

	if result.MCPManifestJSON != "" {
		resp["mcp_manifest_json"] = result.MCPManifestJSON
		resp["mcp_manifest_note"] = "mcp_manifest_json describes the MCP tools this agent exposes."
	}

	c.JSON(http.StatusOK, resp)
}

// RevokeAgent handles POST /agents/:id/revoke — marks agent as revoked.
func (h *AgentHandler) RevokeAgent(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent ID"})
		return
	}

	ctx := c.Request.Context()

	agent, err := h.svc.Get(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get agent"})
		return
	}

	// Authorisation: agent task token OR user token with ownership.
	if h.tokens != nil {
		agentClaims := identity.ClaimsFromCtx(c)
		userClaims := userFromCtx(c)

		authorized := false
		if agentClaims != nil {
			authorized = agentClaims.AgentURI == agent.URI() || identity.HasScope(agentClaims, "nexus:admin")
		}
		if !authorized && userClaims != nil && agent.OwnerUserID != nil {
			uid, _ := uuid.Parse(userClaims.UserID)
			authorized = *agent.OwnerUserID == uid
		}

		if !authorized {
			if agentClaims == nil && userClaims == nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
				return
			}
			c.JSON(http.StatusForbidden, gin.H{"error": "cannot revoke another agent's registration"})
			return
		}
	}

	var body struct {
		Reason string `json:"reason"`
	}
	// Ignore parse errors — reason is optional.
	_ = c.ShouldBindJSON(&body)

	if err := h.svc.Revoke(ctx, id, body.Reason); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke agent"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "revoked",
		"uri":    agent.URI(),
	})
}

// ResolveAgent handles GET /resolve?uri=agent://... — resolves a URI to an endpoint.
func (h *AgentHandler) ResolveAgent(c *gin.Context) {
	trustRoot := c.Query("trust_root")
	capNode := c.Query("capability_node")
	agentID := c.Query("agent_id")

	if trustRoot == "" || capNode == "" || agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "trust_root, capability_node and agent_id are required"})
		return
	}

	agent, err := h.svc.Resolve(c.Request.Context(), trustRoot, capNode, agentID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if agent.Status == model.AgentStatusDeprecated {
		c.Header("X-NAP-Deprecated", "true")
		if agent.SunsetDate != nil {
			c.Header("Sunset", agent.SunsetDate.Format(time.RFC1123))
		}
		if agent.ReplacementURI != "" {
			c.Header("X-NAP-Replacement", agent.ReplacementURI)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"id":          agent.ID,
		"uri":         agent.URI(),
		"endpoint":    agent.Endpoint,
		"status":      agent.Status,
		"cert_serial": agent.CertSerial,
	})
}

// agentCardView is the public-facing shape returned by the lookup endpoint.
// It contains only what a consumer needs to discover and connect to an agent —
// no internal fields like cert_serial or public_key_pem.
type agentCardView struct {
	ID             string           `json:"id"`
	URI            string           `json:"uri"`
	DisplayName    string           `json:"display_name"`
	Description    string           `json:"description"`
	CapabilityNode string           `json:"capability_node"`
	Endpoint       string           `json:"endpoint"`
	TrustTier      model.TrustTier  `json:"trust_tier"`
	Metadata       model.AgentMeta  `json:"metadata,omitempty"`
}

func toCardView(a *model.Agent) agentCardView {
	return agentCardView{
		ID:             a.ID.String(),
		URI:            a.URI(),
		DisplayName:    a.DisplayName,
		Description:    a.Description,
		CapabilityNode: a.CapabilityNode,
		Endpoint:       a.Endpoint,
		TrustTier:      a.TrustTier,
		Metadata:       a.Metadata,
	}
}

// LookupByDomain handles GET /lookup — agent discovery by org namespace and/or capability.
//
// Supported query parameters:
//
//	?org=acme                     — all active agents for that org namespace
//	?capability=finance           — all active agents with that capability (prefix match)
//	?org=acme&capability=finance  — combined filter
//
// At least one of org or capability must be provided.
func (h *AgentHandler) LookupByDomain(c *gin.Context) {
	org := c.Query("org")
	capability := c.Query("capability")

	if org == "" && capability == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one of org or capability is required"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	ctx := c.Request.Context()

	var agents []*model.Agent
	var err error

	if capability != "" {
		// Capability-first (optionally filtered by org).
		agents, err = h.svc.LookupByCapability(ctx, capability, org, limit, offset)
		if err != nil {
			h.logger.Error("lookup by capability", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "lookup failed"})
			return
		}
	} else {
		// Org-only lookup.
		agents, err = h.svc.LookupByOrg(ctx, org, limit, offset)
		if err != nil {
			h.logger.Error("lookup by org", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "lookup failed"})
			return
		}
	}

	cards := make([]agentCardView, len(agents))
	for i, a := range agents {
		cards[i] = toCardView(a)
	}

	resp := gin.H{
		"agents": cards,
		"count":  len(cards),
	}
	if org != "" {
		resp["org"] = org
	}
	if capability != "" {
		resp["capability"] = capability
	}

	c.JSON(http.StatusOK, resp)
}

// GetCapabilities handles GET /capabilities — returns the full three-level taxonomy.
//
// Response shape:
//
//	{ "categories": [{ "name": "finance", "subcategories": [{ "name": "accounting", "items": [...] }] }] }
func (h *AgentHandler) GetCapabilities(c *gin.Context) {
	type item struct {
		Name  string   `json:"name"`
		Items []string `json:"items"`
	}
	type category struct {
		Name          string `json:"name"`
		Subcategories []item `json:"subcategories"`
	}

	cats := model.SortedCategories()
	result := make([]category, len(cats))
	for i, cat := range cats {
		subs := model.SortedSubcategories(cat)
		subItems := make([]item, len(subs))
		for j, sub := range subs {
			subItems[j] = item{
				Name:  sub,
				Items: model.SortedItems(cat, sub),
			}
		}
		result[i] = category{Name: cat, Subcategories: subItems}
	}
	c.JSON(http.StatusOK, gin.H{"categories": result})
}

// GetAgentCard handles GET /agents/:id/agent.json — returns the A2A-spec card for a single agent.
func (h *AgentHandler) GetAgentCard(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent ID"})
		return
	}

	agent, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get agent"})
		return
	}

	cardJSON, err := h.svc.GetAgentCardJSON(agent)
	if err != nil {
		h.logger.Error("generate agent card", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate agent card"})
		return
	}

	c.Header("Content-Type", "application/json")
	c.String(http.StatusOK, cardJSON)
}

// GetMCPManifest handles GET /agents/:id/mcp-manifest.json — returns the MCP manifest for an agent.
func (h *AgentHandler) GetMCPManifest(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent ID"})
		return
	}

	agent, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get agent"})
		return
	}

	manifest, err := h.svc.GenerateMCPManifest(agent)
	if err != nil {
		h.logger.Error("generate MCP manifest", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate MCP manifest"})
		return
	}
	if manifest == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent has no declared MCP tools"})
		return
	}

	c.JSON(http.StatusOK, manifest)
}

// isQuotaError returns true if the error is a free-tier quota violation.
func isQuotaError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "free tier limit")
}

// ListMyAgents handles GET /users/me/agents — returns all agents owned by the
// authenticated user, ordered newest first.
func (h *AgentHandler) ListMyAgents(c *gin.Context) {
	userClaims := userFromCtx(c)
	if userClaims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user authentication required"})
		return
	}

	uid, err := uuid.Parse(userClaims.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID in token"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	agents, err := h.svc.ListByOwnerUserID(c.Request.Context(), uid, limit, offset)
	if err != nil {
		h.logger.Error("list my agents", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list agents"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"agents": agents, "count": len(agents)})
}

// SuspendAgent handles POST /agents/:id/suspend — temporarily disables an agent.
func (h *AgentHandler) SuspendAgent(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent ID"})
		return
	}

	ctx := c.Request.Context()
	if !h.authorizeAgentAction(c, ctx, id, "suspend") {
		return
	}

	if err := h.svc.Suspend(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "suspended"})
}

// RestoreAgent handles POST /agents/:id/restore — re-activates a suspended agent.
func (h *AgentHandler) RestoreAgent(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent ID"})
		return
	}

	ctx := c.Request.Context()
	if !h.authorizeAgentAction(c, ctx, id, "restore") {
		return
	}

	if err := h.svc.Restore(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "active"})
}

// DeprecateAgent handles POST /agents/:id/deprecate — marks an agent as deprecated.
func (h *AgentHandler) DeprecateAgent(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent ID"})
		return
	}

	ctx := c.Request.Context()
	if !h.authorizeAgentAction(c, ctx, id, "deprecate") {
		return
	}

	var req model.DeprecateRequest
	_ = c.ShouldBindJSON(&req) // optional body

	if err := h.svc.Deprecate(ctx, id, &req); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deprecated"})
}

// GetCRL handles GET /crl — returns a JSON certificate revocation list.
func (h *AgentHandler) GetCRL(c *gin.Context) {
	agents, err := h.svc.ListRevokedCerts(c.Request.Context())
	if err != nil {
		h.logger.Error("list revoked certs", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list revoked certificates"})
		return
	}

	type crlEntry struct {
		CertSerial string `json:"cert_serial"`
		Reason     string `json:"reason"`
		RevokedAt  string `json:"revoked_at"`
	}

	entries := make([]crlEntry, 0, len(agents))
	for _, a := range agents {
		entries = append(entries, crlEntry{
			CertSerial: a.CertSerial,
			Reason:     a.RevocationReason,
			RevokedAt:  a.UpdatedAt.Format(time.RFC3339),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"entries":      entries,
		"count":        len(entries),
		"generated_at": time.Now().UTC().Format(time.RFC3339),
	})
}

// BatchResolve handles POST /resolve/batch — resolves multiple URIs in one call.
func (h *AgentHandler) BatchResolve(c *gin.Context) {
	var req struct {
		URIs []string `json:"uris" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.URIs) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "maximum 100 URIs per batch"})
		return
	}

	type batchResult struct {
		URI      string `json:"uri"`
		Endpoint string `json:"endpoint,omitempty"`
		Status   string `json:"status,omitempty"`
		Error    string `json:"error,omitempty"`
	}

	ctx := c.Request.Context()
	results := make([]batchResult, len(req.URIs))
	sem := make(chan struct{}, 10)
	var wg sync.WaitGroup

	for i, rawURI := range req.URIs {
		wg.Add(1)
		go func(idx int, u string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			parsed, err := uri.Parse(u)
			if err != nil {
				results[idx] = batchResult{URI: u, Error: "invalid URI: " + err.Error()}
				return
			}

			agent, err := h.svc.Resolve(ctx, parsed.OrgName, parsed.Category, parsed.AgentID)
			if err != nil {
				results[idx] = batchResult{URI: u, Error: err.Error()}
				return
			}

			results[idx] = batchResult{
				URI:      u,
				Endpoint: agent.Endpoint,
				Status:   string(agent.Status),
			}
		}(i, rawURI)
	}

	wg.Wait()
	c.JSON(http.StatusOK, gin.H{"results": results, "count": len(results)})
}

// ReportAbuseProxy is a placeholder route on the agent resource; the real handler
// is wired by AbuseHandler after construction. This stub returns 501 if the abuse
// system is not wired.
func (h *AgentHandler) ReportAbuseProxy(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "abuse reporting not configured"})
}

// authorizeAgentAction is a shared authorization helper for agent lifecycle actions
// (suspend, restore, deprecate). Returns true if authorized, false if it wrote an error response.
func (h *AgentHandler) authorizeAgentAction(c *gin.Context, ctx context.Context, id uuid.UUID, action string) bool {
	agent, err := h.svc.Get(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return false
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get agent"})
		return false
	}

	if h.tokens != nil || h.userTokens != nil {
		agentClaims := identity.ClaimsFromCtx(c)
		userClaims := userFromCtx(c)

		authorized := false
		if agentClaims != nil {
			authorized = agentClaims.AgentURI == agent.URI() || identity.HasScope(agentClaims, "nexus:admin")
		}
		if !authorized && userClaims != nil && agent.OwnerUserID != nil {
			uid, _ := uuid.Parse(userClaims.UserID)
			authorized = *agent.OwnerUserID == uid
		}

		if !authorized {
			if agentClaims == nil && userClaims == nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
				return false
			}
			c.JSON(http.StatusForbidden, gin.H{"error": "cannot " + action + " another agent's registration"})
			return false
		}
	}

	return true
}
