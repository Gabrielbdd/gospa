// Package team owns the TeamService backend: list/invite/change-role/
// suspend/reactivate for MSP team members. Team members are stored as
// contacts of the MSP's companies row (is_workspace_owner = TRUE) with
// a workspace_grants row assigning their role.
//
// Authorisation is enforced by internal/authz/middleware — all admin-
// only RPCs here are registered in the policy map and rejected before
// reaching this handler if the caller is a technician. The handler's
// only extra gate is the last-admin invariant: demote/suspend of the
// only active admin returns FailedPrecondition.
package team

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"math/big"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Gabrielbdd/gospa/db/sqlc"
	teamv1 "github.com/Gabrielbdd/gospa/gen/gospa/team/v1"
	"github.com/Gabrielbdd/gospa/gen/gospa/team/v1/teamv1connect"
	"github.com/Gabrielbdd/gospa/internal/authz"
	"github.com/Gabrielbdd/gospa/internal/zitadel"
)

// Queries is the narrow subset of sqlc that the team handler needs.
type Queries interface {
	GetWorkspaceCompany(ctx context.Context) (sqlc.Company, error)
	ListTeamMembers(ctx context.Context) ([]sqlc.ListTeamMembersRow, error)
	GetContact(ctx context.Context, id pgtype.UUID) (sqlc.Contact, error)
	ContactExistsByCompanyEmail(ctx context.Context, arg sqlc.ContactExistsByCompanyEmailParams) (bool, error)
	CreateContact(ctx context.Context, arg sqlc.CreateContactParams) (sqlc.Contact, error)
	CreateWorkspaceGrant(ctx context.Context, arg sqlc.CreateWorkspaceGrantParams) (sqlc.WorkspaceGrant, error)
	GetGrantByContactID(ctx context.Context, contactID pgtype.UUID) (sqlc.WorkspaceGrant, error)
	UpdateGrantRole(ctx context.Context, arg sqlc.UpdateGrantRoleParams) error
	UpdateGrantStatus(ctx context.Context, arg sqlc.UpdateGrantStatusParams) error
	CountActiveAdmins(ctx context.Context) (int64, error)
}

// ZitadelClient is the narrow subset of zitadel.Client that the team
// handler needs. Kept as its own interface so test doubles stay small.
type ZitadelClient interface {
	AddHumanUser(ctx context.Context, orgID string, req zitadel.AddHumanUserRequest) (string, error)
	RemoveUser(ctx context.Context, orgID, userID string) error
}

// Handler implements TeamService.
type Handler struct {
	teamv1connect.UnimplementedTeamServiceHandler

	Queries Queries
	Zitadel ZitadelClient
	Logger  *slog.Logger
}

// ListMembers returns every non-archived team member.
func (h *Handler) ListMembers(ctx context.Context, _ *connect.Request[teamv1.ListMembersRequest]) (*connect.Response[teamv1.ListMembersResponse], error) {
	rows, err := h.Queries.ListTeamMembers(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*teamv1.TeamMember, 0, len(rows))
	for _, r := range rows {
		out = append(out, teamMemberFromListRow(r))
	}
	return connect.NewResponse(&teamv1.ListMembersResponse{Members: out}), nil
}

