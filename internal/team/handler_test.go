package team_test

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
	teamv1 "github.com/Gabrielbdd/gospa/gen/gospa/team/v1"
	"github.com/Gabrielbdd/gospa/internal/team"
	"github.com/Gabrielbdd/gospa/internal/zitadel"
)

// --- fakes -----------------------------------------------------------

type fakeQueries struct {
	workspaceCompany sqlc.Company

	listRows []sqlc.ListTeamMembersRow

	contact       sqlc.Contact
	contactErr    error
	grant         sqlc.WorkspaceGrant
	grantErr      error

	existsResp bool
	existsErr  error

	createdContactParams sqlc.CreateContactParams
	createdContact       sqlc.Contact
	createContactErr     error

	createdGrantParams sqlc.CreateWorkspaceGrantParams
	createdGrant       sqlc.WorkspaceGrant
	createGrantErr     error

	updateRoleArg   sqlc.UpdateGrantRoleParams
	updateStatusArg sqlc.UpdateGrantStatusParams

	countResp int64
}

func (f *fakeQueries) GetWorkspaceCompany(_ context.Context) (sqlc.Company, error) {
	return f.workspaceCompany, nil
}
func (f *fakeQueries) ListTeamMembers(_ context.Context) ([]sqlc.ListTeamMembersRow, error) {
	return f.listRows, nil
}
func (f *fakeQueries) GetContact(_ context.Context, _ pgtype.UUID) (sqlc.Contact, error) {
	return f.contact, f.contactErr
}
func (f *fakeQueries) ContactExistsByCompanyEmail(_ context.Context, _ sqlc.ContactExistsByCompanyEmailParams) (bool, error) {
	return f.existsResp, f.existsErr
}
func (f *fakeQueries) CreateContact(_ context.Context, arg sqlc.CreateContactParams) (sqlc.Contact, error) {
	f.createdContactParams = arg
	if f.createContactErr != nil {
		return sqlc.Contact{}, f.createContactErr
	}
	row := sqlc.Contact{
		ID:            pgtype.UUID{Bytes: [16]byte{0xCA, 0xFE}, Valid: true},
		CompanyID:     arg.CompanyID,
		FullName:      arg.FullName,
		Email:         arg.Email,
		ZitadelUserID: arg.ZitadelUserID,
	}
	f.createdContact = row
	return row, nil
}
func (f *fakeQueries) CreateWorkspaceGrant(_ context.Context, arg sqlc.CreateWorkspaceGrantParams) (sqlc.WorkspaceGrant, error) {
	f.createdGrantParams = arg
	if f.createGrantErr != nil {
		return sqlc.WorkspaceGrant{}, f.createGrantErr
	}
	row := sqlc.WorkspaceGrant{
		ContactID: arg.ContactID,
		Role:      arg.Role,
		Status:    arg.Status,
	}
	f.createdGrant = row
	return row, nil
}
func (f *fakeQueries) GetGrantByContactID(_ context.Context, _ pgtype.UUID) (sqlc.WorkspaceGrant, error) {
	return f.grant, f.grantErr
}
func (f *fakeQueries) UpdateGrantRole(_ context.Context, arg sqlc.UpdateGrantRoleParams) error {
	f.updateRoleArg = arg
	return nil
}
func (f *fakeQueries) UpdateGrantStatus(_ context.Context, arg sqlc.UpdateGrantStatusParams) error {
	f.updateStatusArg = arg
	return nil
}
func (f *fakeQueries) CountActiveAdmins(_ context.Context) (int64, error) {
	return f.countResp, nil
}

type fakeZitadel struct {
	addHumanUserResp string
	addHumanUserErr  error
	addHumanUserOrg  string
	addHumanUserReq  zitadel.AddHumanUserRequest

	removeUserCalls []string
	removeUserErr   error
}

