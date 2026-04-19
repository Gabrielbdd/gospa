package authgate_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	runtimeauth "github.com/Gabrielbdd/gofra/runtime/auth"

	"github.com/Gabrielbdd/gospa/internal/authgate"
)

func TestGate_PassesThroughBeforeActivate(t *testing.T) {
	gate := authgate.New(runtimeauth.PublicProcedures())

	var invoked bool
	mw := gate.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		invoked = true
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/gospa.companies.v1.CompaniesService/ListCompanies", nil)
	mw.ServeHTTP(rec, req)

	if !invoked {
		t.Error("expected pass-through to invoke next handler")
	}
	if gate.IsActive() {
		t.Error("gate should be inactive before Activate is called")
	}
}

func TestGate_ActivateBeforeMountReturnsError(t *testing.T) {
	gate := authgate.New(runtimeauth.PublicProcedures())

	err := gate.Activate(t.Context(), "http://unused.local", "some-audience")
	if err == nil {
		t.Fatal("expected error when activating before Middleware has been mounted")
	}
	if !errors.Is(err, authgate.ErrMiddlewareNotMounted) {
		t.Errorf("expected ErrMiddlewareNotMounted, got %v", err)
	}
	if gate.IsActive() {
		t.Error("gate should not be active after a failed Activate")
	}
}

func TestGate_ActivateRejectsEmptyIssuerOrAudience(t *testing.T) {
	gate := authgate.New(runtimeauth.PublicProcedures())
	// Mount middleware so the rejection is for empty args, not the
	// not-mounted guard.
	_ = gate.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

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

// TestGate_ActivateAfterMountSucceeds is the regression test for the
// post-restart eager-activation bug: when the workspace was already
// installed and cmd/app calls gate.Activate at startup, the call must run
// after app.Use(gate.Middleware) so the gate's passthrough is wired.
// Previously cmd/app/main.go invoked Activate before mounting the
// middleware and the failure was only logged at warn level, leaving
// private RPCs unauthenticated after a Pod restart.
func TestGate_ActivateAfterMountSucceeds(t *testing.T) {
	srv := newOIDCDiscoveryStub(t)
	defer srv.Close()

	gate := authgate.New(runtimeauth.PublicProcedures())

	// Mount middleware first — this is what cmd/app/main.go must do
	// before calling Activate.
	_ = gate.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	if err := gate.Activate(t.Context(), srv.URL, "test-audience"); err != nil {
		t.Fatalf("Activate failed after Middleware mount: %v", err)
	}
	if !gate.IsActive() {
		t.Error("gate should be active after successful Activate")
	}
}

// newOIDCDiscoveryStub returns an httptest.Server that serves the minimal
// OIDC discovery document required by runtimeauth.NewJWTVerifier. The JWKS
// endpoint returns an empty key set; that is enough for verifier
// construction (RemoteKeySet fetches lazily on the first token verification,
// which this test does not exercise).
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
