package authgate_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	runtimeauth "github.com/Gabrielbdd/gofra/runtime/auth"

	"github.com/Gabrielbdd/gospa/internal/authgate"
)

const (
	// Connect procedure paths used across the tests. The install
	// service is the canonical "public" Connect procedure (the gate's
	// PublicProcedures matcher passes it through pre-install); the
	// companies service is the canonical "private" app procedure that
	// must be blocked pre-install and JWT-guarded post-install.
	publicProc  = "/gospa.install.v1.InstallService/GetStatus"
	privateProc = "/gospa.companies.v1.CompaniesService/ListCompanies"
)

// --- inactive-gate behavior --------------------------------------------------

func TestGate_InactivePassesNonConnectPath(t *testing.T) {
	gate := authgate.New(runtimeauth.PublicProcedures())

	var invoked bool
	mw := gate.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		invoked = true
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/_gofra/config.js", nil))

	if !invoked {
		t.Error("non-Connect path should pass through to next handler")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("non-Connect path got %d; want 200", rec.Code)
	}
	if gate.IsActive() {
		t.Error("gate should be inactive before Activate is called")
	}
}

func TestGate_InactivePassesPublicConnectProcedure(t *testing.T) {
	gate := authgate.New(runtimeauth.PublicProcedures(publicProc))

	var invoked bool
	mw := gate.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		invoked = true
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, publicProc, nil))

	if !invoked {
		t.Errorf("public Connect procedure should pass through pre-install; got status %d", rec.Code)
	}
}

func TestGate_InactiveBlocksPrivateConnectProcedure(t *testing.T) {
	gate := authgate.New(runtimeauth.PublicProcedures(publicProc))

	var invoked bool
	mw := gate.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		invoked = true
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, privateProc, nil))

	if invoked {
		t.Fatal("private Connect procedure must not reach the handler before install")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want 401", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q; want application/json", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"code":"unauthenticated"`) {
		t.Errorf("body = %s; want Connect-shape unauthenticated error", body)
	}
	if !strings.Contains(body, "complete /install") {
		t.Errorf("body = %s; want hint pointing operator at /install", body)
	}
}

// --- Activate guards --------------------------------------------------------

func TestGate_ActivateRejectsEmptyIssuerOrAudience(t *testing.T) {
	gate := authgate.New(runtimeauth.PublicProcedures())

	if err := gate.Activate(t.Context(), "", "aud"); err == nil {
		t.Error("expected error when issuer is empty")
	}
	if err := gate.Activate(t.Context(), "http://issuer.example.com", ""); err == nil {
		t.Error("expected error when audience is empty")
	}
	if gate.IsActive() {
		t.Error("gate should not be active after rejected Activate calls")
	}
}

// --- mount-order coverage ---------------------------------------------------

// TestGate_ActivateBeforeMiddlewareInvocationStillWorks is the
// regression test for chi's lazy middleware: cmd/app calls Activate
// after app.Use(gate.Middleware) but chi only invokes Middleware on
// the first request. Activate must therefore work without the wrapped
// next handler being supplied yet — the lazily-built authenticated
// chain picks up the verifier on the first request that flows through.
func TestGate_ActivateBeforeMiddlewareInvocationStillWorks(t *testing.T) {
	srv := newOIDCDiscoveryStub(t)
	defer srv.Close()

	gate := authgate.New(runtimeauth.PublicProcedures(publicProc))

	if err := gate.Activate(t.Context(), srv.URL, "test-audience"); err != nil {
		t.Fatalf("Activate before any Middleware invocation failed: %v", err)
	}
	if !gate.IsActive() {
		t.Fatal("gate should be active after successful Activate")
	}

	router := chi.NewRouter()
	router.Use(gate.Middleware)
	router.HandleFunc(privateProc, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, privateProc, nil))

	// Without a Bearer the authenticated chain rejects (401). The
	// only thing that would flip rec.Code to 200 is the gate having
	// failed to install the verifier — which is the bug we are
	// guarding against here.
	if rec.Code == http.StatusOK {
		t.Errorf("expected protected route to be guarded; got 200 — gate did not pick up the pre-Mount Activate")
	}
}

// TestGate_ActivateAfterMountStillWorks covers the inverse ordering:
// chi has already served pre-Activate requests when Activate finally
// runs. Pre-Activate, a non-Connect path passes through; a private
// Connect path is blocked by the inactive-gate fail-closed branch.
// Post-Activate, the private Connect path is now guarded by the JWT
// chain instead.
func TestGate_ActivateAfterMountStillWorks(t *testing.T) {
	srv := newOIDCDiscoveryStub(t)
	defer srv.Close()

	gate := authgate.New(runtimeauth.PublicProcedures(publicProc))

	router := chi.NewRouter()
	router.Use(gate.Middleware)
	router.HandleFunc("/_gofra/config.js", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	router.HandleFunc(privateProc, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Pre-Activate, non-Connect path: passes through.
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/_gofra/config.js", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("pre-Activate non-Connect request blocked: code %d", rec.Code)
	}

	// Pre-Activate, private Connect path: blocked by inactive-gate
	// fail-closed branch (the new behavior in this slice).
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, privateProc, nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("pre-Activate private Connect request: code %d; want 401", rec.Code)
	}

	if err := gate.Activate(t.Context(), srv.URL, "test-audience"); err != nil {
		t.Fatalf("Activate failed: %v", err)
	}

	// Post-Activate, private Connect path: guarded by JWT middleware
	// (still 401 — no bearer — but the rejection is now from the
	// active chain, not the inactive fail-closed branch).
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, privateProc, nil))
	if rec.Code == http.StatusOK {
		t.Errorf("post-Activate private request not guarded; code %d", rec.Code)
	}
}

// --- helpers ----------------------------------------------------------------

// newOIDCDiscoveryStub returns an httptest.Server that serves the minimal
// OIDC discovery document required by runtimeauth.NewJWTVerifier. The JWKS
// endpoint returns an empty key set; that is enough for verifier
// construction (RemoteKeySet fetches lazily on the first token verification,
// which these tests do not exercise — they only check the gating decision).
func newOIDCDiscoveryStub(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":   srv.URL,
			"jwks_uri": srv.URL + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys":[]}`))
	})

	return srv
}