func (z *fakeZitadel) AddHumanUser(_ context.Context, orgID string, req zitadel.AddHumanUserRequest) (string, error) {
	z.addHumanUserOrg = orgID
	z.addHumanUserReq = req
	return z.addHumanUserResp, z.addHumanUserErr
}
func (z *fakeZitadel) RemoveUser(_ context.Context, _, userID string) error {
	z.removeUserCalls = append(z.removeUserCalls, userID)
	return z.removeUserErr
}

func discardLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func contactID(b byte) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte{b, b}, Valid: true}
}

func newHandler(q *fakeQueries, z *fakeZitadel) *team.Handler {
	return &team.Handler{Queries: q, Zitadel: z, Logger: discardLog()}
}

func mspCompany() sqlc.Company {
	return sqlc.Company{
		ID:               pgtype.UUID{Bytes: [16]byte{0xC0}, Valid: true},
		Name:             "Acme MSP",
		ZitadelOrgID:     "org-1",
		IsWorkspaceOwner: true,
	}
}

// --- ListMembers -----------------------------------------------------

func TestListMembers_ReturnsAllMembers(t *testing.T) {
	q := &fakeQueries{
		workspaceCompany: mspCompany(),
		listRows: []sqlc.ListTeamMembersRow{
			{
				ContactID: contactID(0x01),
				FullName:  "Ada Admin",
				Email:     pgtype.Text{String: "ada@msp.test", Valid: true},
				Role:      sqlc.WorkspaceRoleAdmin,
				Status:    sqlc.GrantStatusActive,
			},
			{
				ContactID: contactID(0x02),
				FullName:  "Tec Nico",
				Email:     pgtype.Text{String: "tec@msp.test", Valid: true},
				Role:      sqlc.WorkspaceRoleTechnician,
				Status:    sqlc.GrantStatusNotSignedInYet,
			},
		},
	}
	h := newHandler(q, &fakeZitadel{})

	resp, err := h.ListMembers(context.Background(), connect.NewRequest(&teamv1.ListMembersRequest{}))
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	if got := len(resp.Msg.Members); got != 2 {
		t.Fatalf("members = %d; want 2", got)
	}
	if resp.Msg.Members[0].Role != teamv1.MemberRole_MEMBER_ROLE_ADMIN {
		t.Errorf("first member role = %v; want ADMIN", resp.Msg.Members[0].Role)
	}
	if resp.Msg.Members[1].Status != teamv1.MemberStatus_MEMBER_STATUS_NOT_SIGNED_IN_YET {
		t.Errorf("second member status = %v; want NOT_SIGNED_IN_YET", resp.Msg.Members[1].Status)
	}
}

// --- InviteMember ----------------------------------------------------

func TestInviteMember_SuccessReturnsTempPasswordOnce(t *testing.T) {
	q := &fakeQueries{workspaceCompany: mspCompany()}
	z := &fakeZitadel{addHumanUserResp: "user-new"}
	h := newHandler(q, z)

	resp, err := h.InviteMember(context.Background(), connect.NewRequest(&teamv1.InviteMemberRequest{
		FullName: "New Tech",
		Email:    "new@msp.test",
		Role:     teamv1.MemberRole_MEMBER_ROLE_TECHNICIAN,
	}))
	if err != nil {
		t.Fatalf("InviteMember: %v", err)
	}
	if resp.Msg.TemporaryPassword == "" {
		t.Error("temporary_password must be set on the response exactly once")
	}
	if len(resp.Msg.TemporaryPassword) < 20 {
		t.Errorf("temporary_password too short: %d chars", len(resp.Msg.TemporaryPassword))
	}
	if z.addHumanUserOrg != "org-1" {
		t.Errorf("AddHumanUser called with org = %q; want org-1", z.addHumanUserOrg)
	}
	if !z.addHumanUserReq.PasswordChangeRequired {
		t.Error("PasswordChangeRequired must be true on invite")
	}
	if z.addHumanUserReq.Email != "new@msp.test" {
		t.Errorf("AddHumanUser email = %q; want new@msp.test", z.addHumanUserReq.Email)
	}
	if q.createdContactParams.FullName != "New Tech" {
		t.Errorf("contact full_name = %q; want New Tech", q.createdContactParams.FullName)
	}
	if q.createdGrantParams.Role != sqlc.WorkspaceRoleTechnician {
		t.Errorf("grant role = %q; want technician", q.createdGrantParams.Role)
	}
	if q.createdGrantParams.Status != sqlc.GrantStatusNotSignedInYet {
		t.Errorf("grant status = %q; want not_signed_in_yet", q.createdGrantParams.Status)
	}
	if len(z.removeUserCalls) != 0 {
		t.Errorf("RemoveUser called unexpectedly on success: %v", z.removeUserCalls)
	}
	if resp.Msg.Member == nil {
		t.Error("Member must be populated in response")
	}
}

