package install_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Gabrielbdd/gospa/db/sqlc"
	"github.com/Gabrielbdd/gospa/internal/install"
)

// repairQueries captures the RepairWorkspaceAuthContract call so tests
// can assert exactly what (if anything) the repair path persisted.
type repairQueries struct {
	fakeQueries
	repairCalled bool
	repairParams sqlc.RepairWorkspaceAuthContractParams
}

func (r *repairQueries) RepairWorkspaceAuthContract(_ context.Context, arg sqlc.RepairWorkspaceAuthContractParams) error {
	r.repairCalled = true
	r.repairParams = arg
	return nil
}

func discardLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestRepairAuthContract_NoOpWhenWorkspaceNotReady(t *testing.T) {
	q := &repairQueries{}
	q.workspace = sqlc.Workspace{
		InstallState: sqlc.WorkspaceInstallStateProvisioning,
	}

	install.RepairAuthContract(context.Background(), q, newConfig("", "", "https://admin.example.com"), discardLog())

	if q.repairCalled {
		t.Error("repair should not run for non-ready workspaces")
	}
}

func TestRepairAuthContract_NoOpWhenAllFieldsPopulated(t *testing.T) {
	q := &repairQueries{}
	q.workspace = sqlc.Workspace{
		InstallState:         sqlc.WorkspaceInstallStateReady,
		ZitadelIssuerUrl:     pgtype.Text{String: "https://existing-issuer", Valid: true},
		ZitadelManagementUrl: pgtype.Text{String: "https://existing-mgmt", Valid: true},
		ZitadelApiAudience:   pgtype.Text{String: "existing-audience", Valid: true},
	}

	install.RepairAuthContract(context.Background(), q, newConfig("https://other-issuer", "other-aud", "https://other-mgmt"), discardLog())

	if q.repairCalled {
		t.Error("repair should not run when all three columns are populated")
	}
}

func TestRepairAuthContract_FillsOnlyMissingFields(t *testing.T) {
	q := &repairQueries{}
	q.workspace = sqlc.Workspace{
		InstallState:         sqlc.WorkspaceInstallStateReady,
		ZitadelProjectID:     pgtype.Text{String: "project-42", Valid: true},
		ZitadelManagementUrl: pgtype.Text{String: "https://existing-mgmt", Valid: true},
		// IssuerUrl + ApiAudience deliberately NULL.
	}

	install.RepairAuthContract(context.Background(), q,
		newConfig("https://issuer.example.com", "", "https://admin.example.com"),
		discardLog())

	if !q.repairCalled {
		t.Fatal("expected repair to run because issuer + audience are missing")
	}

	if !q.repairParams.ZitadelIssuerUrl.Valid || q.repairParams.ZitadelIssuerUrl.String != "https://issuer.example.com" {
		t.Errorf("issuer param = %+v; want filled with cfg.Auth.Issuer", q.repairParams.ZitadelIssuerUrl)
	}
	if q.repairParams.ZitadelManagementUrl.Valid {
		t.Errorf("management param = %+v; want left NULL because column already populated", q.repairParams.ZitadelManagementUrl)
	}
	if !q.repairParams.ZitadelApiAudience.Valid || q.repairParams.ZitadelApiAudience.String != "project-42" {
		t.Errorf("audience param = %+v; want filled with workspace.zitadel_project_id fallback", q.repairParams.ZitadelApiAudience)
	}
}

func TestRepairAuthContract_PrefersExplicitAuthAudienceOverProjectID(t *testing.T) {
	q := &repairQueries{}
	q.workspace = sqlc.Workspace{
		InstallState:     sqlc.WorkspaceInstallStateReady,
		ZitadelProjectID: pgtype.Text{String: "project-42", Valid: true},
	}

	install.RepairAuthContract(context.Background(), q,
		newConfig("https://issuer.example.com", "explicit-audience", "https://admin.example.com"),
		discardLog())

	if !q.repairCalled {
		t.Fatal("expected repair to run")
	}
	if q.repairParams.ZitadelApiAudience.String != "explicit-audience" {
		t.Errorf("audience = %q; want explicit cfg.Auth.Audience over project_id", q.repairParams.ZitadelApiAudience.String)
	}
}

func TestRepairAuthContract_LeavesAudienceNullWhenNoSafeSource(t *testing.T) {
	q := &repairQueries{}
	q.workspace = sqlc.Workspace{
		InstallState: sqlc.WorkspaceInstallStateReady,
		// No project_id, no cfg.Auth.Audience — audience cannot be repaired.
	}

	install.RepairAuthContract(context.Background(), q,
		newConfig("https://issuer.example.com", "", "https://admin.example.com"),
		discardLog())

	if !q.repairCalled {
		t.Fatal("expected repair to run for issuer + management even when audience is not derivable")
	}
	if q.repairParams.ZitadelApiAudience.Valid {
		t.Errorf("audience param = %+v; want left NULL when no safe source", q.repairParams.ZitadelApiAudience)
	}
	if !q.repairParams.ZitadelIssuerUrl.Valid || !q.repairParams.ZitadelManagementUrl.Valid {
		t.Error("issuer + management should still be repaired independently of audience derivability")
	}
}

func TestRepairAuthContract_SkipsUpdateEntirelyWhenNothingDerivable(t *testing.T) {
	q := &repairQueries{}
	q.workspace = sqlc.Workspace{
		InstallState:         sqlc.WorkspaceInstallStateReady,
		ZitadelIssuerUrl:     pgtype.Text{String: "kept-issuer", Valid: true},
		ZitadelManagementUrl: pgtype.Text{String: "kept-mgmt", Valid: true},
		// Audience NULL; no project_id; no cfg.Auth.Audience.
	}

	install.RepairAuthContract(context.Background(), q,
		newConfig("", "", "https://admin.example.com"),
		discardLog())

	if q.repairCalled {
		t.Error("repair should not run when no field has a safe derivation")
	}
}
