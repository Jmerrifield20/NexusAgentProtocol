package identity

import (
	"crypto/x509"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	ctxAgentURI    = "nexus_agent_uri"
	ctxTokenClaims = "nexus_token_claims"
	ctxUserClaims  = "nexus_user_claims"
)

// RequireMTLS returns a Gin middleware that enforces mutual TLS on the route.
//
// It verifies the client certificate against the Nexus CA, extracts the
// agent:// URI from the cert's URI SAN, and injects it into the Gin context
// under the "nexus_agent_uri" key.
func RequireMTLS(issuer *Issuer) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.TLS == nil || len(c.Request.TLS.PeerCertificates) == 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "mTLS required: no client certificate presented",
			})
			return
		}

		clientCert := c.Request.TLS.PeerCertificates[0]

		if _, err := issuer.VerifyPeerCert(clientCert); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "client certificate not trusted: " + err.Error(),
			})
			return
		}

		agentURI, err := AgentURIFromCert(clientCert)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.Set(ctxAgentURI, agentURI)
		c.Next()
	}
}

// RequireToken returns a Gin middleware that enforces a valid Bearer Task Token.
//
// On success it injects the *TaskTokenClaims into the context under the
// "nexus_token_claims" key.
func RequireToken(tokens *TokenIssuer) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Bearer token required",
			})
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := tokens.Verify(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid token: " + err.Error(),
			})
			return
		}

		c.Set(ctxTokenClaims, claims)
		c.Next()
	}
}

// AgentURIFromCtx retrieves the authenticated agent URI injected by RequireMTLS.
func AgentURIFromCtx(c *gin.Context) string {
	v, _ := c.Get(ctxAgentURI)
	s, _ := v.(string)
	return s
}

// ClaimsFromCtx retrieves the Task Token claims injected by RequireToken.
func ClaimsFromCtx(c *gin.Context) *TaskTokenClaims {
	v, _ := c.Get(ctxTokenClaims)
	claims, _ := v.(*TaskTokenClaims)
	return claims
}

// OptionalToken returns a Gin middleware that tries to parse a Bearer Task Token.
// Unlike RequireToken, it never aborts — it silently skips injection when the
// header is absent or the token fails verification.
func OptionalToken(tokens *TokenIssuer) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			if claims, err := tokens.Verify(tokenStr); err == nil {
				c.Set(ctxTokenClaims, claims)
			}
		}
		c.Next()
	}
}

// RequireUserToken returns a Gin middleware that enforces a valid user session Bearer token.
//
// On success it injects the *UserTokenClaims into the context under the
// "nexus_user_claims" key.
func RequireUserToken(tokens *UserTokenIssuer) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Bearer user token required",
			})
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := tokens.Verify(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid user token: " + err.Error(),
			})
			return
		}

		c.Set(ctxUserClaims, claims)
		c.Next()
	}
}

// RequireAdmin returns a Gin middleware that enforces a valid admin Bearer token.
// Only tokens with Type="admin" and Role="admin" are accepted.
// Use this on all federation management routes.
func RequireAdmin(tokens *UserTokenIssuer) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "admin Bearer token required",
			})
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := tokens.Verify(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid token: " + err.Error(),
			})
			return
		}

		if claims.Role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "admin role required",
			})
			return
		}

		c.Set(ctxUserClaims, claims)
		c.Next()
	}
}

// UserClaimsFromCtx retrieves the user token claims injected by RequireUserToken.
// Returns nil if no user token is present in the context.
func UserClaimsFromCtx(c *gin.Context) *UserTokenClaims {
	v, _ := c.Get(ctxUserClaims)
	claims, _ := v.(*UserTokenClaims)
	return claims
}

// HasScope checks whether the claims contain the requested scope.
func HasScope(claims *TaskTokenClaims, scope string) bool {
	if claims == nil {
		return false
	}
	for _, s := range claims.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// VerifyClientCertChain is a tls.Config.VerifyPeerCertificate callback that
// validates each raw DER certificate against the Nexus CA pool.
// Use this for strict mTLS where you want validation at the TLS handshake layer.
func VerifyClientCertChain(pool *x509.CertPool) func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
		if len(rawCerts) == 0 {
			return nil // handled per-route by middleware
		}
		cert, err := x509.ParseCertificate(rawCerts[0])
		if err != nil {
			return err
		}
		opts := x509.VerifyOptions{
			Roots:     pool,
			KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		}
		_, err = cert.Verify(opts)
		return err
	}
}
