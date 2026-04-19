// Package publicconfig wraps the generated PublicConfigHandler with a
// runtime mutator that injects workspace-scoped values into the public
// config served to the browser — specifically the ZITADEL organization
// ID and the SPA OIDC client ID the SPA needs to build a working
// org-scoped login URL.
//
// Contract with the SPA: every request to /_gofra/config.js re-runs the
// mutator, so the browser sees workspace.zitadel_org_id and
// workspace.zitadel_spa_client_id as soon as install completes AND the
// page is reloaded. Install completion is a page-lifetime boundary —
// the install wizard triggers a full reload on ready
// (window.location.assign), which re-executes the config script and
// picks up the freshly-populated values. SPA navigation alone is not
// sufficient because /_gofra/config.js is a <script> tag evaluated at
// page boot.
package publicconfig

import (
	"context"
	"log/slog"
	"net/http"

	runtimeconfig "github.com/Gabrielbdd/gofra/runtime/config"

	"github.com/Gabrielbdd/gospa/config"
)

// WorkspaceAuth is the auth-relevant subset of the workspace row the
// browser config depends on. Empty strings are valid (pre-install or
// when the workspace was wiped) and result in the SPA showing
// "login disabled" rather than rendering a broken authorize URL.
type WorkspaceAuth struct {
	OrgID    string
	ClientID string
	Issuer   string
}

// WorkspaceAuthProvider returns the current MSP auth identifiers from
// the workspace singleton. The error return is for transport-level
// failures (DB unreachable); a zero-value struct is always valid.
type WorkspaceAuthProvider interface {
	WorkspaceAuth(ctx context.Context) (WorkspaceAuth, error)
}

// Handler returns an HTTP handler that serves the public config with
// auth.org_id, auth.client_id, and auth.issuer populated from the
// provider. Pre-install all three fields are empty; the SPA treats
// that as "no org-scoped login possible yet".
//
// DB errors reading the workspace row are logged and swallowed — a
// hiccup in the DB must not block /_gofra/config.js and cascade into
// a broken SPA render. The worst-case payload carries empty values,
// which the SPA already handles by disabling the login button.
//
// Note: the static `client_id` from gofra.yaml is a placeholder
// (typically "gospa-web") that ZITADEL never registers under that
// name. The wizard-time persisted `zitadel_spa_client_id` is the
// real one — overriding it here is the only thing that makes the
// browser's authorize URL match a real ZITADEL application. The
// persisted `zitadel_issuer_url` follows the same pattern: ADR 0001
// makes it explicit persisted state instead of static config, so
// deploys that route the browser to a different ZITADEL host than
// the cluster sees can override per workspace without touching the
// app config.
func Handler(cfg *config.Config, provider WorkspaceAuthProvider) http.Handler {
	mutator := runtimeconfig.WithMutator(func(ctx context.Context, _ *http.Request, out *config.PublicConfig) error {
		auth, err := provider.WorkspaceAuth(ctx)
		if err != nil {
			slog.WarnContext(ctx,
				"publicconfig: workspace provider failed; serving /_gofra/config.js with placeholder auth fields",
				"error", err)
			return nil
		}
		out.Auth.OrgID = auth.OrgID
		if auth.ClientID != "" {
			out.Auth.ClientID = auth.ClientID
		}
		if auth.Issuer != "" {
			out.Auth.Issuer = auth.Issuer
		}
		return nil
	})

	return config.PublicConfigHandler(cfg, mutator)
}
