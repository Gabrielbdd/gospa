package authgate_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	runtimeauth "github.com/Gabrielbdd/gofra/runtime/auth"

	"github.com/Gabrielbdd/gospa/internal/authgate"
)

func TestGate_PassesThroughBeforeActivate(t *testing.T) {
	gate := authgate.New("http://unused.local", runtimeauth.PublicProcedures())

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
	gate := authgate.New("http://unused.local", runtimeauth.PublicProcedures())

	err := gate.Activate(t.Context(), "some-audience")
	if err == nil {
		t.Fatal("expected error when activating before Middleware has been mounted")
	}
	if gate.IsActive() {
		t.Error("gate should not be active after a failed Activate")
	}
}
