package users

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/email"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// userRepo is the storage interface consumed by UserService.
type userRepo interface {
	Create(ctx context.Context, u *User) error
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByUsername(ctx context.Context, username string) (*User, error)
	GetByOAuth(ctx context.Context, provider, providerID string) (*User, error)
	LinkOAuth(ctx context.Context, userID uuid.UUID, provider, providerID string) error
	SetEmailVerified(ctx context.Context, userID uuid.UUID) error
	SetPasswordHash(ctx context.Context, userID uuid.UUID, hash string) error
	UpdateProfile(ctx context.Context, userID uuid.UUID, bio, avatarURL, websiteURL string) error
	CreateVerificationToken(ctx context.Context, userID uuid.UUID, token string, expires time.Time) error
	UseVerificationToken(ctx context.Context, token string) (*User, error)
	CreatePasswordResetToken(ctx context.Context, userID uuid.UUID, token string, expires time.Time) error
	UsePasswordResetToken(ctx context.Context, token string) (*User, error)
}

// UserService implements business logic for user account management.
type UserService struct {
	repo        userRepo
	mailer      email.EmailSender
	frontendURL string // base URL of the frontend, used to build verification links
	logger      *zap.Logger
}

// NewUserService creates a new UserService.
func NewUserService(repo userRepo, mailer email.EmailSender, baseURL string, logger *zap.Logger) *UserService {
	return &UserService{repo: repo, mailer: mailer, frontendURL: baseURL, logger: logger}
}

// SetFrontendURL overrides the base URL used to build email verification links.
// Should point to the web frontend (e.g. "http://localhost:3000").
func (s *UserService) SetFrontendURL(url string) {
	s.frontendURL = url
}

// Signup creates a new user with email/password authentication.
// Returns the created user and the raw verification token.
func (s *UserService) Signup(ctx context.Context, emailAddr, password, displayName string) (*User, string, error) {
	if emailAddr == "" || password == "" {
		return nil, "", fmt.Errorf("email and password are required")
	}
	if len(password) < 8 {
		return nil, "", fmt.Errorf("password must be at least 8 characters")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("hash password: %w", err)
	}

	username, err := s.generateUniqueUsername(ctx, emailAddr)
	if err != nil {
		return nil, "", fmt.Errorf("generate username: %w", err)
	}

	if displayName == "" {
		displayName = username
	}

	u := &User{
		Email:        emailAddr,
		PasswordHash: string(hash),
		DisplayName:  displayName,
		Username:     username,
	}

	if err := s.repo.Create(ctx, u); err != nil {
		if errors.Is(err, ErrDuplicateEmail) {
			return nil, "", ErrDuplicateEmail
		}
		return nil, "", fmt.Errorf("create user: %w", err)
	}

	verifyToken, err := s.createAndSendVerification(ctx, u)
	if err != nil {
		// Non-fatal: user is created; they can request a resend
		s.logger.Warn("failed to send verification email",
			zap.String("user_id", u.ID.String()),
			zap.Error(err),
		)
	}

	return u, verifyToken, nil
}

// Login verifies email/password credentials and returns the user on success.
func (s *UserService) Login(ctx context.Context, emailAddr, password string) (*User, error) {
	u, err := s.repo.GetByEmail(ctx, emailAddr)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("invalid credentials")
		}
		return nil, fmt.Errorf("lookup user: %w", err)
	}

	if u.PasswordHash == "" {
		return nil, fmt.Errorf("account uses OAuth login; password not set")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	return u, nil
}

// VerifyEmail consumes a verification token and marks the user's email as verified.
func (s *UserService) VerifyEmail(ctx context.Context, token string) (*User, error) {
	u, err := s.repo.UseVerificationToken(ctx, token)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("verification token not found")
		}
		return nil, fmt.Errorf("verify email: %w", err)
	}

	s.logger.Info("email verified", zap.String("user_id", u.ID.String()))
	return u, nil
}

// GetByID retrieves a user by ID.
func (s *UserService) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	return s.repo.GetByID(ctx, id)
}

// GetByUsername retrieves a user by their username slug.
func (s *UserService) GetByUsername(ctx context.Context, username string) (*User, error) {
	return s.repo.GetByUsername(ctx, username)
}

