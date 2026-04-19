package publicconfig_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Gabrielbdd/gospa/config"
	"github.com/Gabrielbdd/gospa/internal/publicconfig"
)

type fakeProvider struct {
	auth  publicconfig.WorkspaceAuth
	err   error
	calls int
}

func (p *fakeProvider) WorkspaceAuth(_ context.Context) (publicconfig.WorkspaceAuth, error) {
	p.calls++
	return p.auth, p.err
}

// fetchJS hits the handler and returns the response body. The runtime
// config handler serves JavaScript that assigns window.__GOFRA_CONFIG__,
// so we grep the body for the expected field values.
func fetchJS(t *testing.T, h http.Handler) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/_gofra/config.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		body, _ := io.ReadAll(rec.Body)
		t.Fatalf("handler returned %d: %s", rec.Code, body)
	}
	return rec.Body.String()
}

func newConfig() *config.Config {
	return config.DefaultConfig()
}

func TestHandler_InjectsOrgIDWhenWorkspaceProvisioned(t *testing.T) {
	t.Parallel()

	provider := &fakeProvider{auth: publicconfig.WorkspaceAuth{OrgID: "org-xyz-123"}}
	h := publicconfig.Handler(newConfig(), provider)

	body := fetchJS(t, h)

	if !strings.Contains(body, `"orgId":"org-xyz-123"`) {
		t.Errorf("emitted config.js missing orgId; body:\n%s", body)
	}
	if provider.calls == 0 {
		t.Error("expected provider to be called at least once per request")
	}
}

func TestHandler_InjectsClientIDFromWorkspace(t *testing.T) {
	t.Parallel()

	provider := &fakeProvider{auth: publicconfig.WorkspaceAuth{
		OrgID:    "org-xyz-123",
		ClientID: "369303173921243907@gospa",
	}}
	h := publicconfig.Handler(newConfig(), provider)

	body := fetchJS(t, h)

	if !strings.Contains(body, `"clientId":"369303173921243907@gospa"`) {
		t.Errorf("emitted config.js did not override clientId with persisted value; body:\n%s", body)
	}
}

func TestHandler_InjectsIssuerFromWorkspace(t *testing.T) {
	t.Parallel()

	provider := &fakeProvider{auth: publicconfig.WorkspaceAuth{
		OrgID:  "org-xyz-123",
		Issuer: "https://zitadel.example.com",
	}}
	h := publicconfig.Handler(newConfig(), provider)

	body := fetchJS(t, h)

	if !strings.Contains(body, `"issuer":"https://zitadel.example.com"`) {
		t.Errorf("emitted config.js did not override issuer with persisted value; body:\n%s", body)
	}
}

func TestHandler_KeepsStaticClientIDWhenWorkspaceNotInstalled(t *testing.T) {
	t.Parallel()

	provider := &fakeProvider{auth: publicconfig.WorkspaceAuth{}}
	h := publicconfig.Handler(newConfig(), provider)

	body := fetchJS(t, h)

	// Pre-install the workspace has no persisted client_id, so the
	// handler should leave the static placeholder in place. The SPA
	// disables the login button when orgId is empty, so the wrong
	// client_id never reaches ZITADEL in this state.
	if strings.Contains(body, `"clientId":""`) {
		t.Errorf("static client_id was unexpectedly cleared; body:\n%s", body)
	}
	if !strings.Contains(body, `"orgId":""`) {
		t.Errorf("expected empty orgId pre-install; body:\n%s", body)
	}
}

func TestHandler_AppendsOrgAndAudienceScopesWhenDerived(t *testing.T) {
	t.Parallel()

	provider := &fakeProvider{auth: publicconfig.WorkspaceAuth{
		OrgID:         "org-42",
		OrgScope:      "urn:zitadel:iam:org:id:org-42",
		AudienceScope: "urn:zitadel:iam:org:project:id:project-7:aud",
	}}
	h := publicconfig.Handler(newConfig(), provider)

	body := fetchJS(t, h)

	if !strings.Contains(body, "urn:zitadel:iam:org:id:org-42") {
		t.Errorf("emitted config.js missing org scope; body:\n%s", body)
	}
	if !strings.Contains(body, "urn:zitadel:iam:org:project:id:project-7:aud") {
		t.Errorf("emitted config.js missing audience scope; body:\n%s", body)
	}
}

