package identity

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/binary"
	"net/http"

	"github.com/gin-gonic/gin"
)

// OIDCConfig is the OpenID Connect discovery document served at
// /.well-known/openid-configuration.
type OIDCConfig struct {
	Issuer                            string   `json:"issuer"`
	JWKSURI                           string   `json:"jwks_uri"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	SubjectTypesSupported             []string `json:"subject_types_supported"`
	IDTokenSigningAlgValuesSupported  []string `json:"id_token_signing_alg_values_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported"`
}

// JWKSet is a JSON Web Key Set (RFC 7517).
type JWKSet struct {
	Keys []JWK `json:"keys"`
}

// JWK is a JSON Web Key for an RSA public key.
type JWK struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// OIDCProvider exposes the OIDC discovery and JWKS endpoints so that
// third-party verifiers can discover how to validate Nexus Task Tokens.
type OIDCProvider struct {
	issuerURL string
	tokens    *TokenIssuer
}

// NewOIDCProvider creates an OIDCProvider.
func NewOIDCProvider(issuerURL string, tokens *TokenIssuer) *OIDCProvider {
	return &OIDCProvider{issuerURL: issuerURL, tokens: tokens}
}

// RegisterWellKnown attaches the discovery and JWKS routes to the engine.
func (p *OIDCProvider) RegisterWellKnown(engine *gin.Engine) {
	engine.GET("/.well-known/openid-configuration", p.discoveryHandler)
	engine.GET("/.well-known/jwks.json", p.jwksHandler)
}

func (p *OIDCProvider) discoveryHandler(c *gin.Context) {
	c.JSON(http.StatusOK, OIDCConfig{
		Issuer:                           p.issuerURL,
		JWKSURI:                          p.issuerURL + "/.well-known/jwks.json",
		TokenEndpoint:                    p.issuerURL + "/api/v1/token",
		ResponseTypesSupported:           []string{"token"},
		SubjectTypesSupported:            []string{"public"},
		IDTokenSigningAlgValuesSupported: []string{"RS256"},
		GrantTypesSupported:              []string{"client_credentials"},
	})
}

func (p *OIDCProvider) jwksHandler(c *gin.Context) {
	jwk := rsaPublicKeyToJWK(p.tokens.PublicKey(), "nexus-signing-key-1")
	c.JSON(http.StatusOK, JWKSet{Keys: []JWK{jwk}})
}

// rsaPublicKeyToJWK encodes an RSA public key as a JWK (RFC 7518 §6.3).
func rsaPublicKeyToJWK(pub *rsa.PublicKey, kid string) JWK {
	nBytes := pub.N.Bytes()
	n := base64.RawURLEncoding.EncodeToString(nBytes)

	// Encode exponent as big-endian, minimal-length byte slice.
	eBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(eBuf, uint64(pub.E))
	i := 0
	for i < len(eBuf)-1 && eBuf[i] == 0 {
		i++
	}
	e := base64.RawURLEncoding.EncodeToString(eBuf[i:])

	return JWK{Kty: "RSA", Use: "sig", Kid: kid, Alg: "RS256", N: n, E: e}
}