func TestInviteMember_DuplicateEmailRejected(t *testing.T) {
	q := &fakeQueries{workspaceCompany: mspCompany(), existsResp: true}
	z := &fakeZitadel{}
	h := newHandler(q, z)

	_, err := h.InviteMember(context.Background(), connect.NewRequest(&teamv1.InviteMemberRequest{
		FullName: "Dupe",
		Email:    "dupe@msp.test",
		Role:     teamv1.MemberRole_MEMBER_ROLE_TECHNICIAN,
	}))
	if err == nil {
		t.Fatal("expected AlreadyExists error")
	}
	var cerr *connect.Error
	if !errors.As(err, &cerr) || cerr.Code() != connect.CodeAlreadyExists {
		t.Errorf("code = %v; want AlreadyExists", err)
	}
	if z.addHumanUserOrg != "" {
		t.Error("AddHumanUser must not fire when email duplicates")
	}
}

func TestInviteMember_ContactInsertFailureTriggersCleanup(t *testing.T) {
	q := &fakeQueries{
		workspaceCompany: mspCompany(),
		createContactErr: errors.New("unique violation"),
	}
	z := &fakeZitadel{addHumanUserResp: "user-new"}
	h := newHandler(q, z)

	_, err := h.InviteMember(context.Background(), connect.NewRequest(&teamv1.InviteMemberRequest{
		FullName: "New Tech",
		Email:    "new@msp.test",
		Role:     teamv1.MemberRole_MEMBER_ROLE_TECHNICIAN,
	}))
	if err == nil {
		t.Fatal("expected error from CreateContact failure")
	}
	if len(z.removeUserCalls) != 1 || z.removeUserCalls[0] != "user-new" {
		t.Errorf("RemoveUser calls = %v; want [user-new] (cleanup after partial invite)", z.removeUserCalls)
	}
}

func TestInviteMember_GrantInsertFailureTriggersCleanup(t *testing.T) {
	q := &fakeQueries{
		workspaceCompany: mspCompany(),
		createGrantErr:   errors.New("constraint violation"),
	}
	z := &fakeZitadel{addHumanUserResp: "user-new"}
	h := newHandler(q, z)

	_, err := h.InviteMember(context.Background(), connect.NewRequest(&teamv1.InviteMemberRequest{
		FullName: "New Tech",
		Email:    "new@msp.test",
		Role:     teamv1.MemberRole_MEMBER_ROLE_TECHNICIAN,
	}))
	if err == nil {
		t.Fatal("expected error from CreateWorkspaceGrant failure")
	}
	if len(z.removeUserCalls) != 1 {
		t.Errorf("RemoveUser must fire when grant insert fails; calls = %v", z.removeUserCalls)
	}
}

