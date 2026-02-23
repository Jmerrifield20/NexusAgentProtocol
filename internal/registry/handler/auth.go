package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/identity"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/users"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

// OAuthProviderConfig holds OAuth client credentials for a single provider.
type OAuthProviderConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// userSvc is the interface expected by AuthHandler, satisfied by *users.UserService.
type userSvc interface {
	Signup(ctx context.Context, email, password, displayName string) (*users.User, string, error)
	Login(ctx context.Context, email, password string) (*users.User, error)
	VerifyEmail(ctx context.Context, token string) (*users.User, error)
	ResendVerification(ctx context.Context, userID uuid.UUID) error
	ResendVerificationByEmail(ctx context.Context, email string) error
	ForgotPassword(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, token, newPassword string) error
	GetOrCreateFromOAuth(ctx context.Context, provider, providerID, email, displayName string) (*users.User, bool, error)
}

// AuthHandler handles user authentication routes.
type AuthHandler struct {
	users       userSvc
	tokens      *identity.UserTokenIssuer
	oauthCfgs   map[string]*oauth2.Config
	frontendURL string // used to redirect after OAuth callback
	logger      *zap.Logger
}

// NewAuthHandler creates an AuthHandler.
// oauthProviders may be nil or empty to disable OAuth routes.
func NewAuthHandler(
	userSvc userSvc,
	tokens *identity.UserTokenIssuer,
	oauthProviders map[string]OAuthProviderConfig,
	logger *zap.Logger,
) *AuthHandler {
	cfgs := buildOAuthConfigs(oauthProviders)
	return &AuthHandler{
		users:       userSvc,
		tokens:      tokens,
		oauthCfgs:   cfgs,
		frontendURL: "http://localhost:3000",
		logger:      logger,
	}
}

// SetFrontendURL sets the base URL of the frontend for OAuth callback redirects.
func (h *AuthHandler) SetFrontendURL(url string) {
	h.frontendURL = url
}

// buildOAuthConfigs converts the raw provider config map into oauth2.Config instances.
func buildOAuthConfigs(providers map[string]OAuthProviderConfig) map[string]*oauth2.Config {
	cfgs := make(map[string]*oauth2.Config)
	for name, p := range providers {
		if p.ClientID == "" || p.ClientSecret == "" {
			continue
		}
		var endpoint oauth2.Endpoint
		var scopes []string
		switch name {
		case "github":
			endpoint = github.Endpoint
			scopes = []string{"user:email"}
		case "google":
			endpoint = google.Endpoint
			scopes = []string{"openid", "email", "profile"}
		default:
			continue
		}
		cfgs[name] = &oauth2.Config{
			ClientID:     p.ClientID,
			ClientSecret: p.ClientSecret,
			RedirectURL:  p.RedirectURL,
			Scopes:       scopes,
			Endpoint:     endpoint,
		}
	}
	return cfgs
}

// Register mounts all auth routes on the provided router group.
func (h *AuthHandler) Register(rg *gin.RouterGroup) {
	auth := rg.Group("/auth")
	{
		auth.POST("/signup", h.Signup)
		auth.POST("/login", h.Login)
		auth.POST("/logout", h.Logout)
		auth.POST("/verify-email", h.VerifyEmail)
		auth.POST("/resend-verification", h.ResendVerification)
		auth.POST("/forgot-password", h.ForgotPassword)
		auth.POST("/reset-password", h.ResetPassword)
		auth.GET("/oauth/:provider", h.OAuthRedirect)
		auth.GET("/oauth/:provider/callback", h.OAuthCallback)
	}
}

// ─── Request / Response types ────────────────────────────────────────────────

type signupRequest struct {
	Email       string `json:"email"        binding:"required,email"`
	Password    string `json:"password"     binding:"required"`
	DisplayName string `json:"display_name"`
}

type loginRequest struct {
	Email    string `json:"email"    binding:"required"`
	Password string `json:"password" binding:"required"`
}

type verifyEmailRequest struct {
	Token string `json:"token" binding:"required"`
}

type resendVerificationRequest struct {
	Email string `json:"email" binding:"required"`
}

type forgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type resetPasswordRequest struct {
	Token    string `json:"token"    binding:"required"`
	Password string `json:"password" binding:"required"`
}

// ─── Handlers ────────────────────────────────────────────────────────────────

