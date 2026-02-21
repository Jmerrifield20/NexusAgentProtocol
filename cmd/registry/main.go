package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nexus-protocol/nexus/internal/identity"
	"github.com/nexus-protocol/nexus/internal/registry/handler"
	"github.com/nexus-protocol/nexus/internal/registry/repository"
	"github.com/nexus-protocol/nexus/internal/registry/service"
	"github.com/nexus-protocol/nexus/internal/trustledger"
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
	viper.AutomaticEnv()

	viper.SetDefault("registry.port", 8080)
	viper.SetDefault("registry.tls_port", 8443)
	viper.SetDefault("registry.issuer_url", "")
	viper.SetDefault("database.url", "postgres://nexus:nexus@localhost:5432/nexus?sslmode=disable")
	viper.SetDefault("identity.cert_dir", "certs")
	viper.SetDefault("identity.token_ttl_seconds", 3600)
	viper.SetDefault("identity.tls_enabled", true)

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

	// Verify chain integrity on startup; log a warning if the chain is broken
	// but do not abort — the registry should still serve reads.
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
	oidcProvider := identity.NewOIDCProvider(issuerURL, tokens)

	// ── Wire up layers ────────────────────────────────────────────────────────
	repo := repository.NewAgentRepository(db)
	dnsRepo := repository.NewDNSChallengeRepository(db)
	dnsSvc := service.NewDNSChallengeService(dnsRepo, nil, logger) // nil = real DNS lookups
	svc := service.NewAgentService(repo, issuer, ledger, dnsSvc, logger)
	agentHandler := handler.NewAgentHandler(svc, tokens, logger)
	identityHandler := handler.NewIdentityHandler(issuer, tokens, logger)
	ledgerHandler := handler.NewLedgerHandler(ledger, logger)
	dnsHandler := handler.NewDNSHandler(dnsSvc, logger)
	wkHandler := handler.NewWellKnownHandler(svc, logger)

	// ── HTTP Router ───────────────────────────────────────────────────────────
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(requestLogger(logger))

	// Health (public, no auth)
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// OIDC well-known endpoints (public)
	oidcProvider.RegisterWellKnown(router)

	// Agent discovery endpoint (public)
	router.GET("/.well-known/agent-card.json", wkHandler.ServeAgentCard)

	// API v1
	v1 := router.Group("/api/v1")
	agentHandler.Register(v1)
	identityHandler.Register(v1)
	ledgerHandler.Register(v1)
	dnsHandler.Register(v1)

	// ── TLS Server (mTLS) on port 8443 ────────────────────────────────────────
	tlsEnabled := viper.GetBool("identity.tls_enabled")
	tlsPort := viper.GetInt("registry.tls_port")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Plain HTTP server (health + public API)
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

	// TLS/mTLS server
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
