// Package contacts owns the ContactsService backend: CRUD over
// customer-company contacts. Contacts are storage + attribution
// records; they are not logins in V1 (identity_source / zitadel_user_id
// columns are populated only by the install and team invite flows).
//
// Email uniqueness is per-company and enforced by a partial unique
// index on (company_id, lower(email)) WHERE email IS NOT NULL AND
// archived_at IS NULL. The handler relies on the constraint for
// correctness and translates the resulting error into a Connect
// AlreadyExists.
//
// Archival is a soft delete (archived_at = now()) rather than a row
// delete so historical attributions (future: tickets, time entries)
// keep pointing at a valid row. ArchiveContact is rejected on contacts
// that back a workspace_grants row — the team-suspend flow is the
// right removal path for MSP team members.
package contacts

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Gabrielbdd/gospa/db/sqlc"
	contactsv1 "github.com/Gabrielbdd/gospa/gen/gospa/contacts/v1"
	"github.com/Gabrielbdd/gospa/gen/gospa/contacts/v1/contactsv1connect"
)

// Queries is the narrow subset of sqlc the contacts handler needs.
type Queries interface {
	GetContact(ctx context.Context, id pgtype.UUID) (sqlc.Contact, error)
	ListContactsByCompany(ctx context.Context, companyID pgtype.UUID) ([]sqlc.Contact, error)
	CreateContact(ctx context.Context, arg sqlc.CreateContactParams) (sqlc.Contact, error)
	UpdateContact(ctx context.Context, arg sqlc.UpdateContactParams) (sqlc.Contact, error)
	ArchiveContact(ctx context.Context, id pgtype.UUID) error
	ContactExistsByCompanyEmail(ctx context.Context, arg sqlc.ContactExistsByCompanyEmailParams) (bool, error)
	ContactHasWorkspaceGrant(ctx context.Context, contactID pgtype.UUID) (bool, error)
}

// Handler implements ContactsService.
type Handler struct {
	contactsv1connect.UnimplementedContactsServiceHandler

	Queries Queries
	Logger  *slog.Logger
}

// ListContacts returns every active contact at the given company.
func (h *Handler) ListContacts(ctx context.Context, req *connect.Request[contactsv1.ListContactsRequest]) (*connect.Response[contactsv1.ListContactsResponse], error) {
	companyID, err := parseUUID(req.Msg.CompanyId, "company_id")
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rows, err := h.Queries.ListContactsByCompany(ctx, companyID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*contactsv1.Contact, 0, len(rows))
	for _, r := range rows {
		out = append(out, toProtoContact(r))
	}
	return connect.NewResponse(&contactsv1.ListContactsResponse{Contacts: out}), nil
}

// GetContact returns a single active contact by id.
func (h *Handler) GetContact(ctx context.Context, req *connect.Request[contactsv1.GetContactRequest]) (*connect.Response[contactsv1.GetContactResponse], error) {
	id, err := parseUUID(req.Msg.Id, "id")
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	row, err := h.Queries.GetContact(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("contact not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&contactsv1.GetContactResponse{Contact: toProtoContact(row)}), nil
}

// CreateContact inserts a new contact. Email uniqueness is checked
// up-front to give the SPA a targeted AlreadyExists before the DB
// constraint trips.
func (h *Handler) CreateContact(ctx context.Context, req *connect.Request[contactsv1.CreateContactRequest]) (*connect.Response[contactsv1.CreateContactResponse], error) {
	companyID, err := parseUUID(req.Msg.CompanyId, "company_id")
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	fullName := strings.TrimSpace(req.Msg.FullName)
	if fullName == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("full_name is required"))
	}
	email := strings.TrimSpace(req.Msg.Email)

	if email != "" {
		exists, err := h.Queries.ContactExistsByCompanyEmail(ctx, sqlc.ContactExistsByCompanyEmailParams{
			CompanyID: companyID,
			Lower:     email,
		})
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if exists {
			return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("a contact with this email already exists in this company"))
		}
	}

	row, err := h.Queries.CreateContact(ctx, sqlc.CreateContactParams{
		CompanyID:      companyID,
		FullName:       fullName,
		JobTitle:       nullableText(req.Msg.JobTitle),
		Email:          nullableText(email),
		Phone:          nullableText(req.Msg.Phone),
		Mobile:         nullableText(req.Msg.Mobile),
		Notes:          nullableText(req.Msg.Notes),
		ZitadelUserID:  pgtype.Text{Valid: false},
		IdentitySource: "manual",
		ExternalID:     pgtype.Text{Valid: false},
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&contactsv1.CreateContactResponse{Contact: toProtoContact(row)}), nil
}

