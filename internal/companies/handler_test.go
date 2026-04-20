package companies_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"connectrpc.com/connect"
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

	listRows []sqlc.Company
	listErr  error
}

func (f *fakeQueries) GetWorkspace(ctx context.Context) (sqlc.Workspace, error) {
	return f.workspace, f.workspaceErr
}
func (f *fakeQueries) CreateCompany(ctx context.Context, arg sqlc.CreateCompanyParams) (sqlc.Company, error) {
	f.createCalled = true
	f.createParams = arg
	return f.createRow, f.createErr
}
func (f *fakeQueries) ListCompanies(ctx context.Context) ([]sqlc.Company, error) {
	return f.listRows, f.listErr
}
func (f *fakeQueries) GetCompany(ctx context.Context, _ pgtype.UUID) (sqlc.Company, error) {
	return sqlc.Company{}, nil
}
func (f *fakeQueries) ArchiveCompany(ctx context.Context, _ pgtype.UUID) error {
	return nil
}

type fakeZitadel struct {
	addOrgResp string
	addOrgErr  error
	addOrgName string
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

	_, err := h.CreateCompany(context.Background(), connect.NewRequest(&companiesv1.CreateCompanyRequest{
	}))
	if err == nil {
		t.Fatal("expected error")
	}
	if z.addOrgName != "" {
		t.Error("AddOrganization should not be called on validation failure")
	}
}
