package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nexus-protocol/nexus/internal/registry/service"
	"go.uber.org/zap"
)

// DNSHandler handles HTTP requests for the DNS-01 domain verification flow.
type DNSHandler struct {
	svc    *service.DNSChallengeService
	logger *zap.Logger
}

// NewDNSHandler creates a new DNSHandler.
func NewDNSHandler(svc *service.DNSChallengeService, logger *zap.Logger) *DNSHandler {
	return &DNSHandler{svc: svc, logger: logger}
}

// Register mounts the DNS challenge routes on the given router group.
func (h *DNSHandler) Register(rg *gin.RouterGroup) {
	dns := rg.Group("/dns/challenge")
	{
		dns.POST("", h.StartChallenge)
		dns.GET("/:id", h.GetChallenge)
		dns.POST("/:id/verify", h.VerifyChallenge)
	}
}

// StartChallenge handles POST /dns/challenge.
//
// Request body: {"domain": "example.com"}
//
// Response: challenge details including the TXT record the owner must publish.
func (h *DNSHandler) StartChallenge(c *gin.Context) {
	var req struct {
		Domain string `json:"domain" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ch, err := h.svc.StartChallenge(c.Request.Context(), req.Domain)
	if err != nil {
		h.logger.Error("start DNS challenge", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start challenge"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":         ch.ID,
		"domain":     ch.Domain,
		"txt_host":   ch.TXTHost,
		"txt_record": ch.TXTRecord,
		"expires_at": ch.ExpiresAt,
		"verified":   ch.Verified,
		"instructions": "Publish the following DNS TXT record, then call POST /dns/challenge/" + ch.ID.String() + "/verify",
	})
}

// GetChallenge handles GET /dns/challenge/:id — returns challenge status.
func (h *DNSHandler) GetChallenge(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid challenge ID"})
		return
	}

	ch, err := h.svc.GetChallenge(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrChallengeNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "challenge not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get challenge"})
		return
	}

	c.JSON(http.StatusOK, ch)
}

// VerifyChallenge handles POST /dns/challenge/:id/verify.
// It performs the DNS TXT lookup and marks the challenge verified on success.
func (h *DNSHandler) VerifyChallenge(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid challenge ID"})
		return
	}

	ch, err := h.svc.VerifyChallenge(c.Request.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrChallengeNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "challenge not found"})
		case errors.Is(err, service.ErrChallengeExpired):
			c.JSON(http.StatusGone, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrVerificationFailed):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		default:
			h.logger.Error("verify DNS challenge", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "verification error"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"verified":   ch.Verified,
		"domain":     ch.Domain,
		"id":         ch.ID,
		"message":    "Domain ownership verified. You may now activate agents registered under this domain.",
	})
}
