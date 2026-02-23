package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/trustledger"
	"go.uber.org/zap"
)

// LedgerHandler exposes read-only HTTP endpoints for the trust ledger.
type LedgerHandler struct {
	ledger trustledger.Ledger
	logger *zap.Logger
}

// NewLedgerHandler creates a new LedgerHandler.
func NewLedgerHandler(ledger trustledger.Ledger, logger *zap.Logger) *LedgerHandler {
	return &LedgerHandler{ledger: ledger, logger: logger}
}

// Register mounts the ledger routes on the given router group.
func (h *LedgerHandler) Register(rg *gin.RouterGroup) {
	l := rg.Group("/ledger")
	{
		l.GET("", h.Overview)
		l.GET("/verify", h.Verify)
		l.GET("/entries/:idx", h.GetEntry)
	}
}

// Overview handles GET /ledger — returns the chain length and current root hash.
func (h *LedgerHandler) Overview(c *gin.Context) {
	ctx := c.Request.Context()

	count, err := h.ledger.Len(ctx)
	if err != nil {
		h.logger.Error("ledger Len", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query ledger"})
		return
	}

	root, err := h.ledger.Root(ctx)
	if err != nil {
		h.logger.Error("ledger Root", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query ledger root"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"entries": count,
		"root":    root,
	})
}

// Verify handles GET /ledger/verify — walks the full chain and reports integrity.
func (h *LedgerHandler) Verify(c *gin.Context) {
	ctx := c.Request.Context()

	if err := h.ledger.Verify(ctx); err != nil {
		h.logger.Warn("ledger integrity check failed", zap.Error(err))
		c.JSON(http.StatusOK, gin.H{
			"valid": false,
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"valid": true})
}

// GetEntry handles GET /ledger/entries/:idx — returns a single ledger entry.
func (h *LedgerHandler) GetEntry(c *gin.Context) {
	ctx := c.Request.Context()

	idxStr := c.Param("idx")
	idx, err := strconv.Atoi(idxStr)
	if err != nil || idx < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "idx must be a non-negative integer"})
		return
	}

	entry, err := h.ledger.Get(ctx, idx)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "entry not found"})
		return
	}

	c.JSON(http.StatusOK, entry)
}
