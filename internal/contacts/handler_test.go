package contacts_test

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
	contactsv1 "github.com/Gabrielbdd/gospa/gen/gospa/contacts/v1"
	"github.com/Gabrielbdd/gospa/internal/contacts"
)

type fakeQueries struct {
	listRows []sqlc.Contact
	listErr  error

	getRow sqlc.Contact
	getErr error

	createArg sqlc.CreateContactParams
	createRow sqlc.Contact
	createErr error

	updateArg sqlc.UpdateContactParams
	updateRow sqlc.Contact
	updateErr error

	archiveCalls []pgtype.UUID
	archiveErr   error

	existsResp bool

	hasGrantResp bool
}

func (f *fakeQueries) GetContact(_ context.Context, _ pgtype.UUID) (sqlc.Contact, error) {
	return f.getRow, f.getErr
}
func (f *fakeQueries) ListContactsByCompany(_ context.Context, _ pgtype.UUID) ([]sqlc.Contact, error) {
	return f.listRows, f.listErr
}
func (f *fakeQueries) CreateContact(_ context.Context, arg sqlc.CreateContactParams) (sqlc.Contact, error) {
	f.createArg = arg
	if f.createErr != nil {
		return sqlc.Contact{}, f.createErr
	}
	if f.createRow.ID.Valid {
		return f.createRow, nil
	}
	return sqlc.Contact{
		ID:        pgtype.UUID{Bytes: [16]byte{0xCA}, Valid: true},
		CompanyID: arg.CompanyID,
		FullName:  arg.FullName,
		Email:     arg.Email,
	}, nil
}
func (f *fakeQueries) UpdateContact(_ context.Context, arg sqlc.UpdateContactParams) (sqlc.Contact, error) {
	f.updateArg = arg
	return f.updateRow, f.updateErr
}
func (f *fakeQueries) ArchiveContact(_ context.Context, id pgtype.UUID) error {
	f.archiveCalls = append(f.archiveCalls, id)
	return f.archiveErr
}
func (f *fakeQueries) ContactExistsByCompanyEmail(_ context.Context, _ sqlc.ContactExistsByCompanyEmailParams) (bool, error) {
	return f.existsResp, nil
}
func (f *fakeQueries) ContactHasWorkspaceGrant(_ context.Context, _ pgtype.UUID) (bool, error) {
	return f.hasGrantResp, nil
}

