package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	runtimeauth "github.com/Gabrielbdd/gofra/runtime/auth"
	runtimeconfig "github.com/Gabrielbdd/gofra/runtime/config"
	runtimedatabase "github.com/Gabrielbdd/gofra/runtime/database"
	runtimehealth "github.com/Gabrielbdd/gofra/runtime/health"
	runtimeserve "github.com/Gabrielbdd/gofra/runtime/serve"
	zitadelsecret "github.com/Gabrielbdd/gofra/runtime/zitadel/secret"

	"github.com/Gabrielbdd/gospa/config"
	"github.com/Gabrielbdd/gospa/db"
	"github.com/Gabrielbdd/gospa/db/sqlc"
	"github.com/Gabrielbdd/gospa/gen/gospa/companies/v1/companiesv1connect"
	"github.com/Gabrielbdd/gospa/gen/gospa/install/v1/installv1connect"
	"github.com/Gabrielbdd/gospa/internal/authgate"
	"github.com/Gabrielbdd/gospa/internal/companies"
	"github.com/Gabrielbdd/gospa/internal/install"
	"github.com/Gabrielbdd/gospa/internal/publicconfig"
	"github.com/Gabrielbdd/gospa/internal/zitadel"
	"github.com/Gabrielbdd/gospa/web"
)

// workspaceOrgProvider adapts sqlc.Queries.GetWorkspace to the
// publicconfig.WorkspaceOrgIDProvider interface.
type workspaceOrgProvider struct {
	queries *sqlc.Queries
}

func (p workspaceOrgProvider) WorkspaceOrgID(ctx context.Context) (string, error) {
	ws, err := p.queries.GetWorkspace(ctx)
	if err != nil {
		return "", err
	}
	if !ws.ZitadelOrgID.Valid {
		return "", nil
	}
	return ws.ZitadelOrgID.String, nil
}

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

	// --- Zitadel admin client + install orchestrator + auth gate -----------

	zitadelClient := zitadel.NewHTTPClient(cfg.Zitadel.AdminAPIURL, provisionerPAT, nil)
	queries := sqlc.New(pool)

	// The auth gate starts as a pass-through so the /install wizard
	// (which runs before any user exists) remains reachable. The
	// install orchestrator flips it to authenticated the moment the
	// pipeline marks the workspace ready — no process restart.
	//
	// /install RPCs stay public forever: even after activation they
	// return FailedPrecondition if install_state != not_initialized, so
	// repeated clicks are safe to serve without auth.
	gate := authgate.New(cfg.Zitadel.AdminAPIURL, runtimeauth.PublicProcedures(
		installv1connect.InstallServiceGetStatusProcedure,
		installv1connect.InstallServiceInstallProcedure,
	))

	installOrchestrator := &install.Orchestrator{
		Queries: queries,
		Zitadel: zitadelClient,
		Logger:  slog.Default(),
		OnReady: func(_ context.Context, projectID string) error {
			// Detach from the request context — the install request has
			// already returned by the time Activate runs.
			return gate.Activate(context.Background(), projectID)
		},
	}
	installHandler := &install.Handler{
		Queries:      queries,
		Orchestrator: installOrchestrator,
		Logger:       slog.Default(),
		APIBaseURL:   cfg.Public.APIBaseURL,
	}
	companiesHandler := &companies.Handler{
		Queries: queries,
		Zitadel: zitadelClient,
		Logger:  slog.Default(),
	}

	// Recover from crashes mid-install. If the previous process died
	// between MarkWorkspaceProvisioning and MarkWorkspaceReady/Failed,
	// the singleton row is stuck in `provisioning` and POST /install
	// returns FailedPrecondition forever. The old orchestrator
	// goroutine is dead (it lived in the previous process), so it is
	// safe to flip the state to `failed` here and let the user retry
	// via the wizard.
	if ws, wsErr := queries.GetWorkspace(ctx); wsErr == nil && string(ws.InstallState) == "provisioning" {
		recoverMsg := "previous process exited during provisioning; retry from /install"
		if err := queries.MarkWorkspaceFailed(ctx, pgtype.Text{String: recoverMsg, Valid: true}); err != nil {
			slog.Warn("could not recover workspace from provisioning state", "error", err)
		} else {
			slog.Warn("workspace was stuck in provisioning; transitioned to failed so /install can retry")
		}
	}

	// Eager activation for established deployments: if the workspace is
	// already installed (pod restart, container rolling update), turn
	// auth on immediately instead of waiting for a new install.
	if ws, wsErr := queries.GetWorkspace(ctx); wsErr == nil &&
		string(ws.InstallState) == "ready" && ws.ZitadelProjectID.Valid {
		if err := gate.Activate(ctx, ws.ZitadelProjectID.String); err != nil {
			slog.Warn("auth gate startup activation failed", "error", err)
		}
	} else {
		slog.Info("auth disabled: workspace not installed yet; /install flow remains public")
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

	// App router. The auth gate is always mounted; it no-ops until the
	// install orchestrator activates it.
	app := chi.NewRouter()
	app.Use(gate.Middleware)
	// /_gofra/config.js is served by the publicconfig wrapper so the
	// browser receives auth.orgId from the workspace singleton. The SPA
	// uses that field to scope its OIDC login request to the MSP org.
	app.Handle(runtimeconfig.DefaultPath, publicconfig.Handler(cfg, workspaceOrgProvider{queries: queries}))

	// Install service is mounted on the app router; auth middleware
	// (when enabled) skips it because its procedures are registered
	// public. Once workspace.install_state = ready, POST /install
	// returns FailedPrecondition, so repeated clicks are safe.
	installPath, installConnectHandler := installv1connect.NewInstallServiceHandler(installHandler)
	app.Handle(installPath+"*", installConnectHandler)

	// Companies service is always protected: each handler method checks
	// workspace.install_state = ready and returns FailedPrecondition
	// pointing users at /install when the workspace is not yet set up.
	companiesPath, companiesConnectHandler := companiesv1connect.NewCompaniesServiceHandler(companiesHandler)
	app.Handle(companiesPath+"*", companiesConnectHandler)

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
