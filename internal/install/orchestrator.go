// Package install owns the /install wizard backend: the Connect handler
// surface for status + submission, and the orchestrator that provisions
// the MSP workspace in ZITADEL on a submission.
package install

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Gabrielbdd/gospa/config"
	"github.com/Gabrielbdd/gospa/db/sqlc"
	"github.com/Gabrielbdd/gospa/internal/zitadel"
	"github.com/Gabrielbdd/gospa/internal/zitadelcontract"
)

// Queries is the subset of sqlc's generated interface the orchestrator and
// handler consume. Keeping it narrow here makes test doubles trivial.
type Queries interface {
	GetWorkspace(ctx context.Context) (sqlc.Workspace, error)
	MarkWorkspaceProvisioning(ctx context.Context, arg sqlc.MarkWorkspaceProvisioningParams) error
	MarkWorkspaceFailed(ctx context.Context, installError pgtype.Text) error
	MarkWorkspaceReady(ctx context.Context) error
	PersistZitadelIDs(ctx context.Context, arg sqlc.PersistZitadelIDsParams) error
	RepairWorkspaceAuthContract(ctx context.Context, arg sqlc.RepairWorkspaceAuthContractParams) error
}

// Input captures the workspace and initial-admin fields the wizard
// collects. The orchestrator validates required fields and uses them to
// drive the six-step pipeline below.
type Input struct {
	WorkspaceName string
	WorkspaceSlug string
	Timezone      string
	CurrencyCode  string
	AdminEmail    string
	AdminFirst    string
	AdminLast     string
	// AdminPassword is set on the freshly-created admin user so they
	// can log in immediately. Without it ZITADEL would email an init
	// code, which fails on any deploy without configured SMTP.
	AdminPassword string
	// APIBaseURL is the browser-visible base URL the OIDC SPA should
	// redirect back to; used to derive the OIDC app's RedirectURIs and
	// post-logout URIs. Typically cfg.Public.ApiBaseUrl.
	APIBaseURL string
}

// Orchestrator runs the install pipeline. Construct once at startup; call
// Run in a goroutine after POST /install flips the workspace state to
// provisioning.
type Orchestrator struct {
	Queries Queries
	Zitadel zitadel.Client
	Config  *config.Config
	Logger  *slog.Logger

	// OnReady, if set, runs after MarkWorkspaceReady succeeds. It
	// receives the freshly-derived auth contract (issuer + audience +
	// management) and is wired by cmd/app to authgate.Activate so the
	// JWT middleware flips from pass-through to authenticated the
	// moment install finishes — no process restart required. Errors
	// are logged but do not fail the install.
	OnReady func(ctx context.Context, contract zitadelcontract.Contract) error
}

