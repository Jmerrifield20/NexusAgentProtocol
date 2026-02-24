package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/federation"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/identity"
	"go.uber.org/zap"
)

// federationSvc is the subset of federation.FederationService used by the handler.
type federationSvc interface {
	Register(ctx context.Context, req *federation.RegisterRequest) (*federation.RegisteredRegistry, error)
	Approve(ctx context.Context, id uuid.UUID) (*federation.RegisteredRegistry, error)
	Suspend(ctx context.Context, id uuid.UUID) (*federation.RegisteredRegistry, error)
	List(ctx context.Context, status federation.RegistryStatus, limit, offset int) ([]*federation.RegisteredRegistry, error)
	IssueIntermediateCA(ctx context.Context, trustRoot string) (*federation.IssueCAResponse, error)
	UpdateMaxPathLen(ctx context.Context, id uuid.UUID, maxPathLen int) error
}

// FederationHandler handles HTTP requests for the registry-of-registries API.
type FederationHandler struct {
	svc        federationSvc
	role       federation.Role
	userTokens *identity.UserTokenIssuer
	logger     *zap.Logger
}

// NewFederationHandler creates a FederationHandler.
func NewFederationHandler(svc federationSvc, role federation.Role, userTokens *identity.UserTokenIssuer, logger *zap.Logger) *FederationHandler {
	return &FederationHandler{svc: svc, role: role, userTokens: userTokens, logger: logger}
}

// Register mounts the federation routes onto the API group.
//
// Route security:
//   - POST /federation/register — requires admin token (operator onboarding is admin-initiated)
//   - All other routes — require admin token
//
// Root-only routes (issue-ca, approve, suspend, list) are only wired when role == RoleRoot.
func (h *FederationHandler) Register(rg *gin.RouterGroup) {
	fed := rg.Group("/federation")
	fed.Use(identity.RequireAdmin(h.userTokens))

	// Available in federated and root modes (admin initiates operator onboarding).
	fed.POST("/register", h.RegisterRegistry)

	// Root-only routes.
	if h.role == federation.RoleRoot {
		fed.GET("/registries", h.ListRegistries)
		fed.POST("/issue-ca", h.IssueCA)
		fed.POST("/registries/:id/approve", h.ApproveRegistry)
		fed.POST("/registries/:id/suspend", h.SuspendRegistry)
		fed.PATCH("/registries/:id/delegation", h.UpdateDelegation)
	}
}

// RegisterRegistry handles POST /api/v1/federation/register.
func (h *FederationHandler) RegisterRegistry(c *gin.Context) {
	var req federation.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	reg, err := h.svc.Register(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("register federation registry", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, reg)
}

// ListRegistries handles GET /api/v1/federation/registries (root only).
func (h *FederationHandler) ListRegistries(c *gin.Context) {
	status := federation.RegistryStatus(c.Query("status"))
	registries, err := h.svc.List(c.Request.Context(), status, 50, 0)
	if err != nil {
		h.logger.Error("list federation registries", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list registries"})
		return
	}
	if registries == nil {
		registries = []*federation.RegisteredRegistry{}
	}
	c.JSON(http.StatusOK, gin.H{"registries": registries})
}

// IssueCA handles POST /api/v1/federation/issue-ca (root only).
func (h *FederationHandler) IssueCA(c *gin.Context) {
	var req federation.IssueCARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	resp, err := h.svc.IssueIntermediateCA(c.Request.Context(), req.TrustRoot)
	if err != nil {
		h.logger.Error("issue intermediate CA", zap.String("trust_root", req.TrustRoot), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ApproveRegistry handles POST /api/v1/federation/registries/:id/approve (root only).
func (h *FederationHandler) ApproveRegistry(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid registry id"})
		return
	}

	reg, err := h.svc.Approve(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("approve registry", zap.String("id", id.String()), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, reg)
}

// SuspendRegistry handles POST /api/v1/federation/registries/:id/suspend (root only).
func (h *FederationHandler) SuspendRegistry(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid registry id"})
		return
	}

	reg, err := h.svc.Suspend(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("suspend registry", zap.String("id", id.String()), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, reg)
}

// UpdateDelegation handles PATCH /api/v1/federation/registries/:id/delegation (root only).
// It updates the sub-delegation depth (max_path_len) for a registered registry.
// The new value takes effect on the next CA re-issuance.
func (h *FederationHandler) UpdateDelegation(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid registry id"})
		return
	}

	var req federation.UpdateDelegationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	if err := h.svc.UpdateMaxPathLen(c.Request.Context(), id, req.MaxPathLen); err != nil {
		h.logger.Error("update delegation", zap.String("id", id.String()), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"id": id.String(), "max_path_len": req.MaxPathLen})
}
