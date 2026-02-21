package handler

import (
	"errors"
	"net/http"
	"strconv"

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
	svc    *service.AgentService
	tokens *identity.TokenIssuer // nil = no auth enforcement (dev/open mode)
	logger *zap.Logger
}

// NewAgentHandler creates a new AgentHandler.
// tokens may be nil to disable JWT auth on protected routes.
func NewAgentHandler(svc *service.AgentService, tokens *identity.TokenIssuer, logger *zap.Logger) *AgentHandler {
	return &AgentHandler{svc: svc, tokens: tokens, logger: logger}
}

// requireToken returns the RequireToken middleware when auth is configured,
// or a no-op middleware for development/open mode.
func (h *AgentHandler) requireToken() gin.HandlerFunc {
	if h.tokens == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return identity.RequireToken(h.tokens)
}

// Register registers all agent routes on the given router group.
func (h *AgentHandler) Register(rg *gin.RouterGroup) {
	agents := rg.Group("/agents")
	{
		agents.POST("", h.CreateAgent)
		agents.GET("", h.ListAgents)
		agents.GET("/:id", h.GetAgent)
		agents.PATCH("/:id", h.UpdateAgent)
		agents.DELETE("/:id", h.requireToken(), h.DeleteAgent)
		agents.POST("/:id/activate", h.ActivateAgent)
		agents.POST("/:id/revoke", h.requireToken(), h.RevokeAgent)
	}

	rg.GET("/resolve", h.ResolveAgent)
}

// CreateAgent handles POST /agents — registers a new agent.
func (h *AgentHandler) CreateAgent(c *gin.Context) {
	var req model.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	agent, err := h.svc.Register(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("register agent", zap.Error(err))
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

	// Authorisation: token must belong to this agent or carry nexus:admin scope.
	if h.tokens != nil {
		claims := identity.ClaimsFromCtx(c)
		if claims == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		if claims.AgentURI != agent.URI() && !identity.HasScope(claims, "nexus:admin") {
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

// ActivateAgent handles POST /agents/:id/activate — transitions agent to active
// and, when the registry is configured with an Issuer, issues an X.509 agent
// identity certificate.
//
// Response includes the signed certificate and the private key. The private key
// is delivered ONCE and is not persisted by the registry — the caller must store
// it securely.
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

	c.JSON(http.StatusOK, resp)
}

// RevokeAgent handles POST /agents/:id/revoke — marks agent as revoked.
//
// When auth is configured (tokens != nil), the request must carry a valid
// Bearer token. The token's agent_uri must match the target agent's URI, or
// the token must carry the "nexus:admin" scope.
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

	// Authorisation: token must belong to this agent or carry nexus:admin scope.
	if h.tokens != nil {
		claims := identity.ClaimsFromCtx(c)
		if claims == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		if claims.AgentURI != agent.URI() && !identity.HasScope(claims, "nexus:admin") {
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