// GetPublicProfile returns the public-facing profile for the given username.
// Returns ErrNotFound if the user does not exist or has public_profile = false.
func (s *UserService) GetPublicProfile(ctx context.Context, username string) (*PublicProfile, error) {
	u, err := s.repo.GetByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if !u.PublicProfile {
		return nil, ErrNotFound
	}
	return &PublicProfile{
		Username:      u.Username,
		DisplayName:   u.DisplayName,
		Bio:           u.Bio,
		AvatarURL:     u.AvatarURL,
		WebsiteURL:    u.WebsiteURL,
		EmailVerified: u.EmailVerified,
		MemberSince:   u.CreatedAt,
	}, nil
}

// UpdateProfile updates the bio, avatar URL, and website URL for a user.
func (s *UserService) UpdateProfile(ctx context.Context, userID uuid.UUID, bio, avatarURL, websiteURL string) error {
	return s.repo.UpdateProfile(ctx, userID, bio, avatarURL, websiteURL)
}

// IsEmailVerified returns true if the user's email has been verified.
// Satisfies the registry service.UserEmailChecker interface.
func (s *UserService) IsEmailVerified(ctx context.Context, userID uuid.UUID) (bool, error) {
	u, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return false, err
	}
	return u.EmailVerified, nil
}

// GetOrCreateFromOAuth retrieves an existing user linked to the OAuth identity,
// or creates a new one. Returns the user and true if newly created.
func (s *UserService) GetOrCreateFromOAuth(ctx context.Context, provider, providerID, emailAddr, displayName string) (*User, bool, error) {
	// Try existing OAuth link first
	u, err := s.repo.GetByOAuth(ctx, provider, providerID)
	if err == nil {
		return u, false, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, false, fmt.Errorf("lookup oauth user: %w", err)
	}

	// Try by email (link existing account)
	existing, err := s.repo.GetByEmail(ctx, emailAddr)
	if err == nil {
		if linkErr := s.repo.LinkOAuth(ctx, existing.ID, provider, providerID); linkErr != nil {
			s.logger.Warn("link oauth to existing account",
				zap.String("user_id", existing.ID.String()),
				zap.Error(linkErr),
			)
		}
		// OAuth login implies verified email
		if !existing.EmailVerified {
			_ = s.repo.SetEmailVerified(ctx, existing.ID)
			existing.EmailVerified = true
		}
		return existing, false, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, false, fmt.Errorf("lookup by email: %w", err)
	}

	// Create new OAuth user
	username, err := s.generateUniqueUsername(ctx, emailAddr)
	if err != nil {
		return nil, false, fmt.Errorf("generate username: %w", err)
	}
	if displayName == "" {
		displayName = username
	}

	u = &User{
		Email:         emailAddr,
		DisplayName:   displayName,
		Username:      username,
		EmailVerified: true, // OAuth login = email verified by provider
	}
	if err := s.repo.Create(ctx, u); err != nil {
		return nil, false, fmt.Errorf("create oauth user: %w", err)
	}
	if err := s.repo.LinkOAuth(ctx, u.ID, provider, providerID); err != nil {
		s.logger.Warn("link oauth after create", zap.Error(err))
	}

	return u, true, nil
}

// ResendVerification generates a new verification token and sends the email.
func (s *UserService) ResendVerification(ctx context.Context, userID uuid.UUID) error {
	u, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}
	if u.EmailVerified {
		return fmt.Errorf("email already verified")
	}

	_, err = s.createAndSendVerification(ctx, u)
	return err
}

// createAndSendVerification generates a token, persists it, and emails the user.
func (s *UserService) createAndSendVerification(ctx context.Context, u *User) (string, error) {
	token, err := generateSecureToken(32)
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}

	expires := time.Now().UTC().Add(24 * time.Hour)
	if err := s.repo.CreateVerificationToken(ctx, u.ID, token, expires); err != nil {
		return "", fmt.Errorf("persist verification token: %w", err)
	}

	link := s.frontendURL + "/verify-email?token=" + token
	body := fmt.Sprintf(
		"Hello %s,\n\nVerify your NAP account email:\n\n  %s\n\nThis link expires in 24 hours.\n\nIf you did not sign up, ignore this email.\n",
		u.DisplayName, link,
	)
	if err := s.mailer.Send(ctx, u.Email, "Verify your NAP account email", body); err != nil {
		return token, fmt.Errorf("send verification email: %w", err)
	}
	return token, nil
}