func TestInviteMember_RejectsMissingFields(t *testing.T) {
	h := newHandler(&fakeQueries{workspaceCompany: mspCompany()}, &fakeZitadel{})

	cases := []struct {
		name string
		req  *teamv1.InviteMemberRequest
	}{
		{"no full_name", &teamv1.InviteMemberRequest{Email: "e@x", Role: teamv1.MemberRole_MEMBER_ROLE_TECHNICIAN}},
		{"no email", &teamv1.InviteMemberRequest{FullName: "F", Role: teamv1.MemberRole_MEMBER_ROLE_TECHNICIAN}},
		{"unspecified role", &teamv1.InviteMemberRequest{FullName: "F", Email: "e@x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := h.InviteMember(context.Background(), connect.NewRequest(tc.req))
			if err == nil {
				t.Fatal("expected InvalidArgument")
			}
			var cerr *connect.Error
			if errors.As(err, &cerr) && cerr.Code() != connect.CodeInvalidArgument {
				t.Errorf("code = %v; want InvalidArgument", cerr.Code())
			}
		})
	}
}

// --- ChangeRole ------------------------------------------------------

func TestChangeRole_DBOnlyUpdatesGrant(t *testing.T) {
	cid := contactID(0x55)
	q := &fakeQueries{
		workspaceCompany: mspCompany(),
		contact:          sqlc.Contact{ID: cid, FullName: "Tec"},
		grant:            sqlc.WorkspaceGrant{ContactID: cid, Role: sqlc.WorkspaceRoleTechnician, Status: sqlc.GrantStatusActive},
		countResp:        2,
	}
	h := newHandler(q, &fakeZitadel{})

	_, err := h.ChangeRole(context.Background(), connect.NewRequest(&teamv1.ChangeRoleRequest{
		ContactId: uuidText(cid),
		Role:      teamv1.MemberRole_MEMBER_ROLE_ADMIN,
	}))
	if err != nil {
		t.Fatalf("ChangeRole: %v", err)
	}
	if q.updateRoleArg.Role != sqlc.WorkspaceRoleAdmin {
		t.Errorf("UpdateGrantRole role = %q; want admin", q.updateRoleArg.Role)
	}
}

func TestChangeRole_LastAdminDemotionRejected(t *testing.T) {
	cid := contactID(0xAA)
	q := &fakeQueries{
		workspaceCompany: mspCompany(),
		contact:          sqlc.Contact{ID: cid, FullName: "Only Admin"},
		grant:            sqlc.WorkspaceGrant{ContactID: cid, Role: sqlc.WorkspaceRoleAdmin, Status: sqlc.GrantStatusActive},
		countResp:        1, // only this admin
	}
	h := newHandler(q, &fakeZitadel{})

	_, err := h.ChangeRole(context.Background(), connect.NewRequest(&teamv1.ChangeRoleRequest{
		ContactId: uuidText(cid),
		Role:      teamv1.MemberRole_MEMBER_ROLE_TECHNICIAN,
	}))
	if err == nil {
		t.Fatal("expected FailedPrecondition from last-admin invariant")
	}
	var cerr *connect.Error
	if !errors.As(err, &cerr) || cerr.Code() != connect.CodeFailedPrecondition {
		t.Errorf("code = %v; want FailedPrecondition", cerr.Code())
	}
	if !errors.Is(cerr, cerr) {
		// placeholder assertion; real check below
	}
	// Error message must carry the "last_admin" code so the SPA can
	// surface a tailored message.
	if !containsAll(err.Error(), "last_admin") {
		t.Errorf("error missing last_admin marker: %v", err)
	}
}

func TestChangeRole_ContactNotFoundReturns404(t *testing.T) {
	q := &fakeQueries{
		workspaceCompany: mspCompany(),
		contactErr:       pgx.ErrNoRows,
	}
	h := newHandler(q, &fakeZitadel{})

	_, err := h.ChangeRole(context.Background(), connect.NewRequest(&teamv1.ChangeRoleRequest{
		ContactId: uuidText(contactID(0xEE)),
		Role:      teamv1.MemberRole_MEMBER_ROLE_ADMIN,
	}))
	if err == nil {
		t.Fatal("expected NotFound")
	}
	var cerr *connect.Error
	if errors.As(err, &cerr) && cerr.Code() != connect.CodeNotFound {
		t.Errorf("code = %v; want NotFound", cerr.Code())
	}
}

