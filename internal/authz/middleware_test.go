package authz_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	runtimeauth "github.com/Gabrielbdd/gofra/runtime/auth"

	companiesv1connect "github.com/Gabrielbdd/gospa/gen/gospa/companies/v1/companiesv1connect"
	installv1connect "github.com/Gabrielbdd/gospa/gen/gospa/install/v1/installv1connect"

	"github.com/Gabrielbdd/gospa/db/sqlc"
	"github.com/Gabrielbdd/gospa/internal/authz"
)

// fakeQueries captures every call so each test can assert exactly the
// shape of the authz round-trip.
type fakeQueries struct {
	resolveResp sqlc.ResolveTeamCallerByZitadelUserIDRow
	resolveErr  error
	resolveArg  pgtype.Text

	activateCalls []pgtype.UUID
	activateErr   error

	touchCalls []pgtype.UUID
	touchErr   error
}

func (f *fakeQueries) ResolveTeamCallerByZitadelUserID(_ context.Context, arg pgtype.Text) (sqlc.ResolveTeamCallerByZitadelUserIDRow, error) {
	f.resolveArg = arg
	return f.resolveResp, f.resolveErr
}

func (f *fakeQueries) ActivatePendingGrant(_ context.Context, contactID pgtype.UUID) error {
	f.activateCalls = append(f.activateCalls, contactID)
	return f.activateErr
}

func (f *fakeQueries) TouchLastSeen(_ context.Context, contactID pgtype.UUID) error {
	f.touchCalls = append(f.touchCalls, contactID)
	return f.touchErr
}

func discardLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// downstream is a tiny inner handler that records the Caller it sees
// in context, so tests can assert that the middleware attached the
// expected identity (or didn't run, for the bypass cases).
type downstream struct {
	calls  int
	caller authz.Caller
	hadCaller bool
}

func (d *downstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	d.calls++
	d.caller, d.hadCaller = authz.CallerFromContext(r.Context())
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// authedRequest builds a request with a runtimeauth.User attached
// (mimicking what the auth gate would do upstream) and points it at
// the given Connect procedure.
func authedRequest(procedure, userID string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, procedure, nil)
	if userID != "" {
		r = r.WithContext(runtimeauth.WithUser(r.Context(), runtimeauth.User{ID: userID}))
	}
	return r
}

func contactID(b byte) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte{b, b}, Valid: true}
}

func adminGrantRow(id pgtype.UUID) sqlc.ResolveTeamCallerByZitadelUserIDRow {
	return sqlc.ResolveTeamCallerByZitadelUserIDRow{
		ContactID:   id,
		GrantRole:   sqlc.NullWorkspaceRole{WorkspaceRole: sqlc.WorkspaceRoleAdmin, Valid: true},
		GrantStatus: sqlc.NullGrantStatus{GrantStatus: sqlc.GrantStatusActive, Valid: true},
	}
}

func technicianGrantRow(id pgtype.UUID) sqlc.ResolveTeamCallerByZitadelUserIDRow {
	return sqlc.ResolveTeamCallerByZitadelUserIDRow{
		ContactID:   id,
		GrantRole:   sqlc.NullWorkspaceRole{WorkspaceRole: sqlc.WorkspaceRoleTechnician, Valid: true},
		GrantStatus: sqlc.NullGrantStatus{GrantStatus: sqlc.GrantStatusActive, Valid: true},
	}
}

func suspendedGrantRow(id pgtype.UUID) sqlc.ResolveTeamCallerByZitadelUserIDRow {
	return sqlc.ResolveTeamCallerByZitadelUserIDRow{
		ContactID:   id,
		GrantRole:   sqlc.NullWorkspaceRole{WorkspaceRole: sqlc.WorkspaceRoleAdmin, Valid: true},
		GrantStatus: sqlc.NullGrantStatus{GrantStatus: sqlc.GrantStatusSuspended, Valid: true},
	}
}