// generateUniqueUsername derives a slug from email and appends a suffix if taken.
func (s *UserService) generateUniqueUsername(ctx context.Context, emailAddr string) (string, error) {
	base := slugifyEmail(emailAddr)
	if base == "" {
		base = "user"
	}

	// Try base first
	if _, err := s.repo.GetByUsername(ctx, base); errors.Is(err, ErrNotFound) {
		return base, nil
	}

	// Try with numeric suffixes
	for i := 2; i <= 9999; i++ {
		candidate := fmt.Sprintf("%s%d", base, i)
		if _, err := s.repo.GetByUsername(ctx, candidate); errors.Is(err, ErrNotFound) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not generate unique username for %q", emailAddr)
}

// slugifyEmail converts "alice@example.com" → "alice".
func slugifyEmail(emailAddr string) string {
	local := emailAddr
	if at := strings.Index(emailAddr, "@"); at > 0 {
		local = emailAddr[:at]
	}
	var b strings.Builder
	for _, r := range strings.ToLower(local) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			b.WriteRune(r)
		}
	}
	result := strings.Trim(b.String(), "-")
	if len(result) > 32 {
		result = result[:32]
	}
	return result
}

// ResendVerificationByEmail looks up a user by email and resends the
// verification email if the account exists and is not yet verified.
// Always returns nil — callers must not reveal whether the email is registered.
func (s *UserService) ResendVerificationByEmail(ctx context.Context, emailAddr string) error {
	u, err := s.repo.GetByEmail(ctx, emailAddr)
	if err != nil {
		// User not found or other error — return silently for security.
		return nil
	}
	if u.EmailVerified {
		return nil
	}
	if _, err := s.createAndSendVerification(ctx, u); err != nil {
		s.logger.Warn("resend verification by email failed",
			zap.String("user_id", u.ID.String()),
			zap.Error(err),
		)
	}
	return nil
}

// ForgotPassword generates a password-reset token and emails it to the user.
// Always returns nil — callers must not reveal whether the email is registered.
func (s *UserService) ForgotPassword(ctx context.Context, emailAddr string) error {
	u, err := s.repo.GetByEmail(ctx, emailAddr)
	if err != nil {
		return nil // silent — don't reveal account existence
	}

	if u.PasswordHash == "" {
		// OAuth-only account — send a helpful email instead of a reset link.
		body := fmt.Sprintf(
			"Hello %s,\n\nYour NAP account was created with GitHub or Google — there is no password to reset.\n\nSign in using the OAuth button on the login page.\n",
			u.DisplayName,
		)
		_ = s.mailer.Send(ctx, u.Email, "NAP account — no password set", body)
		return nil
	}

	token, err := generateSecureToken(32)
	if err != nil {
		s.logger.Error("generate password reset token", zap.Error(err))
		return nil
	}

	expires := time.Now().UTC().Add(1 * time.Hour)
	if err := s.repo.CreatePasswordResetToken(ctx, u.ID, token, expires); err != nil {
		s.logger.Error("persist password reset token", zap.Error(err))
		return nil
	}

	link := s.frontendURL + "/reset-password?token=" + token
	body := fmt.Sprintf(
		"Hello %s,\n\nReset your NAP account password:\n\n  %s\n\nThis link expires in 1 hour.\n\nIf you did not request a password reset, ignore this email — your password has not changed.\n",
		u.DisplayName, link,
	)
	if err := s.mailer.Send(ctx, u.Email, "Reset your NAP account password", body); err != nil {
		s.logger.Warn("send password reset email",
			zap.String("user_id", u.ID.String()),
			zap.Error(err),
		)
	}
	return nil
}

// ResetPassword validates a password-reset token and sets the new password.
func (s *UserService) ResetPassword(ctx context.Context, token, newPassword string) error {
	if len(newPassword) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}

	u, err := s.repo.UsePasswordResetToken(ctx, token)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return fmt.Errorf("reset token not found or expired")
		}
		return fmt.Errorf("reset password: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if err := s.repo.SetPasswordHash(ctx, u.ID, string(hash)); err != nil {
		return fmt.Errorf("set password: %w", err)
	}

	s.logger.Info("password reset", zap.String("user_id", u.ID.String()))
	return nil
}

// generateSecureToken returns a hex-encoded random token of the given byte length.
func generateSecureToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
