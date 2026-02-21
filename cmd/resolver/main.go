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

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	resolverv1 "github.com/nexus-protocol/nexus/api/proto/resolver/v1"
	"github.com/nexus-protocol/nexus/internal/resolver"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/encoding/protojson"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync() //nolint:errcheck

	if err := run(logger); err != nil {
		logger.Fatal("resolver exited with error", zap.Error(err))
	}
}

func run(logger *zap.Logger) error {
	// ── Configuration ─────────────────────────────────────────────────────────
	viper.SetConfigName("resolver")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("configs")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()

	viper.SetDefault("resolver.grpc_port", 9090)
	viper.SetDefault("resolver.http_port", 9091)    // grpc-gateway REST port
	viper.SetDefault("resolver.registry_addr", "localhost:8080")
	viper.SetDefault("resolver.cache_ttl_seconds", 60)
	viper.SetDefault("resolver.http_timeout_seconds", 5)
	viper.SetDefault("resolver.eviction_interval_seconds", 60)

	if err := viper.ReadInConfig(); err != nil {
		var cfgNotFound viper.ConfigFileNotFoundError
		if !errors.As(err, &cfgNotFound) {
			return fmt.Errorf("read config: %w", err)
		}
		logger.Warn("no config file found, using defaults and env vars")
	}

	grpcPort := viper.GetInt("resolver.grpc_port")
	httpPort := viper.GetInt("resolver.http_port")
	registryAddr := viper.GetString("resolver.registry_addr")
	cacheTTL := time.Duration(viper.GetInt("resolver.cache_ttl_seconds")) * time.Second
	httpTimeout := time.Duration(viper.GetInt("resolver.http_timeout_seconds")) * time.Second
	evictionInterval := time.Duration(viper.GetInt("resolver.eviction_interval_seconds")) * time.Second

	// ── Resolver service ──────────────────────────────────────────────────────
	cfg := resolver.Config{
		RegistryAddr: registryAddr,
		CacheTTL:     cacheTTL,
		HTTPTimeout:  httpTimeout,
	}
	svc := resolver.New(cfg, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start background cache eviction
	svc.StartCacheEviction(ctx, evictionInterval)

	// ── gRPC server ───────────────────────────────────────────────────────────
	grpcLis, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		return fmt.Errorf("gRPC listen on :%d: %w", grpcPort, err)
	}

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(loggingInterceptor(logger)),
	)

	resolverv1.RegisterResolverServiceServer(grpcServer, svc)

	// Standard gRPC health service
	healthSvc := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthSvc)
	healthSvc.SetServingStatus(
		resolverv1.ResolverService_ServiceDesc.ServiceName,
		grpc_health_v1.HealthCheckResponse_SERVING,
	)

	// gRPC reflection (for grpcurl and Evans)
	reflection.Register(grpcServer)

	// ── grpc-gateway HTTP/JSON reverse proxy ──────────────────────────────────
	gwMux := runtime.NewServeMux(
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
			MarshalOptions: protojson.MarshalOptions{
				UseProtoNames:   true,
				EmitUnpopulated: false,
			},
		}),
	)

	// Register the gateway against the local gRPC server
	grpcAddr := fmt.Sprintf("localhost:%d", grpcPort)
	dialOpts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	if err := resolverv1.RegisterResolverServiceHandlerFromEndpoint(ctx, gwMux, grpcAddr, dialOpts); err != nil {
		return fmt.Errorf("register grpc-gateway: %w", err)
	}

	httpMux := http.NewServeMux()
	httpMux.Handle("/", gwMux)
	httpMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok","service":"resolver"}`)
	})
	httpMux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"cache_entries":%d}`, svc.CacheStats())
	})

	httpSrv := &http.Server{
		Addr:              fmt.Sprintf(":%d", httpPort),
		Handler:           httpMux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// ── Start both servers ─────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("resolver gRPC listening",
			zap.Int("port", grpcPort),
			zap.String("registry", registryAddr),
			zap.Duration("cache_ttl", cacheTTL),
		)
		if err := grpcServer.Serve(grpcLis); err != nil {
			logger.Fatal("gRPC serve error", zap.Error(err))
		}
	}()

	go func() {
		logger.Info("resolver HTTP/JSON gateway listening", zap.Int("port", httpPort))
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("HTTP serve error", zap.Error(err))
		}
	}()

	// ── Graceful shutdown ──────────────────────────────────────────────────────
	<-quit
	logger.Info("shutting down resolver...")
	cancel() // stop cache eviction

	grpcServer.GracefulStop()

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	if err := httpSrv.Shutdown(shutCtx); err != nil {
		logger.Error("HTTP gateway shutdown", zap.Error(err))
	}

	logger.Info("resolver stopped")
	return nil
}

// loggingInterceptor returns a gRPC unary server interceptor that logs each call.
func loggingInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		code := "OK"
		if err != nil {
			code = grpc.Code(err).String() //nolint:staticcheck
		}
		logger.Info("grpc",
			zap.String("method", info.FullMethod),
			zap.String("code", code),
			zap.Duration("latency", time.Since(start)),
		)
		return resp, err
	}
}
