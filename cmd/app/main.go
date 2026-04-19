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
	zitadelsecret "github.com/Gabrielbdd/gofra/runtime/zitadel/secret"

	"github.com/Gabrielbdd/gospa/config"
	"github.com/Gabrielbdd/gospa/db"
	"github.com/Gabrielbdd/gospa/db/sqlc"
	"github.com/Gabrielbdd/gospa/gen/gospa/install/v1/installv1connect"
	"github.com/Gabrielbdd/gospa/internal/install"
	"github.com/Gabrielbdd/gospa/internal/zitadel"
	"github.com/Gabrielbdd/gospa/web"
)

// provisionerPATEnv overrides zitadel.provisioner_pat_file from the config.
// Kubernetes deploys set this to the mount path of the secret volume.
const provisionerPATEnv = "GOSPA_ZITADEL_PROVISIONER_PAT_FILE"

func main() {
	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// --- Zitadel provisioner PAT (hard startup contract) -------------------
	// Gospa refuses to start unless a valid provisioner PAT is already on
	// disk. In local dev `mise run infra` materialises the file; in
	// Kubernetes the operator mounts a Secret at the configured path.

	provisionerPAT, err := loadProvisionerPAT(cfg)
	if err != nil {
		slog.Error("zitadel provisioner PAT unavailable; refusing to start", "error", err)
		os.Exit(1)
	}
	slog.Info("zitadel provisioner PAT loaded")

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

	// --- Zitadel admin client + install handler -----------------------------

	zitadelClient := zitadel.NewHTTPClient(cfg.Zitadel.AdminAPIURL, provisionerPAT, nil)
	queries := sqlc.New(pool)

	installOrchestrator := &install.Orchestrator{
		Queries: queries,
		Zitadel: zitadelClient,
		Logger:  slog.Default(),
	}
	installHandler := &install.Handler{
		Queries:      queries,
		Orchestrator: installOrchestrator,
		Logger:       slog.Default(),
		APIBaseURL:   cfg.Public.APIBaseURL,
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
			// /install runs before any user exists, so its RPCs cannot
			// require a valid token. Operators are responsible for
			// keeping /install behind a private ingress until the
			// workspace is ready (the SPA also shows a banner warning
			// about this).
			installv1connect.InstallServiceGetStatusProcedure,
			installv1connect.InstallServiceInstallProcedure,
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

	// Install service is mounted on the app router; auth middleware
	// (when enabled) skips it because its procedures are registered
	// public. Once workspace.install_state = ready, POST /install
	// returns FailedPrecondition, so repeated clicks are safe.
	installPath, installConnectHandler := installv1connect.NewInstallServiceHandler(installHandler)
	app.Handle(installPath+"*", installConnectHandler)

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

// loadProvisionerPAT resolves the PAT path (env-var override beats config)
// and reads the file. Returns an actionable error if the path is empty, the
// file is missing, or the file is empty. Called at startup; the returned
// token is consumed by later handlers (install, companies) once those wire
// in.
func loadProvisionerPAT(cfg *config.Config) (string, error) {
	path := os.Getenv(provisionerPATEnv)
	if path == "" {
		path = cfg.Zitadel.ProvisionerPatFile
	}
	if path == "" {
		return "", fmt.Errorf(
			"no provisioner PAT path configured: set %s or zitadel.provisioner_pat_file in gofra.yaml",
			provisionerPATEnv,
		)
	}
	pat, err := zitadelsecret.Read(zitadelsecret.Source{FilePath: path})
	if err != nil {
		return "", fmt.Errorf(
			"reading provisioner PAT at %q: %w (run `mise run infra` locally, or mount the Kubernetes Secret before starting)",
			path, err,
		)
	}
	return pat, nil
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
