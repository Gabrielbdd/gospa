package companies_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Gabrielbdd/gospa/db/sqlc"
	companiesv1 "github.com/Gabrielbdd/gospa/gen/gospa/companies/v1"
	"github.com/Gabrielbdd/gospa/internal/companies"
	"github.com/Gabrielbdd/gospa/internal/zitadel"
)

type fakeQueries struct {
	workspace  sqlc.Workspace
	workspaceErr error

	createCalled bool
	createParams sqlc.CreateCompanyParams
	createRow    sqlc.Company
	createErr    error

	listRows []sqlc.ListCompaniesRow
	listErr  error

	getCompanyRow sqlc.GetCompanyRow
	getCompanyErr error

	getIncludingRow sqlc.GetCompanyIncludingArchivedRow
	getIncludingErr error

	updateParams sqlc.UpdateCompanyParams
	updateRow    sqlc.Company
	updateErr    error

	updateWorkspaceParams sqlc.UpdateWorkspaceCompanyParams
	updateWorkspaceRow    sqlc.Company
	updateWorkspaceErr    error

	restoreRow sqlc.Company
	restoreErr error

	workspaceCompany sqlc.Company
}

func (f *fakeQueries) GetWorkspace(ctx context.Context) (sqlc.Workspace, error) {
	return f.workspace, f.workspaceErr
}
func (f *fakeQueries) CreateCompany(ctx context.Context, arg sqlc.CreateCompanyParams) (sqlc.Company, error) {
	f.createCalled = true
	f.createParams = arg
	return f.createRow, f.createErr
}
func (f *fakeQueries) ListCompanies(ctx context.Context) ([]sqlc.ListCompaniesRow, error) {
	return f.listRows, f.listErr
}
func (f *fakeQueries) GetCompany(ctx context.Context, _ pgtype.UUID) (sqlc.GetCompanyRow, error) {
	return f.getCompanyRow, f.getCompanyErr
}
func (f *fakeQueries) GetCompanyIncludingArchived(ctx context.Context, _ pgtype.UUID) (sqlc.GetCompanyIncludingArchivedRow, error) {
	if f.getIncludingErr != nil {
		return sqlc.GetCompanyIncludingArchivedRow{}, f.getIncludingErr
	}
	if f.getIncludingRow.ID.Valid {
		return f.getIncludingRow, nil
	}
	// Fall back to whichever mutation row is populated so tests that
	// only set createRow/updateRow/restoreRow still see a coherent
	// hydrated re-fetch without boilerplate.
	for _, c := range []sqlc.Company{f.restoreRow, f.updateRow, f.createRow} {
		if c.ID.Valid {
			return companyToGetIncludingRow(c), nil
		}
	}
	return sqlc.GetCompanyIncludingArchivedRow{}, nil
}

func companyToGetIncludingRow(c sqlc.Company) sqlc.GetCompanyIncludingArchivedRow {
	return sqlc.GetCompanyIncludingArchivedRow{
		ID:               c.ID,
		Name:             c.Name,
		ZitadelOrgID:     c.ZitadelOrgID,
		CreatedAt:        c.CreatedAt,
		ArchivedAt:       c.ArchivedAt,
		IsWorkspaceOwner: c.IsWorkspaceOwner,
		AddressLine1:     c.AddressLine1,
		AddressLine2:     c.AddressLine2,
		City:             c.City,
		Region:           c.Region,
		PostalCode:       c.PostalCode,
		Country:          c.Country,
		Timezone:         c.Timezone,
		OwnerContactID:   c.OwnerContactID,
	}
}
func (f *fakeQueries) ArchiveCompany(ctx context.Context, _ pgtype.UUID) error {
	return nil
}
func (f *fakeQueries) RestoreCompany(ctx context.Context, _ pgtype.UUID) (sqlc.Company, error) {
	return f.restoreRow, f.restoreErr
}
func (f *fakeQueries) UpdateCompany(ctx context.Context, arg sqlc.UpdateCompanyParams) (sqlc.Company, error) {
	f.updateParams = arg
	return f.updateRow, f.updateErr
}
func (f *fakeQueries) UpdateWorkspaceCompany(ctx context.Context, arg sqlc.UpdateWorkspaceCompanyParams) (sqlc.Company, error) {
	f.updateWorkspaceParams = arg
	return f.updateWorkspaceRow, f.updateWorkspaceErr
}
func (f *fakeQueries) GetWorkspaceCompany(ctx context.Context) (sqlc.Company, error) {
	return f.workspaceCompany, nil
}

