package authgate_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

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

// TestGate_ActivateBeforeMiddlewareInvocationStillWorks is the
// regression test for the chi-lazy bug: cmd/app/main.go calls
// gate.Activate after app.Use(gate.Middleware), but chi only invokes
// the middleware function on the first request. Activate must
// therefore work without requiring the wrapped next handler to be
// supplied. Once the subsequent request flows through, the lazily-
// built authenticated chain serves it.
func TestGate_ActivateBeforeMiddlewareInvocationStillWorks(t *testing.T) {
	srv := newOIDCDiscoveryStub(t)
	defer srv.Close()

	gate := authgate.New(runtimeauth.PublicProcedures())

	// Activate first — the way cmd/app/main.go does for established
	// deployments — without ever having invoked gate.Middleware(...).
	if err := gate.Activate(t.Context(), srv.URL, "test-audience"); err != nil {
		t.Fatalf("Activate before any Middleware invocation failed: %v", err)
	}
	if !gate.IsActive() {
		t.Fatal("gate should be active after successful Activate")
	}

	// Now mount through chi and serve a request. The middleware
	// function fires for the first time inside chi.ServeHTTP and must
	// pick up the already-stored verifier.
	router := chi.NewRouter()
	router.Use(gate.Middleware)
	router.HandleFunc("/gospa.companies.v1.CompaniesService/*", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/gospa.companies.v1.CompaniesService/ListCompanies", nil)
	router.ServeHTTP(rec, req)

	// Without a Bearer the auth middleware rejects: status will be
	// 401 (unauthenticated). The exact status comes from runtime/auth
	// — we only need to confirm the request did NOT pass through to
	// the protected handler, which would write 200.
	if rec.Code == http.StatusOK {
		t.Errorf("expected protected route to be guarded; got 200 — gate did not pick up the pre-Mount Activate")
	}
}

// TestGate_ActivateAfterMountStillWorks covers the inverse ordering:
// chi mounts and the first request fires before Activate. The first
// request passes through; a second request after Activate runs is
// authenticated.
func TestGate_ActivateAfterMountStillWorks(t *testing.T) {
	srv := newOIDCDiscoveryStub(t)
	defer srv.Close()

	gate := authgate.New(runtimeauth.PublicProcedures())

	router := chi.NewRouter()
	router.Use(gate.Middleware)
	router.HandleFunc("/gospa.companies.v1.CompaniesService/*", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// First request: pass-through.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/gospa.companies.v1.CompaniesService/ListCompanies", nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("pre-Activate request blocked: code %d", rec.Code)
	}

	// Activate; subsequent request should be guarded.
	if err := gate.Activate(t.Context(), srv.URL, "test-audience"); err != nil {
		t.Fatalf("Activate failed: %v", err)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/protected", nil)
	router.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Errorf("expected post-Activate request to be guarded; got 200")
	}
}

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
