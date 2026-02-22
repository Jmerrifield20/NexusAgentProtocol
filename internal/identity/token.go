package identity

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TaskTokenClaims are the JWT claims for a Nexus Task Token.
// Task Tokens are short-lived credentials bound to a specific agent identity
// and a set of scopes, issued after successful mTLS authentication.
type TaskTokenClaims struct {
	jwt.RegisteredClaims
	AgentURI string   `json:"agent_uri"`
	Scopes   []string `json:"scopes"`
}

// TokenIssuer issues and verifies Task Tokens signed with RS256.
// It reuses the CA's RSA key so that token signatures can be verified
// using the same JWKS endpoint that serves the CA public key.
type TokenIssuer struct {
	key    *rsa.PrivateKey
	pub    *rsa.PublicKey
	issuer string
	ttl    time.Duration
}

// NewTokenIssuer creates a TokenIssuer.
//
//	issuerURL — The "iss" claim value; typically the registry's base URL.
//	ttl        — Token lifetime (default: 1 hour).
func NewTokenIssuer(key *rsa.PrivateKey, issuerURL string, ttl time.Duration) *TokenIssuer {
	if ttl == 0 {
		ttl = time.Hour
	}
	return &TokenIssuer{
		key:    key,
		pub:    &key.PublicKey,
		issuer: issuerURL,
		ttl:    ttl,
	}
}

// Issue creates a signed Task Token for agentURI with the requested scopes.
func (t *TokenIssuer) Issue(agentURI string, scopes []string) (string, error) {
	now := time.Now().UTC()
	claims := TaskTokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    t.issuer,
			Subject:   agentURI,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(t.ttl)),
			ID:        uuid.New().String(),
		},
		AgentURI: agentURI,
		Scopes:   scopes,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(t.key)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

// Verify parses and validates a Task Token, returning its claims on success.
func (t *TokenIssuer) Verify(tokenStr string) (*TaskTokenClaims, error) {
	token, err := jwt.ParseWithClaims(
		tokenStr,
		&TaskTokenClaims{},
		func(tok *jwt.Token) (any, error) {
			if _, ok := tok.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", tok.Header["alg"])
			}
			return t.pub, nil
		},
		jwt.WithIssuer(t.issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, fmt.Errorf("verify token: %w", err)
	}

	claims, ok := token.Claims.(*TaskTokenClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

// PublicKey returns the RSA public key used to verify tokens.
func (t *TokenIssuer) PublicKey() *rsa.PublicKey { return t.pub }

// PublicKeyPEM returns the RSA public key in PKIX PEM format.
func (t *TokenIssuer) PublicKeyPEM() (string, error) {
	der, err := x509.MarshalPKIXPublicKey(t.pub)
	if err != nil {
		return "", fmt.Errorf("marshal public key: %w", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})), nil
}

// TTL returns the configured token lifetime.
func (t *TokenIssuer) TTL() time.Duration { return t.ttl }

// NAPEndorsementClaims are the JWT claims for a NAP agent endorsement.
// The endorsement is embedded in the agent's A2A card (nap:endorsement field)
// and can be verified by any party with access to the registry's JWKS endpoint.
type NAPEndorsementClaims struct {
	jwt.RegisteredClaims
	AgentURI   string `json:"nap:uri"`
	TrustTier  string `json:"nap:trust_tier"`
	CertSerial string `json:"nap:cert_serial,omitempty"`
	Registry   string `json:"nap:registry"`
}

// IssueEndorsement creates a signed NAP endorsement JWT for an agent.
// The JWT is intended for embedding in an A2A agent card as the nap:endorsement field.
// validFor defaults to 365 days when zero.
func (t *TokenIssuer) IssueEndorsement(agentURI, trustTier, certSerial, registry string, validFor time.Duration) (string, error) {
	if validFor == 0 {
		validFor = 365 * 24 * time.Hour
	}
	now := time.Now().UTC()
	claims := NAPEndorsementClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    t.issuer,
			Subject:   agentURI,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(validFor)),
			ID:        uuid.New().String(),
		},
		AgentURI:   agentURI,
		TrustTier:  trustTier,
		CertSerial: certSerial,
		Registry:   registry,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(t.key)
	if err != nil {
		return "", fmt.Errorf("sign endorsement: %w", err)
	}
	return signed, nil
}