func newHandler(q *fakeQueries) *contacts.Handler {
	return &contacts.Handler{
		Queries: q,
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func uuidText(u pgtype.UUID) string {
	v, _ := u.Value()
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func idBytes(b byte) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte{b, b}, Valid: true}
}

// --- CreateContact ---------------------------------------------------

func TestCreateContact_MinimalSuccess(t *testing.T) {
	companyID := idBytes(0xC0)
	q := &fakeQueries{}
	h := newHandler(q)

	resp, err := h.CreateContact(context.Background(), connect.NewRequest(&contactsv1.CreateContactRequest{
		CompanyId: uuidText(companyID),
		FullName:  "Pedro Alves",
	}))
	if err != nil {
		t.Fatalf("CreateContact: %v", err)
	}
	if resp.Msg.Contact.FullName != "Pedro Alves" {
		t.Errorf("full_name = %q", resp.Msg.Contact.FullName)
	}
	if q.createArg.IdentitySource != "manual" {
		t.Errorf("identity_source = %q; want manual", q.createArg.IdentitySource)
	}
	if q.createArg.Email.Valid {
		t.Errorf("email should be NULL when empty: %+v", q.createArg.Email)
	}
}

func TestCreateContact_RejectsEmptyFullName(t *testing.T) {
	q := &fakeQueries{}
	h := newHandler(q)

	_, err := h.CreateContact(context.Background(), connect.NewRequest(&contactsv1.CreateContactRequest{
		CompanyId: uuidText(idBytes(0xC0)),
	}))
	if err == nil {
		t.Fatal("expected InvalidArgument")
	}
}

func TestCreateContact_DuplicateEmailReturnsAlreadyExists(t *testing.T) {
	q := &fakeQueries{existsResp: true}
	h := newHandler(q)

	_, err := h.CreateContact(context.Background(), connect.NewRequest(&contactsv1.CreateContactRequest{
		CompanyId: uuidText(idBytes(0xC0)),
		FullName:  "Pedro",
		Email:     "pedro@acme.com",
	}))
	if err == nil {
		t.Fatal("expected AlreadyExists")
	}
	var cerr *connect.Error
	if errors.As(err, &cerr) && cerr.Code() != connect.CodeAlreadyExists {
		t.Errorf("code = %v; want AlreadyExists", cerr.Code())
	}
}

// --- UpdateContact ---------------------------------------------------

func TestUpdateContact_PassesFieldsThrough(t *testing.T) {
	cid := idBytes(0xAA)
	q := &fakeQueries{updateRow: sqlc.Contact{ID: cid, FullName: "Ana Costa"}}
	h := newHandler(q)

	_, err := h.UpdateContact(context.Background(), connect.NewRequest(&contactsv1.UpdateContactRequest{
		Id:       uuidText(cid),
		FullName: "Ana Costa",
		JobTitle: "CFO",
		Email:    "ana@acme.com",
	}))
	if err != nil {
		t.Fatalf("UpdateContact: %v", err)
	}
	if q.updateArg.FullName != "Ana Costa" {
		t.Errorf("full_name = %q", q.updateArg.FullName)
	}
	if !q.updateArg.JobTitle.Valid || q.updateArg.JobTitle.String != "CFO" {
		t.Errorf("job_title = %+v", q.updateArg.JobTitle)
	}
}

func TestUpdateContact_NotFoundReturns404(t *testing.T) {
	q := &fakeQueries{updateErr: pgx.ErrNoRows}
	h := newHandler(q)

	_, err := h.UpdateContact(context.Background(), connect.NewRequest(&contactsv1.UpdateContactRequest{
		Id:       uuidText(idBytes(0xEE)),
		FullName: "X",
	}))
	if err == nil {
		t.Fatal("expected NotFound")
	}
	var cerr *connect.Error
	if errors.As(err, &cerr) && cerr.Code() != connect.CodeNotFound {
		t.Errorf("code = %v; want NotFound", cerr.Code())
	}
}

// --- ArchiveContact --------------------------------------------------

func TestArchiveContact_RejectsTeamMemberContact(t *testing.T) {
	cid := idBytes(0xA1)
	q := &fakeQueries{hasGrantResp: true}
	h := newHandler(q)

	_, err := h.ArchiveContact(context.Background(), connect.NewRequest(&contactsv1.ArchiveContactRequest{
		Id: uuidText(cid),
	}))
	if err == nil {
		t.Fatal("expected FailedPrecondition")
	}
	if len(q.archiveCalls) != 0 {
		t.Error("ArchiveContact must not fire for a team-member contact")
	}
}

func TestArchiveContact_NonTeamMemberSucceeds(t *testing.T) {
	cid := idBytes(0xA2)
	q := &fakeQueries{}
	h := newHandler(q)

	_, err := h.ArchiveContact(context.Background(), connect.NewRequest(&contactsv1.ArchiveContactRequest{
		Id: uuidText(cid),
	}))
	if err != nil {
		t.Fatalf("ArchiveContact: %v", err)
	}
	if len(q.archiveCalls) != 1 {
		t.Fatalf("ArchiveContact calls = %d; want 1", len(q.archiveCalls))
	}
}

// --- ListContacts ----------------------------------------------------

func TestListContacts_ReturnsActiveRows(t *testing.T) {
	q := &fakeQueries{listRows: []sqlc.Contact{
		{ID: idBytes(0x01), FullName: "A"},
		{ID: idBytes(0x02), FullName: "B"},
	}}
	h := newHandler(q)

	resp, err := h.ListContacts(context.Background(), connect.NewRequest(&contactsv1.ListContactsRequest{
		CompanyId: uuidText(idBytes(0xC0)),
	}))
	if err != nil {
		t.Fatalf("ListContacts: %v", err)
	}
	if got := len(resp.Msg.Contacts); got != 2 {
		t.Fatalf("contacts = %d; want 2", got)
	}
}
