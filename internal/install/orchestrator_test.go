package install_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Gabrielbdd/gospa/db/sqlc"
	"github.com/Gabrielbdd/gospa/internal/install"
	"github.com/Gabrielbdd/gospa/internal/zitadel"
)

// fakeQueries is a hand-written stand-in for the sqlc Queries struct,
// capturing every state transition for assertions.
type fakeQueries struct {
	mu              sync.Mutex
	workspace       sqlc.Workspace
	markedProvisioning sqlc.MarkWorkspaceProvisioningParams
	markedReady     bool
	markedFailedErr pgtype.Text
	persistedIDs    sqlc.PersistZitadelIDsParams

	getWorkspaceErr       error
	markProvisioningErr   error
	markReadyErr          error
	markFailedErr         error
	persistIDsErr         error
}

func (f *fakeQueries) GetWorkspace(ctx context.Context) (sqlc.Workspace, error) {
	return f.workspace, f.getWorkspaceErr
}

func (f *fakeQueries) MarkWorkspaceProvisioning(ctx context.Context, arg sqlc.MarkWorkspaceProvisioningParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.markedProvisioning = arg
	return f.markProvisioningErr
}

func (f *fakeQueries) MarkWorkspaceFailed(ctx context.Context, err pgtype.Text) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.markedFailedErr = err
	return f.markFailedErr
}

func (f *fakeQueries) MarkWorkspaceReady(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.markedReady = true
	return f.markReadyErr
}

func (f *fakeQueries) PersistZitadelIDs(ctx context.Context, arg sqlc.PersistZitadelIDsParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.persistedIDs = arg
	return f.persistIDsErr
}

// fakeZitadel captures calls and serves configurable responses/errors.
type fakeZitadel struct {
	setUpOrgResp   zitadel.SetUpOrgResponse
	setUpOrgErr    error
	addProjectResp string
	addProjectErr  error
	addOIDCResp    zitadel.AddOIDCAppResponse
	addOIDCErr     error

	setUpOrgReq   zitadel.SetUpOrgRequest
	addProjectOrg string
	addOIDCOrg    string
	addOIDCProj   string
	addOIDCReq    zitadel.AddOIDCAppRequest
}

func (z *fakeZitadel) SetUpOrg(ctx context.Context, req zitadel.SetUpOrgRequest) (zitadel.SetUpOrgResponse, error) {
	z.setUpOrgReq = req
	return z.setUpOrgResp, z.setUpOrgErr
}
func (z *fakeZitadel) AddProject(ctx context.Context, orgID, name string) (string, error) {
	z.addProjectOrg = orgID
	return z.addProjectResp, z.addProjectErr
}
func (z *fakeZitadel) AddOIDCApp(ctx context.Context, orgID, projectID string, req zitadel.AddOIDCAppRequest) (zitadel.AddOIDCAppResponse, error) {
	z.addOIDCOrg = orgID
	z.addOIDCProj = projectID
	z.addOIDCReq = req
	return z.addOIDCResp, z.addOIDCErr
}
func (z *fakeZitadel) AddOrganization(ctx context.Context, name string) (string, error) {
	return "", nil
}

func newLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newInput() install.Input {
	return install.Input{
		WorkspaceName: "Acme MSP",
		WorkspaceSlug: "acme",
		Timezone:      "America/Sao_Paulo",
		CurrencyCode:  "BRL",
		AdminEmail:    "admin@acme.test",
		AdminFirst:    "Adam",
		AdminLast:     "Admin",
		APIBaseURL:    "http://localhost:3000",
	}
}

func TestOrchestrator_Run_HappyPath_PersistsAllIDsAndMarksReady(t *testing.T) {
	q := &fakeQueries{}
	z := &fakeZitadel{
		setUpOrgResp:   zitadel.SetUpOrgResponse{OrgID: "org-1", UserID: "user-1"},
		addProjectResp: "proj-1",
		addOIDCResp:    zitadel.AddOIDCAppResponse{AppID: "app-1", ClientID: "cli-1"},
	}
	o := install.Orchestrator{Queries: q, Zitadel: z, Logger: newLogger()}

	if err := o.Run(context.Background(), newInput()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if z.setUpOrgReq.OrgName != "Acme MSP" {
		t.Errorf("org name not forwarded: %+v", z.setUpOrgReq)
	}
	if z.addProjectOrg != "org-1" {
		t.Errorf("project created under wrong org: %q", z.addProjectOrg)
	}
	if z.addOIDCProj != "proj-1" || z.addOIDCOrg != "org-1" {
		t.Errorf("OIDC app created under wrong project/org: proj=%q org=%q", z.addOIDCProj, z.addOIDCOrg)
	}
	if want := "http://localhost:3000/auth/callback"; len(z.addOIDCReq.RedirectURIs) == 0 || z.addOIDCReq.RedirectURIs[0] != want {
		t.Errorf("redirect URIs = %v; want [%q]", z.addOIDCReq.RedirectURIs, want)
	}
	if q.persistedIDs.ZitadelOrgID.String != "org-1" ||
		q.persistedIDs.ZitadelProjectID.String != "proj-1" ||
		q.persistedIDs.ZitadelSpaAppID.String != "app-1" ||
		q.persistedIDs.ZitadelSpaClientID.String != "cli-1" {
		t.Errorf("persisted IDs incorrect: %+v", q.persistedIDs)
	}
	if !q.markedReady {
		t.Error("expected workspace marked ready")
	}
	if q.markedFailedErr.Valid {
		t.Errorf("unexpected failure mark: %+v", q.markedFailedErr)
	}
}

func TestOrchestrator_Run_OrgFailureMarksFailedAndStops(t *testing.T) {
	q := &fakeQueries{}
	z := &fakeZitadel{setUpOrgErr: errors.New("zitadel unavailable")}
	o := install.Orchestrator{Queries: q, Zitadel: z, Logger: newLogger()}

	err := o.Run(context.Background(), newInput())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "create_org") {
		t.Errorf("error = %v; want it to mention create_org", err)
	}
	if q.markedReady {
		t.Error("workspace should not be marked ready on org failure")
	}
	if !q.markedFailedErr.Valid || !strings.Contains(q.markedFailedErr.String, "create_org") {
		t.Errorf("failure mark = %+v; want create_org error", q.markedFailedErr)
	}
	if z.addProjectOrg != "" {
		t.Error("AddProject should not have been called after org failure")
	}
}

func TestOrchestrator_Run_PersistFailureStopsBeforeReady(t *testing.T) {
	q := &fakeQueries{persistIDsErr: errors.New("db write failed")}
	z := &fakeZitadel{
		setUpOrgResp:   zitadel.SetUpOrgResponse{OrgID: "o", UserID: "u"},
		addProjectResp: "p",
		addOIDCResp:    zitadel.AddOIDCAppResponse{AppID: "a", ClientID: "c"},
	}
	o := install.Orchestrator{Queries: q, Zitadel: z, Logger: newLogger()}

	err := o.Run(context.Background(), newInput())
	if err == nil {
		t.Fatal("expected error")
	}
	if q.markedReady {
		t.Error("workspace should not be marked ready when persist fails")
	}
	if !strings.Contains(q.markedFailedErr.String, "persist_ids") {
		t.Errorf("failure mark = %+v; want persist_ids error", q.markedFailedErr)
	}
}