type fakeZitadel struct {
	addOrgResp string
	addOrgErr  error
	addOrgName string

	renameOrgCalls []renameOrgCall
	renameOrgErr   error
}

func (z *fakeZitadel) AddOrganization(ctx context.Context, name string) (string, error) {
	z.addOrgName = name
	return z.addOrgResp, z.addOrgErr
}
func (z *fakeZitadel) SetUpOrg(ctx context.Context, _ zitadel.SetUpOrgRequest) (zitadel.SetUpOrgResponse, error) {
	return zitadel.SetUpOrgResponse{}, nil
}
func (z *fakeZitadel) AddProject(ctx context.Context, _, _ string) (string, error) {
	return "", nil
}
func (z *fakeZitadel) AddOIDCApp(ctx context.Context, _, _ string, _ zitadel.AddOIDCAppRequest) (zitadel.AddOIDCAppResponse, error) {
	return zitadel.AddOIDCAppResponse{}, nil
}
func (z *fakeZitadel) RemoveOrg(ctx context.Context, _ string) error {
	return nil
}
func (z *fakeZitadel) AddHumanUser(ctx context.Context, _ string, _ zitadel.AddHumanUserRequest) (string, error) {
	return "", nil
}
func (z *fakeZitadel) RemoveUser(ctx context.Context, _, _ string) error {
	return nil
}

// renameOrgCall captures the last RenameOrg arguments so tests can
// assert the propagation actually fired.
type renameOrgCall struct {
	orgID string
	name  string
}

func (z *fakeZitadel) RenameOrg(ctx context.Context, orgID, name string) error {
	z.renameOrgCalls = append(z.renameOrgCalls, renameOrgCall{orgID: orgID, name: name})
	return z.renameOrgErr
}

func readyWorkspace() sqlc.Workspace {
	return sqlc.Workspace{InstallState: sqlc.WorkspaceInstallStateReady}
}

