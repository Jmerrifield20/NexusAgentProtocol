package main

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/email"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/federation"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/identity"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/registry/handler"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/registry/repository"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/registry/service"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/threat"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/trustledger"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/users"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync() //nolint:errcheck

	if err := run(logger); err != nil {
		logger.Fatal("registry exited with error", zap.Error(err))
	}
}

func run(logger *zap.Logger) error {
	// ── Configuration ────────────────────────────────────────────────────────
	viper.SetConfigName("registry")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("configs")
	viper.AddConfigPath(".")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	viper.SetDefault("registry.port", 8080)
	viper.SetDefault("registry.tls_port", 8443)
	viper.SetDefault("registry.issuer_url", "")
	viper.SetDefault("database.url", "postgres://nexus:nexus@localhost:5432/nexus?sslmode=disable")
	viper.SetDefault("identity.cert_dir", "certs")
	viper.SetDefault("identity.token_ttl_seconds", 3600)
	viper.SetDefault("identity.tls_enabled", true)
	viper.SetDefault("registry.cors_origins", []string{"http://localhost:3000"})
	viper.SetDefault("registry.rate_limit_rps", 20)
	viper.SetDefault("registry.skip_dns_verify", false)
	viper.SetDefault("free_tier.trust_root", "nexusagentprotocol.com")
	viper.SetDefault("free_tier.max_agents", 3)
	viper.SetDefault("email.smtp_host", "")
	viper.SetDefault("email.smtp_port", 587)
	viper.SetDefault("email.smtp_username", "")
	viper.SetDefault("email.smtp_password", "")
	viper.SetDefault("email.from_address", "noreply@nexusagentprotocol.com")
	viper.SetDefault("registry.frontend_url", "http://localhost:3000")
	viper.SetDefault("registry.role", "standalone")
	viper.SetDefault("registry.admin_secret", "")
	viper.SetDefault("federation.root_registry_url", "")
	viper.SetDefault("federation.intermediate_ca_cert", "")
	viper.SetDefault("federation.intermediate_ca_key", "")
	viper.SetDefault("federation.dns_discovery_enabled", true)
	viper.SetDefault("federation.remote_resolve_timeout", "5s")

	if err := viper.ReadInConfig(); err != nil {
		var cfgNotFound viper.ConfigFileNotFoundError
		if !errors.As(err, &cfgNotFound) {
			return fmt.Errorf("read config: %w", err)
		}
		logger.Warn("no config file found, using defaults and env vars")
	}

	// ── Database ─────────────────────────────────────────────────────────────
	db, err := pgxpool.New(context.Background(), viper.GetString("database.url"))
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer db.Close()

	if err := db.Ping(context.Background()); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}
	logger.Info("connected to postgres")

	// ── Trust Ledger ──────────────────────────────────────────────────────────
	ledger := trustledger.NewPostgresLedger(db, logger)

	startCtx := context.Background()
	if err := ledger.Verify(startCtx); err != nil {
		logger.Warn("trust ledger integrity check FAILED", zap.Error(err))
	} else {
		n, _ := ledger.Len(startCtx)
		root, _ := ledger.Root(startCtx)
		logger.Info("trust ledger verified",
			zap.Int("entries", n),
			zap.String("root", root),
		)
	}

	// ── Identity (CA + Issuer + Tokens) ───────────────────────────────────────
	certDir := viper.GetString("identity.cert_dir")
	ca := identity.NewCAManager(certDir)
	if err := ca.LoadOrCreate(); err != nil {
		return fmt.Errorf("CA setup failed: %w", err)
	}
	logger.Info("CA ready", zap.String("cert_dir", certDir))

	issuer := identity.NewIssuer(ca)

	httpPort := viper.GetInt("registry.port")
	issuerURL := viper.GetString("registry.issuer_url")
	if issuerURL == "" {
		issuerURL = fmt.Sprintf("http://localhost:%d", httpPort)
	}

	tokenTTL := time.Duration(viper.GetInt("identity.token_ttl_seconds")) * time.Second
	tokens := identity.NewTokenIssuer(ca.Key(), issuerURL, tokenTTL)
	userTokens := identity.NewUserTokenIssuer(ca.Key(), issuerURL, 24*time.Hour)
	oidcProvider := identity.NewOIDCProvider(issuerURL, tokens)

	// ── Email Sender ──────────────────────────────────────────────────────────
	var mailer email.EmailSender
	smtpHost := viper.GetString("email.smtp_host")
	if smtpHost != "" {
		mailer = email.NewSMTPSender(
			smtpHost,
			viper.GetInt("email.smtp_port"),
			viper.GetString("email.smtp_username"),
			viper.GetString("email.smtp_password"),
			viper.GetString("email.from_address"),
		)
		logger.Info("SMTP email sender configured", zap.String("host", smtpHost))
	} else {
		mailer = email.NewNoopSender(logger)
		logger.Info("email sender: noop (set email.smtp_host to enable SMTP)")
	}

	// ── Wire up layers ────────────────────────────────────────────────────────
	repo := repository.NewAgentRepository(db)
	dnsRepo := repository.NewDNSChallengeRepository(db)
	dnsSvc := service.NewDNSChallengeService(dnsRepo, nil, logger)

	var dnsVerifier service.DomainVerifier = dnsSvc
	if viper.GetBool("registry.skip_dns_verify") {
		logger.Warn("DNS verification disabled — REGISTRY_SKIP_DNS_VERIFY is set; do not use in production")
		dnsVerifier = nil
	}

	svc := service.NewAgentService(repo, issuer, ledger, dnsVerifier, logger)

	// Free-tier configuration
	freeTierCfg := service.FreeTierConfig{
		TrustRoot: viper.GetString("free_tier.trust_root"),
		MaxAgents: viper.GetInt("free_tier.max_agents"),
	}
	svc.SetFreeTierConfig(freeTierCfg)
	svc.SetTokenIssuer(tokens)
	svc.SetRegistryURL(issuerURL)
	svc.SetThreatScorer(threat.NewRuleBasedScorer())

	// User service
	userRepo := users.NewUserRepository(db)
	userSvc := users.NewUserService(userRepo, mailer, issuerURL, logger)
	userSvc.SetFrontendURL(viper.GetString("registry.frontend_url"))
	svc.SetEmailChecker(userSvc)

	// OAuth provider configs
	oauthCfgs := map[string]handler.OAuthProviderConfig{
		"github": {
			ClientID:     viper.GetString("oauth.github.client_id"),
			ClientSecret: viper.GetString("oauth.github.client_secret"),
			RedirectURL:  viper.GetString("oauth.github.redirect_url"),
		},
		"google": {
			ClientID:     viper.GetString("oauth.google.client_id"),
			ClientSecret: viper.GetString("oauth.google.client_secret"),
			RedirectURL:  viper.GetString("oauth.google.redirect_url"),
		},
	}
	viper.SetDefault("oauth.github.redirect_url", fmt.Sprintf("http://localhost:%d/api/v1/auth/oauth/github/callback", httpPort))
	viper.SetDefault("oauth.google.redirect_url", fmt.Sprintf("http://localhost:%d/api/v1/auth/oauth/google/callback", httpPort))

	// ── Federation (role-based wiring) ───────────────────────────────────────
	role := federation.Role(viper.GetString("registry.role"))
	resolveTimeout, _ := time.ParseDuration(viper.GetString("federation.remote_resolve_timeout"))
	if resolveTimeout == 0 {
		resolveTimeout = 5 * time.Second
	}
	dnsFedEnabled := viper.GetBool("federation.dns_discovery_enabled")

	var fedHandler *handler.FederationHandler
	switch role {
	case federation.RoleRoot:
		fedRepo := federation.NewFederationRepository(db, logger)
		fedSvc := federation.NewFederationService(fedRepo, issuer, logger)
		fedHandler = handler.NewFederationHandler(fedSvc, role, userTokens, logger)
		resolver := federation.NewRemoteResolver(fedSvc, "", dnsFedEnabled, resolveTimeout, logger)
		svc.SetRemoteResolver(resolver)
		logger.Info("federation role: root — registry-of-registries enabled")

	case federation.RoleFederated:
		certPath := viper.GetString("federation.intermediate_ca_cert")
		keyPath := viper.GetString("federation.intermediate_ca_key")
		rootURL := viper.GetString("federation.root_registry_url")

		if certPath != "" && keyPath != "" {
			certPEM, certErr := os.ReadFile(certPath)
			keyPEM, keyErr := os.ReadFile(keyPath)
			if certErr != nil || keyErr != nil {
				logger.Warn("federated mode: cannot read intermediate CA files; falling back to local CA",
					zap.String("cert_path", certPath),
					zap.String("key_path", keyPath),
				)
			} else {
				intermediateCert, intermediateKey, parseErr := identity.LoadCertAndKey(certPEM, keyPEM)
				if parseErr != nil {
					logger.Warn("federated mode: cannot parse intermediate CA; falling back to local CA",
						zap.Error(parseErr),
					)
				} else {
					// Fetch root CA pool from the configured root registry.
					var rootCAPool *x509.CertPool
					if rootURL != "" {
						caURL := rootURL + "/api/v1/ca.crt"
						pool, fetchErr := identity.FetchRootCAPool(context.Background(), caURL, 10*time.Second)
						if fetchErr != nil {
							logger.Warn("federated mode: cannot fetch root CA pool; using system pool",
								zap.String("url", caURL),
								zap.Error(fetchErr),
							)
						} else {
							rootCAPool = pool
						}
					}
					issuer = identity.NewIssuerWithIntermediate(intermediateCert, intermediateKey, rootCAPool)
					logger.Info("federation role: federated — intermediate CA loaded",
						zap.String("cn", intermediateCert.Subject.CommonName),
					)
				}
			}
		}

		fedRepo := federation.NewFederationRepository(db, logger)
		fedSvc := federation.NewFederationService(fedRepo, nil, logger) // nil issuer: cannot issue sub-CAs
		fedHandler = handler.NewFederationHandler(fedSvc, role, userTokens, logger)
		resolver := federation.NewRemoteResolver(nil, rootURL, dnsFedEnabled, resolveTimeout, logger)
		svc.SetRemoteResolver(resolver)
		logger.Info("federation role: federated")

	default:
		logger.Info("federation role: standalone")
	}

	agentHandler := handler.NewAgentHandler(svc, tokens, logger)
	agentHandler.SetUserTokenIssuer(userTokens)
	identityHandler := handler.NewIdentityHandler(issuer, tokens, logger)
	ledgerHandler := handler.NewLedgerHandler(ledger, logger)
	dnsHandler := handler.NewDNSHandler(dnsSvc, logger)
	wkHandler := handler.NewWellKnownHandler(svc, logger)
	authHandler := handler.NewAuthHandler(userSvc, userTokens, oauthCfgs, logger)
	authHandler.SetFrontendURL(viper.GetString("registry.frontend_url"))
	authHandler.SetAdminSecret(viper.GetString("registry.admin_secret"))

	// ── HTTP Router ───────────────────────────────────────────────────────────
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Recovery())

	// CORS
	corsOrigins := viper.GetStringSlice("registry.cors_origins")
	corsConfig := cors.Config{
		AllowOrigins:     corsOrigins,
		AllowMethods:     []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Accept"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: !containsWildcard(corsOrigins),
		MaxAge:           12 * time.Hour,
	}
	router.Use(cors.New(corsConfig))

	// Security headers
	router.Use(func(c *gin.Context) {
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Next()
	})

	// Request body size limit (1 MB)
	router.Use(func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
		c.Next()
	})

	// Per-IP rate limiting
	rps := viper.GetInt("registry.rate_limit_rps")
	if rps > 0 {
		router.Use(handler.RateLimiter(rps, rps*2))
	}

	router.Use(requestLogger(logger))

	// Health (public, no auth)
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// OIDC well-known endpoints (public)
	oidcProvider.RegisterWellKnown(router)

	// Agent discovery endpoints (public)
	router.GET("/.well-known/agent-card.json", wkHandler.ServeAgentCard)
	router.GET("/.well-known/agent.json", wkHandler.ServeA2ACard)

	// API v1
	v1 := router.Group("/api/v1")
	agentHandler.Register(v1)
	identityHandler.Register(v1)
	ledgerHandler.Register(v1)
	dnsHandler.Register(v1)
	authHandler.Register(v1)
	if fedHandler != nil {
		fedHandler.Register(v1)
	}

	// ── TLS Server (mTLS) on port 8443 ────────────────────────────────────────
	tlsEnabled := viper.GetBool("identity.tls_enabled")
	tlsPort := viper.GetInt("registry.tls_port")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// ── Background: expire stale DNS challenges every 5 minutes ──────────────
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				if _, err := dnsSvc.DeleteExpired(ctx); err != nil {
					logger.Warn("dns challenge cleanup error", zap.Error(err))
				}
				cancel()
			case <-quit:
				return
			}
		}
	}()

	httpSrv := &http.Server{
		Addr:              fmt.Sprintf(":%d", httpPort),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("registry HTTP listening", zap.Int("port", httpPort))
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("HTTP listen error", zap.Error(err))
		}
	}()

	var tlsSrv *http.Server
	if tlsEnabled {
		serverCert, err := issuer.IssueServerCert(
			[]string{"localhost", "registry"},
			[]net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
			365*24*time.Hour,
		)
		if err != nil {
			return fmt.Errorf("issue server certificate: %w", err)
		}

		tlsCert, err := serverCert.TLSCertificate()
		if err != nil {
			return fmt.Errorf("parse server TLS certificate: %w", err)
		}

		tlsSrv = &http.Server{
			Addr:              fmt.Sprintf(":%d", tlsPort),
			Handler:           router,
			TLSConfig:         ca.TLSConfig(tlsCert),
			ReadHeaderTimeout: 10 * time.Second,
		}

		go func() {
			logger.Info("registry HTTPS/mTLS listening", zap.Int("port", tlsPort))
			if err := tlsSrv.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Fatal("TLS listen error", zap.Error(err))
			}
		}()
	}

	// ── Graceful shutdown ──────────────────────────────────────────────────────
	<-quit
	logger.Info("shutting down registry...")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := httpSrv.Shutdown(ctx); err != nil {
		logger.Error("HTTP shutdown error", zap.Error(err))
	}
	if tlsSrv != nil {
		if err := tlsSrv.Shutdown(ctx); err != nil {
			logger.Error("TLS shutdown error", zap.Error(err))
		}
	}

	logger.Info("registry stopped")
	return nil
}

// containsWildcard returns true if origins includes "*".
func containsWildcard(origins []string) bool {
	for _, o := range origins {
		if strings.TrimSpace(o) == "*" {
			return true
		}
	}
	return false
}

// requestLogger returns a Gin middleware that logs each request with zap.
func requestLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logger.Info("request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("client_ip", c.ClientIP()),
		)
	}
}
