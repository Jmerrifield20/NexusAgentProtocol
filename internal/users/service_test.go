package users_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nexus-protocol/nexus/internal/users"
	"go.uber.org/zap"
)

// ── Stub repo ─────────────────────────────────────────────────────────────

type stubUserRepo struct {
	mu              sync.RWMutex
	byID            map[uuid.UUID]*users.User
	byEmail         map[string]uuid.UUID
	byUsername      map[string]uuid.UUID
	oauthLinks      map[string]uuid.UUID // "provider:providerID" → userID
	verifyTokens    map[string]*verifyTokenRecord
}

type verifyTokenRecord struct {
	userID    uuid.UUID
	expiresAt time.Time
	usedAt    *time.Time
}

func newStubUserRepo() *stubUserRepo {
	return &stubUserRepo{
		byID:         make(map[uuid.UUID]*users.User),
		byEmail:      make(map[string]uuid.UUID),
		byUsername:   make(map[string]uuid.UUID),
		oauthLinks:   make(map[string]uuid.UUID),
		verifyTokens: make(map[string]*verifyTokenRecord),
	}
}

func (r *stubUserRepo) Create(_ context.Context, u *users.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byEmail[u.Email]; exists {
		return users.ErrDuplicateEmail
	}
	u.ID = uuid.New()
	now := time.Now()
	u.CreatedAt = now
	u.UpdatedAt = now
	cp := *u
	r.byID[u.ID] = &cp
	r.byEmail[u.Email] = u.ID
	r.byUsername[u.Username] = u.ID
	return nil
}

func (r *stubUserRepo) GetByID(_ context.Context, id uuid.UUID) (*users.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.byID[id]
	if !ok {
		return nil, users.ErrNotFound
	}
	cp := *u
	return &cp, nil
}

func (r *stubUserRepo) GetByEmail(_ context.Context, email string) (*users.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.byEmail[email]
	if !ok {
		return nil, users.ErrNotFound
	}
	cp := *r.byID[id]
	return &cp, nil
}

func (r *stubUserRepo) GetByUsername(_ context.Context, username string) (*users.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.byUsername[username]
	if !ok {
		return nil, users.ErrNotFound
	}
	cp := *r.byID[id]
	return &cp, nil
}

func (r *stubUserRepo) GetByOAuth(_ context.Context, provider, providerID string) (*users.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	key := provider + ":" + providerID
	id, ok := r.oauthLinks[key]
	if !ok {
		return nil, users.ErrNotFound
	}
	cp := *r.byID[id]
	return &cp, nil
}

func (r *stubUserRepo) LinkOAuth(_ context.Context, userID uuid.UUID, provider, providerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.oauthLinks[provider+":"+providerID] = userID
	return nil
}

func (r *stubUserRepo) SetEmailVerified(_ context.Context, userID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if u, ok := r.byID[userID]; ok {
		u.EmailVerified = true
	}
	return nil
}

func (r *stubUserRepo) CreateVerificationToken(_ context.Context, userID uuid.UUID, token string, expires time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.verifyTokens[token] = &verifyTokenRecord{userID: userID, expiresAt: expires}
	return nil
}

func (r *stubUserRepo) UseVerificationToken(ctx context.Context, token string) (*users.User, error) {
	r.mu.Lock()
	rec, ok := r.verifyTokens[token]
	if !ok {
		r.mu.Unlock()
		return nil, users.ErrNotFound
	}
	if rec.usedAt != nil {
		r.mu.Unlock()
		return nil, errors.New("token already used")
	}
	if time.Now().After(rec.expiresAt) {
		r.mu.Unlock()
		return nil, errors.New("token expired")
	}
	now := time.Now()
	rec.usedAt = &now
	if u, ok := r.byID[rec.userID]; ok {
		u.EmailVerified = true
	}
	userID := rec.userID
	r.mu.Unlock()
	return r.GetByID(ctx, userID)
}