func newHandler(q *fakeQueries, z *fakeZitadel) *companies.Handler {
	return &companies.Handler{
		Queries: q,
		Zitadel: z,
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestCreateCompany_CreatesOrgThenPersistsRow(t *testing.T) {
	q := &fakeQueries{
		workspace: readyWorkspace(),
		createRow: sqlc.Company{
			ID:           pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
			Name:         "Acme",
			ZitadelOrgID: "org-new",
		},
	}
	z := &fakeZitadel{addOrgResp: "org-new"}
	h := newHandler(q, z)

	resp, err := h.CreateCompany(context.Background(), connect.NewRequest(&companiesv1.CreateCompanyRequest{
		Name: "Acme",
	}))
	if err != nil {
		t.Fatalf("CreateCompany: %v", err)
	}

	if z.addOrgName != "Acme" {
		t.Errorf("org name not forwarded: %q", z.addOrgName)
	}
	if !q.createCalled {
		t.Fatal("CreateCompany not called on DB")
	}
	if q.createParams.ZitadelOrgID != "org-new" {
		t.Errorf("persisted zitadel_org_id = %q; want org-new", q.createParams.ZitadelOrgID)
	}
	if resp.Msg.Company.ZitadelOrgId != "org-new" {
		t.Errorf("response zitadel_org_id = %q", resp.Msg.Company.ZitadelOrgId)
	}
}

func TestCreateCompany_OrgFailurePreventsRow(t *testing.T) {
	q := &fakeQueries{workspace: readyWorkspace()}
	z := &fakeZitadel{addOrgErr: errors.New("zitadel 500")}
	h := newHandler(q, z)

	_, err := h.CreateCompany(context.Background(), connect.NewRequest(&companiesv1.CreateCompanyRequest{
		Name: "Acme",
	}))
	if err == nil {
		t.Fatal("expected error")
	}
	if q.createCalled {
		t.Error("CreateCompany should not hit DB when ZITADEL fails")
	}
}

func TestCreateCompany_RequiresReadyWorkspace(t *testing.T) {
	q := &fakeQueries{workspace: sqlc.Workspace{InstallState: sqlc.WorkspaceInstallStateNotInitialized}}
	z := &fakeZitadel{addOrgResp: "org"}
	h := newHandler(q, z)

	_, err := h.CreateCompany(context.Background(), connect.NewRequest(&companiesv1.CreateCompanyRequest{
		Name: "Acme",
	}))
	if err == nil {
		t.Fatal("expected FailedPrecondition")
	}
	var cerr *connect.Error
	if errors.As(err, &cerr) && cerr.Code() != connect.CodeFailedPrecondition {
		t.Errorf("code = %v; want FailedPrecondition", cerr.Code())
	}
	if z.addOrgName != "" {
		t.Error("AddOrganization should not be called when workspace is not ready")
	}
}

func TestCreateCompany_RejectsEmptyName(t *testing.T) {
	q := &fakeQueries{workspace: readyWorkspace()}
	z := &fakeZitadel{}
	h := newHandler(q, z)

	_, err := h.CreateCompany(context.Background(), connect.NewRequest(&companiesv1.CreateCompanyRequest{}))
	if err == nil {
		t.Fatal("expected error")
	}
	if z.addOrgName != "" {
		t.Error("AddOrganization should not be called on validation failure")
	}
}

func TestCreateCompany_PersistsAddressFields(t *testing.T) {
	q := &fakeQueries{
		workspace: readyWorkspace(),
		createRow: sqlc.Company{
			ID:           pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
			Name:         "Acme",
			ZitadelOrgID: "org-new",
			AddressLine1: "R. Teste 1",
			City:         "São Paulo",
			Country:      "BR",
			Timezone:     "America/Sao_Paulo",
		},
	}
	z := &fakeZitadel{addOrgResp: "org-new"}
	h := newHandler(q, z)

	_, err := h.CreateCompany(context.Background(), connect.NewRequest(&companiesv1.CreateCompanyRequest{
		Name: "Acme",
		Address: &companiesv1.Address{
			Line1:    "R. Teste 1",
			City:     "São Paulo",
			Country:  "BR",
			Timezone: "America/Sao_Paulo",
		},
	}))
	if err != nil {
		t.Fatalf("CreateCompany: %v", err)
	}
	if q.createParams.AddressLine1 != "R. Teste 1" || q.createParams.City != "São Paulo" {
		t.Errorf("address not forwarded to DB: %+v", q.createParams)
	}
	if q.createParams.Timezone != "America/Sao_Paulo" {
		t.Errorf("timezone = %q; want America/Sao_Paulo", q.createParams.Timezone)
	}
}

func TestCreateCompany_MissingTimezoneDefaultsToUTC(t *testing.T) {
	q := &fakeQueries{workspace: readyWorkspace(), createRow: sqlc.Company{ZitadelOrgID: "org"}}
	z := &fakeZitadel{addOrgResp: "org"}
	h := newHandler(q, z)

	_, err := h.CreateCompany(context.Background(), connect.NewRequest(&companiesv1.CreateCompanyRequest{
		Name: "Acme",
	}))
	if err != nil {
		t.Fatalf("CreateCompany: %v", err)
	}
	if q.createParams.Timezone != "UTC" {
		t.Errorf("default timezone = %q; want UTC", q.createParams.Timezone)
	}
}

// --- UpdateCompany ----------------------------------------------------

func TestUpdateCompany_SuccessPersists(t *testing.T) {
	id := pgtype.UUID{Bytes: [16]byte{0xAB}, Valid: true}
	q := &fakeQueries{
		workspace: readyWorkspace(),
		updateRow: sqlc.Company{ID: id, Name: "Updated", ZitadelOrgID: "org-1"},
	}
	h := newHandler(q, &fakeZitadel{})

	v, _ := id.Value()
	resp, err := h.UpdateCompany(context.Background(), connect.NewRequest(&companiesv1.UpdateCompanyRequest{
		Id:   v.(string),
		Name: "Updated",
	}))
	if err != nil {
		t.Fatalf("UpdateCompany: %v", err)
	}
	if resp.Msg.Company.Name != "Updated" {
		t.Errorf("response name = %q; want Updated", resp.Msg.Company.Name)
	}
}

func TestUpdateCompany_WorkspaceIDRedirectsToSettingsEndpoint(t *testing.T) {
	wsID := pgtype.UUID{Bytes: [16]byte{0xC0}, Valid: true}
	q := &fakeQueries{
		workspace:        readyWorkspace(),
		updateErr:        sql_NoRows(),
		workspaceCompany: sqlc.Company{ID: wsID, IsWorkspaceOwner: true},
	}
	h := newHandler(q, &fakeZitadel{})

	v, _ := wsID.Value()
	_, err := h.UpdateCompany(context.Background(), connect.NewRequest(&companiesv1.UpdateCompanyRequest{
		Id:   v.(string),
		Name: "Acme MSP",
	}))
	if err == nil {
		t.Fatal("expected FailedPrecondition")
	}
	if !containsString(err.Error(), "workspace_company_update_via_settings_endpoint") {
		t.Errorf("error missing redirect marker: %v", err)
	}
}

func TestUpdateCompany_UnknownIDReturns404(t *testing.T) {
	unknownID := pgtype.UUID{Bytes: [16]byte{0xEE}, Valid: true}
	q := &fakeQueries{
		workspace: readyWorkspace(),
		updateErr: sql_NoRows(),
		// No workspaceCompany set, so explainMissingCompany won't
		// match the workspace id; it should fall through to NotFound.
	}
	h := newHandler(q, &fakeZitadel{})

	v, _ := unknownID.Value()
	_, err := h.UpdateCompany(context.Background(), connect.NewRequest(&companiesv1.UpdateCompanyRequest{
		Id:   v.(string),
		Name: "X",
	}))
	if err == nil {
		t.Fatal("expected NotFound")
	}
	var cerr *connect.Error
	if errors.As(err, &cerr) && cerr.Code() != connect.CodeNotFound {
		t.Errorf("code = %v; want NotFound", cerr.Code())
	}
}

// --- UpdateWorkspaceCompany -------------------------------------------

func TestUpdateWorkspaceCompany_PropagatesNameToZitadel(t *testing.T) {
	q := &fakeQueries{
		workspace: readyWorkspace(),
		updateWorkspaceRow: sqlc.Company{
			Name:             "New MSP Name",
			ZitadelOrgID:     "org-1",
			IsWorkspaceOwner: true,
		},
	}
	z := &fakeZitadel{}
	h := newHandler(q, z)

	_, err := h.UpdateWorkspaceCompany(context.Background(), connect.NewRequest(&companiesv1.UpdateWorkspaceCompanyRequest{
		Name: "New MSP Name",
	}))
	if err != nil {
		t.Fatalf("UpdateWorkspaceCompany: %v", err)
	}
	if len(z.renameOrgCalls) != 1 {
		t.Fatalf("RenameOrg calls = %d; want 1", len(z.renameOrgCalls))
	}
	if z.renameOrgCalls[0].orgID != "org-1" || z.renameOrgCalls[0].name != "New MSP Name" {
		t.Errorf("RenameOrg call = %+v; want {org-1, New MSP Name}", z.renameOrgCalls[0])
	}
	if q.updateWorkspaceParams.Name != "New MSP Name" {
		t.Errorf("DB update name = %q; want New MSP Name", q.updateWorkspaceParams.Name)
	}
}

func TestUpdateWorkspaceCompany_ZitadelFailureDoesNotBlockDBUpdate(t *testing.T) {
	q := &fakeQueries{
		workspace: readyWorkspace(),
		updateWorkspaceRow: sqlc.Company{
			Name:             "New Name",
			ZitadelOrgID:     "org-1",
			IsWorkspaceOwner: true,
		},
	}
	z := &fakeZitadel{renameOrgErr: errors.New("zitadel down")}
	h := newHandler(q, z)

	resp, err := h.UpdateWorkspaceCompany(context.Background(), connect.NewRequest(&companiesv1.UpdateWorkspaceCompanyRequest{
		Name: "New Name",
	}))
	if err != nil {
		t.Fatalf("UpdateWorkspaceCompany should still succeed when ZITADEL rename fails: %v", err)
	}
	if resp.Msg.Company.Name != "New Name" {
		t.Errorf("response name = %q; want New Name", resp.Msg.Company.Name)
	}
}

// --- GetWorkspaceCompany ---------------------------------------------

func TestGetWorkspaceCompany_ReturnsMSPRow(t *testing.T) {
	q := &fakeQueries{
		workspace: readyWorkspace(),
		workspaceCompany: sqlc.Company{
			Name:             "Acme MSP",
			ZitadelOrgID:     "org-1",
			IsWorkspaceOwner: true,
		},
	}
	h := newHandler(q, &fakeZitadel{})

	resp, err := h.GetWorkspaceCompany(context.Background(), connect.NewRequest(&companiesv1.GetWorkspaceCompanyRequest{}))
	if err != nil {
		t.Fatalf("GetWorkspaceCompany: %v", err)
	}
	if !resp.Msg.Company.IsWorkspaceOwner {
		t.Error("IsWorkspaceOwner must be TRUE on GetWorkspaceCompany response")
	}
	if resp.Msg.Company.Name != "Acme MSP" {
		t.Errorf("name = %q; want Acme MSP", resp.Msg.Company.Name)
	}
}

// --- helpers ----------------------------------------------------------

// sql_NoRows returns the pgx ErrNoRows sentinel so tests can simulate
// the UPDATE that matched zero rows because of the workspace-company
// filter or a non-existent id.
func sql_NoRows() error {
	return pgx.ErrNoRows
}

func containsString(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
