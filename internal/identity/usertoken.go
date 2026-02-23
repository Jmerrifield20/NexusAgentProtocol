package identity

import (
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// UserTokenClaims are the JWT claims for a Nexus user session token.
// These are separate from TaskTokenClaims, which are agent-specific.
type UserTokenClaims struct {
	jwt.RegisteredClaims
	UserID   string `json:"user_id"`
	Email    string `json:"email"`
	Username string `json:"username"`
	Type     string `json:"type"` // "user", "admin", or "oauth-state"
	Role     string `json:"role,omitempty"` // "admin" when set
}

// UserTokenIssuer issues and verifies user session JWTs using the Nexus CA RSA key.
type UserTokenIssuer struct {
	key    *rsa.PrivateKey
	pub    *rsa.PublicKey
	issuer string
	ttl    time.Duration
}

// NewUserTokenIssuer creates a UserTokenIssuer.
//
//	issuerURL — The "iss" claim value; matches the registry's base URL.
//	ttl        — Token lifetime (default: 24 hours).
func NewUserTokenIssuer(key *rsa.PrivateKey, issuerURL string, ttl time.Duration) *UserTokenIssuer {
	if ttl == 0 {
		ttl = 24 * time.Hour
	}
	return &UserTokenIssuer{
		key:    key,
		pub:    &key.PublicKey,
		issuer: issuerURL,
		ttl:    ttl,
	}
}

// Issue creates a signed user session token.
func (u *UserTokenIssuer) Issue(userID, email, username string) (string, error) {
	now := time.Now().UTC()
	claims := UserTokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    u.issuer,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(u.ttl)),
			ID:        uuid.New().String(),
		},
		UserID:   userID,
		Email:    email,
		Username: username,
		Type:     "user",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(u.key)
	if err != nil {
		return "", fmt.Errorf("sign user token: %w", err)
	}
	return signed, nil
}

// Verify parses and validates a user session token, returning its claims.
func (u *UserTokenIssuer) Verify(tokenStr string) (*UserTokenClaims, error) {
	token, err := jwt.ParseWithClaims(
		tokenStr,
		&UserTokenClaims{},
		func(tok *jwt.Token) (any, error) {
			if _, ok := tok.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", tok.Header["alg"])
			}
			return u.pub, nil
		},
		jwt.WithIssuer(u.issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, fmt.Errorf("verify user token: %w", err)
	}
	claims, ok := token.Claims.(*UserTokenClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid user token claims")
	}
	if claims.Type != "user" && claims.Type != "admin" {
		return nil, fmt.Errorf("not a user session token")
	}
	return claims, nil
}

// IssueAdminToken creates a signed admin token. Admin tokens carry Type="admin"
// and Role="admin" and are used to authenticate federation management endpoints.
// They are issued only in exchange for the static admin secret — never via OAuth or password.
func (u *UserTokenIssuer) IssueAdminToken(ttl time.Duration) (string, error) {
	if ttl == 0 {
		ttl = 8 * time.Hour
	}
	now := time.Now().UTC()
	claims := UserTokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    u.issuer,
			Subject:   "admin",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			ID:        uuid.New().String(),
		},
		UserID: "admin",
		Type:   "admin",
		Role:   "admin",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(u.key)
	if err != nil {
		return "", fmt.Errorf("sign admin token: %w", err)
	}
	return signed, nil
}

// IssueOAuthState creates a short-lived JWT used as the OAuth state parameter.
// The provider name is embedded in the token so the callback can verify it.
func (u *UserTokenIssuer) IssueOAuthState(provider string) (string, error) {
	now := time.Now().UTC()
	claims := UserTokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    u.issuer,
			Subject:   "oauth-state",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
			ID:        uuid.New().String(),
		},
		UserID: provider, // encode provider in UserID field
		Type:   "oauth-state",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(u.key)
	if err != nil {
		return "", fmt.Errorf("sign oauth state: %w", err)
	}
	return signed, nil
}

// VerifyOAuthState validates an OAuth state JWT and returns the embedded provider.
func (u *UserTokenIssuer) VerifyOAuthState(tokenStr string) (provider string, err error) {
	token, err := jwt.ParseWithClaims(
		tokenStr,
		&UserTokenClaims{},
		func(tok *jwt.Token) (any, error) {
			if _, ok := tok.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return u.pub, nil
		},
		jwt.WithIssuer(u.issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return "", fmt.Errorf("invalid oauth state: %w", err)
	}
	claims, ok := token.Claims.(*UserTokenClaims)
	if !ok || claims.Type != "oauth-state" {
		return "", fmt.Errorf("not an oauth state token")
	}
	return claims.UserID, nil
}