// Signup handles POST /auth/signup — creates a new user account.
func (h *AuthHandler) Signup(c *gin.Context) {
	var req signupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	u, _, err := h.users.Signup(c.Request.Context(), req.Email, req.Password, req.DisplayName)
	if err != nil {
		if errors.Is(err, users.ErrDuplicateEmail) {
			c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
			return
		}
		h.logger.Error("signup", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "signup failed"})
		return
	}

	tok, err := h.tokens.Issue(u.ID.String(), u.Email, u.Username, string(u.Tier))
	if err != nil {
		h.logger.Error("issue user token after signup", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token issuance failed"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"user":  u,
		"token": tok,
		"note":  "A verification email has been sent. Verify your email before activating hosted agents.",
	})
}

// Login handles POST /auth/login — authenticates with email/password.
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	u, err := h.users.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	tok, err := h.tokens.Issue(u.ID.String(), u.Email, u.Username, string(u.Tier))
	if err != nil {
		h.logger.Error("issue user token after login", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token issuance failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": u, "token": tok})
}

// VerifyEmail handles POST /auth/verify-email — consumes a verification token.
func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	// Accept token from both JSON body and query parameter
	token := c.Query("token")
	if token == "" {
		var req verifyEmailRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "token is required"})
			return
		}
		token = req.Token
	}

	u, err := h.users.VerifyEmail(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "email verified",
		"user":    u,
	})
}

// ResendVerification handles POST /auth/resend-verification.
// Accepts either an authenticated request (user JWT in Authorization header)
// or an unauthenticated request with an email in the body. Both paths always
// return the same response to prevent account-enumeration.
func (h *AuthHandler) ResendVerification(c *gin.Context) {
	claims := identity.UserClaimsFromCtx(c)

	if claims != nil {
		// Authenticated path — resend for the logged-in user.
		uid, err := uuid.Parse(claims.UserID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID in token"})
			return
		}
		if err := h.users.ResendVerification(c.Request.Context(), uid); err != nil {
			// "email already verified" is a safe message to surface.
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "verification email sent"})
		return
	}

	// Unauthenticated path — look up by email and send if unverified.
	var req resendVerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email is required"})
		return
	}
	// Always call the service (which is silent on unknown emails) and always
	// return the same response so callers cannot enumerate accounts.
	_ = h.users.ResendVerificationByEmail(c.Request.Context(), req.Email)
	c.JSON(http.StatusOK, gin.H{"message": "if an account with that email exists and is unverified, a new link has been sent"})
}

// Logout handles POST /auth/logout.
// JWTs are stateless so revocation is client-side: the client discards the token.
// This endpoint exists to give clients a clean logout call and returns the token
// lifetime so the client knows when the existing token would have expired anyway.
func (h *AuthHandler) Logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "logged out — discard your token client-side",
		"note":    "NAP JWTs are stateless; remove the token from local storage to complete logout",
	})
}

// ForgotPassword handles POST /auth/forgot-password.
// Always returns 200 — never reveals whether the email is registered.
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req forgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Non-blocking: always succeed from the caller's perspective.
	_ = h.users.ForgotPassword(c.Request.Context(), req.Email)

	c.JSON(http.StatusOK, gin.H{
		"message": "if an account with that email exists, a password reset link has been sent",
	})
}

// ResetPassword handles POST /auth/reset-password.
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req resetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.users.ResetPassword(c.Request.Context(), req.Token, req.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "password updated — please log in with your new password"})
}

// OAuthRedirect handles GET /auth/oauth/:provider — redirects to the OAuth provider.
func (h *AuthHandler) OAuthRedirect(c *gin.Context) {
	provider := c.Param("provider")
	cfg, ok := h.oauthCfgs[provider]
	if !ok {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("OAuth provider %q not configured", provider)})
		return
	}

	state, err := h.tokens.IssueOAuthState(provider)
	if err != nil {
		h.logger.Error("generate oauth state", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate OAuth state"})
		return
	}

	url := cfg.AuthCodeURL(state, oauth2.AccessTypeOnline)
	c.Redirect(http.StatusFound, url)
}

