// Package companies owns the CompaniesService backend: CRUD over the
// companies table with eager ZITADEL organization creation, plus the
// dedicated Workspace-company read/write path used by Settings >
// Workspace.
//
// The eager-org rule is load-bearing for the MVP: every customer
// company maps to exactly one ZITADEL org so client-side SSO and
// per-company IAM boundary work from day one. If ZITADEL rejects the
// org creation, the company row is never persisted — the DB cannot
// hold a company without zitadel_org_id (NOT NULL), so ordering
// failures correctly is essential.
//
// The MSP itself is also a companies row (is_workspace_owner = TRUE)
// but it is hidden from ListCompanies and rejected by UpdateCompany /
// ArchiveCompany. Editing the MSP goes through UpdateWorkspaceCompany,
// which also propagates the name change to the ZITADEL org
// best-effort (failure logs a warning; the DB update stands).
package companies

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
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
	UpdateCompany(ctx context.Context, arg sqlc.UpdateCompanyParams) (sqlc.Company, error)
	UpdateWorkspaceCompany(ctx context.Context, arg sqlc.UpdateWorkspaceCompanyParams) (sqlc.Company, error)
	GetWorkspaceCompany(ctx context.Context) (sqlc.Company, error)
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
// creates the ZITADEL org, then persists the company row.
func (h *Handler) CreateCompany(ctx context.Context, req *connect.Request[companiesv1.CreateCompanyRequest]) (*connect.Response[companiesv1.CreateCompanyResponse], error) {
	if err := h.requireReady(ctx); err != nil {
		return nil, err
	}
	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}

	orgID, err := h.Zitadel.AddOrganization(ctx, req.Msg.Name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	addr := req.Msg.Address
	row, err := h.Queries.CreateCompany(ctx, sqlc.CreateCompanyParams{
		Name:         req.Msg.Name,
		ZitadelOrgID: orgID,
		AddressLine1: addressField(addr, func(a *companiesv1.Address) string { return a.Line1 }),
		AddressLine2: addressField(addr, func(a *companiesv1.Address) string { return a.Line2 }),
		City:         addressField(addr, func(a *companiesv1.Address) string { return a.City }),
		Region:       addressField(addr, func(a *companiesv1.Address) string { return a.Region }),
		PostalCode:   addressField(addr, func(a *companiesv1.Address) string { return a.PostalCode }),
		Country:      addressField(addr, func(a *companiesv1.Address) string { return a.Country }),
		Timezone:     timezoneOrDefault(addr),
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

// UpdateCompany edits a non-workspace company. The MSP row is
// protected by the WHERE clause in the SQL query — if the id matches
// the workspace company the UPDATE affects zero rows and the handler
// returns FailedPrecondition so the caller learns to use
// UpdateWorkspaceCompany instead.
func (h *Handler) UpdateCompany(ctx context.Context, req *connect.Request[companiesv1.UpdateCompanyRequest]) (*connect.Response[companiesv1.UpdateCompanyResponse], error) {
	if err := h.requireReady(ctx); err != nil {
		return nil, err
	}
	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	id, err := parseCompanyID(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	addr := req.Msg.Address
	row, err := h.Queries.UpdateCompany(ctx, sqlc.UpdateCompanyParams{
		ID:           id,
		Name:         req.Msg.Name,
		AddressLine1: addressField(addr, func(a *companiesv1.Address) string { return a.Line1 }),
		AddressLine2: addressField(addr, func(a *companiesv1.Address) string { return a.Line2 }),
		City:         addressField(addr, func(a *companiesv1.Address) string { return a.City }),
		Region:       addressField(addr, func(a *companiesv1.Address) string { return a.Region }),
		PostalCode:   addressField(addr, func(a *companiesv1.Address) string { return a.PostalCode }),
		Country:      addressField(addr, func(a *companiesv1.Address) string { return a.Country }),
		Timezone:     timezoneOrDefault(addr),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Either the id doesn't exist, the row is archived, or the
			// caller tried to edit the workspace company through the
			// generic endpoint. Disambiguate by reading the row
			// unfiltered.
			return nil, h.explainMissingCompany(ctx, id)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&companiesv1.UpdateCompanyResponse{
		Company: toProtoCompany(row),
	}), nil
}

// ArchiveCompany soft-deletes a non-workspace company. The workspace
// row is protected by the same WHERE predicate as UpdateCompany.
func (h *Handler) ArchiveCompany(ctx context.Context, req *connect.Request[companiesv1.ArchiveCompanyRequest]) (*connect.Response[companiesv1.ArchiveCompanyResponse], error) {
	if err := h.requireReady(ctx); err != nil {
		return nil, err
	}
	id, err := parseCompanyID(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Look up first so we can return a specific error if the id points
	// at the workspace company. ArchiveCompany's query already filters
	// `is_workspace_owner = FALSE` but the UPDATE returns no error on
	// zero-match; we want the caller to see a targeted message.
	existing, err := h.Queries.GetCompany(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("company not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if existing.IsWorkspaceOwner {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("workspace_company_archive_forbidden: the MSP row cannot be archived"))
	}

	if err := h.Queries.ArchiveCompany(ctx, id); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&companiesv1.ArchiveCompanyResponse{}), nil
}

// GetWorkspaceCompany returns the singleton MSP row. Accessible to
// every authenticated team member (so the Settings > Workspace page
// can render it read-only for technicians).
func (h *Handler) GetWorkspaceCompany(ctx context.Context, _ *connect.Request[companiesv1.GetWorkspaceCompanyRequest]) (*connect.Response[companiesv1.GetWorkspaceCompanyResponse], error) {
	if err := h.requireReady(ctx); err != nil {
		return nil, err
	}
	row, err := h.Queries.GetWorkspaceCompany(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("workspace company row missing; reinstall"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&companiesv1.GetWorkspaceCompanyResponse{
		Company: toProtoCompany(row),
	}), nil
}

// UpdateWorkspaceCompany edits the MSP row. The policy map registers
// this as admin_only; the handler does not recheck. Name changes are
// propagated to the ZITADEL org best-effort: failure is logged but
// does not block the DB update, so a ZITADEL hiccup never blocks a
// workspace rename.
func (h *Handler) UpdateWorkspaceCompany(ctx context.Context, req *connect.Request[companiesv1.UpdateWorkspaceCompanyRequest]) (*connect.Response[companiesv1.UpdateWorkspaceCompanyResponse], error) {
	if err := h.requireReady(ctx); err != nil {
		return nil, err
	}
	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}

	addr := req.Msg.Address
	row, err := h.Queries.UpdateWorkspaceCompany(ctx, sqlc.UpdateWorkspaceCompanyParams{
		Name:         req.Msg.Name,
		AddressLine1: addressField(addr, func(a *companiesv1.Address) string { return a.Line1 }),
		AddressLine2: addressField(addr, func(a *companiesv1.Address) string { return a.Line2 }),
		City:         addressField(addr, func(a *companiesv1.Address) string { return a.City }),
		Region:       addressField(addr, func(a *companiesv1.Address) string { return a.Region }),
		PostalCode:   addressField(addr, func(a *companiesv1.Address) string { return a.PostalCode }),
		Country:      addressField(addr, func(a *companiesv1.Address) string { return a.Country }),
		Timezone:     timezoneOrDefault(addr),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("workspace company row missing; reinstall"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Best-effort propagation to ZITADEL. Silent no-op if the ZITADEL
	// client is a fake that doesn't implement RenameOrg — the update
	// proto-column is what drives the Gospa UI.
	if renamer, ok := h.Zitadel.(orgRenamer); ok {
		if rerr := renamer.RenameOrg(ctx, row.ZitadelOrgID, row.Name); rerr != nil {
			h.Logger.WarnContext(ctx, "workspace org rename in ZITADEL failed; Gospa-side name stands",
				"zitadel_org_id", row.ZitadelOrgID,
				"error", rerr)
		}
	}

	return connect.NewResponse(&companiesv1.UpdateWorkspaceCompanyResponse{
		Company: toProtoCompany(row),
	}), nil
}

// orgRenamer is an optional capability on the ZITADEL client. Keeping
// it as a separate interface means the existing zitadel.Client mock
// keeps working without stubbing RenameOrg and the real HTTPClient
// implements it when it supports the call.
type orgRenamer interface {
	RenameOrg(ctx context.Context, orgID, name string) error
}

// explainMissingCompany returns a specific error depending on WHY the
// id didn't match for UpdateCompany: workspace row (caller should use
// UpdateWorkspaceCompany), archived row, or truly missing.
func (h *Handler) explainMissingCompany(ctx context.Context, id pgtype.UUID) error {
	// GetCompany filters archived rows. Do a second unfiltered read
	// via the MSP-row detector — if it lands on the workspace row,
	// we know the caller used the wrong endpoint.
	ws, err := h.Queries.GetWorkspaceCompany(ctx)
	if err == nil && uuidEq(ws.ID, id) {
		return connect.NewError(
			connect.CodeFailedPrecondition,
			errors.New("workspace_company_update_via_settings_endpoint: use UpdateWorkspaceCompany for the MSP row"),
		)
	}
	return connect.NewError(connect.CodeNotFound, errors.New("company not found"))
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
		Id:               r.ID.String(),
		Name:             r.Name,
		ZitadelOrgId:     r.ZitadelOrgID,
		IsWorkspaceOwner: r.IsWorkspaceOwner,
		Address: &companiesv1.Address{
			Line1:      r.AddressLine1,
			Line2:      r.AddressLine2,
			City:       r.City,
			Region:     r.Region,
			PostalCode: r.PostalCode,
			Country:    r.Country,
			Timezone:   r.Timezone,
		},
	}
	if r.CreatedAt.Valid {
		c.CreatedAt = r.CreatedAt.Time.UTC().Format(time.RFC3339)
	}
	if r.ArchivedAt.Valid {
		c.ArchivedAt = r.ArchivedAt.Time.UTC().Format(time.RFC3339)
	}
	return c
}

func addressField(addr *companiesv1.Address, pick func(*companiesv1.Address) string) string {
	if addr == nil {
		return ""
	}
	return pick(addr)
}

// timezoneOrDefault returns addr.Timezone when set, otherwise "UTC".
// Matches the NOT NULL DEFAULT 'UTC' on the column so clients that
// omit the field get the same value the DB would have generated.
func timezoneOrDefault(addr *companiesv1.Address) string {
	if addr != nil && addr.Timezone != "" {
		return addr.Timezone
	}
	return "UTC"
}

func parseCompanyID(s string) (pgtype.UUID, error) {
	if s == "" {
		return pgtype.UUID{}, errors.New("id is required")
	}
	var id pgtype.UUID
	if err := id.Scan(s); err != nil {
		return pgtype.UUID{}, errors.New("id is not a valid UUID")
	}
	return id, nil
}

func uuidEq(a, b pgtype.UUID) bool {
	if !a.Valid || !b.Valid {
		return false
	}
	return a.Bytes == b.Bytes
}
