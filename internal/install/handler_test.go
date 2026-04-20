package install_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Gabrielbdd/gospa/db/sqlc"
	installv1 "github.com/Gabrielbdd/gospa/gen/gospa/install/v1"
	"github.com/Gabrielbdd/gospa/internal/install"
)

// stubQueries is the minimum surface Install() touches before the
// orchestrator goroutine spawns. It only needs to return a workspace
// row in the not_initialized state.
type stubQueries struct {
	install.Queries
}

func (stubQueries) GetWorkspace(_ context.Context) (sqlc.Workspace, error) {
	return sqlc.Workspace{InstallState: sqlc.WorkspaceInstallStateNotInitialized}, nil
}

func (stubQueries) MarkWorkspaceProvisioning(_ context.Context, _ sqlc.MarkWorkspaceProvisioningParams) error {
	return nil
}

func (stubQueries) MarkWorkspaceFailed(_ context.Context, _ pgtype.Text) error {
	return nil
}

func (stubQueries) MarkWorkspaceReady(_ context.Context) error {
	return nil
}

func (stubQueries) PersistZitadelIDs(_ context.Context, _ sqlc.PersistZitadelIDsParams) error {
	return nil
}

type stubOrchestrator struct{}

func (stubOrchestrator) Run(_ context.Context, _ install.Input) error { return nil }

func newHandler(token string) *install.Handler {
	return &install.Handler{
		Queries:      stubQueries{},
		Orchestrator: stubOrchestrator{},
		Logger:       slog.Default(),
		APIBaseURL:   "http://localhost:3000",
		InstallToken: token,
	}
}

func validRequest() *connect.Request[installv1.InstallRequest] {
	return connect.NewRequest(&installv1.InstallRequest{
		WorkspaceName: "Acme",
		Timezone:      "UTC",
		CurrencyCode:  "USD",
		InitialUser: &installv1.InitialUser{
			Email:      "admin@acme.test",
			GivenName:  "Ada",
			FamilyName: "Lovelace",
			Password:   "correct-horse-battery-staple",
		},
	})
}

func TestInstall_RejectsRequestWithoutToken(t *testing.T) {
	h := newHandler("expected-token")
	req := validRequest()

	_, err := h.Install(t.Context(), req)
	if err == nil {
		t.Fatal("expected Unauthenticated error when X-Install-Token is missing")
	}
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) || connectErr.Code() != connect.CodeUnauthenticated {
		t.Errorf("got %v; want connect.CodeUnauthenticated", err)
	}
}

func TestInstall_RejectsRequestWithWrongToken(t *testing.T) {
	h := newHandler("expected-token")
	req := validRequest()
	req.Header().Set(install.InstallTokenHeader, "wrong-token")

	_, err := h.Install(t.Context(), req)
	if err == nil {
		t.Fatal("expected Unauthenticated error for wrong token")
	}
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) || connectErr.Code() != connect.CodeUnauthenticated {
		t.Errorf("got %v; want connect.CodeUnauthenticated", err)
	}
}

func TestInstall_AcceptsRequestWithMatchingToken(t *testing.T) {
	h := newHandler("expected-token")
	req := validRequest()
	req.Header().Set(install.InstallTokenHeader, "expected-token")

	resp, err := h.Install(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Msg.State != installv1.InstallState_INSTALL_STATE_PROVISIONING {
		t.Errorf("state = %v; want PROVISIONING", resp.Msg.State)
	}
}

func TestGetStatus_DoesNotRequireToken(t *testing.T) {
	h := newHandler("expected-token")
	req := connect.NewRequest(&installv1.GetStatusRequest{})

	resp, err := h.GetStatus(t.Context(), req)
	if err != nil {
		t.Fatalf("GetStatus rejected unauthenticated request: %v", err)
	}
	if resp.Msg.State != installv1.InstallState_INSTALL_STATE_NOT_INITIALIZED {
		t.Errorf("state = %v; want NOT_INITIALIZED", resp.Msg.State)
	}
}
