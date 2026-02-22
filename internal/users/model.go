package users

import (
	"time"

	"github.com/google/uuid"
)

// UserTier represents the subscription tier for a user account.
type UserTier string

const (
	TierFree       UserTier = "free"
	TierPro        UserTier = "pro"
	TierEnterprise UserTier = "enterprise"
)

// User represents an authenticated NAP account holder.
type User struct {
	ID            uuid.UUID `json:"id"             db:"id"`
	Email         string    `json:"email"          db:"email"`
	PasswordHash  string    `json:"-"              db:"password_hash"`
	DisplayName   string    `json:"display_name"   db:"display_name"`
	Username      string    `json:"username"       db:"username"`
	Tier          UserTier  `json:"tier"           db:"tier"`
	EmailVerified bool      `json:"email_verified" db:"email_verified"`
	CreatedAt     time.Time `json:"created_at"     db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"     db:"updated_at"`
}

// OAuthAccount links a user to an OAuth provider identity.
type OAuthAccount struct {
	ID         uuid.UUID `json:"id"          db:"id"`
	UserID     uuid.UUID `json:"user_id"     db:"user_id"`
	Provider   string    `json:"provider"    db:"provider"`
	ProviderID string    `json:"provider_id" db:"provider_id"`
	CreatedAt  time.Time `json:"created_at"  db:"created_at"`
}