func TestHandler_LeavesScopesUntouchedWhenWorkspaceNotInstalled(t *testing.T) {
	t.Parallel()

	provider := &fakeProvider{auth: publicconfig.WorkspaceAuth{}}
	h := publicconfig.Handler(newConfig(), provider)

	body := fetchJS(t, h)

	// Pre-install: no derived scopes appended; the static defaults
	// (openid/profile/email) stay alone. The SPA disables login via
	// empty orgId so the partial scope set never reaches ZITADEL.
	if strings.Contains(body, "urn:zitadel:iam:org:id:") {
		t.Errorf("org scope unexpectedly emitted pre-install; body:\n%s", body)
	}
	if strings.Contains(body, "urn:zitadel:iam:org:project:id:") {
		t.Errorf("audience scope unexpectedly emitted pre-install; body:\n%s", body)
	}
}

func TestHandler_KeepsStaticIssuerWhenWorkspaceNotInstalled(t *testing.T) {
	t.Parallel()

	provider := &fakeProvider{auth: publicconfig.WorkspaceAuth{}}
	h := publicconfig.Handler(newConfig(), provider)

	body := fetchJS(t, h)

	// Pre-install the workspace has no persisted issuer, so the
	// handler should leave the static placeholder in place. The SPA
	// already disables the login button via empty orgId, so the
	// (potentially wrong) issuer never reaches ZITADEL in this state.
	if strings.Contains(body, `"issuer":""`) {
		t.Errorf("static issuer was unexpectedly cleared; body:\n%s", body)
	}
}

func TestHandler_SwallowsProviderErrorAndServesEmptyOrgID(t *testing.T) {
	t.Parallel()

	provider := &fakeProvider{err: errors.New("db unreachable")}
	h := publicconfig.Handler(newConfig(), provider)

	body := fetchJS(t, h)

	// A DB hiccup must not break /_gofra/config.js for the whole SPA.
	// The config still renders with an empty orgId; the SPA handles
	// that by disabling the login button.
	if !strings.Contains(body, `"orgId":""`) {
		t.Errorf("expected empty orgId when provider errors; body:\n%s", body)
	}
}

func TestHandler_CallsProviderOnEveryRequest(t *testing.T) {
	t.Parallel()

	provider := &fakeProvider{auth: publicconfig.WorkspaceAuth{OrgID: "org-1"}}
	h := publicconfig.Handler(newConfig(), provider)

	_ = fetchJS(t, h)
	_ = fetchJS(t, h)
	_ = fetchJS(t, h)

	if provider.calls < 3 {
		t.Errorf("expected provider called at least 3 times across 3 requests; got %d", provider.calls)
	}

	// Emulate a workspace that just transitioned from not-ready to
	// ready between requests: the next request must see the new org
	// AND the freshly-persisted client id and issuer.
	provider.auth = publicconfig.WorkspaceAuth{
		OrgID:    "org-post-install",
		ClientID: "post-install-client@gospa",
		Issuer:   "https://zitadel-post-install.example.com",
	}
	body := fetchJS(t, h)
	if !strings.Contains(body, `"orgId":"org-post-install"`) {
		t.Errorf("expected next-request to carry newly-populated orgId; body:\n%s", body)
	}
	if !strings.Contains(body, `"clientId":"post-install-client@gospa"`) {
		t.Errorf("expected next-request to carry newly-populated clientId; body:\n%s", body)
	}
	if !strings.Contains(body, `"issuer":"https://zitadel-post-install.example.com"`) {
		t.Errorf("expected next-request to carry newly-populated issuer; body:\n%s", body)
	}
}