// --- SuspendMember / ReactivateMember ---------------------------------

func TestSuspendMember_DBOnlyNoZitadelCall(t *testing.T) {
	cid := contactID(0xBB)
	q := &fakeQueries{
		workspaceCompany: mspCompany(),
		contact:          sqlc.Contact{ID: cid, FullName: "Tec"},
		grant:            sqlc.WorkspaceGrant{ContactID: cid, Role: sqlc.WorkspaceRoleTechnician, Status: sqlc.GrantStatusActive},
		countResp:        2,
	}
	z := &fakeZitadel{}
	h := newHandler(q, z)

	resp, err := h.SuspendMember(context.Background(), connect.NewRequest(&teamv1.SuspendMemberRequest{
		ContactId: uuidText(cid),
	}))
	if err != nil {
		t.Fatalf("SuspendMember: %v", err)
	}
	if q.updateStatusArg.Status != sqlc.GrantStatusSuspended {
		t.Errorf("UpdateGrantStatus status = %q; want suspended", q.updateStatusArg.Status)
	}
	if len(z.removeUserCalls) != 0 {
		t.Errorf("Suspend must not call RemoveUser (Gospa-only suspend); calls = %v", z.removeUserCalls)
	}
	if resp.Msg.Member.Status != teamv1.MemberStatus_MEMBER_STATUS_SUSPENDED {
		t.Errorf("response status = %v; want SUSPENDED", resp.Msg.Member.Status)
	}
}

func TestSuspendMember_LastAdminRejected(t *testing.T) {
	cid := contactID(0xAA)
	q := &fakeQueries{
		workspaceCompany: mspCompany(),
		contact:          sqlc.Contact{ID: cid},
		grant:            sqlc.WorkspaceGrant{ContactID: cid, Role: sqlc.WorkspaceRoleAdmin, Status: sqlc.GrantStatusActive},
		countResp:        1,
	}
	h := newHandler(q, &fakeZitadel{})

	_, err := h.SuspendMember(context.Background(), connect.NewRequest(&teamv1.SuspendMemberRequest{
		ContactId: uuidText(cid),
	}))
	if err == nil {
		t.Fatal("expected FailedPrecondition")
	}
	if !containsAll(err.Error(), "last_admin") {
		t.Errorf("error missing last_admin: %v", err)
	}
}

func TestReactivateMember_FlipsStatus(t *testing.T) {
	cid := contactID(0xCC)
	q := &fakeQueries{
		workspaceCompany: mspCompany(),
		contact:          sqlc.Contact{ID: cid, FullName: "Tec"},
		grant:            sqlc.WorkspaceGrant{ContactID: cid, Role: sqlc.WorkspaceRoleTechnician, Status: sqlc.GrantStatusSuspended},
	}
	h := newHandler(q, &fakeZitadel{})

	resp, err := h.ReactivateMember(context.Background(), connect.NewRequest(&teamv1.ReactivateMemberRequest{
		ContactId: uuidText(cid),
	}))
	if err != nil {
		t.Fatalf("ReactivateMember: %v", err)
	}
	if q.updateStatusArg.Status != sqlc.GrantStatusActive {
		t.Errorf("UpdateGrantStatus status = %q; want active", q.updateStatusArg.Status)
	}
	if resp.Msg.Member.Status != teamv1.MemberStatus_MEMBER_STATUS_ACTIVE {
		t.Errorf("response status = %v; want ACTIVE", resp.Msg.Member.Status)
	}
}

// --- helpers ---------------------------------------------------------

func uuidText(u pgtype.UUID) string {
	v, _ := u.Value()
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func containsAll(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
