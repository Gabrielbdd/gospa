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
	"github.com/Gabrielbdd/gospa/internal/zitadelcontract"
)

// fakeQueries is a hand-written stand-in for the sqlc Queries struct,
// capturing every state transition for assertions.
type fakeQueries struct {
	mu                 sync.Mutex
	workspace          sqlc.Workspace
	markedProvisioning sqlc.MarkWorkspaceProvisioningParams
	markedReady        bool
	markedFailedErr    pgtype.Text
	persistedIDs       sqlc.PersistZitadelIDsParams

	createdWorkspaceCompanyParams sqlc.CreateWorkspaceCompanyParams
	createdWorkspaceCompany       sqlc.Company
	createdContactParams          sqlc.CreateContactParams
	createdContact                sqlc.Contact
	createdGrantParams            sqlc.CreateWorkspaceGrantParams

	getWorkspaceErr           error
	markProvisioningErr       error
	markReadyErr              error
	markFailedErr             error
	persistIDsErr             error
	createWorkspaceCompanyErr error
	createContactErr          error
	createGrantErr            error
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

func (f *fakeQueries) CreateWorkspaceCompany(ctx context.Context, arg sqlc.CreateWorkspaceCompanyParams) (sqlc.Company, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createdWorkspaceCompanyParams = arg
	if f.createWorkspaceCompanyErr != nil {
		return sqlc.Company{}, f.createWorkspaceCompanyErr
	}
	if f.createdWorkspaceCompany.ID.Valid {
		return f.createdWorkspaceCompany, nil
	}
	// Default: synthesise a row that mirrors the params so downstream
	// CreateContact has a non-empty company_id to attach to.
	row := sqlc.Company{
		ID:               pgtype.UUID{Bytes: [16]byte{0xC, 0xC}, Valid: true},
		Name:             arg.Name,
		ZitadelOrgID:     arg.ZitadelOrgID,
		IsWorkspaceOwner: true,
	}
	f.createdWorkspaceCompany = row
	return row, nil
}

func (f *fakeQueries) CreateContact(ctx context.Context, arg sqlc.CreateContactParams) (sqlc.Contact, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createdContactParams = arg
	if f.createContactErr != nil {
		return sqlc.Contact{}, f.createContactErr
	}
	row := sqlc.Contact{
		ID:             pgtype.UUID{Bytes: [16]byte{0xA, 0xA}, Valid: true},
		CompanyID:      arg.CompanyID,
		FullName:       arg.FullName,
		Email:          arg.Email,
		ZitadelUserID:  arg.ZitadelUserID,
		IdentitySource: arg.IdentitySource,
	}
	f.createdContact = row
	return row, nil
}

func (f *fakeQueries) CreateWorkspaceGrant(ctx context.Context, arg sqlc.CreateWorkspaceGrantParams) (sqlc.WorkspaceGrant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createdGrantParams = arg
	if f.createGrantErr != nil {
		return sqlc.WorkspaceGrant{}, f.createGrantErr
	}
	return sqlc.WorkspaceGrant{
		ContactID: arg.ContactID,
		Role:      arg.Role,
		Status:    arg.Status,
	}, nil
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
	removeOrgErr   error

	setUpOrgReq    zitadel.SetUpOrgRequest
	addProjectOrg  string
	addOIDCOrg     string
	addOIDCProj    string
	addOIDCReq     zitadel.AddOIDCAppRequest
	removeOrgCalls []string // org ids passed to RemoveOrg, in order
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
func (z *fakeZitadel) RemoveOrg(ctx context.Context, orgID string) error {
	z.removeOrgCalls = append(z.removeOrgCalls, orgID)
	return z.removeOrgErr
}
func (z *fakeZitadel) AddHumanUser(ctx context.Context, _ string, _ zitadel.AddHumanUserRequest) (string, error) {
	return "", nil
}
func (z *fakeZitadel) RemoveUser(ctx context.Context, _, _ string) error {
	return nil
}

func newLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newInput() install.Input {
	return install.Input{
		WorkspaceName: "Acme MSP",
		Timezone:      "America/Sao_Paulo",
		CurrencyCode:  "BRL",
		AdminEmail:    "admin@acme.test",
		AdminFirst:    "Adam",
		AdminLast:     "Admin",
		AdminPassword: "correct-horse-battery-staple",
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
	var onReadyContract zitadelcontract.Contract
	o := install.Orchestrator{
		Queries: q,
		Zitadel: z,
		Config:  newConfig("https://issuer.example.com", "", "https://admin.example.com"),
		Logger:  newLogger(),
		OnReady: func(_ context.Context, contract zitadelcontract.Contract) error {
			onReadyContract = contract
			return nil
		},
	}

	if err := o.Run(context.Background(), newInput()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// OnReady receives the freshly-derived contract, not just the
	// project id. cmd/app uses it to call gate.Activate(issuer, audience).
	if onReadyContract.IssuerURL != "https://issuer.example.com" {
		t.Errorf("OnReady contract issuer = %q; want explicit cfg.Auth.Issuer", onReadyContract.IssuerURL)
	}
	if onReadyContract.APIAudience != "proj-1" {
		t.Errorf("OnReady contract audience = %q; want project_id", onReadyContract.APIAudience)
	}
	if onReadyContract.ManagementURL != "https://admin.example.com" {
		t.Errorf("OnReady contract management = %q; want admin URL", onReadyContract.ManagementURL)
	}

	if z.setUpOrgReq.OrgName != "Acme MSP" {
		t.Errorf("org name not forwarded: %+v", z.setUpOrgReq)
	}
	if z.setUpOrgReq.Password != "correct-horse-battery-staple" {
		t.Errorf("admin password not forwarded to SetUpOrg: %+v", z.setUpOrgReq)
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
		OnReady: func(_ context.Context, _ zitadelcontract.Contract) error {
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

// --- S15: opportunistic ZITADEL cleanup -------------------------------------

func TestOrchestrator_CleansUpOrgOnAddProjectFailure(t *testing.T) {
	q := &fakeQueries{}
	z := &fakeZitadel{
		setUpOrgResp:  zitadel.SetUpOrgResponse{OrgID: "org-1"},
		addProjectErr: errors.New("zitadel down"),
	}
	o := install.Orchestrator{
		Queries: q,
		Zitadel: z,
		Config:  newConfig("", "", "https://admin.example.com"),
		Logger:  newLogger(),
	}

	if err := o.Run(context.Background(), newInput()); err == nil {
		t.Fatal("expected error from AddProject failure")
	}
	if len(z.removeOrgCalls) != 1 || z.removeOrgCalls[0] != "org-1" {
		t.Errorf("RemoveOrg calls = %v; want [org-1]", z.removeOrgCalls)
	}
	if !q.markedFailedErr.Valid {
		t.Fatal("expected workspace marked failed")
	}
	if !strings.Contains(q.markedFailedErr.String, "create_project") {
		t.Errorf("install_error = %q; want to mention create_project", q.markedFailedErr.String)
	}
	if strings.Contains(q.markedFailedErr.String, "cleanup failed") {
		t.Errorf("install_error mentions cleanup failure unexpectedly: %q", q.markedFailedErr.String)
	}
}

func TestOrchestrator_CleansUpOrgOnAddOIDCAppFailure(t *testing.T) {
	q := &fakeQueries{}
	z := &fakeZitadel{
		setUpOrgResp:   zitadel.SetUpOrgResponse{OrgID: "org-1"},
		addProjectResp: "proj-1",
		addOIDCErr:     errors.New("zitadel rejected app"),
	}
	o := install.Orchestrator{
		Queries: q,
		Zitadel: z,
		Config:  newConfig("", "", "https://admin.example.com"),
		Logger:  newLogger(),
	}

	if err := o.Run(context.Background(), newInput()); err == nil {
		t.Fatal("expected error from AddOIDCApp failure")
	}
	if len(z.removeOrgCalls) != 1 || z.removeOrgCalls[0] != "org-1" {
		t.Errorf("RemoveOrg calls = %v; want [org-1] (cascade removes project + app)", z.removeOrgCalls)
	}
	if !strings.Contains(q.markedFailedErr.String, "create_oidc_app") {
		t.Errorf("install_error = %q; want to mention create_oidc_app", q.markedFailedErr.String)
	}
}

func TestOrchestrator_CleansUpOrgOnPersistFailure(t *testing.T) {
	q := &fakeQueries{persistIDsErr: errors.New("db write failed")}
	z := &fakeZitadel{
		setUpOrgResp:   zitadel.SetUpOrgResponse{OrgID: "org-1"},
		addProjectResp: "proj-1",
		addOIDCResp:    zitadel.AddOIDCAppResponse{AppID: "app-1", ClientID: "cli-1"},
	}
	o := install.Orchestrator{
		Queries: q,
		Zitadel: z,
		Config:  newConfig("", "", "https://admin.example.com"),
		Logger:  newLogger(),
	}

	if err := o.Run(context.Background(), newInput()); err == nil {
		t.Fatal("expected error from persist failure")
	}
	if len(z.removeOrgCalls) != 1 || z.removeOrgCalls[0] != "org-1" {
		t.Errorf("RemoveOrg calls = %v; want [org-1]", z.removeOrgCalls)
	}
	if !strings.Contains(q.markedFailedErr.String, "persist_ids") {
		t.Errorf("install_error = %q; want to mention persist_ids", q.markedFailedErr.String)
	}
}

func TestOrchestrator_RecordsCleanupFailureInInstallError(t *testing.T) {
	q := &fakeQueries{}
	z := &fakeZitadel{
		setUpOrgResp:  zitadel.SetUpOrgResponse{OrgID: "org-1"},
		addProjectErr: errors.New("zitadel transient"),
		removeOrgErr:  errors.New("ZITADEL still down"),
	}
	o := install.Orchestrator{
		Queries: q,
		Zitadel: z,
		Config:  newConfig("", "", "https://admin.example.com"),
		Logger:  newLogger(),
	}

	if err := o.Run(context.Background(), newInput()); err == nil {
		t.Fatal("expected error")
	}
	if len(z.removeOrgCalls) != 1 {
		t.Errorf("RemoveOrg calls = %v; want exactly one attempt (no retry)", z.removeOrgCalls)
	}
	msg := q.markedFailedErr.String
	if !strings.Contains(msg, "create_project") {
		t.Errorf("install_error missing original failure: %q", msg)
	}
	if !strings.Contains(msg, "cleanup failed") {
		t.Errorf("install_error missing cleanup failure marker: %q", msg)
	}
	if !strings.Contains(msg, "ZITADEL still down") {
		t.Errorf("install_error missing cleanup error message: %q", msg)
	}
}

// --- Slice 0: team materialisation -----------------------------------

func TestOrchestrator_MaterializesTeamBootstrap(t *testing.T) {
	q := &fakeQueries{}
	z := &fakeZitadel{
		setUpOrgResp:   zitadel.SetUpOrgResponse{OrgID: "org-1", UserID: "user-1"},
		addProjectResp: "proj-1",
		addOIDCResp:    zitadel.AddOIDCAppResponse{AppID: "app-1", ClientID: "cli-1"},
	}
	o := install.Orchestrator{
		Queries: q,
		Zitadel: z,
		Config:  newConfig("https://issuer.example.com", "", "https://admin.example.com"),
		Logger:  newLogger(),
	}

	if err := o.Run(context.Background(), newInput()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// MSP company materialised with the workspace name + org id.
	if q.createdWorkspaceCompanyParams.Name != "Acme MSP" {
		t.Errorf("workspace company name = %q; want Acme MSP", q.createdWorkspaceCompanyParams.Name)
	}
	if q.createdWorkspaceCompanyParams.ZitadelOrgID != "org-1" {
		t.Errorf("workspace company zitadel_org_id = %q; want org-1", q.createdWorkspaceCompanyParams.ZitadelOrgID)
	}

	// Admin contact attached to the MSP company with the recovered
	// identity fields. FullName is the wizard's first+last when both
	// are present.
	if !q.createdContactParams.ZitadelUserID.Valid || q.createdContactParams.ZitadelUserID.String != "user-1" {
		t.Errorf("contact zitadel_user_id = %+v; want user-1", q.createdContactParams.ZitadelUserID)
	}
	if q.createdContactParams.FullName != "Adam Admin" {
		t.Errorf("contact full_name = %q; want Adam Admin", q.createdContactParams.FullName)
	}
	if !q.createdContactParams.Email.Valid || q.createdContactParams.Email.String != "admin@acme.test" {
		t.Errorf("contact email = %+v; want admin@acme.test", q.createdContactParams.Email)
	}
	if q.createdContactParams.IdentitySource != "manual" {
		t.Errorf("contact identity_source = %q; want manual", q.createdContactParams.IdentitySource)
	}

	// Admin grant points at the contact, role admin, status active.
	if q.createdGrantParams.Role != sqlc.WorkspaceRoleAdmin {
		t.Errorf("grant role = %q; want admin", q.createdGrantParams.Role)
	}
	if q.createdGrantParams.Status != sqlc.GrantStatusActive {
		t.Errorf("grant status = %q; want active", q.createdGrantParams.Status)
	}
}

func TestOrchestrator_CleansUpOrgOnTeamMaterialisationFailure(t *testing.T) {
	q := &fakeQueries{createContactErr: errors.New("contact insert failed")}
	z := &fakeZitadel{
		setUpOrgResp:   zitadel.SetUpOrgResponse{OrgID: "org-1", UserID: "user-1"},
		addProjectResp: "proj-1",
		addOIDCResp:    zitadel.AddOIDCAppResponse{AppID: "app-1", ClientID: "cli-1"},
	}
	o := install.Orchestrator{
		Queries: q,
		Zitadel: z,
		Config:  newConfig("", "", "https://admin.example.com"),
		Logger:  newLogger(),
	}

	if err := o.Run(context.Background(), newInput()); err == nil {
		t.Fatal("expected error from CreateContact failure")
	}
	if len(z.removeOrgCalls) != 1 || z.removeOrgCalls[0] != "org-1" {
		t.Errorf("RemoveOrg calls = %v; want [org-1] (cascade after team materialisation failure)", z.removeOrgCalls)
	}
	if !strings.Contains(q.markedFailedErr.String, "materialize_team") {
		t.Errorf("install_error = %q; want to mention materialize_team", q.markedFailedErr.String)
	}
	if q.markedReady {
		t.Error("workspace must not be marked ready when team materialisation fails")
	}
}

func TestOrchestrator_NoCleanupWhenSetUpOrgFails(t *testing.T) {
	q := &fakeQueries{}
	z := &fakeZitadel{setUpOrgErr: errors.New("zitadel rejected")}
	o := install.Orchestrator{
		Queries: q,
		Zitadel: z,
		Config:  newConfig("", "", "https://admin.example.com"),
		Logger:  newLogger(),
	}

	if err := o.Run(context.Background(), newInput()); err == nil {
		t.Fatal("expected error")
	}
	if len(z.removeOrgCalls) != 0 {
		t.Errorf("RemoveOrg calls = %v; want none — no orgID was returned, nothing to clean", z.removeOrgCalls)
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