// Run executes the six ordered steps, writing each outcome to the
// workspace row. Any step failure flips install_state to failed with a
// human-readable message. Callers do not need to handle the returned
// error beyond logging — state is already persisted.
//
// When a step after SetUpOrg fails, the orchestrator opportunistically
// asks ZITADEL to remove the org it just created (cascade-deletes the
// project + OIDC app). This keeps the next install attempt from
// stacking new orgs alongside orphans. The cleanup is best-effort —
// if RemoveOrg itself fails (ZITADEL down, network), the original
// failure and the cleanup failure are both recorded in install_error
// and the operator is alerted via logs. No retry; saga is out of
// scope.
//
// SetUpOrg failures intentionally do not trigger cleanup: ZITADEL may
// have created the org partially (without returning an id), and we
// have no handle to delete what we cannot identify. Operator cleanup
// stays the contract for that case.
func (o *Orchestrator) Run(ctx context.Context, in Input) error {
	log := o.Logger
	if log == nil {
		log = slog.Default()
	}

	var createdOrgID string

	fail := func(step string, err error) error {
		msg := fmt.Sprintf("%s: %s", step, err.Error())
		if createdOrgID != "" {
			if rmErr := o.Zitadel.RemoveOrg(context.Background(), createdOrgID); rmErr != nil {
				msg += " | cleanup failed: " + rmErr.Error()
				log.ErrorContext(ctx, "install cleanup failed; org left orphaned in ZITADEL",
					"org_id", createdOrgID,
					"step", step,
					"error", rmErr,
				)
			} else {
				log.InfoContext(ctx, "install rolled back created org",
					"org_id", createdOrgID,
					"step", step,
				)
			}
		}
		log.ErrorContext(ctx, "install step failed", "step", step, "error", err)
		if markErr := o.Queries.MarkWorkspaceFailed(ctx, pgtype.Text{String: msg, Valid: true}); markErr != nil {
			log.ErrorContext(ctx, "failed to mark workspace failed", "error", markErr)
		}
		return fmt.Errorf("%s: %w", step, err)
	}

	// Step 1: create MSP org (and initial admin user in one shot).
	log.InfoContext(ctx, "install step starting", "step", "create_org")
	orgResp, err := o.Zitadel.SetUpOrg(ctx, zitadel.SetUpOrgRequest{
		OrgName:   in.WorkspaceName,
		UserEmail: in.AdminEmail,
		FirstName: in.AdminFirst,
		LastName:  in.AdminLast,
		Password:  in.AdminPassword,
	})
	if err != nil {
		return fail("create_org", err)
	}
	// From here on, any step failure must compensate by deleting the
	// org we just created (ZITADEL cascades to project + OIDC app).
	createdOrgID = orgResp.OrgID

	// Step 2 is implicit in SetUpOrg — the first human user is created
	// together with the org. Kept as a visible log line so operators see
	// the six-step flow in logs.
	log.InfoContext(ctx, "install step complete", "step", "create_initial_user", "user_id", orgResp.UserID)

	// Step 3: create project inside the MSP org.
	log.InfoContext(ctx, "install step starting", "step", "create_project")
	projectID, err := o.Zitadel.AddProject(ctx, orgResp.OrgID, "Gospa")
	if err != nil {
		return fail("create_project", err)
	}

	// Step 4: create OIDC SPA application inside the project.
	log.InfoContext(ctx, "install step starting", "step", "create_oidc_app")
	appResp, err := o.Zitadel.AddOIDCApp(ctx, orgResp.OrgID, projectID, zitadel.AddOIDCAppRequest{
		Name:           "Gospa Web",
		RedirectURIs:   []string{in.APIBaseURL + "/auth/callback"},
		PostLogoutURIs: []string{in.APIBaseURL + "/"},
		DevMode:        true,
	})
	if err != nil {
		return fail("create_oidc_app", err)
	}

	// Step 5: persist the seven-field auth contract on the singleton row.
	// Identifiers come straight from the ZITADEL responses; the auth
	// contract (issuer/management/audience) is derived once via the
	// zitadelcontract helper so the orchestrator and the startup
	// read-repair path agree on the rules.
	log.InfoContext(ctx, "install step starting", "step", "persist_ids")
	contract := zitadelcontract.DeriveFresh(o.Config, projectID)
	if err := o.Queries.PersistZitadelIDs(ctx, sqlc.PersistZitadelIDsParams{
		ZitadelOrgID:         pgtype.Text{String: orgResp.OrgID, Valid: true},
		ZitadelProjectID:     pgtype.Text{String: projectID, Valid: true},
		ZitadelSpaAppID:      pgtype.Text{String: appResp.AppID, Valid: true},
		ZitadelSpaClientID:   pgtype.Text{String: appResp.ClientID, Valid: true},
		ZitadelIssuerUrl:     pgtype.Text{String: contract.IssuerURL, Valid: contract.IssuerURL != ""},
		ZitadelManagementUrl: pgtype.Text{String: contract.ManagementURL, Valid: contract.ManagementURL != ""},
		ZitadelApiAudience:   pgtype.Text{String: contract.APIAudience, Valid: contract.APIAudience != ""},
	}); err != nil {
		return fail("persist_ids", err)
	}

	// Step 6: flip the workspace row to ready.
	log.InfoContext(ctx, "install step starting", "step", "mark_ready")
	if err := o.Queries.MarkWorkspaceReady(ctx); err != nil {
		return fail("mark_ready", err)
	}

	log.InfoContext(ctx, "install complete", "org_id", orgResp.OrgID, "project_id", projectID, "app_id", appResp.AppID)

	if o.OnReady != nil {
		if err := o.OnReady(ctx, contract); err != nil {
			log.WarnContext(ctx, "install OnReady hook failed", "error", err)
		}
	}
	return nil
}