func pendingGrantRow(id pgtype.UUID) sqlc.ResolveTeamCallerByZitadelUserIDRow {
	return sqlc.ResolveTeamCallerByZitadelUserIDRow{
		ContactID:   id,
		GrantRole:   sqlc.NullWorkspaceRole{WorkspaceRole: sqlc.WorkspaceRoleTechnician, Valid: true},
		GrantStatus: sqlc.NullGrantStatus{GrantStatus: sqlc.GrantStatusNotSignedInYet, Valid: true},
	}
}

func contactWithoutGrantRow(id pgtype.UUID) sqlc.ResolveTeamCallerByZitadelUserIDRow {
	return sqlc.ResolveTeamCallerByZitadelUserIDRow{
		ContactID:   id,
		GrantRole:   sqlc.NullWorkspaceRole{Valid: false},
		GrantStatus: sqlc.NullGrantStatus{Valid: false},
	}
}

// publicMatcher mimics what cmd/app uses: install RPCs are public.
func publicMatcher() runtimeauth.ProcedureMatcher {
	return runtimeauth.PublicProcedures(
		installv1connect.InstallServiceGetStatusProcedure,
		installv1connect.InstallServiceInstallProcedure,
	)
}

func decodeError(t *testing.T, body io.Reader) (code, message string) {
	t.Helper()
	var got struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(body).Decode(&got); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	return got.Code, got.Message
}

// --- Bypass paths ----------------------------------------------------

func TestWrap_NonConnectPathBypassesAuthz(t *testing.T) {
	q := &fakeQueries{}
	mw := authz.New(q, publicMatcher(), discardLog())
	d := &downstream{}

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/_gofra/config.js", nil)
	mw.Wrap(d).ServeHTTP(rec, r)

	if d.calls != 1 {
		t.Fatalf("downstream calls = %d; want 1 (non-Connect bypassed authz)", d.calls)
	}
	if d.hadCaller {
		t.Errorf("non-Connect path should not attach a Caller")
	}
	if q.resolveArg.Valid {
		t.Errorf("authz should not query DB for non-Connect paths")
	}
}

func TestWrap_PublicConnectProcedureBypassesAuthz(t *testing.T) {
	q := &fakeQueries{}
	mw := authz.New(q, publicMatcher(), discardLog())
	d := &downstream{}

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, installv1connect.InstallServiceInstallProcedure, nil)
	mw.Wrap(d).ServeHTTP(rec, r)

	if d.calls != 1 {
		t.Fatalf("downstream calls = %d; want 1 (install RPCs are public)", d.calls)
	}
	if q.resolveArg.Valid {
		t.Errorf("authz must not query DB for public procedures")
	}
}

// --- Failure paths ---------------------------------------------------

func TestWrap_MissingUserReturns401(t *testing.T) {
	q := &fakeQueries{}
	mw := authz.New(q, publicMatcher(), discardLog())
	d := &downstream{}

	rec := httptest.NewRecorder()
	// No User in context — the gate would normally have rejected, but
	// authz must defend in depth.
	r := httptest.NewRequest(http.MethodPost, companiesv1connect.CompaniesServiceListCompaniesProcedure, nil)
	mw.Wrap(d).ServeHTTP(rec, r)

	if d.calls != 0 {
		t.Errorf("downstream should not run when User is missing")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want 401", rec.Code)
	}
	code, _ := decodeError(t, rec.Body)
	if code != "unauthenticated" {
		t.Errorf("error code = %q; want unauthenticated", code)
	}
}

func TestWrap_NotProvisionedReturns401(t *testing.T) {
	q := &fakeQueries{resolveErr: pgx.ErrNoRows}
	mw := authz.New(q, publicMatcher(), discardLog())
	d := &downstream{}

	rec := httptest.NewRecorder()
	r := authedRequest(companiesv1connect.CompaniesServiceListCompaniesProcedure, "user-1")
	mw.Wrap(d).ServeHTTP(rec, r)

	if d.calls != 0 {
		t.Errorf("downstream should not run when contact is missing")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want 401", rec.Code)
	}
	code, _ := decodeError(t, rec.Body)
	if code != "not_provisioned" {
		t.Errorf("error code = %q; want not_provisioned", code)
	}
}

