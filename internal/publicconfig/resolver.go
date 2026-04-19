// Package publicconfig wraps the generated PublicConfigHandler with a
// runtime mutator that injects workspace-scoped values into the public
// config served to the browser — specifically the ZITADEL organization
// ID the SPA needs to build an org-scoped OIDC login URL.
//
// Contract with the SPA: every request to /_gofra/config.js re-runs the
// mutator, so the browser sees workspace.zitadel_org_id as soon as
// install completes AND the page is reloaded. Install completion is a
// page-lifetime boundary — the install wizard triggers a full reload
// on ready (window.location.assign), which re-executes the config
// script and picks up the freshly-populated org id. SPA navigation
// alone is not sufficient because /_gofra/config.js is a <script> tag
// evaluated at page boot.
package publicconfig

import (
	"context"
	"log/slog"
	"net/http"

	runtimeconfig "github.com/Gabrielbdd/gofra/runtime/config"

	"github.com/Gabrielbdd/gospa/config"
)

// WorkspaceOrgIDProvider returns the current MSP ZITADEL organization
// ID or the empty string when the workspace is not yet installed. The
// second return is an optional error for transport-level failures
// (DB unreachable, etc.) that the caller can choose to log; an empty
// string is always valid and results in the SPA showing "login
// disabled" rather than rendering a broken authorize URL.
type WorkspaceOrgIDProvider interface {
	WorkspaceOrgID(ctx context.Context) (string, error)
}

// Handler returns an HTTP handler that serves the public config with
// auth.org_id populated from the provider. Pre-install the field is
// empty; the SPA treats that as "no org-scoped login possible yet".
//
// DB errors reading the workspace row are logged and swallowed — a
// hiccup in the DB must not block /_gofra/config.js and cascade into
// a broken SPA render. The worst-case payload carries an empty orgId,
// which the SPA already handles by disabling the login button.
func Handler(cfg *config.Config, provider WorkspaceOrgIDProvider) http.Handler {
	mutator := runtimeconfig.WithMutator(func(ctx context.Context, _ *http.Request, out *config.PublicConfig) error {
		orgID, err := provider.WorkspaceOrgID(ctx)
		if err != nil {
			slog.WarnContext(ctx,
				"publicconfig: workspace provider failed; serving /_gofra/config.js with empty auth.orgId",
				"error", err)
			return nil
		}
		out.Auth.OrgID = orgID
		return nil
	})

	return config.PublicConfigHandler(cfg, mutator)
}
