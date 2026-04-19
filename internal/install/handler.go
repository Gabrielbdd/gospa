package install

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"connectrpc.com/connect"

	installv1 "github.com/Gabrielbdd/gospa/gen/gospa/install/v1"
	"github.com/Gabrielbdd/gospa/gen/gospa/install/v1/installv1connect"
)

// Handler implements the InstallService Connect RPCs.
type Handler struct {
	installv1connect.UnimplementedInstallServiceHandler

	Queries      Queries
	Orchestrator *Orchestrator
	Logger       *slog.Logger

	// APIBaseURL is forwarded into the orchestrator Input so OIDC app
	// redirect URIs resolve against the same base the SPA uses.
	APIBaseURL string

	mu       sync.Mutex
	inflight bool
}

// GetStatus returns the current workspace install state. Callers use this
// to render /install, redirect to /, or poll for /install progress.
func (h *Handler) GetStatus(ctx context.Context, _ *connect.Request[installv1.GetStatusRequest]) (*connect.Response[installv1.GetStatusResponse], error) {
	ws, err := h.Queries.GetWorkspace(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &installv1.GetStatusResponse{
		State: toProtoState(string(ws.InstallState)),
	}
	if ws.InstallError.Valid {
		resp.InstallError = ws.InstallError.String
	}
	if ws.ZitadelOrgID.Valid {
		resp.ZitadelOrgId = ws.ZitadelOrgID.String
	}
	return connect.NewResponse(resp), nil
}

// Install accepts the wizard submission, flips the workspace to
// provisioning, and spawns the orchestrator goroutine. It returns
// immediately; the client polls GetStatus.
func (h *Handler) Install(ctx context.Context, req *connect.Request[installv1.InstallRequest]) (*connect.Response[installv1.InstallResponse], error) {
	if err := validateInstallRequest(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	ws, err := h.Queries.GetWorkspace(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if string(ws.InstallState) != "not_initialized" && string(ws.InstallState) != "failed" {
		return nil, connect.NewError(
			connect.CodeFailedPrecondition,
			errors.New("workspace is not eligible for install: "+string(ws.InstallState)),
		)
	}

	// Single-flight guard: one install at a time per process.
	h.mu.Lock()
	if h.inflight {
		h.mu.Unlock()
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("install already in progress"))
	}
	h.inflight = true
	h.mu.Unlock()

	input := Input{
		WorkspaceName: req.Msg.WorkspaceName,
		WorkspaceSlug: req.Msg.WorkspaceSlug,
		Timezone:      req.Msg.Timezone,
		CurrencyCode:  req.Msg.CurrencyCode,
		AdminEmail:    req.Msg.InitialUser.GetEmail(),
		AdminFirst:    req.Msg.InitialUser.GetGivenName(),
		AdminLast:     req.Msg.InitialUser.GetFamilyName(),
		APIBaseURL:    h.APIBaseURL,
	}

	if err := h.Queries.MarkWorkspaceProvisioning(ctx, mapProvisioningParams(input)); err != nil {
		h.mu.Lock()
		h.inflight = false
		h.mu.Unlock()
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	go func() {
		defer func() {
			h.mu.Lock()
			h.inflight = false
			h.mu.Unlock()
		}()
		// Run uses background context so the orchestrator keeps going
		// after the RPC response is sent.
		if err := h.Orchestrator.Run(context.Background(), input); err != nil {
			h.Logger.Error("install orchestrator failed", "error", err)
		}
	}()

	return connect.NewResponse(&installv1.InstallResponse{
		State: installv1.InstallState_INSTALL_STATE_PROVISIONING,
	}), nil
}

func validateInstallRequest(r *installv1.InstallRequest) error {
	if r == nil {
		return errors.New("empty request")
	}
	if r.WorkspaceName == "" {
		return errors.New("workspace_name is required")
	}
	if r.WorkspaceSlug == "" {
		return errors.New("workspace_slug is required")
	}
	if r.InitialUser == nil || r.InitialUser.Email == "" {
		return errors.New("initial_user.email is required")
	}
	return nil
}

func toProtoState(s string) installv1.InstallState {
	switch s {
	case "not_initialized":
		return installv1.InstallState_INSTALL_STATE_NOT_INITIALIZED
	case "provisioning":
		return installv1.InstallState_INSTALL_STATE_PROVISIONING
	case "ready":
		return installv1.InstallState_INSTALL_STATE_READY
	case "failed":
		return installv1.InstallState_INSTALL_STATE_FAILED
	default:
		return installv1.InstallState_INSTALL_STATE_UNSPECIFIED
	}
}
