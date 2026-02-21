package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nexus-protocol/nexus/internal/identity"
	"github.com/nexus-protocol/nexus/internal/registry/model"
	"github.com/nexus-protocol/nexus/internal/registry/repository"
	"github.com/nexus-protocol/nexus/internal/registry/service"
	"go.uber.org/zap"
)

// AgentHandler handles HTTP requests for the agent registry.
type AgentHandler struct {
	svc        *service.AgentService
	tokens     *identity.TokenIssuer     // nil = no agent token auth enforcement
	userTokens *identity.UserTokenIssuer // nil = no user token support
	logger     *zap.Logger
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
		agents.PATCH("/:id", h.optionalAgentToken(), h.optionalUserToken(), h.UpdateAgent)
		agents.DELETE("/:id", h.requireToken(), h.optionalUserToken(), h.DeleteAgent)
		agents.POST("/:id/activate", h.ActivateAgent)
		agents.POST("/:id/revoke", h.requireToken(), h.optionalUserToken(), h.RevokeAgent)
	}

	rg.GET("/resolve", h.ResolveAgent)
	rg.GET("/lookup", h.LookupByDomain)
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

	agent, err := h.svc.Register(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("register agent", zap.Error(err))
		if isQuotaError(err) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "registration failed"})
		return
	}

	c.JSON(http.StatusCreated, agent)
}

// ListAgents handles GET /agents — returns paginated agent list.
func (h *AgentHandler) ListAgents(c *gin.Context) {
	trustRoot := c.Query("trust_root")
	capNode := c.Query("capability_node")

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	agents, err := h.svc.List(c.Request.Context(), trustRoot, capNode, limit, offset)
	if err != nil {
		h.logger.Error("list agents", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list agents"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"agents": agents, "count": len(agents)})
}

// GetAgent handles GET /agents/:id — retrieves a single agent by UUID.
func (h *AgentHandler) GetAgent(c *gin.Context) {
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

	c.JSON(http.StatusOK, agent)
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

	result, err := h.svc.Activate(c.Request.Context(), id)
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

	if err := h.svc.Revoke(ctx, id); err != nil {
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
		URI:            a.URI(),
		DisplayName:    a.DisplayName,
		Description:    a.Description,
		CapabilityNode: a.CapabilityNode,
		Endpoint:       a.Endpoint,
		TrustTier:      a.TrustTier,
		Metadata:       a.Metadata,
	}
}

// LookupByDomain handles GET /lookup?domain=acme.com — returns all active,
// domain-verified agents registered under the given trust-root domain.
func (h *AgentHandler) LookupByDomain(c *gin.Context) {
	domain := c.Query("domain")
	if domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain query parameter is required"})
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

	agents, err := h.svc.LookupByDomain(c.Request.Context(), domain, limit, offset)
	if err != nil {
		h.logger.Error("lookup by domain", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "lookup failed"})
		return
	}

	cards := make([]agentCardView, len(agents))
	for i, a := range agents {
		cards[i] = toCardView(a)
	}

	c.JSON(http.StatusOK, gin.H{
		"domain": domain,
		"agents": cards,
		"count":  len(cards),
	})
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
