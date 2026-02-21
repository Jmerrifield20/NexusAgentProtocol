package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nexus-protocol/nexus/internal/identity"
	"go.uber.org/zap"
)

// IdentityHandler handles authentication, token issuance, and identity endpoints.
type IdentityHandler struct {
	issuer *identity.Issuer
	tokens *identity.TokenIssuer
	logger *zap.Logger
}

// NewIdentityHandler creates an IdentityHandler.
func NewIdentityHandler(issuer *identity.Issuer, tokens *identity.TokenIssuer, logger *zap.Logger) *IdentityHandler {
	return &IdentityHandler{issuer: issuer, tokens: tokens, logger: logger}
}

// Register wires the identity routes onto the API group.
// The token endpoint requires mTLS; the CA cert endpoint is public.
func (h *IdentityHandler) Register(rg *gin.RouterGroup) {
	// mTLS-protected: exchange client cert → Task Token
	rg.POST("/token", identity.RequireMTLS(h.issuer), h.IssueToken)

	// Public: download the Nexus CA certificate (needed to configure mTLS clients)
	rg.GET("/ca.crt", h.GetCACert)
}

// IssueToken handles POST /api/v1/token.
//
// The caller must authenticate with a valid mTLS client certificate issued by
// the Nexus CA. On success it returns a short-lived JWT Task Token.
//
//	Request (form or JSON):
//	  grant_type: "client_credentials"   (required)
//	  scope:      "agent:resolve agent:call"  (optional; defaults to full set)
//
//	Response:
//	  {"access_token":"...", "token_type":"Bearer", "expires_in":3600, "scope":"..."}
func (h *IdentityHandler) IssueToken(c *gin.Context) {
	agentURI := identity.AgentURIFromCtx(c)
	if agentURI == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "agent identity not established"})
		return
	}

	var req struct {
		GrantType string `json:"grant_type" form:"grant_type"`
		Scope     string `json:"scope"      form:"scope"`
	}
	_ = c.ShouldBind(&req)

	if req.GrantType != "" && req.GrantType != "client_credentials" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":             "unsupported_grant_type",
			"error_description": "only client_credentials is supported",
		})
		return
	}

	// Parse requested scopes (space-separated); fall back to defaults.
	scopes := defaultScopes()
	if req.Scope != "" {
		scopes = splitScopes(req.Scope)
	}

	token, err := h.tokens.Issue(agentURI, scopes)
	if err != nil {
		h.logger.Error("issue token", zap.String("agent_uri", agentURI), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to issue token"})
		return
	}

	h.logger.Info("token issued",
		zap.String("agent_uri", agentURI),
		zap.Strings("scopes", scopes),
	)

	c.JSON(http.StatusOK, gin.H{
		"access_token": token,
		"token_type":   "Bearer",
		"expires_in":   int(h.tokens.TTL().Seconds()),
		"scope":        strings.Join(scopes, " "),
	})
}

// GetCACert handles GET /api/v1/ca.crt — returns the Nexus CA certificate in PEM format.
// Clients download this to configure their TLS trust store before connecting with mTLS.
func (h *IdentityHandler) GetCACert(c *gin.Context) {
	// The CA cert is embedded in the Issuer via the CAManager.
	// We surface it through the token issuer's public key endpoint instead of
	// coupling the handler to the CAManager directly.
	// For now we return a helpful redirect message.
	c.JSON(http.StatusOK, gin.H{
		"message": "Download the CA certificate from the filesystem or the JWKS endpoint.",
		"jwks_uri": "/.well-known/jwks.json",
	})
}

// defaultScopes returns the standard set of Task Token scopes.
func defaultScopes() []string {
	return []string{"agent:resolve", "agent:call", "agent:register"}
}

// splitScopes parses a space-separated scope string.
func splitScopes(scope string) []string {
	parts := strings.Fields(scope)
	if len(parts) == 0 {
		return defaultScopes()
	}
	return parts
}