// InviteMember validates input, fails fast on duplicate email, asks
// ZITADEL to mint the human user, and in one DB transaction inserts
// the local contact + workspace_grants row. Returns the temporary
// password exactly once.
func (h *Handler) InviteMember(ctx context.Context, req *connect.Request[teamv1.InviteMemberRequest]) (*connect.Response[teamv1.InviteMemberResponse], error) {
	fullName := strings.TrimSpace(req.Msg.FullName)
	email := strings.TrimSpace(req.Msg.Email)
	if fullName == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("full_name is required"))
	}
	if email == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("email is required"))
	}
	role, err := grantRoleFromProto(req.Msg.Role)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	workspaceCompany, err := h.Queries.GetWorkspaceCompany(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Pre-check for duplicate email at the MSP company. The DB-level
	// unique index also guards this, but the pre-check avoids a wasted
	// ZITADEL round-trip on the common "already exists" case.
	exists, err := h.Queries.ContactExistsByCompanyEmail(ctx, sqlc.ContactExistsByCompanyEmailParams{
		CompanyID: workspaceCompany.ID,
		Lower:     email,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if exists {
		return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("a team member with this email already exists"))
	}

	tempPassword, err := generateTempPassword()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	firstName, lastName := splitFullName(fullName)
	userID, err := h.Zitadel.AddHumanUser(ctx, workspaceCompany.ZitadelOrgID, zitadel.AddHumanUserRequest{
		Email:                  email,
		FirstName:              firstName,
		LastName:               lastName,
		Password:               tempPassword,
		PasswordChangeRequired: true,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// From here on, any failure must compensate by removing the just-
	// created ZITADEL user (mirrors the RemoveOrg cleanup in S15). The
	// defer uses a background context so it still runs if the request
	// context was canceled.
	orgID := workspaceCompany.ZitadelOrgID
	var committed bool
	defer func() {
		if committed {
			return
		}
		if rmErr := h.Zitadel.RemoveUser(context.Background(), orgID, userID); rmErr != nil {
			h.Logger.ErrorContext(ctx, "team invite cleanup failed; ZITADEL user left orphan",
				"zitadel_user_id", userID,
				"error", rmErr)
		}
	}()

	contact, err := h.Queries.CreateContact(ctx, sqlc.CreateContactParams{
		CompanyID:      workspaceCompany.ID,
		FullName:       fullName,
		Email:          pgtype.Text{String: email, Valid: true},
		ZitadelUserID:  pgtype.Text{String: userID, Valid: true},
		IdentitySource: "manual",
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	grant, err := h.Queries.CreateWorkspaceGrant(ctx, sqlc.CreateWorkspaceGrantParams{
		ContactID: contact.ID,
		Role:      role,
		Status:    sqlc.GrantStatusNotSignedInYet,
		GrantedByContactID: callerContactID(ctx),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	committed = true

	return connect.NewResponse(&teamv1.InviteMemberResponse{
		Member:            teamMemberFromContactAndGrant(contact, grant),
		TemporaryPassword: tempPassword,
	}), nil
}

// ChangeRole flips the role on an existing grant. The change applies
// on the affected member's next request (no JWT-TTL staleness because
// role lives in Postgres).
func (h *Handler) ChangeRole(ctx context.Context, req *connect.Request[teamv1.ChangeRoleRequest]) (*connect.Response[teamv1.ChangeRoleResponse], error) {
	contactID, err := parseContactID(req.Msg.ContactId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	newRole, err := grantRoleFromProto(req.Msg.Role)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	contact, grant, err := h.loadContactAndGrant(ctx, contactID)
	if err != nil {
		return nil, err
	}

	// Last-admin invariant: cannot demote the only active admin.
	if grant.Role == sqlc.WorkspaceRoleAdmin && newRole != sqlc.WorkspaceRoleAdmin {
		if err := h.ensureAnotherActiveAdmin(ctx); err != nil {
			return nil, err
		}
	}

	if err := h.Queries.UpdateGrantRole(ctx, sqlc.UpdateGrantRoleParams{
		ContactID: contactID,
		Role:      newRole,
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	grant.Role = newRole

	return connect.NewResponse(&teamv1.ChangeRoleResponse{
		Member: teamMemberFromContactAndGrant(contact, grant),
	}), nil
}

// SuspendMember flips status=suspended in Postgres. The ZITADEL user
// stays active — the authz middleware rejects every request from a
// suspended grant, so the member is locked out of Gospa even though
// their identity still works against ZITADEL.
func (h *Handler) SuspendMember(ctx context.Context, req *connect.Request[teamv1.SuspendMemberRequest]) (*connect.Response[teamv1.SuspendMemberResponse], error) {
	contactID, err := parseContactID(req.Msg.ContactId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	contact, grant, err := h.loadContactAndGrant(ctx, contactID)
	if err != nil {
		return nil, err
	}

	// Last-admin invariant: cannot suspend the only active admin.
	if grant.Role == sqlc.WorkspaceRoleAdmin && grant.Status == sqlc.GrantStatusActive {
		if err := h.ensureAnotherActiveAdmin(ctx); err != nil {
			return nil, err
		}
	}

	if err := h.Queries.UpdateGrantStatus(ctx, sqlc.UpdateGrantStatusParams{
		ContactID: contactID,
		Status:    sqlc.GrantStatusSuspended,
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	grant.Status = sqlc.GrantStatusSuspended

	return connect.NewResponse(&teamv1.SuspendMemberResponse{
		Member: teamMemberFromContactAndGrant(contact, grant),
	}), nil
}

// ReactivateMember flips a suspended grant back to active.
func (h *Handler) ReactivateMember(ctx context.Context, req *connect.Request[teamv1.ReactivateMemberRequest]) (*connect.Response[teamv1.ReactivateMemberResponse], error) {
	contactID, err := parseContactID(req.Msg.ContactId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	contact, grant, err := h.loadContactAndGrant(ctx, contactID)
	if err != nil {
		return nil, err
	}
	if err := h.Queries.UpdateGrantStatus(ctx, sqlc.UpdateGrantStatusParams{
		ContactID: contactID,
		Status:    sqlc.GrantStatusActive,
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	grant.Status = sqlc.GrantStatusActive
	return connect.NewResponse(&teamv1.ReactivateMemberResponse{
		Member: teamMemberFromContactAndGrant(contact, grant),
	}), nil
}

// loadContactAndGrant fetches both rows and returns a precise Connect
// error on missing rows. Centralises the error mapping so every RPC
// shares the same shape.
func (h *Handler) loadContactAndGrant(ctx context.Context, contactID pgtype.UUID) (sqlc.Contact, sqlc.WorkspaceGrant, error) {
	contact, err := h.Queries.GetContact(ctx, contactID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.Contact{}, sqlc.WorkspaceGrant{}, connect.NewError(connect.CodeNotFound, errors.New("contact not found"))
		}
		return sqlc.Contact{}, sqlc.WorkspaceGrant{}, connect.NewError(connect.CodeInternal, err)
	}
	grant, err := h.Queries.GetGrantByContactID(ctx, contactID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.Contact{}, sqlc.WorkspaceGrant{}, connect.NewError(connect.CodeNotFound, errors.New("contact is not a team member"))
		}
		return sqlc.Contact{}, sqlc.WorkspaceGrant{}, connect.NewError(connect.CodeInternal, err)
	}
	return contact, grant, nil
}

// ensureAnotherActiveAdmin returns a FailedPrecondition error with code
// "last_admin" if this operation would leave zero active admins. The
// check runs before the mutation, so the final CountActiveAdmins after
// the mutation would still return ≥ 1.
func (h *Handler) ensureAnotherActiveAdmin(ctx context.Context) error {
	count, err := h.Queries.CountActiveAdmins(ctx)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if count <= 1 {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("last_admin: at least one active admin must remain"))
	}
	return nil
}

// callerContactID returns the caller's contact id from the authz
// context, or a zero-value pgtype.UUID if absent. Used to fill
// granted_by_contact_id on invite — a best-effort audit field.
func callerContactID(ctx context.Context) pgtype.UUID {
	id, ok := authz.ContactID(ctx)
	if !ok {
		return pgtype.UUID{Valid: false}
	}
	return id
}

// --- helpers: proto <-> sqlc mapping -----------------------------------

func teamMemberFromListRow(r sqlc.ListTeamMembersRow) *teamv1.TeamMember {
	m := &teamv1.TeamMember{
		Id:       uuidString(r.ContactID),
		FullName: r.FullName,
		Role:     protoRoleFromGrant(r.Role),
		Status:   protoStatusFromGrant(r.Status),
	}
	if r.Email.Valid {
		m.Email = r.Email.String
	}
	if r.LastSeenAt.Valid {
		m.LastSeenAt = r.LastSeenAt.Time.UTC().Format(time.RFC3339)
	}
	if r.CreatedAt.Valid {
		m.CreatedAt = r.CreatedAt.Time.UTC().Format(time.RFC3339)
	}
	return m
}

func teamMemberFromContactAndGrant(contact sqlc.Contact, grant sqlc.WorkspaceGrant) *teamv1.TeamMember {
	m := &teamv1.TeamMember{
		Id:       uuidString(contact.ID),
		FullName: contact.FullName,
		Role:     protoRoleFromGrant(grant.Role),
		Status:   protoStatusFromGrant(grant.Status),
	}
	if contact.Email.Valid {
		m.Email = contact.Email.String
	}
	if grant.LastSeenAt.Valid {
		m.LastSeenAt = grant.LastSeenAt.Time.UTC().Format(time.RFC3339)
	}
	if contact.CreatedAt.Valid {
		m.CreatedAt = contact.CreatedAt.Time.UTC().Format(time.RFC3339)
	}
	return m
}

func protoRoleFromGrant(r sqlc.WorkspaceRole) teamv1.MemberRole {
	switch r {
	case sqlc.WorkspaceRoleAdmin:
		return teamv1.MemberRole_MEMBER_ROLE_ADMIN
	case sqlc.WorkspaceRoleTechnician:
		return teamv1.MemberRole_MEMBER_ROLE_TECHNICIAN
	default:
		return teamv1.MemberRole_MEMBER_ROLE_UNSPECIFIED
	}
}

func protoStatusFromGrant(s sqlc.GrantStatus) teamv1.MemberStatus {
	switch s {
	case sqlc.GrantStatusActive:
		return teamv1.MemberStatus_MEMBER_STATUS_ACTIVE
	case sqlc.GrantStatusNotSignedInYet:
		return teamv1.MemberStatus_MEMBER_STATUS_NOT_SIGNED_IN_YET
	case sqlc.GrantStatusSuspended:
		return teamv1.MemberStatus_MEMBER_STATUS_SUSPENDED
	default:
		return teamv1.MemberStatus_MEMBER_STATUS_UNSPECIFIED
	}
}

func grantRoleFromProto(r teamv1.MemberRole) (sqlc.WorkspaceRole, error) {
	switch r {
	case teamv1.MemberRole_MEMBER_ROLE_ADMIN:
		return sqlc.WorkspaceRoleAdmin, nil
	case teamv1.MemberRole_MEMBER_ROLE_TECHNICIAN:
		return sqlc.WorkspaceRoleTechnician, nil
	default:
		return "", errors.New("role must be admin or technician")
	}
}

func parseContactID(s string) (pgtype.UUID, error) {
	if s == "" {
		return pgtype.UUID{}, errors.New("contact_id is required")
	}
	var id pgtype.UUID
	if err := id.Scan(s); err != nil {
		return pgtype.UUID{}, errors.New("contact_id is not a valid UUID")
	}
	return id, nil
}

// uuidString returns the canonical text form of a pgtype.UUID.
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

// splitFullName takes "Ada Lovelace" -> ("Ada", "Lovelace"). A single
// token becomes ("Ada", ""); an empty string becomes ("", ""). The
// split is intentionally naïve — Gospa records full_name as the
// canonical display form and only separates first/last for the
// ZITADEL profile (where the two fields are required).
func splitFullName(full string) (first, last string) {
	full = strings.TrimSpace(full)
	if full == "" {
		return "", ""
	}
	parts := strings.Fields(full)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], strings.Join(parts[1:], " ")
}

// generateTempPassword returns a cryptographically-random 24-character
// password drawn from a charset that meets ZITADEL's default policy
// (upper + lower + digit). The alphabet excludes visually ambiguous
// characters so the admin can read the password aloud or paste it
// without transcription errors.
func generateTempPassword() (string, error) {
	const (
		length  = 24
		// Excludes 0/O/I/l/1 to reduce read-aloud mistakes; still
		// contains all three character classes ZITADEL's default
		// policy expects.
		alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789"
	)
	out := make([]byte, length)
	max := big.NewInt(int64(len(alphabet)))
	for i := range out {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		out[i] = alphabet[n.Int64()]
	}
	return string(out), nil
}