// UpdateContact edits the mutable fields.
func (h *Handler) UpdateContact(ctx context.Context, req *connect.Request[contactsv1.UpdateContactRequest]) (*connect.Response[contactsv1.UpdateContactResponse], error) {
	id, err := parseUUID(req.Msg.Id, "id")
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	fullName := strings.TrimSpace(req.Msg.FullName)
	if fullName == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("full_name is required"))
	}
	email := strings.TrimSpace(req.Msg.Email)

	row, err := h.Queries.UpdateContact(ctx, sqlc.UpdateContactParams{
		ID:       id,
		FullName: fullName,
		JobTitle: nullableText(req.Msg.JobTitle),
		Email:    nullableText(email),
		Phone:    nullableText(req.Msg.Phone),
		Mobile:   nullableText(req.Msg.Mobile),
		Notes:    nullableText(req.Msg.Notes),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("contact not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&contactsv1.UpdateContactResponse{Contact: toProtoContact(row)}), nil
}

// ArchiveContact soft-deletes a contact that is not a team member.
// Team members must be removed via TeamService.SuspendMember so the
// workspace_grants row is updated in lockstep; a silent archive of a
// grant-bearing contact would leave the authz middleware resolving
// against an archived row.
func (h *Handler) ArchiveContact(ctx context.Context, req *connect.Request[contactsv1.ArchiveContactRequest]) (*connect.Response[contactsv1.ArchiveContactResponse], error) {
	id, err := parseUUID(req.Msg.Id, "id")
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	hasGrant, err := h.Queries.ContactHasWorkspaceGrant(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if hasGrant {
		return nil, connect.NewError(
			connect.CodeFailedPrecondition,
			errors.New("team_member_archive_via_team_endpoint: use TeamService.SuspendMember to remove team members"),
		)
	}
	if err := h.Queries.ArchiveContact(ctx, id); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&contactsv1.ArchiveContactResponse{}), nil
}

// --- helpers ---------------------------------------------------------

func toProtoContact(r sqlc.Contact) *contactsv1.Contact {
	c := &contactsv1.Contact{
		Id:        uuidString(r.ID),
		CompanyId: uuidString(r.CompanyID),
		FullName:  r.FullName,
	}
	if r.JobTitle.Valid {
		c.JobTitle = r.JobTitle.String
	}
	if r.Email.Valid {
		c.Email = r.Email.String
	}
	if r.Phone.Valid {
		c.Phone = r.Phone.String
	}
	if r.Mobile.Valid {
		c.Mobile = r.Mobile.String
	}
	if r.Notes.Valid {
		c.Notes = r.Notes.String
	}
	if r.CreatedAt.Valid {
		c.CreatedAt = r.CreatedAt.Time.UTC().Format(time.RFC3339)
	}
	if r.ArchivedAt.Valid {
		c.ArchivedAt = r.ArchivedAt.Time.UTC().Format(time.RFC3339)
	}
	return c
}

func parseUUID(s, field string) (pgtype.UUID, error) {
	if s == "" {
		return pgtype.UUID{}, errors.New(field + " is required")
	}
	var id pgtype.UUID
	if err := id.Scan(s); err != nil {
		return pgtype.UUID{}, errors.New(field + " is not a valid UUID")
	}
	return id, nil
}

// nullableText maps "" to pgtype.Text{Valid: false} so optional string
// fields that are empty end up as NULL in the DB, matching the
// semantics of the column definitions (all optional contact fields
// are nullable).
func nullableText(s string) pgtype.Text {
	s = strings.TrimSpace(s)
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}

func uuidString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	v, err := u.Value()
	if err != nil || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
