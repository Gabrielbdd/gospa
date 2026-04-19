// Package publicconfig wraps the generated PublicConfigHandler with a
// runtime mutator that injects workspace-scoped values into the public
// config served to the browser — specifically the ZITADEL organization
// ID the SPA needs to build a org-scoped OIDC login URL.
package publicconfig

import (
	"context"
	"net/http"

	runtimeconfig "github.com/Gabrielbdd/gofra/runtime/config"

	"github.com/Gabrielbdd/gospa/config"
)

// WorkspaceOrgIDProvider returns the current MSP ZITADEL organization ID
// or the empty string when the workspace is not yet installed.
type WorkspaceOrgIDProvider interface {
	WorkspaceOrgID(ctx context.Context) (string, error)
}

// Handler returns an HTTP handler that serves the public config with
// auth.org_id populated from the provider. Before install completes, the
// provider returns "" and the field is left empty — the SPA interprets
// an empty org_id as "no org-scoped login possible yet".
func Handler(cfg *config.Config, provider WorkspaceOrgIDProvider) http.Handler {
	mutator := runtimeconfig.WithMutator(func(ctx context.Context, _ *http.Request, out *config.PublicConfig) error {
		orgID, err := provider.WorkspaceOrgID(ctx)
		if err != nil {
			// Surfacing the error would break /_gofra/config.js for
			// every request while ZITADEL or DB hiccup; log is enough
			// once observability lands. For now, swallow so the SPA
			// always renders, even if pre-install.
			return nil
		}
		out.Auth.OrgID = orgID
		return nil
	})

	return config.PublicConfigHandler(cfg, mutator)
}