func (r *stubUserRepo) SetPasswordHash(_ context.Context, userID uuid.UUID, hash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if u, ok := r.byID[userID]; ok {
		u.PasswordHash = hash
	}
	return nil
}

func (r *stubUserRepo) CreatePasswordResetToken(_ context.Context, userID uuid.UUID, token string, expires time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.verifyTokens["reset:"+token] = &verifyTokenRecord{userID: userID, expiresAt: expires}
	return nil
}

func (r *stubUserRepo) UsePasswordResetToken(ctx context.Context, token string) (*users.User, error) {
	r.mu.Lock()
	rec, ok := r.verifyTokens["reset:"+token]
	if !ok {
		r.mu.Unlock()
		return nil, users.ErrNotFound
	}
	if rec.usedAt != nil {
		r.mu.Unlock()
		return nil, errors.New("token already used")
	}
	if time.Now().After(rec.expiresAt) {
		r.mu.Unlock()
		return nil, errors.New("token expired")
	}
	now := time.Now()
	rec.usedAt = &now
	userID := rec.userID
	r.mu.Unlock()
	return r.GetByID(ctx, userID)
}

// ── Noop email sender ─────────────────────────────────────────────────────

type noopMailer struct{}

func (n *noopMailer) Send(_ context.Context, _, _, _ string) error { return nil }

// ── Helper ────────────────────────────────────────────────────────────────

func newTestUserService(repo *stubUserRepo) *users.UserService {
	return users.NewUserService(repo, &noopMailer{}, "http://localhost:8080", zap.NewNop())
}

// ── Tests ─────────────────────────────────────────────────────────────────

func TestSignup_success(t *testing.T) {
	repo := newStubUserRepo()
	svc := newTestUserService(repo)

	u, token, err := svc.Signup(context.Background(), "alice@example.com", "password123", "Alice")
	if err != nil {
		t.Fatalf("Signup: %v", err)
	}
	if u.Email != "alice@example.com" {
		t.Errorf("email mismatch: %s", u.Email)
	}
	if u.Username == "" {
		t.Error("expected non-empty username")
	}
	if u.Tier != users.TierFree {
		t.Errorf("expected free tier, got %s", u.Tier)
	}
	if u.EmailVerified {
		t.Error("email should not be verified immediately")
	}
	if token == "" {
		t.Error("expected a verification token")
	}
}

func TestSignup_duplicateEmail(t *testing.T) {
	repo := newStubUserRepo()
	svc := newTestUserService(repo)

	_, _, err := svc.Signup(context.Background(), "alice@example.com", "password123", "Alice")
	if err != nil {
		t.Fatalf("first signup: %v", err)
	}

	_, _, err = svc.Signup(context.Background(), "alice@example.com", "password456", "Alice2")
	if !errors.Is(err, users.ErrDuplicateEmail) {
		t.Errorf("expected ErrDuplicateEmail, got %v", err)
	}
}

func TestSignup_shortPassword(t *testing.T) {
	svc := newTestUserService(newStubUserRepo())
	_, _, err := svc.Signup(context.Background(), "bob@example.com", "short", "Bob")
	if err == nil {
		t.Error("expected error for short password")
	}
}

func TestLogin_success(t *testing.T) {
	repo := newStubUserRepo()
	svc := newTestUserService(repo)

	svc.Signup(context.Background(), "alice@example.com", "password123", "Alice")

	u, err := svc.Login(context.Background(), "alice@example.com", "password123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if u.Email != "alice@example.com" {
		t.Errorf("email mismatch: %s", u.Email)
	}
}

func TestLogin_wrongPassword(t *testing.T) {
	repo := newStubUserRepo()
	svc := newTestUserService(repo)
	svc.Signup(context.Background(), "alice@example.com", "password123", "Alice")

	_, err := svc.Login(context.Background(), "alice@example.com", "wrongpass")
	if err == nil {
		t.Error("expected error for wrong password")
	}
}

