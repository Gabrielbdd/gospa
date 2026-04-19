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
	orgID string
	err   error
	calls int
}

func (p *fakeProvider) WorkspaceOrgID(_ context.Context) (string, error) {
	p.calls++
	return p.orgID, p.err
}

// fetchJS hits the handler and returns the response body. The runtime
// config handler serves JavaScript that assigns window.__GOFRA_CONFIG__,
// so we grep the body for the expected orgId value.
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

	provider := &fakeProvider{orgID: "org-xyz-123"}
	h := publicconfig.Handler(newConfig(), provider)

	body := fetchJS(t, h)

	if !strings.Contains(body, `"orgId":"org-xyz-123"`) {
		t.Errorf("emitted config.js missing orgId; body:\n%s", body)
	}
	if provider.calls == 0 {
		t.Error("expected provider to be called at least once per request")
	}
}

func TestHandler_EmptyOrgIDWhenWorkspaceNotReady(t *testing.T) {
	t.Parallel()

	provider := &fakeProvider{orgID: ""}
	h := publicconfig.Handler(newConfig(), provider)

	body := fetchJS(t, h)

	if !strings.Contains(body, `"orgId":""`) {
		t.Errorf("expected empty orgId in emitted config.js; body:\n%s", body)
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

	provider := &fakeProvider{orgID: "org-1"}
	h := publicconfig.Handler(newConfig(), provider)

	_ = fetchJS(t, h)
	_ = fetchJS(t, h)
	_ = fetchJS(t, h)

	if provider.calls < 3 {
		t.Errorf("expected provider called at least 3 times across 3 requests; got %d", provider.calls)
	}

	// Emulate a workspace that just transitioned from not-ready to
	// ready between requests: the next request must see the new org.
	provider.orgID = "org-post-install"
	body := fetchJS(t, h)
	if !strings.Contains(body, `"orgId":"org-post-install"`) {
		t.Errorf("expected next-request to carry newly-populated orgId; body:\n%s", body)
	}
}
