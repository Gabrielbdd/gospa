package install

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Gabrielbdd/gospa/config"
	"github.com/Gabrielbdd/gospa/db/sqlc"
	"github.com/Gabrielbdd/gospa/internal/zitadelcontract"
)

// RepairAuthContract fills any missing auth-contract columns on the
// workspace row when install_state = ready. Designed to be idempotent
// and safe across restarts:
//
//   - no-op for workspaces that are not in the ready state
//   - no-op when all three columns are already populated
//   - per-field: only fills the columns that are currently NULL
//   - skips entirely when no field has a safe derivation
//
// COALESCE on the SQL side gives the same per-field guarantee from a
// concurrency standpoint: even if two replicas race the repair, the
// later UPDATE never overwrites a value that is already non-NULL.
//
// Designed to be called once at startup from cmd/app, after the
// crash-recovery sweep and before the auth gate is mounted.
func RepairAuthContract(ctx context.Context, queries Queries, cfg *config.Config, log *slog.Logger) {
	if log == nil {
		log = slog.Default()
	}

	ws, err := queries.GetWorkspace(ctx)
	if err != nil {
		log.WarnContext(ctx, "auth-contract read-repair: could not load workspace", "error", err)
		return
	}
	if string(ws.InstallState) != "ready" {
		return
	}

	hasIssuer := ws.ZitadelIssuerUrl.Valid && ws.ZitadelIssuerUrl.String != ""
	hasManagement := ws.ZitadelManagementUrl.Valid && ws.ZitadelManagementUrl.String != ""
	hasAudience := ws.ZitadelApiAudience.Valid && ws.ZitadelApiAudience.String != ""
	if hasIssuer && hasManagement && hasAudience {
		return
	}

	derived := zitadelcontract.DeriveRepair(cfg, ws)

	params := sqlc.RepairWorkspaceAuthContractParams{}
	if !hasIssuer && derived.IssuerURL != "" {
		params.ZitadelIssuerUrl = pgtype.Text{String: derived.IssuerURL, Valid: true}
	}
	if !hasManagement && derived.ManagementURL != "" {
		params.ZitadelManagementUrl = pgtype.Text{String: derived.ManagementURL, Valid: true}
	}
	if !hasAudience && derived.APIAudience != "" {
		params.ZitadelApiAudience = pgtype.Text{String: derived.APIAudience, Valid: true}
	}

	if !params.ZitadelIssuerUrl.Valid && !params.ZitadelManagementUrl.Valid && !params.ZitadelApiAudience.Valid {
		log.WarnContext(ctx, "auth-contract read-repair: no safe derivation for missing fields; leaving them NULL",
			"missing_issuer", !hasIssuer,
			"missing_management", !hasManagement,
			"missing_audience", !hasAudience,
		)
		return
	}

	if err := queries.RepairWorkspaceAuthContract(ctx, params); err != nil {
		log.WarnContext(ctx, "auth-contract read-repair: update failed", "error", err)
		return
	}
	log.InfoContext(ctx, "auth-contract read-repair: filled missing columns",
		"filled_issuer", params.ZitadelIssuerUrl.Valid,
		"filled_management", params.ZitadelManagementUrl.Valid,
		"filled_audience", params.ZitadelApiAudience.Valid,
	)
}
