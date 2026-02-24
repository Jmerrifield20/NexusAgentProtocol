package webhooks

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/identity"
	"go.uber.org/zap"
)

// Handler handles HTTP requests for webhook subscriptions.
type Handler struct {
	svc        *Service
	userTokens *identity.UserTokenIssuer
	logger     *zap.Logger
}

// NewHandler creates a new webhook Handler.
func NewHandler(svc *Service, userTokens *identity.UserTokenIssuer, logger *zap.Logger) *Handler {
	return &Handler{svc: svc, userTokens: userTokens, logger: logger}
}

// Register registers all webhook routes on the given router group.
func (h *Handler) Register(rg *gin.RouterGroup) {
	wh := rg.Group("/webhooks")
	wh.Use(h.requireUserToken())
	{
		wh.POST("", h.CreateSubscription)
		wh.GET("", h.ListSubscriptions)
		wh.DELETE("/:id", h.DeleteSubscription)
	}
}

func (h *Handler) requireUserToken() gin.HandlerFunc {
	if h.userTokens == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return identity.RequireUserToken(h.userTokens)
}

// CreateSubscription handles POST /webhooks — creates a new subscription.
func (h *Handler) CreateSubscription(c *gin.Context) {
	userClaims := identity.UserClaimsFromCtx(c)
	if userClaims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user authentication required"})
		return
	}
	userID, err := uuid.Parse(userClaims.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	var req CreateSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sub, err := h.svc.Subscribe(c.Request.Context(), userID, &req)
	if err != nil {
		h.logger.Error("create webhook subscription", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create subscription"})
		return
	}

	// Return the secret once so the user can store it.
	c.JSON(http.StatusCreated, gin.H{
		"subscription": sub,
		"secret":       sub.Secret,
		"note":         "Store the secret securely. It will not be shown again.",
	})
}

// ListSubscriptions handles GET /webhooks — lists the user's subscriptions.
func (h *Handler) ListSubscriptions(c *gin.Context) {
	userClaims := identity.UserClaimsFromCtx(c)
	if userClaims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user authentication required"})
		return
	}
	userID, err := uuid.Parse(userClaims.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	subs, err := h.svc.ListByUser(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("list webhook subscriptions", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list subscriptions"})
		return
	}
	if subs == nil {
		subs = []*WebhookSubscription{}
	}

	c.JSON(http.StatusOK, gin.H{"subscriptions": subs, "count": len(subs)})
}

// DeleteSubscription handles DELETE /webhooks/:id — deletes a subscription.
func (h *Handler) DeleteSubscription(c *gin.Context) {
	userClaims := identity.UserClaimsFromCtx(c)
	if userClaims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user authentication required"})
		return
	}
	userID, err := uuid.Parse(userClaims.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	subID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid subscription ID"})
		return
	}

	if err := h.svc.Unsubscribe(c.Request.Context(), userID, subID); err != nil {
		h.logger.Error("delete webhook subscription", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete subscription"})
		return
	}

	c.Status(http.StatusNoContent)
}
