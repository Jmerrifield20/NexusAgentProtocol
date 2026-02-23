package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/registry/service"
	"github.com/jmerrifield20/NexusAgentProtocol/pkg/agentcard"
	"go.uber.org/zap"
)

// WellKnownHandler serves the /.well-known/agent-card.json endpoint.
// It lists all active agents for a domain and returns a standard AgentCard.
type WellKnownHandler struct {
	svc    *service.AgentService
	logger *zap.Logger
}

// NewWellKnownHandler creates a new WellKnownHandler.
func NewWellKnownHandler(svc *service.AgentService, logger *zap.Logger) *WellKnownHandler {
	return &WellKnownHandler{svc: svc, logger: logger}
}

// ServeAgentCard handles GET /.well-known/agent-card.json?domain=example.com
//
// Returns an AgentCard listing all active agents registered under the given domain.
// Responds 400 if domain is missing, 404 if no active agents are found.
func (h *WellKnownHandler) ServeAgentCard(c *gin.Context) {
	domain := c.Query("domain")
	if domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain query parameter is required"})
		return
	}

	ctx := c.Request.Context()
	agents, err := h.svc.ListByOwnerDomain(ctx, domain, 100, 0)
	if err != nil {
		h.logger.Error("list agents by domain", zap.String("domain", domain), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list agents"})
		return
	}

	if len(agents) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "no active agents found for domain"})
		return
	}

	trustRoot := agents[0].TrustRoot
	entries := make([]agentcard.AgentEntry, 0, len(agents))
	for _, a := range agents {
		entries = append(entries, agentcard.AgentEntry{
			URI:            a.URI(),
			DisplayName:    a.DisplayName,
			Description:    a.Description,
			Endpoint:       a.Endpoint,
			CapabilityNode: a.CapabilityNode,
			Status:         string(a.Status),
			Metadata:       a.Metadata,
			NAPTrustTier:   string(a.ComputeTrustTier()),
		})
	}

	card := agentcard.AgentCard{
		SchemaVersion: "1.0",
		Domain:        domain,
		TrustRoot:     trustRoot,
		Agents:        entries,
		UpdatedAt:     time.Now().UTC(),
	}

	c.JSON(http.StatusOK, card)
}
