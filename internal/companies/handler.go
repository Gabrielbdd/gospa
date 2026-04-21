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
//
// The owner_contact_id column (migration 00008) points at a contact
// responsible for the account. Reads LEFT JOIN contacts so the SPA
// gets the owner's full_name denormalised; writes take the contact_id
// only. An empty string on Create/Update clears the owner (NULL in DB).
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
	ListCompanies(ctx context.Context) ([]sqlc.ListCompaniesRow, error)
	GetCompany(ctx context.Context, id pgtype.UUID) (sqlc.GetCompanyRow, error)
	GetCompanyIncludingArchived(ctx context.Context, id pgtype.UUID) (sqlc.GetCompanyIncludingArchivedRow, error)
	UpdateCompany(ctx context.Context, arg sqlc.UpdateCompanyParams) (sqlc.Company, error)
	UpdateWorkspaceCompany(ctx context.Context, arg sqlc.UpdateWorkspaceCompanyParams) (sqlc.Company, error)
	GetWorkspaceCompany(ctx context.Context) (sqlc.Company, error)
	ArchiveCompany(ctx context.Context, id pgtype.UUID) error
	RestoreCompany(ctx context.Context, id pgtype.UUID) (sqlc.Company, error)
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
	ownerID, err := parseOptionalUUID(req.Msg.OwnerContactId, "owner_contact_id")
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	orgID, err := h.Zitadel.AddOrganization(ctx, req.Msg.Name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	addr := req.Msg.Address
	row, err := h.Queries.CreateCompany(ctx, sqlc.CreateCompanyParams{
		Name:           req.Msg.Name,
		ZitadelOrgID:   orgID,
		OwnerContactID: ownerID,
		AddressLine1:   addressField(addr, func(a *companiesv1.Address) string { return a.Line1 }),
		AddressLine2:   addressField(addr, func(a *companiesv1.Address) string { return a.Line2 }),
		City:           addressField(addr, func(a *companiesv1.Address) string { return a.City }),
		Region:         addressField(addr, func(a *companiesv1.Address) string { return a.Region }),
		PostalCode:     addressField(addr, func(a *companiesv1.Address) string { return a.PostalCode }),
		Country:        addressField(addr, func(a *companiesv1.Address) string { return a.Country }),
		Timezone:       timezoneOrDefault(addr),
	})
	if err != nil {
		h.Logger.ErrorContext(ctx, "company row insert failed after ZITADEL org was created",
			"zitadel_org_id", orgID,
			"error", err,
		)
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Re-fetch with LEFT JOIN so the response carries the denormalised
	// Owner. Two round-trips for the happy path is fine — Create is not
	// hot.
	return connect.NewResponse(&companiesv1.CreateCompanyResponse{
		Company: h.hydratedCompany(ctx, row),
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
		out = append(out, listRowToProto(r))
	}
	return connect.NewResponse(&companiesv1.ListCompaniesResponse{Companies: out}), nil
}

// GetCompany returns a single company including archived rows so the
// Detail page can still render + Restore an archived record when the
// operator follows a direct link.
func (h *Handler) GetCompany(ctx context.Context, req *connect.Request[companiesv1.GetCompanyRequest]) (*connect.Response[companiesv1.GetCompanyResponse], error) {
	if err := h.requireReady(ctx); err != nil {
		return nil, err
	}
	id, err := parseCompanyID(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	row, err := h.Queries.GetCompanyIncludingArchived(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("company not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if row.IsWorkspaceOwner {
		return nil, connect.NewError(
			connect.CodeFailedPrecondition,
			errors.New("workspace_company_get_via_settings_endpoint: use GetWorkspaceCompany for the MSP row"),
		)
	}
	return connect.NewResponse(&companiesv1.GetCompanyResponse{
		Company: getIncludingArchivedRowToProto(row),
	}), nil
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
	ownerID, err := parseOptionalUUID(req.Msg.OwnerContactId, "owner_contact_id")
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	addr := req.Msg.Address
	row, err := h.Queries.UpdateCompany(ctx, sqlc.UpdateCompanyParams{
		ID:             id,
		Name:           req.Msg.Name,
		OwnerContactID: ownerID,
		AddressLine1:   addressField(addr, func(a *companiesv1.Address) string { return a.Line1 }),
		AddressLine2:   addressField(addr, func(a *companiesv1.Address) string { return a.Line2 }),
		City:           addressField(addr, func(a *companiesv1.Address) string { return a.City }),
		Region:         addressField(addr, func(a *companiesv1.Address) string { return a.Region }),
		PostalCode:     addressField(addr, func(a *companiesv1.Address) string { return a.PostalCode }),
		Country:        addressField(addr, func(a *companiesv1.Address) string { return a.Country }),
		Timezone:       timezoneOrDefault(addr),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, h.explainMissingCompany(ctx, id)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&companiesv1.UpdateCompanyResponse{
		Company: h.hydratedCompany(ctx, row),
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

// RestoreCompany reverses ArchiveCompany. Reads the row via the
// including-archived path first so a "not found vs workspace row"
// disambiguation stays cheap, mirroring ArchiveCompany.
func (h *Handler) RestoreCompany(ctx context.Context, req *connect.Request[companiesv1.RestoreCompanyRequest]) (*connect.Response[companiesv1.RestoreCompanyResponse], error) {
	if err := h.requireReady(ctx); err != nil {
		return nil, err
	}
	id, err := parseCompanyID(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	existing, err := h.Queries.GetCompanyIncludingArchived(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("company not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if existing.IsWorkspaceOwner {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("workspace_company_restore_forbidden: the MSP row is never archived"))
	}
	if !existing.ArchivedAt.Valid {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("company_not_archived: already active"))
	}

	row, err := h.Queries.RestoreCompany(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&companiesv1.RestoreCompanyResponse{
		Company: h.hydratedCompany(ctx, row),
	}), nil
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
		Company: companyToProto(row),
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

	if renamer, ok := h.Zitadel.(orgRenamer); ok {
		if rerr := renamer.RenameOrg(ctx, row.ZitadelOrgID, row.Name); rerr != nil {
			h.Logger.WarnContext(ctx, "workspace org rename in ZITADEL failed; Gospa-side name stands",
				"zitadel_org_id", row.ZitadelOrgID,
				"error", rerr)
		}
	}

	return connect.NewResponse(&companiesv1.UpdateWorkspaceCompanyResponse{
		Company: companyToProto(row),
	}), nil
}

type orgRenamer interface {
	RenameOrg(ctx context.Context, orgID, name string) error
}

func (h *Handler) explainMissingCompany(ctx context.Context, id pgtype.UUID) error {
	ws, err := h.Queries.GetWorkspaceCompany(ctx)
	if err == nil && uuidEq(ws.ID, id) {
		return connect.NewError(
			connect.CodeFailedPrecondition,
			errors.New("workspace_company_update_via_settings_endpoint: use UpdateWorkspaceCompany for the MSP row"),
		)
	}
	return connect.NewError(connect.CodeNotFound, errors.New("company not found"))
}

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

// hydratedCompany re-reads the row via LEFT JOIN so the response
// carries the denormalised Owner. Used by Create/Update/Restore where
// the underlying mutation returns just sqlc.Company.
func (h *Handler) hydratedCompany(ctx context.Context, row sqlc.Company) *companiesv1.Company {
	full, err := h.Queries.GetCompanyIncludingArchived(ctx, row.ID)
	if err != nil {
		// Fall back to the un-hydrated row — UI can resolve the owner
		// name on next list refetch. Logged to surface query misses
		// during development.
		h.Logger.WarnContext(ctx, "post-mutation re-fetch failed; returning un-hydrated company",
			"company_id", row.ID.String(),
			"error", err,
		)
		return companyToProto(row)
	}
	return getIncludingArchivedRowToProto(full)
}

// -- proto builders ---------------------------------------------------

func companyToProto(r sqlc.Company) *companiesv1.Company {
	return buildProtoCompany(
		r.ID, r.Name, r.ZitadelOrgID, r.CreatedAt, r.ArchivedAt, r.IsWorkspaceOwner,
		addressFieldsFromCompany(r),
		r.OwnerContactID, pgtype.Text{},
	)
}

func listRowToProto(r sqlc.ListCompaniesRow) *companiesv1.Company {
	return buildProtoCompany(
		r.ID, r.Name, r.ZitadelOrgID, r.CreatedAt, r.ArchivedAt, r.IsWorkspaceOwner,
		addressFields{
			Line1:      r.AddressLine1,
			Line2:      r.AddressLine2,
			City:       r.City,
			Region:     r.Region,
			PostalCode: r.PostalCode,
			Country:    r.Country,
			Timezone:   r.Timezone,
		},
		r.OwnerContactID, r.OwnerFullName,
	)
}

func getIncludingArchivedRowToProto(r sqlc.GetCompanyIncludingArchivedRow) *companiesv1.Company {
	return buildProtoCompany(
		r.ID, r.Name, r.ZitadelOrgID, r.CreatedAt, r.ArchivedAt, r.IsWorkspaceOwner,
		addressFields{
			Line1:      r.AddressLine1,
			Line2:      r.AddressLine2,
			City:       r.City,
			Region:     r.Region,
			PostalCode: r.PostalCode,
			Country:    r.Country,
			Timezone:   r.Timezone,
		},
		r.OwnerContactID, r.OwnerFullName,
	)
}

type addressFields struct {
	Line1, Line2, City, Region, PostalCode, Country, Timezone string
}

func addressFieldsFromCompany(r sqlc.Company) addressFields {
	return addressFields{
		Line1:      r.AddressLine1,
		Line2:      r.AddressLine2,
		City:       r.City,
		Region:     r.Region,
		PostalCode: r.PostalCode,
		Country:    r.Country,
		Timezone:   r.Timezone,
	}
}

func buildProtoCompany(
	id pgtype.UUID,
	name string,
	zitadelOrgID string,
	createdAt pgtype.Timestamptz,
	archivedAt pgtype.Timestamptz,
	isWorkspaceOwner bool,
	addr addressFields,
	ownerContactID pgtype.UUID,
	ownerFullName pgtype.Text,
) *companiesv1.Company {
	c := &companiesv1.Company{
		Id:               id.String(),
		Name:             name,
		ZitadelOrgId:     zitadelOrgID,
		IsWorkspaceOwner: isWorkspaceOwner,
		Address: &companiesv1.Address{
			Line1:      addr.Line1,
			Line2:      addr.Line2,
			City:       addr.City,
			Region:     addr.Region,
			PostalCode: addr.PostalCode,
			Country:    addr.Country,
			Timezone:   addr.Timezone,
		},
	}
	if createdAt.Valid {
		c.CreatedAt = createdAt.Time.UTC().Format(time.RFC3339)
	}
	if archivedAt.Valid {
		c.ArchivedAt = archivedAt.Time.UTC().Format(time.RFC3339)
	}
	if ownerContactID.Valid {
		c.Owner = &companiesv1.Owner{
			ContactId: ownerContactID.String(),
			FullName:  ownerFullName.String,
		}
	}
	return c
}

func addressField(addr *companiesv1.Address, pick func(*companiesv1.Address) string) string {
	if addr == nil {
		return ""
	}
	return pick(addr)
}

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

// parseOptionalUUID returns an invalid pgtype.UUID (Valid: false) when s
// is empty so NULL is persisted; otherwise parses the string.
func parseOptionalUUID(s, field string) (pgtype.UUID, error) {
	if s == "" {
		return pgtype.UUID{}, nil
	}
	var id pgtype.UUID
	if err := id.Scan(s); err != nil {
		return pgtype.UUID{}, errors.New(field + " is not a valid UUID")
	}
	return id, nil
}

func uuidEq(a, b pgtype.UUID) bool {
	if !a.Valid || !b.Valid {
		return false
	}
	return a.Bytes == b.Bytes
}
