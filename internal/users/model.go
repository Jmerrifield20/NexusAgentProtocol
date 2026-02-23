package users

import (
	"time"

	"github.com/google/uuid"
)

// User represents an authenticated NAP account holder.
type User struct {
	ID            uuid.UUID `json:"id"             db:"id"`
	Email         string    `json:"email"          db:"email"`
	PasswordHash  string    `json:"-"              db:"password_hash"`
	DisplayName   string    `json:"display_name"   db:"display_name"`
	Username      string    `json:"username"       db:"username"`
	EmailVerified bool      `json:"email_verified" db:"email_verified"`
	CreatedAt     time.Time `json:"created_at"     db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"     db:"updated_at"`
	Bio           string    `json:"bio"            db:"bio"`
	AvatarURL     string    `json:"avatar_url"     db:"avatar_url"`
	WebsiteURL    string    `json:"website_url"    db:"website_url"`
	PublicProfile bool      `json:"public_profile" db:"public_profile"`
}

// PublicProfile is the publicly visible subset of a user's account.
// It does not expose email or password hash.
type PublicProfile struct {
	Username        string    `json:"username"`
	DisplayName     string    `json:"display_name"`
	Bio             string    `json:"bio"`
	AvatarURL       string    `json:"avatar_url"`
	WebsiteURL      string    `json:"website_url"`
	EmailVerified   bool      `json:"email_verified"`
	VerifiedDomains []string  `json:"verified_domains"`
	AgentCount      int       `json:"agent_count"`
	MemberSince     time.Time `json:"member_since"`
}

// OAuthAccount links a user to an OAuth provider identity.
type OAuthAccount struct {
	ID         uuid.UUID `json:"id"          db:"id"`
	UserID     uuid.UUID `json:"user_id"     db:"user_id"`
	Provider   string    `json:"provider"    db:"provider"`
	ProviderID string    `json:"provider_id" db:"provider_id"`
	CreatedAt  time.Time `json:"created_at"  db:"created_at"`
}
