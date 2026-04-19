package zitadel_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Gabrielbdd/gospa/internal/zitadel"
)

// staticPAT returns a func() string suitable for NewHTTPClient that
// always yields the same value — the common case in these tests.
func staticPAT(v string) func() string {
	return func() string { return v }
}

func TestHTTPClient_SetUpOrg_SetsAuthAndSends(t *testing.T) {
	var (
		gotAuth, gotCT, gotPath, gotBody string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotCT = r.Header.Get("Content-Type")
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_ = json.NewEncoder(w).Encode(map[string]string{"orgId": "org-123", "userId": "user-456"})
	}))
	t.Cleanup(srv.Close)

	c := zitadel.NewHTTPClient(srv.URL, staticPAT("pat-abc"), srv.Client())
	resp, err := c.SetUpOrg(context.Background(), zitadel.SetUpOrgRequest{
		OrgName:   "Acme MSP",
		UserEmail: "admin@example.com",
		FirstName: "Admin",
		LastName:  "User",
	})
	if err != nil {
		t.Fatalf("SetUpOrg: %v", err)
	}

	if resp.OrgID != "org-123" || resp.UserID != "user-456" {
		t.Errorf("parsed response = %+v; want OrgID=org-123, UserID=user-456", resp)
	}
	if gotAuth != "Bearer pat-abc" {
		t.Errorf("Authorization = %q; want Bearer pat-abc", gotAuth)
	}
	if gotCT != "application/json" {
		t.Errorf("Content-Type = %q", gotCT)
	}
	if gotPath != "/admin/v1/orgs/_setup" {
		t.Errorf("path = %q", gotPath)
	}
	if !strings.Contains(gotBody, `"name":"Acme MSP"`) {
		t.Errorf("body missing org name: %s", gotBody)
	}
	if !strings.Contains(gotBody, `"firstName":"Admin"`) {
		t.Errorf("body missing first name: %s", gotBody)
	}
}

func TestHTTPClient_AddProject_SetsOrgHeader(t *testing.T) {
	var gotOrg string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotOrg = r.Header.Get("x-zitadel-orgid")
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "proj-1"})
	}))
	t.Cleanup(srv.Close)

	c := zitadel.NewHTTPClient(srv.URL, staticPAT("pat"), srv.Client())
	id, err := c.AddProject(context.Background(), "org-7", "gospa-app")
	if err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	if id != "proj-1" {
		t.Errorf("project id = %q", id)
	}
	if gotOrg != "org-7" {
		t.Errorf("x-zitadel-orgid = %q; want org-7", gotOrg)
	}
}

func TestHTTPClient_ReturnsErrorOn4xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"insufficient grants"}`))
	}))
	t.Cleanup(srv.Close)

	c := zitadel.NewHTTPClient(srv.URL, staticPAT("pat"), srv.Client())
	_, err := c.AddProject(context.Background(), "org", "proj")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error = %v; want it to mention 403", err)
	}
}

// TestHTTPClient_PATProviderReadOnEachCall is the regression test for
// the patwatch hot-reload contract: the client must read the current
// PAT at request time, not bind it once at construction. A patwatch
// rotation between two AddProject calls should change the bearer
// token observed by ZITADEL.
func TestHTTPClient_PATProviderReadOnEachCall(t *testing.T) {
	var lastAuth atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastAuth.Store(r.Header.Get("Authorization"))
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "p"})
	}))
	t.Cleanup(srv.Close)

	var current atomic.Value
	current.Store("pat-v1")
	c := zitadel.NewHTTPClient(srv.URL, func() string {
		return current.Load().(string)
	}, srv.Client())

	if _, err := c.AddProject(context.Background(), "org", "p1"); err != nil {
		t.Fatalf("first AddProject: %v", err)
	}
	if got := lastAuth.Load().(string); got != "Bearer pat-v1" {
		t.Errorf("first call Authorization = %q; want Bearer pat-v1", got)
	}

	current.Store("pat-v2-rotated")

	if _, err := c.AddProject(context.Background(), "org", "p2"); err != nil {
		t.Fatalf("second AddProject: %v", err)
	}
	if got := lastAuth.Load().(string); got != "Bearer pat-v2-rotated" {
		t.Errorf("second call Authorization = %q; want Bearer pat-v2-rotated (provider was not consulted on this call)", got)
	}
}

func TestHTTPClient_PATProviderEmptyFailsLocally(t *testing.T) {
	// Server should never be hit when the provider returns empty.
	var hit atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hit.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := zitadel.NewHTTPClient(srv.URL, staticPAT(""), srv.Client())
	_, err := c.AddProject(context.Background(), "org", "p")
	if err == nil {
		t.Fatal("expected error when patProvider returns empty")
	}
	if hit.Load() {
		t.Error("server was hit despite empty PAT — should fail locally before request")
	}
	if !strings.Contains(err.Error(), "PAT not available") {
		t.Errorf("error = %v; want mention of missing PAT", err)
	}
}
