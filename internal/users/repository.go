package users

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a user lookup finds no matching record.
var ErrNotFound = errors.New("user not found")

// ErrDuplicateEmail is returned when a signup attempts to use an already-registered email.
var ErrDuplicateEmail = errors.New("email already registered")

// ErrDuplicateUsername is returned when the generated username is already taken.
var ErrDuplicateUsername = errors.New("username already taken")

// UserRepository provides CRUD operations for users against PostgreSQL.
type UserRepository struct {
	db *pgxpool.Pool
}

// NewUserRepository creates a new UserRepository.
func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

// Create inserts a new user record. Sets ID, CreatedAt, UpdatedAt on the user.
func (r *UserRepository) Create(ctx context.Context, u *User) error {
	u.ID = uuid.New()
	now := time.Now().UTC()
	u.CreatedAt = now
	u.UpdatedAt = now

	q := `
		INSERT INTO users (id, email, password_hash, display_name, username, email_verified, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err := r.db.Exec(ctx, q,
		u.ID, u.Email, u.PasswordHash, u.DisplayName, u.Username,
		u.EmailVerified, u.CreatedAt, u.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if pgErr.ConstraintName == "users_email_key" {
				return ErrDuplicateEmail
			}
			return ErrDuplicateUsername
		}
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// GetByID retrieves a user by their internal UUID.
func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	return r.scanOne(ctx, `SELECT * FROM users WHERE id = $1`, id)
}

// GetByEmail retrieves a user by their email address.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	return r.scanOne(ctx, `SELECT * FROM users WHERE email = $1`, email)
}

// GetByUsername retrieves a user by their username slug.
func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*User, error) {
	return r.scanOne(ctx, `SELECT * FROM users WHERE username = $1`, username)
}

// GetByOAuth retrieves a user linked to the given OAuth provider identity.
func (r *UserRepository) GetByOAuth(ctx context.Context, provider, providerID string) (*User, error) {
	q := `
		SELECT u.* FROM users u
		JOIN user_oauth o ON o.user_id = u.id
		WHERE o.provider = $1 AND o.provider_id = $2`
	return r.scanOne(ctx, q, provider, providerID)
}

// LinkOAuth adds an OAuth provider link to an existing user account.
// Silently ignores duplicate links.
func (r *UserRepository) LinkOAuth(ctx context.Context, userID uuid.UUID, provider, providerID string) error {
	q := `
		INSERT INTO user_oauth (id, user_id, provider, provider_id, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (provider, provider_id) DO NOTHING`
	_, err := r.db.Exec(ctx, q, uuid.New(), userID, provider, providerID, time.Now().UTC())
	return err
}

// SetEmailVerified marks the user's email as verified.
func (r *UserRepository) SetEmailVerified(ctx context.Context, userID uuid.UUID) error {
	q := `UPDATE users SET email_verified = true, updated_at = $2 WHERE id = $1`
	_, err := r.db.Exec(ctx, q, userID, time.Now().UTC())
	return err
}

// CreateVerificationToken stores a new email-verification token for the user.
func (r *UserRepository) CreateVerificationToken(ctx context.Context, userID uuid.UUID, token string, expires time.Time) error {
	return r.createToken(ctx, userID, token, "email_verification", expires)
}

// CreatePasswordResetToken stores a new password-reset token for the user.
func (r *UserRepository) CreatePasswordResetToken(ctx context.Context, userID uuid.UUID, token string, expires time.Time) error {
	return r.createToken(ctx, userID, token, "password_reset", expires)
}

// createToken is the shared implementation for both token types.
func (r *UserRepository) createToken(ctx context.Context, userID uuid.UUID, token, tokenType string, expires time.Time) error {
	q := `
		INSERT INTO email_verifications (id, user_id, token, token_type, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := r.db.Exec(ctx, q, uuid.New(), userID, token, tokenType, expires, time.Now().UTC())
	return err
}

// UseVerificationToken atomically marks an email-verification token as used,
// sets email_verified = true on the user, and returns the verified user.
// Returns ErrNotFound for unknown or wrong-type tokens.
func (r *UserRepository) UseVerificationToken(ctx context.Context, token string) (*User, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var userID uuid.UUID
	var expiresAt time.Time
	var usedAt *time.Time
	q := `SELECT user_id, expires_at, used_at FROM email_verifications WHERE token = $1 AND token_type = 'email_verification'`
	if err := tx.QueryRow(ctx, q, token).Scan(&userID, &expiresAt, &usedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("query verification token: %w", err)
	}

	if usedAt != nil {
		return nil, fmt.Errorf("verification token already used")
	}
	if time.Now().After(expiresAt) {
		return nil, fmt.Errorf("verification token expired")
	}

	now := time.Now().UTC()
	if _, err := tx.Exec(ctx,
		`UPDATE email_verifications SET used_at = $2 WHERE token = $1`, token, now,
	); err != nil {
		return nil, fmt.Errorf("mark token used: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE users SET email_verified = true, updated_at = $2 WHERE id = $1`, userID, now,
	); err != nil {
		return nil, fmt.Errorf("set email verified: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return r.GetByID(ctx, userID)
}

// UsePasswordResetToken atomically marks a password-reset token as used and
// returns the owning user. Unlike UseVerificationToken it does NOT touch
// email_verified — that is unchanged by a password reset.
// Returns ErrNotFound for unknown, wrong-type, already-used, or expired tokens.
func (r *UserRepository) UsePasswordResetToken(ctx context.Context, token string) (*User, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var userID uuid.UUID
	var expiresAt time.Time
	var usedAt *time.Time
	q := `SELECT user_id, expires_at, used_at FROM email_verifications WHERE token = $1 AND token_type = 'password_reset'`
	if err := tx.QueryRow(ctx, q, token).Scan(&userID, &expiresAt, &usedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("query reset token: %w", err)
	}

	if usedAt != nil {
		return nil, fmt.Errorf("password reset token already used")
	}
	if time.Now().After(expiresAt) {
		return nil, fmt.Errorf("password reset token expired")
	}

	now := time.Now().UTC()
	if _, err := tx.Exec(ctx,
		`UPDATE email_verifications SET used_at = $2 WHERE token = $1`, token, now,
	); err != nil {
		return nil, fmt.Errorf("mark token used: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return r.GetByID(ctx, userID)
}

// SetPasswordHash updates a user's password hash.
func (r *UserRepository) SetPasswordHash(ctx context.Context, userID uuid.UUID, hash string) error {
	q := `UPDATE users SET password_hash = $2, updated_at = $3 WHERE id = $1`
	_, err := r.db.Exec(ctx, q, userID, hash, time.Now().UTC())
	return err
}

// UpdateProfile updates the bio, avatar_url, and website_url for a user.
func (r *UserRepository) UpdateProfile(ctx context.Context, userID uuid.UUID, bio, avatarURL, websiteURL string) error {
	q := `UPDATE users SET bio = $2, avatar_url = $3, website_url = $4, updated_at = $5 WHERE id = $1`
	_, err := r.db.Exec(ctx, q, userID, bio, avatarURL, websiteURL, time.Now().UTC())
	return err
}

// scanOne executes a single-row query and scans the result into a User.
// Column order: id, email, password_hash, display_name, username, email_verified,
// created_at, updated_at, bio, avatar_url, website_url, public_profile
// (matches the schema definition order; migration 008 appended the last four).
func (r *UserRepository) scanOne(ctx context.Context, q string, args ...any) (*User, error) {
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, ErrNotFound
	}

	var u User
	if err := rows.Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName, &u.Username,
		&u.EmailVerified, &u.CreatedAt, &u.UpdatedAt,
		&u.Bio, &u.AvatarURL, &u.WebsiteURL, &u.PublicProfile,
	); err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	return &u, rows.Err()
}
