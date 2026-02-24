package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/identity"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/registry/model"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/registry/repository"
	"go.uber.org/zap"
)

// AbuseHandler handles HTTP requests for abuse reporting.
type AbuseHandler struct {
	repo       *repository.AbuseReportRepository
	userTokens *identity.UserTokenIssuer
	logger     *zap.Logger
}

// NewAbuseHandler creates a new AbuseHandler.
func NewAbuseHandler(repo *repository.AbuseReportRepository, userTokens *identity.UserTokenIssuer, logger *zap.Logger) *AbuseHandler {
	return &AbuseHandler{repo: repo, userTokens: userTokens, logger: logger}
}

// Register registers all abuse report routes on the given router group.
func (h *AbuseHandler) Register(rg *gin.RouterGroup) {
	rg.POST("/agents/:id/report-abuse", h.requireUserToken(), h.ReportAbuse)

	admin := rg.Group("/admin")
	admin.Use(h.requireUserToken())
	{
		admin.GET("/abuse-reports", h.ListAbuseReports)
		admin.PATCH("/abuse-reports/:id", h.ResolveAbuseReport)
	}
}

func (h *AbuseHandler) requireUserToken() gin.HandlerFunc {
	if h.userTokens == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return identity.RequireUserToken(h.userTokens)
}

// ReportAbuse handles POST /agents/:id/report-abuse — files an abuse report.
func (h *AbuseHandler) ReportAbuse(c *gin.Context) {
	agentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent ID"})
		return
	}

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

	var req model.CreateAbuseReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()

	// Limit: max 3 open reports per user per agent.
	count, err := h.repo.CountByAgentAndReporter(ctx, agentID, userID)
	if err != nil {
		h.logger.Error("count abuse reports", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check report limit"})
		return
	}
	if count >= 3 {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "maximum 3 open reports per agent"})
		return
	}

	report := &model.AbuseReport{
		AgentID:        agentID,
		ReporterUserID: userID,
		Reason:         req.Reason,
		Details:        req.Details,
	}

	if err := h.repo.Create(ctx, report); err != nil {
		h.logger.Error("create abuse report", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create report"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"report": report})
}

// ListAbuseReports handles GET /admin/abuse-reports — returns paginated reports.
func (h *AbuseHandler) ListAbuseReports(c *gin.Context) {
	status := c.Query("status")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	reports, err := h.repo.List(c.Request.Context(), status, limit, offset)
	if err != nil {
		h.logger.Error("list abuse reports", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list reports"})
		return
	}
	if reports == nil {
		reports = []*model.AbuseReport{}
	}

	c.JSON(http.StatusOK, gin.H{"reports": reports, "count": len(reports)})
}

// ResolveAbuseReport handles PATCH /admin/abuse-reports/:id — resolves or dismisses.
func (h *AbuseHandler) ResolveAbuseReport(c *gin.Context) {
	reportID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid report ID"})
		return
	}

	userClaims := identity.UserClaimsFromCtx(c)
	if userClaims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user authentication required"})
		return
	}
	resolverID, _ := uuid.Parse(userClaims.UserID)

	var req model.ResolveAbuseReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Status != model.AbuseStatusResolved && req.Status != model.AbuseStatusDismissed && req.Status != model.AbuseStatusInvestigating {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status must be 'resolved', 'dismissed', or 'investigating'"})
		return
	}

	if err := h.repo.Resolve(c.Request.Context(), reportID, req.Status, req.ResolutionNote, resolverID); err != nil {
		h.logger.Error("resolve abuse report", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve report"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": req.Status})
}
