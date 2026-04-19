// Package companies owns the CompaniesService backend: CRUD over the
// companies table with eager ZITADEL organization creation.
//
// The eager-org rule is load-bearing for the MVP: every company maps to
// exactly one ZITADEL org so client-side SSO and per-company IAM boundary
// work from day one. If ZITADEL rejects the org creation, the company row
// is never persisted — the DB cannot hold a company without
// zitadel_org_id (NOT NULL), so ordering failures correctly is essential.
package companies

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Gabrielbdd/gospa/db/sqlc"
	companiesv1 "github.com/Gabrielbdd/gospa/gen/gospa/companies/v1"
	"github.com/Gabrielbdd/gospa/gen/gospa/companies/v1/companiesv1connect"
	"github.com/Gabrielbdd/gospa/internal/zitadel"
)

// Queries is the subset of the sqlc Querier the handler needs. Narrow
// interface keeps test doubles trivial.
type Queries interface {
	GetWorkspace(ctx context.Context) (sqlc.Workspace, error)
	CreateCompany(ctx context.Context, arg sqlc.CreateCompanyParams) (sqlc.Company, error)
	ListCompanies(ctx context.Context) ([]sqlc.Company, error)
	GetCompany(ctx context.Context, id pgtype.UUID) (sqlc.Company, error)
	ArchiveCompany(ctx context.Context, id pgtype.UUID) error
}

// Handler implements CompaniesService.
type Handler struct {
	companiesv1connect.UnimplementedCompaniesServiceHandler

	Queries Queries
	Zitadel zitadel.Client
	Logger  *slog.Logger
}

// CreateCompany validates input, requires workspace.install_state=ready,
// creates the ZITADEL org, then persists the company row. If org creation
// succeeds but DB insert fails, we log the orphaned org — manual cleanup
// in ZITADEL is required. This is a documented MVP debt (P14).
func (h *Handler) CreateCompany(ctx context.Context, req *connect.Request[companiesv1.CreateCompanyRequest]) (*connect.Response[companiesv1.CreateCompanyResponse], error) {
	if err := h.requireReady(ctx); err != nil {
		return nil, err
	}
	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	if req.Msg.Slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("slug is required"))
	}

	orgID, err := h.Zitadel.AddOrganization(ctx, req.Msg.Name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	row, err := h.Queries.CreateCompany(ctx, sqlc.CreateCompanyParams{
		Name:         req.Msg.Name,
		Slug:         req.Msg.Slug,
		ZitadelOrgID: orgID,
	})
	if err != nil {
		h.Logger.ErrorContext(ctx, "company row insert failed after ZITADEL org was created",
			"zitadel_org_id", orgID,
			"error", err,
		)
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&companiesv1.CreateCompanyResponse{
		Company: toProtoCompany(row),
	}), nil
}

func (h *Handler) ListCompanies(ctx context.Context, _ *connect.Request[companiesv1.ListCompaniesRequest]) (*connect.Response[companiesv1.ListCompaniesResponse], error) {
	if err := h.requireReady(ctx); err != nil {
		return nil, err
	}
	rows, err := h.Queries.ListCompanies(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*companiesv1.Company, 0, len(rows))
	for _, r := range rows {
		out = append(out, toProtoCompany(r))
	}
	return connect.NewResponse(&companiesv1.ListCompaniesResponse{Companies: out}), nil
}

func (h *Handler) ArchiveCompany(ctx context.Context, req *connect.Request[companiesv1.ArchiveCompanyRequest]) (*connect.Response[companiesv1.ArchiveCompanyResponse], error) {
	if err := h.requireReady(ctx); err != nil {
		return nil, err
	}

	var id pgtype.UUID
	if err := id.Scan(req.Msg.Id); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := h.Queries.ArchiveCompany(ctx, id); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&companiesv1.ArchiveCompanyResponse{}), nil
}

// requireReady returns FailedPrecondition (with a /install pointer) when
// the workspace is not yet installed.
func (h *Handler) requireReady(ctx context.Context) error {
	ws, err := h.Queries.GetWorkspace(ctx)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if string(ws.InstallState) != "ready" {
		return connect.NewError(
			connect.CodeFailedPrecondition,
			errors.New("workspace is not installed yet; complete /install first"),
		)
	}
	return nil
}

func toProtoCompany(r sqlc.Company) *companiesv1.Company {
	c := &companiesv1.Company{
		Id:           r.ID.String(),
		Name:         r.Name,
		Slug:         r.Slug,
		ZitadelOrgId: r.ZitadelOrgID,
	}
	if r.CreatedAt.Valid {
		c.CreatedAt = r.CreatedAt.Time.UTC().Format(time.RFC3339)
	}
	if r.ArchivedAt.Valid {
		c.ArchivedAt = r.ArchivedAt.Time.UTC().Format(time.RFC3339)
	}
	return c
}
