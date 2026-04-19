package zitadel_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Gabrielbdd/gospa/internal/zitadel"
)

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

	c := zitadel.NewHTTPClient(srv.URL, "pat-abc", srv.Client())
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

	c := zitadel.NewHTTPClient(srv.URL, "pat", srv.Client())
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

	c := zitadel.NewHTTPClient(srv.URL, "pat", srv.Client())
	_, err := c.AddProject(context.Background(), "org", "proj")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error = %v; want it to mention 403", err)
	}
}
