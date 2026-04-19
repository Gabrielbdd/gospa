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

	"github.com/Gabrielbdd/gospa/config"
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

// RepairWorkspaceAuthContract is part of the install.Queries interface
// for the startup read-repair path. The orchestrator never calls it,
// so the stub is a noop here; the cmd/app integration covers it
// elsewhere.
func (f *fakeQueries) RepairWorkspaceAuthContract(ctx context.Context, _ sqlc.RepairWorkspaceAuthContractParams) error {
	return nil
}

func newConfig(issuer, audience, adminURL string) *config.Config {
	return &config.Config{
		Auth:    config.AuthConfig{Issuer: issuer, Audience: audience},
		Zitadel: config.ZitadelConfig{AdminAPIURL: adminURL},
	}
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

func TestOrchestrator_Run_HappyPath_PersistsAllSevenFieldsAndMarksReady(t *testing.T) {
	q := &fakeQueries{}
	z := &fakeZitadel{
		setUpOrgResp:   zitadel.SetUpOrgResponse{OrgID: "org-1", UserID: "user-1"},
		addProjectResp: "proj-1",
		addOIDCResp:    zitadel.AddOIDCAppResponse{AppID: "app-1", ClientID: "cli-1"},
	}
	var onReadyProject string
	o := install.Orchestrator{
		Queries: q,
		Zitadel: z,
		Config:  newConfig("https://issuer.example.com", "", "https://admin.example.com"),
		Logger:  newLogger(),
		OnReady: func(_ context.Context, projectID string) error {
			onReadyProject = projectID
			return nil
		},
	}

	if err := o.Run(context.Background(), newInput()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if onReadyProject != "proj-1" {
		t.Errorf("OnReady received project id %q; want %q", onReadyProject, "proj-1")
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
	// All 7 auth-contract fields must land on the singleton row.
	if q.persistedIDs.ZitadelOrgID.String != "org-1" ||
		q.persistedIDs.ZitadelProjectID.String != "proj-1" ||
		q.persistedIDs.ZitadelSpaAppID.String != "app-1" ||
		q.persistedIDs.ZitadelSpaClientID.String != "cli-1" {
		t.Errorf("persisted identifiers incorrect: %+v", q.persistedIDs)
	}
	if !q.persistedIDs.ZitadelIssuerUrl.Valid || q.persistedIDs.ZitadelIssuerUrl.String != "https://issuer.example.com" {
		t.Errorf("persisted issuer = %+v; want https://issuer.example.com", q.persistedIDs.ZitadelIssuerUrl)
	}
	if !q.persistedIDs.ZitadelManagementUrl.Valid || q.persistedIDs.ZitadelManagementUrl.String != "https://admin.example.com" {
		t.Errorf("persisted management = %+v; want https://admin.example.com", q.persistedIDs.ZitadelManagementUrl)
	}
	if !q.persistedIDs.ZitadelApiAudience.Valid || q.persistedIDs.ZitadelApiAudience.String != "proj-1" {
		t.Errorf("persisted audience = %+v; want proj-1", q.persistedIDs.ZitadelApiAudience)
	}
	if !q.markedReady {
		t.Error("expected workspace marked ready")
	}
	if q.markedFailedErr.Valid {
		t.Errorf("unexpected failure mark: %+v", q.markedFailedErr)
	}
}

func TestOrchestrator_Run_IssuerFallsBackToAdminAPIURLWhenAuthIssuerEmpty(t *testing.T) {
	q := &fakeQueries{}
	z := &fakeZitadel{
		setUpOrgResp:   zitadel.SetUpOrgResponse{OrgID: "org-1"},
		addProjectResp: "proj-1",
		addOIDCResp:    zitadel.AddOIDCAppResponse{AppID: "app-1", ClientID: "cli-1"},
	}
	o := install.Orchestrator{
		Queries: q,
		Zitadel: z,
		// cfg.Auth.Issuer intentionally empty — orchestrator should
		// fall back to cfg.Zitadel.AdminAPIURL for the persisted issuer.
		Config: newConfig("", "", "https://admin.example.com"),
		Logger: newLogger(),
	}

	if err := o.Run(context.Background(), newInput()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !q.persistedIDs.ZitadelIssuerUrl.Valid || q.persistedIDs.ZitadelIssuerUrl.String != "https://admin.example.com" {
		t.Errorf("persisted issuer = %+v; want fallback to admin URL", q.persistedIDs.ZitadelIssuerUrl)
	}
}

func TestOrchestrator_Run_OrgFailureMarksFailedAndStops(t *testing.T) {
	q := &fakeQueries{}
	z := &fakeZitadel{setUpOrgErr: errors.New("zitadel unavailable")}
	var onReadyCalled bool
	o := install.Orchestrator{
		Queries: q,
		Zitadel: z,
		Logger:  newLogger(),
		OnReady: func(_ context.Context, _ string) error {
			onReadyCalled = true
			return nil
		},
	}

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
	if onReadyCalled {
		t.Error("OnReady should not fire on install failure")
	}
}

func TestOrchestrator_Run_PersistFailureStopsBeforeReady(t *testing.T) {
	q := &fakeQueries{persistIDsErr: errors.New("db write failed")}
	z := &fakeZitadel{
		setUpOrgResp:   zitadel.SetUpOrgResponse{OrgID: "o", UserID: "u"},
		addProjectResp: "p",
		addOIDCResp:    zitadel.AddOIDCAppResponse{AppID: "a", ClientID: "c"},
	}
	o := install.Orchestrator{
		Queries: q,
		Zitadel: z,
		Config:  newConfig("", "", "https://admin.example.com"),
		Logger:  newLogger(),
	}

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