// OAuthCallback handles GET /auth/oauth/:provider/callback.
func (h *AuthHandler) OAuthCallback(c *gin.Context) {
	provider := c.Param("provider")
	cfg, ok := h.oauthCfgs[provider]
	if !ok {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("OAuth provider %q not configured", provider)})
		return
	}

	// Validate state to prevent CSRF
	stateParam := c.Query("state")
	gotProvider, err := h.tokens.VerifyOAuthState(stateParam)
	if err != nil || gotProvider != provider {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid OAuth state"})
		return
	}

	code := c.Query("code")
	if code == "" {
		errMsg := c.Query("error_description")
		if errMsg == "" {
			errMsg = c.Query("error")
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "OAuth authorization failed: " + errMsg})
		return
	}

	oauthToken, err := cfg.Exchange(c.Request.Context(), code)
	if err != nil {
		h.logger.Error("oauth code exchange", zap.String("provider", provider), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "OAuth code exchange failed"})
		return
	}

	providerID, email, displayName, err := fetchOAuthUserInfo(c.Request.Context(), provider, oauthToken.AccessToken)
	if err != nil {
		h.logger.Error("fetch oauth user info", zap.String("provider", provider), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch user info from provider"})
		return
	}

	u, _, err := h.users.GetOrCreateFromOAuth(c.Request.Context(), provider, providerID, email, displayName)
	if err != nil {
		h.logger.Error("get or create oauth user", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process OAuth login"})
		return
	}

	tok, err := h.tokens.Issue(u.ID.String(), u.Email, u.Username, string(u.Tier))
	if err != nil {
		h.logger.Error("issue user token after oauth", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token issuance failed"})
		return
	}

	// Redirect the browser to the frontend callback page with the token in the
	// URL fragment (#). Fragments are never sent to the server, so the token
	// stays client-side only.
	c.Redirect(http.StatusFound, h.frontendURL+"/oauth/callback#token="+tok)
}

// ─── OAuth user-info helpers ──────────────────────────────────────────────────

// fetchOAuthUserInfo calls the provider's user-info API and returns
// (providerID, email, displayName).
func fetchOAuthUserInfo(ctx context.Context, provider, accessToken string) (string, string, string, error) {
	switch provider {
	case "github":
		return fetchGitHubUserInfo(ctx, accessToken)
	case "google":
		return fetchGoogleUserInfo(ctx, accessToken)
	default:
		return "", "", "", fmt.Errorf("unsupported provider: %s", provider)
	}
}

func fetchGitHubUserInfo(ctx context.Context, accessToken string) (id, email, name string, err error) {
	body, err := oauthAPIGet(ctx, "https://api.github.com/user", accessToken)
	if err != nil {
		return "", "", "", err
	}

	var info struct {
		ID    int    `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return "", "", "", fmt.Errorf("parse github user info: %w", err)
	}

	// GitHub may not return a public email; fall back to fetching from /user/emails
	if info.Email == "" {
		info.Email, _ = fetchGitHubPrimaryEmail(ctx, accessToken)
	}

	displayName := info.Name
	if displayName == "" {
		displayName = info.Login
	}

	return fmt.Sprintf("%d", info.ID), info.Email, displayName, nil
}

func fetchGitHubPrimaryEmail(ctx context.Context, accessToken string) (string, error) {
	body, err := oauthAPIGet(ctx, "https://api.github.com/user/emails", accessToken)
	if err != nil {
		return "", err
	}
	var emails []struct {
		Email   string `json:"email"`
		Primary bool   `json:"primary"`
	}
	if err := json.Unmarshal(body, &emails); err != nil {
		return "", err
	}
	for _, e := range emails {
		if e.Primary {
			return e.Email, nil
		}
	}
	if len(emails) > 0 {
		return emails[0].Email, nil
	}
	return "", nil
}

func fetchGoogleUserInfo(ctx context.Context, accessToken string) (id, email, name string, err error) {
	body, err := oauthAPIGet(ctx, "https://www.googleapis.com/oauth2/v2/userinfo", accessToken)
	if err != nil {
		return "", "", "", err
	}
	var info struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return "", "", "", fmt.Errorf("parse google user info: %w", err)
	}
	return info.ID, info.Email, info.Name, nil
}

func oauthAPIGet(ctx context.Context, url, accessToken string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	// GitHub requires a User-Agent header
	if strings.Contains(url, "github.com") {
		req.Header.Set("User-Agent", "nexus-registry/1.0")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("api get %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("api returned %d: %s", resp.StatusCode, body)
	}
	return body, nil
}
