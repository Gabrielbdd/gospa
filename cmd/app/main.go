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
	"github.com/Gabrielbdd/gospa/db/sqlc"
	"github.com/Gabrielbdd/gospa/gen/gospa/companies/v1/companiesv1connect"
	"github.com/Gabrielbdd/gospa/gen/gospa/install/v1/installv1connect"
	"github.com/Gabrielbdd/gospa/internal/authgate"
	"github.com/Gabrielbdd/gospa/internal/companies"
	"github.com/Gabrielbdd/gospa/internal/install"
	"github.com/Gabrielbdd/gospa/internal/installtoken"
	"github.com/Gabrielbdd/gospa/internal/patwatch"
	"github.com/Gabrielbdd/gospa/internal/publicconfig"
	"github.com/Gabrielbdd/gospa/internal/zitadel"
	"github.com/Gabrielbdd/gospa/internal/zitadelcontract"
	"github.com/Gabrielbdd/gospa/web"
)

// workspaceAuthProvider adapts sqlc.Queries.GetWorkspace to the
// publicconfig.WorkspaceAuthProvider interface so the browser receives
// the ZITADEL org ID and the real OIDC SPA client ID after install.
type workspaceAuthProvider struct {
	queries *sqlc.Queries
}

func (p workspaceAuthProvider) WorkspaceAuth(ctx context.Context) (publicconfig.WorkspaceAuth, error) {
	ws, err := p.queries.GetWorkspace(ctx)
	if err != nil {
		return publicconfig.WorkspaceAuth{}, err
	}
	out := publicconfig.WorkspaceAuth{}
	if ws.ZitadelOrgID.Valid {
		out.OrgID = ws.ZitadelOrgID.String
		out.OrgScope = zitadelcontract.OrgScope(ws.ZitadelOrgID.String)
	}
	if ws.ZitadelSpaClientID.Valid {
		out.ClientID = ws.ZitadelSpaClientID.String
	}
	if ws.ZitadelIssuerUrl.Valid {
		out.Issuer = ws.ZitadelIssuerUrl.String
	}
	// AudienceScope is derived from the same field the auth gate
	// validates (workspace.zitadel_api_audience), not from
	// workspace.zitadel_project_id. Today the two are equal because
	// DeriveFresh sets api_audience = project_id, but the contract
	// allows them to diverge (cfg.Auth.Audience can override).
	// Anchoring the browser scope on api_audience guarantees the JWT's
	// aud claim matches what the gate expects, no matter which
	// derivation rule produced the persisted value.
	if ws.ZitadelApiAudience.Valid {
		out.AudienceScope = zitadelcontract.AudienceScope(ws.ZitadelApiAudience.String)
	}
	return out, nil
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

	// --- Zitadel provisioner PAT (hard startup contract + hot reload) -----
	// Gospa refuses to start unless a valid provisioner PAT is already on
	// disk. In local dev `mise run infra` materialises the file; in
	// Kubernetes the operator mounts a Secret at the configured path.
	//
	// patwatch keeps the value fresh after startup: rotating the file
	// (or atomically renaming a Kubernetes Secret) is picked up by the
	// next ZITADEL request without a process restart. Last-known-good
	// semantics mean a transient empty/missing read never erases the
	// runtime PAT — the Watcher just keeps serving the previous value
	// and logs a warning.

	patPath, err := resolveProvisionerPATPath(cfg)
	if err != nil {
		slog.Error("zitadel provisioner PAT path unset; refusing to start", "error", err)
		os.Exit(1)
	}
	patWatcher, err := patwatch.New(patPath, slog.Default())
	if err != nil {
		slog.Error("zitadel provisioner PAT unavailable; refusing to start", "error", err)
		os.Exit(1)
	}
	defer patWatcher.Close()
	slog.Info("zitadel provisioner PAT loaded and watched", "path", patPath)

	// --- Install token (bootstrap secret for /install) ---------------------

	installSecret, installSecretSrc, err := installtoken.Load()
	if err != nil {
		slog.Error("install token unavailable; refusing to start", "error", err)
		os.Exit(1)
	}
	logInstallTokenSource(installSecret, installSecretSrc)

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

	zitadelClient := zitadel.NewHTTPClient(cfg.Zitadel.AdminAPIURL, patWatcher.Get, nil)
	queries := sqlc.New(pool)

	// The auth gate starts as a pass-through so the /install wizard
	// (which runs before any user exists) remains reachable. The
	// install orchestrator flips it to authenticated the moment the
	// pipeline marks the workspace ready — no process restart.
	//
	// /install RPCs stay public forever: even after activation they
	// return FailedPrecondition if install_state != not_initialized, so
	// repeated clicks are safe to serve without auth.
	//
	// The gate no longer carries a fixed issuer; both issuer and
	// audience come from the persisted auth contract on every Activate
	// call, so deploys can rotate either independently of the static
	// app config.
	gate := authgate.New(runtimeauth.PublicProcedures(
		installv1connect.InstallServiceGetStatusProcedure,
		installv1connect.InstallServiceInstallProcedure,
	))

	installOrchestrator := &install.Orchestrator{
		Queries: queries,
		Zitadel: zitadelClient,
		Config:  cfg,
		Logger:  slog.Default(),
		OnReady: func(_ context.Context, contract zitadelcontract.Contract) error {
			// Detach from the request context — the install request has
			// already returned by the time Activate runs. Issuer +
			// audience come from the freshly-derived contract so the
			// gate matches what was just persisted.
			return gate.Activate(context.Background(), contract.IssuerURL, contract.APIAudience)
		},
	}
	installHandler := &install.Handler{
		Queries:      queries,
		Orchestrator: installOrchestrator,
		Logger:       slog.Default(),
		APIBaseURL:   cfg.Public.APIBaseURL,
		InstallToken: installSecret,
	}
	companiesHandler := &companies.Handler{
		Queries: queries,
		Zitadel: zitadelClient,
		Logger:  slog.Default(),
	}

	// Pre-v1 contract: no read-repair, no backfill, no mid-install
	// recovery. Every schema or state change assumes the operator runs
	// a fresh install (`mise run infra:reset` + /install). If a process
	// dies mid-install, the workspace row is stuck in `provisioning`
	// and /install returns FailedPrecondition — the fix is to reset
	// infra, not to have the app self-heal. See AGENTS.md "No repair
	// before v1" rule.

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

	// Eager activation for established deployments: if the workspace is
	// already installed (pod restart, container rolling update), turn
	// auth on immediately instead of waiting for a new install. Must run
	// after app.Use(gate.Middleware) so the gate's passthrough is wired —
	// otherwise Activate fails with ErrMiddlewareNotMounted.
	//
	// Reads the persisted contract directly: issuer + audience are
	// already on the workspace row (from install). If either is
	// missing the workspace is in a broken state and the app refuses
	// to start rather than fall back to inferring from
	// cfg.Zitadel.AdminAPIURL like the legacy path used to. Pre-v1
	// the fix is `mise run infra:reset` + fresh install, not
	// self-healing.
	if ws, wsErr := queries.GetWorkspace(ctx); wsErr == nil && string(ws.InstallState) == "ready" {
		issuer := ""
		if ws.ZitadelIssuerUrl.Valid {
			issuer = ws.ZitadelIssuerUrl.String
		}
		audience := ""
		if ws.ZitadelApiAudience.Valid {
			audience = ws.ZitadelApiAudience.String
		}
		if issuer == "" || audience == "" {
			slog.Error(
				"workspace is ready but persisted auth contract is incomplete; refusing to start",
				"issuer_present", issuer != "",
				"audience_present", audience != "",
				"hint", "pre-v1 contract: drop the DB and reinstall; no auto-repair",
			)
			pool.Close()
			os.Exit(1)
		}
		if err := gate.Activate(ctx, issuer, audience); err != nil {
			slog.Error("auth gate startup activation failed; refusing to start", "error", err)
			pool.Close()
			os.Exit(1)
		}
	} else {
		slog.Info("auth disabled: workspace not installed yet; /install flow remains public")
	}
	// /_gofra/config.js is served by the publicconfig wrapper so the
	// browser receives auth.orgId from the workspace singleton. The SPA
	// uses that field to scope its OIDC login request to the MSP org.
	app.Handle(runtimeconfig.DefaultPath, publicconfig.Handler(cfg, workspaceAuthProvider{queries: queries}))

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

// logInstallTokenSource emits a startup line that an operator can use to
// find the install token. For env/file sources it confirms the path so a
// silent misload (file points at the wrong copy) is visible. For the
// generated fallback it prints the literal token to stdout because that
// is the only place an operator can find it — restart regenerates it.
func logInstallTokenSource(token string, src installtoken.Source) {
	switch src {
	case installtoken.SourceLiteralEnv:
		slog.Info("install token loaded from env", "var", installtoken.EnvLiteral)
	case installtoken.SourceFile:
		slog.Info("install token loaded from file", "path", os.Getenv(installtoken.EnvFile))
	case installtoken.SourceGenerated:
		slog.Warn(
			"install token generated in-process (not persisted across restarts) — paste it into the /install wizard",
			"token", token,
			"persist_hint", "set "+installtoken.EnvFile+" or "+installtoken.EnvLiteral+" to keep it stable",
		)
	default:
		slog.Warn("install token loaded from unknown source")
	}
}

// resolveProvisionerPATPath returns the filesystem path the patwatch
// should observe: env-var override (Kubernetes Secret mount) beats
// the gofra.yaml config value. Path-only — actual file read +
// last-known-good behavior live in internal/patwatch.
func resolveProvisionerPATPath(cfg *config.Config) (string, error) {
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
	return path, nil
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