func TestWrap_DBErrorReturns500(t *testing.T) {
	q := &fakeQueries{resolveErr: errors.New("db down")}
	mw := authz.New(q, publicMatcher(), discardLog())
	d := &downstream{}

	rec := httptest.NewRecorder()
	r := authedRequest(companiesv1connect.CompaniesServiceListCompaniesProcedure, "user-1")
	mw.Wrap(d).ServeHTTP(rec, r)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want 500", rec.Code)
	}
}

func TestWrap_ContactWithoutGrantReturns403(t *testing.T) {
	q := &fakeQueries{resolveResp: contactWithoutGrantRow(contactID(0xC0))}
	mw := authz.New(q, publicMatcher(), discardLog())
	d := &downstream{}

	rec := httptest.NewRecorder()
	r := authedRequest(companiesv1connect.CompaniesServiceListCompaniesProcedure, "user-1")
	mw.Wrap(d).ServeHTTP(rec, r)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d; want 403", rec.Code)
	}
	code, _ := decodeError(t, rec.Body)
	if code != "no_grant" {
		t.Errorf("error code = %q; want no_grant", code)
	}
}

func TestWrap_SuspendedReturns401AccountSuspended(t *testing.T) {
	q := &fakeQueries{resolveResp: suspendedGrantRow(contactID(0x55))}
	mw := authz.New(q, publicMatcher(), discardLog())
	d := &downstream{}

	rec := httptest.NewRecorder()
	r := authedRequest(companiesv1connect.CompaniesServiceListCompaniesProcedure, "user-1")
	mw.Wrap(d).ServeHTTP(rec, r)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want 401", rec.Code)
	}
	code, _ := decodeError(t, rec.Body)
	if code != "account_suspended" {
		t.Errorf("error code = %q; want account_suspended", code)
	}
}

func TestWrap_PolicyUndefinedReturns403(t *testing.T) {
	q := &fakeQueries{resolveResp: adminGrantRow(contactID(0xA1))}
	mw := authz.New(q, publicMatcher(), discardLog())
	d := &downstream{}

	rec := httptest.NewRecorder()
	// Connect-shaped path that isn't in the policy map.
	r := authedRequest("/some.unknown.v1.Service/Method", "user-1")
	mw.Wrap(d).ServeHTTP(rec, r)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d; want 403", rec.Code)
	}
	code, _ := decodeError(t, rec.Body)
	if code != "policy_undefined" {
		t.Errorf("error code = %q; want policy_undefined", code)
	}
}

func TestWrap_AdminOnlyRPC_TechnicianRejected(t *testing.T) {
	q := &fakeQueries{resolveResp: technicianGrantRow(contactID(0x77))}
	mw := authz.New(q, publicMatcher(), discardLog())
	d := &downstream{}

	rec := httptest.NewRecorder()
	r := authedRequest(companiesv1connect.CompaniesServiceArchiveCompanyProcedure, "user-1")
	mw.Wrap(d).ServeHTTP(rec, r)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d; want 403", rec.Code)
	}
	code, _ := decodeError(t, rec.Body)
	if code != "admin_required" {
		t.Errorf("error code = %q; want admin_required", code)
	}
}

// --- Success paths ---------------------------------------------------

