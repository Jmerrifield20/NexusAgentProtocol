package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/identity"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/registry/handler"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/users"
	"go.uber.org/zap"
)

// ── Stub UserService ──────────────────────────────────────────────────────

type stubUserSvc struct {
	signupUser  *users.User
	signupErr   error
	loginUser   *users.User
	loginErr    error
	verifyUser  *users.User
	verifyErr   error
	resendErr   error
	oauthUser   *users.User
	oauthNew    bool
	oauthErr    error
}

func (s *stubUserSvc) Signup(_ context.Context, email, _, _ string) (*users.User, string, error) {
	if s.signupErr != nil {
		return nil, "", s.signupErr
	}
	if s.signupUser != nil {
		return s.signupUser, "tok-"+email, nil
	}
	u := &users.User{
		ID:       uuid.New(),
		Email:    email,
		Username: "alice",
		Tier:     users.TierFree,
	}
	return u, "tok-" + email, nil
}

func (s *stubUserSvc) Login(_ context.Context, email, _ string) (*users.User, error) {
	if s.loginErr != nil {
		return nil, s.loginErr
	}
	if s.loginUser != nil {
		return s.loginUser, nil
	}
	return &users.User{ID: uuid.New(), Email: email, Username: "alice", Tier: users.TierFree}, nil
}

func (s *stubUserSvc) VerifyEmail(_ context.Context, _ string) (*users.User, error) {
	if s.verifyErr != nil {
		return nil, s.verifyErr
	}
	if s.verifyUser != nil {
		return s.verifyUser, nil
	}
	return &users.User{ID: uuid.New(), EmailVerified: true}, nil
}

func (s *stubUserSvc) ResendVerification(_ context.Context, _ uuid.UUID) error {
	return s.resendErr
}

func (s *stubUserSvc) ResendVerificationByEmail(_ context.Context, _ string) error { return nil }
func (s *stubUserSvc) ForgotPassword(_ context.Context, _ string) error            { return nil }
func (s *stubUserSvc) ResetPassword(_ context.Context, _, _ string) error          { return nil }

func (s *stubUserSvc) GetOrCreateFromOAuth(_ context.Context, _, _, email, _ string) (*users.User, bool, error) {
	if s.oauthErr != nil {
		return nil, false, s.oauthErr
	}
	if s.oauthUser != nil {
		return s.oauthUser, s.oauthNew, nil
	}
	return &users.User{ID: uuid.New(), Email: email, Username: "alice", Tier: users.TierFree}, true, nil
}

// ── Test setup ────────────────────────────────────────────────────────────

func setupAuthRouter(t *testing.T, svc *stubUserSvc) (*gin.Engine, *identity.UserTokenIssuer) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	ca := testCA(t)
	userTokens := identity.NewUserTokenIssuer(ca.Key(), "http://test", time.Hour)

	h := handler.NewAuthHandler(svc, userTokens, nil, zap.NewNop())

	r := gin.New()
	v1 := r.Group("/api/v1")
	h.Register(v1)
	return r, userTokens
}

// ── Tests ─────────────────────────────────────────────────────────────────

func TestSignup_201(t *testing.T) {
	router, _ := setupAuthRouter(t, &stubUserSvc{})

	body := `{"email":"alice@example.com","password":"password123","display_name":"Alice"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["token"] == nil {
		t.Error("expected token in response")
	}
	if resp["user"] == nil {
		t.Error("expected user in response")
	}
}

func TestSignup_400_missingEmail(t *testing.T) {
	router, _ := setupAuthRouter(t, &stubUserSvc{})

	body := `{"password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSignup_409_duplicateEmail(t *testing.T) {
	router, _ := setupAuthRouter(t, &stubUserSvc{signupErr: users.ErrDuplicateEmail})

	body := `{"email":"alice@example.com","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLogin_200(t *testing.T) {
	router, _ := setupAuthRouter(t, &stubUserSvc{})

	body := `{"email":"alice@example.com","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["token"] == nil {
		t.Error("expected token in response")
	}
}

func TestLogin_401_badCredentials(t *testing.T) {
	router, _ := setupAuthRouter(t, &stubUserSvc{loginErr: errors.New("invalid credentials")})

	body := `{"email":"alice@example.com","password":"wrong"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestVerifyEmail_200_fromBody(t *testing.T) {
	router, _ := setupAuthRouter(t, &stubUserSvc{})

	body := `{"token":"valid-token-abc"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify-email", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVerifyEmail_200_fromQueryParam(t *testing.T) {
	router, _ := setupAuthRouter(t, &stubUserSvc{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify-email?token=valid-token-abc", bytes.NewReader(nil))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVerifyEmail_400_invalidToken(t *testing.T) {
	router, _ := setupAuthRouter(t, &stubUserSvc{verifyErr: errors.New("verification token not found")})

	body := `{"token":"bad-token"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify-email", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestOAuthRedirect_422_unconfiguredProvider(t *testing.T) {
	router, _ := setupAuthRouter(t, &stubUserSvc{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/github", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
}
