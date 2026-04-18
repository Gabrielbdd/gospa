package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"

	runtimeauth "github.com/Gabrielbdd/gofra/runtime/auth"
	runtimeconfig "github.com/Gabrielbdd/gofra/runtime/config"
	runtimedatabase "github.com/Gabrielbdd/gofra/runtime/database"
	runtimehealth "github.com/Gabrielbdd/gofra/runtime/health"
	runtimeserve "github.com/Gabrielbdd/gofra/runtime/serve"

	"github.com/Gabrielbdd/gospa/config"
	"github.com/Gabrielbdd/gospa/db"
	"github.com/Gabrielbdd/gospa/web"
)

func main() {
	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// --- Database -----------------------------------------------------------

	pool, err := runtimedatabase.Open(ctx, runtimedatabase.Config{
		DSN:               cfg.Database.DSN,
		MaxConns:          cfg.Database.MaxConns,
		MinConns:          cfg.Database.MinConns,
		MaxConnLifetime:   parseDuration(cfg.Database.MaxConnLifetime),
		MaxConnIdleTime:   parseDuration(cfg.Database.MaxConnIdleTime),
		HealthCheckPeriod: parseDuration(cfg.Database.HealthCheckPeriod),
	})
	if err != nil {
		slog.Error("database connection failed", "error", err)
		os.Exit(1)
	}

	if cfg.Database.AutoMigrate {
		results, err := runtimedatabase.Migrate(ctx, pool, db.Migrations)
		if err != nil {
			pool.Close()
			slog.Error("migration failed", "error", err)
			os.Exit(1)
		}
		for _, r := range results {
			slog.Info("migration applied", "version", r.Source.Version, "duration", r.Duration)
		}
	}

	// --- Auth ---------------------------------------------------------------
	// Auth is opt-in: when issuer and audience are both configured, the
	// middleware validates JWT access tokens on Connect RPC procedures.
	// When either is empty the middleware is a no-op, so a fresh starter
	// remains runnable without ZITADEL infrastructure.

	var authMiddleware func(http.Handler) http.Handler

	if cfg.Auth.Issuer != "" && cfg.Auth.Audience != "" {
		verifier, err := runtimeauth.NewJWTVerifier(ctx, cfg.Auth.Issuer, cfg.Auth.Audience)
		if err != nil {
			slog.Error("auth verifier setup failed", "error", err)
			os.Exit(1)
		}

		isPublic := runtimeauth.PublicProcedures(
			// Add public RPC procedures here, e.g.:
			// "/myapp.v1.PublicService/GetInfo",
		)

		authMiddleware = runtimeauth.NewMiddleware(verifier, isPublic)
		slog.Info("auth enabled", "issuer", cfg.Auth.Issuer)
	} else {
		slog.Warn("auth disabled: auth.issuer and auth.audience must both be set")
	}

	// --- Health & Routing ---------------------------------------------------

	health := runtimehealth.New(runtimehealth.Check{
		Name: "postgres",
		Fn:   runtimedatabase.HealthCheck(pool),
	})

	// Health endpoints on the root mux, structurally outside app middleware.
	root := http.NewServeMux()
	root.Handle(runtimehealth.DefaultStartupPath, health.StartupHandler())
	root.Handle(runtimehealth.DefaultLivenessPath, health.LivenessHandler())
	root.Handle(runtimehealth.DefaultReadinessPath, health.ReadinessHandler())

	// App router — auth middleware protects Connect RPC procedures when enabled.
	app := chi.NewRouter()
	if authMiddleware != nil {
		app.Use(authMiddleware)
	}
	app.Handle(runtimeconfig.DefaultPath, config.PublicConfigHandler(cfg))
	app.Handle("/*", web.Handler())

	root.Handle("/", app)

	// --- Start --------------------------------------------------------------

	addr := fmt.Sprintf(":%d", cfg.App.Port)
	slog.Info("starting app", "app", cfg.App.Name, "addr", addr)

	if err := runtimeserve.Serve(ctx, runtimeserve.Config{
		Handler: root,
		Addr:    addr,
		Health:  health,
		OnShutdown: func(ctx context.Context) error {
			pool.Close()
			return nil
		},
	}); err != nil {
		slog.Error("server stopped with error", "error", err)
		os.Exit(1)
	}
}

// parseDuration parses a Go duration string (e.g. "30m", "1h"). Returns zero
// for empty strings, which leaves the pgxpool default in place.
func parseDuration(s string) time.Duration {
	if s == "" {
		return 0
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		slog.Error("invalid duration in config, using default", "value", s, "error", err)
		return 0
	}
	return d
}