func TestLogin_unknownUser(t *testing.T) {
	svc := newTestUserService(newStubUserRepo())
	_, err := svc.Login(context.Background(), "nobody@example.com", "password123")
	if err == nil {
		t.Error("expected error for unknown user")
	}
}

func TestVerifyEmail_success(t *testing.T) {
	repo := newStubUserRepo()
	svc := newTestUserService(repo)

	_, token, _ := svc.Signup(context.Background(), "alice@example.com", "password123", "Alice")

	u, err := svc.VerifyEmail(context.Background(), token)
	if err != nil {
		t.Fatalf("VerifyEmail: %v", err)
	}
	if !u.EmailVerified {
		t.Error("expected email_verified = true")
	}
}

func TestVerifyEmail_invalidToken(t *testing.T) {
	svc := newTestUserService(newStubUserRepo())
	_, err := svc.VerifyEmail(context.Background(), "bad-token")
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestIsEmailVerified_returnsFalseBeforeVerification(t *testing.T) {
	repo := newStubUserRepo()
	svc := newTestUserService(repo)

	u, _, _ := svc.Signup(context.Background(), "alice@example.com", "password123", "Alice")

	verified, err := svc.IsEmailVerified(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("IsEmailVerified: %v", err)
	}
	if verified {
		t.Error("expected false before verification")
	}
}

func TestIsEmailVerified_returnsTrueAfterVerification(t *testing.T) {
	repo := newStubUserRepo()
	svc := newTestUserService(repo)

	u, token, _ := svc.Signup(context.Background(), "alice@example.com", "password123", "Alice")
	svc.VerifyEmail(context.Background(), token)

	verified, err := svc.IsEmailVerified(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("IsEmailVerified: %v", err)
	}
	if !verified {
		t.Error("expected true after verification")
	}
}

func TestGetOrCreateFromOAuth_createsNewUser(t *testing.T) {
	repo := newStubUserRepo()
	svc := newTestUserService(repo)

	u, created, err := svc.GetOrCreateFromOAuth(context.Background(), "github", "12345", "bob@github.com", "Bob")
	if err != nil {
		t.Fatalf("GetOrCreateFromOAuth: %v", err)
	}
	if !created {
		t.Error("expected created=true for new OAuth user")
	}
	if u.Email != "bob@github.com" {
		t.Errorf("email mismatch: %s", u.Email)
	}
	if !u.EmailVerified {
		t.Error("OAuth users should have email verified")
	}
}

func TestGetOrCreateFromOAuth_returnsExistingUser(t *testing.T) {
	repo := newStubUserRepo()
	svc := newTestUserService(repo)

	svc.GetOrCreateFromOAuth(context.Background(), "github", "12345", "bob@github.com", "Bob")
	u2, created, err := svc.GetOrCreateFromOAuth(context.Background(), "github", "12345", "bob@github.com", "Bob")
	if err != nil {
		t.Fatalf("second GetOrCreateFromOAuth: %v", err)
	}
	if created {
		t.Error("expected created=false for existing OAuth user")
	}
	if u2.Email != "bob@github.com" {
		t.Errorf("email mismatch: %s", u2.Email)
	}
}

func TestSignup_usernameSlugified(t *testing.T) {
	repo := newStubUserRepo()
	svc := newTestUserService(repo)

	u, _, _ := svc.Signup(context.Background(), "alice.smith+tag@example.com", "password123", "")
	if u.Username == "" {
		t.Error("expected non-empty username")
	}
	// Username should be lowercase alphanumeric
	for _, r := range u.Username {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			t.Errorf("username contains invalid character %q: %s", r, u.Username)
		}
	}
}

func TestSignup_deduplicatesUsername(t *testing.T) {
	repo := newStubUserRepo()
	svc := newTestUserService(repo)

	u1, _, _ := svc.Signup(context.Background(), "alice@example.com", "password123", "Alice")
	u2, _, _ := svc.Signup(context.Background(), "alice@other.com", "password123", "Alice2")

	if u1.Username == u2.Username {
		t.Errorf("expected unique usernames, both got %q", u1.Username)
	}
}