func TestWrap_AdminCanCallAdminOnlyRPC(t *testing.T) {
	cid := contactID(0xAD)
	q := &fakeQueries{resolveResp: adminGrantRow(cid)}
	mw := authz.New(q, publicMatcher(), discardLog())
	d := &downstream{}

	rec := httptest.NewRecorder()
	r := authedRequest(companiesv1connect.CompaniesServiceArchiveCompanyProcedure, "user-1")
	mw.Wrap(d).ServeHTTP(rec, r)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want 200", rec.Code)
	}
	if d.calls != 1 {
		t.Fatalf("downstream calls = %d; want 1", d.calls)
	}
	if !d.hadCaller {
		t.Fatal("downstream should have received Caller in context")
	}
	if d.caller.ContactID != cid {
		t.Errorf("Caller contact_id = %v; want %v", d.caller.ContactID, cid)
	}
	if d.caller.Role != sqlc.WorkspaceRoleAdmin {
		t.Errorf("Caller role = %q; want admin", d.caller.Role)
	}
}

func TestWrap_TechnicianCanCallAuthenticatedRPC(t *testing.T) {
	q := &fakeQueries{resolveResp: technicianGrantRow(contactID(0x77))}
	mw := authz.New(q, publicMatcher(), discardLog())
	d := &downstream{}

	rec := httptest.NewRecorder()
	r := authedRequest(companiesv1connect.CompaniesServiceListCompaniesProcedure, "user-1")
	mw.Wrap(d).ServeHTTP(rec, r)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want 200", rec.Code)
	}
	if d.caller.Role != sqlc.WorkspaceRoleTechnician {
		t.Errorf("Caller role = %q; want technician", d.caller.Role)
	}
}

func TestWrap_PendingGrantTriggersActivation(t *testing.T) {
	cid := contactID(0xBE)
	q := &fakeQueries{resolveResp: pendingGrantRow(cid)}
	mw := authz.New(q, publicMatcher(), discardLog())
	d := &downstream{}

	rec := httptest.NewRecorder()
	r := authedRequest(companiesv1connect.CompaniesServiceListCompaniesProcedure, "user-1")
	mw.Wrap(d).ServeHTTP(rec, r)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want 200 (pending grants still serve, with activation side effect)", rec.Code)
	}
	if len(q.activateCalls) != 1 || q.activateCalls[0] != cid {
		t.Errorf("ActivatePendingGrant calls = %v; want one call with %v", q.activateCalls, cid)
	}
	if d.calls != 1 {
		t.Fatalf("downstream calls = %d; want 1", d.calls)
	}
}

func TestWrap_LastSeenThrottle(t *testing.T) {
	cid := contactID(0x42)
	q := &fakeQueries{resolveResp: adminGrantRow(cid)}

	// Tighten the throttle so the test stays deterministic without
	// sleeping. Restore on exit.
	saved := authz.LastSeenThrottle
	authz.LastSeenThrottle = 50 * time.Millisecond
	t.Cleanup(func() { authz.LastSeenThrottle = saved })

	mw := authz.New(q, publicMatcher(), discardLog())
	d := &downstream{}
	handler := mw.Wrap(d)

	// First request — touch fires.
	for i := 0; i < 3; i++ {
		rec := httptest.NewRecorder()
		r := authedRequest(companiesv1connect.CompaniesServiceListCompaniesProcedure, "user-1")
		handler.ServeHTTP(rec, r)
	}
	if len(q.touchCalls) != 1 {
		t.Errorf("after 3 rapid requests TouchLastSeen calls = %d; want 1 (throttled)", len(q.touchCalls))
	}

	// Wait past the throttle window — next request should touch again.
	time.Sleep(60 * time.Millisecond)

	rec := httptest.NewRecorder()
	r := authedRequest(companiesv1connect.CompaniesServiceListCompaniesProcedure, "user-1")
	handler.ServeHTTP(rec, r)

	if len(q.touchCalls) != 2 {
		t.Errorf("after throttle expiry TouchLastSeen calls = %d; want 2", len(q.touchCalls))
	}
}

// CallerFromContext on a context without authz returns false.
func TestCallerFromContext_AbsentReturnsFalse(t *testing.T) {
	if _, ok := authz.CallerFromContext(context.Background()); ok {
		t.Error("CallerFromContext on bare ctx should return false")
	}
}
