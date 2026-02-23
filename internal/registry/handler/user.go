package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/identity"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/registry/model"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/users"
	"go.uber.org/zap"
)

// userPublicSvc is the subset of users.UserService used by UserHandler.
type userPublicSvc interface {
	GetByUsername(ctx context.Context, username string) (*users.User, error)
	GetPublicProfile(ctx context.Context, username string) (*users.PublicProfile, error)
	UpdateProfile(ctx context.Context, userID uuid.UUID, bio, avatarURL, websiteURL string) error
	GetByID(ctx context.Context, id uuid.UUID) (*users.User, error)
}

// agentPublicSvc is the subset of service.AgentService used by UserHandler.
type agentPublicSvc interface {
	ListActiveByOwnerUserID(ctx context.Context, ownerUserID uuid.UUID, limit, offset int) ([]*model.Agent, error)
	CountActiveByOwnerUserID(ctx context.Context, ownerUserID uuid.UUID) (int, error)
	ListVerifiedDomainsByUserID(ctx context.Context, ownerUserID uuid.UUID) ([]string, error)
}

// UserHandler handles HTTP requests for public user profiles and profile edits.
type UserHandler struct {
	users      userPublicSvc
	agents     agentPublicSvc
	userTokens *identity.UserTokenIssuer
	logger     *zap.Logger
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(userSvc userPublicSvc, agentSvc agentPublicSvc, logger *zap.Logger) *UserHandler {
	return &UserHandler{users: userSvc, agents: agentSvc, logger: logger}
}

// SetUserTokenIssuer configures the user JWT issuer for protected routes.
func (h *UserHandler) SetUserTokenIssuer(ut *identity.UserTokenIssuer) {
	h.userTokens = ut
}

// requireUserToken returns the RequireUserToken middleware when auth is configured,
// or a no-op middleware otherwise.
func (h *UserHandler) requireUserToken() gin.HandlerFunc {
	if h.userTokens == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return identity.RequireUserToken(h.userTokens)
}

// Register registers UserHandler routes on the given router group.
func (h *UserHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/users/:username", h.GetUserProfile)
	rg.GET("/users/:username/agents", h.ListUserAgents)
	rg.PATCH("/users/me/profile", h.requireUserToken(), h.UpdateMyProfile)
}

// GetUserProfile handles GET /users/:username — returns the public profile.
func (h *UserHandler) GetUserProfile(c *gin.Context) {
	username := c.Param("username")
	if username == "me" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "use /users/me/profile for authenticated profile access"})
		return
	}

	ctx := c.Request.Context()

	profile, err := h.users.GetPublicProfile(ctx, username)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found or profile is private"})
			return
		}
		h.logger.Error("get public profile", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get profile"})
		return
	}

	// Enrich with agent stats (non-fatal if user lookup fails).
	u, err := h.users.GetByUsername(ctx, username)
	if err == nil {
		count, _ := h.agents.CountActiveByOwnerUserID(ctx, u.ID)
		domains, _ := h.agents.ListVerifiedDomainsByUserID(ctx, u.ID)
		profile.AgentCount = count
		if domains == nil {
			domains = []string{}
		}
		profile.VerifiedDomains = domains
	}

	c.JSON(http.StatusOK, profile)
}

// ListUserAgents handles GET /users/:username/agents — lists a user's active agents.
func (h *UserHandler) ListUserAgents(c *gin.Context) {
	username := c.Param("username")

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	ctx := c.Request.Context()

	u, err := h.users.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		h.logger.Error("get user by username", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to look up user"})
		return
	}

	agents, err := h.agents.ListActiveByOwnerUserID(ctx, u.ID, limit, offset)
	if err != nil {
		h.logger.Error("list user agents", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list agents"})
		return
	}
	if agents == nil {
		agents = []*model.Agent{}
	}

	c.JSON(http.StatusOK, gin.H{"agents": agents, "count": len(agents)})
}

// updateProfileRequest is the body for PATCH /users/me/profile.
type updateProfileRequest struct {
	Bio        string `json:"bio"`
	AvatarURL  string `json:"avatar_url"`
	WebsiteURL string `json:"website_url"`
}

// UpdateMyProfile handles PATCH /users/me/profile — updates the authenticated user's profile.
func (h *UserHandler) UpdateMyProfile(c *gin.Context) {
	userClaims := identity.UserClaimsFromCtx(c)
	if userClaims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user authentication required"})
		return
	}

	uid, err := uuid.Parse(userClaims.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID in token"})
		return
	}

	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()

	if err := h.users.UpdateProfile(ctx, uid, req.Bio, req.AvatarURL, req.WebsiteURL); err != nil {
		h.logger.Error("update profile", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update profile"})
		return
	}

	u, err := h.users.GetByID(ctx, uid)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"status": "updated"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"username":     u.Username,
		"display_name": u.DisplayName,
		"bio":          u.Bio,
		"avatar_url":   u.AvatarURL,
		"website_url":  u.WebsiteURL,
	})
}
