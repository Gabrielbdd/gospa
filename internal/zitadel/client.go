// Package zitadel is Gospa's thin HTTP client for the ZITADEL Admin and
// Management APIs used by the /install wizard and company creation flow.
//
// Gofra's runtime/zitadel deliberately does not bundle generated Connect
// clients (decision #153). Gospa hand-wires the small subset it needs
// directly against ZITADEL's JSON REST endpoints: this is the cheapest
// path to MVP and does not couple Gospa to a ZITADEL Go module's release
// cadence.
package zitadel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is the interface the install orchestrator and company handler
// call. Implementations exist for the real ZITADEL HTTP API (HTTPClient)
// and for tests (see fake_test.go under the internal/install and
// internal/companies packages).
type Client interface {
	SetUpOrg(ctx context.Context, req SetUpOrgRequest) (SetUpOrgResponse, error)
	AddProject(ctx context.Context, orgID, name string) (string, error)
	AddOIDCApp(ctx context.Context, orgID, projectID string, req AddOIDCAppRequest) (AddOIDCAppResponse, error)
	AddOrganization(ctx context.Context, name string) (string, error)
}

// SetUpOrgRequest is the input for the Admin SetUpOrg call, which creates
// a new organization together with its first human admin user (granted
// ORG_OWNER implicitly). Used by the install orchestrator for the MSP
// root org.
//
// Password is required: without it ZITADEL emails an "init code" the
// user must redeem to set their password, which fails on any deploy
// without configured SMTP (the local docker-compose ZITADEL has none).
// Supplying a password up front makes the freshly-created admin
// immediately able to log in with email + password.
type SetUpOrgRequest struct {
	OrgName   string
	UserEmail string
	FirstName string
	LastName  string
	Password  string
}

type SetUpOrgResponse struct {
	OrgID  string
	UserID string
}

// AddOIDCAppRequest is the minimal subset of ZITADEL's OIDC application
// fields Gospa configures for its browser SPA.
type AddOIDCAppRequest struct {
	Name              string
	RedirectURIs      []string
	PostLogoutURIs    []string
	DevMode           bool
}

type AddOIDCAppResponse struct {
	AppID    string
	ClientID string
}

// HTTPClient is the real implementation backed by net/http.
type HTTPClient struct {
	baseURL string
	pat     string
	http    *http.Client
}

// NewHTTPClient constructs an HTTPClient. baseURL is the ZITADEL instance
// root (e.g. "http://localhost:8081"). pat is the provisioner Personal
// Access Token with IAM_OWNER grant.
func NewHTTPClient(baseURL, pat string, httpClient *http.Client) *HTTPClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &HTTPClient{baseURL: baseURL, pat: pat, http: httpClient}
}

// --- Admin API ---------------------------------------------------------

type setUpOrgHumanWire struct {
	UserName string              `json:"userName"`
	Profile  setUpOrgProfileWire `json:"profile"`
	Email    setUpOrgEmailWire   `json:"email"`
	// Password is the initial password the user logs in with. The
	// ZITADEL Admin _setup endpoint accepts a plain string here and
	// hashes it server-side. Omitted via omitempty so older callers
	// (or the AddOrganization path) that do not provide a password
	// still produce a valid request.
	Password string `json:"password,omitempty"`
}

type setUpOrgProfileWire struct {
	FirstName         string `json:"firstName"`
	LastName          string `json:"lastName"`
	PreferredLanguage string `json:"preferredLanguage,omitempty"`
}

type setUpOrgEmailWire struct {
	Email           string `json:"email"`
	IsEmailVerified bool   `json:"isEmailVerified"`
}

type setUpOrgOrgWire struct {
	Name string `json:"name"`
}

type setUpOrgWireRequest struct {
	Org   setUpOrgOrgWire   `json:"org"`
	Human setUpOrgHumanWire `json:"human"`
}

type setUpOrgWireResponse struct {
	OrgID  string `json:"orgId"`
	UserID string `json:"userId"`
}

func (c *HTTPClient) SetUpOrg(ctx context.Context, req SetUpOrgRequest) (SetUpOrgResponse, error) {
	wire := setUpOrgWireRequest{
		Org: setUpOrgOrgWire{Name: req.OrgName},
		Human: setUpOrgHumanWire{
			UserName: req.UserEmail,
			Profile: setUpOrgProfileWire{
				FirstName: req.FirstName,
				LastName:  req.LastName,
			},
			Email: setUpOrgEmailWire{
				Email:           req.UserEmail,
				IsEmailVerified: true,
			},
			Password: req.Password,
		},
	}
	var out setUpOrgWireResponse
	if err := c.post(ctx, "/admin/v1/orgs/_setup", "", wire, &out); err != nil {
		return SetUpOrgResponse{}, fmt.Errorf("zitadel SetUpOrg: %w", err)
	}
	return SetUpOrgResponse{OrgID: out.OrgID, UserID: out.UserID}, nil
}

type addOrgWireRequest struct {
	Name string `json:"name"`
}

type addOrgWireResponse struct {
	ID string `json:"id"`
}

// AddOrganization creates a new organization inside the ZITADEL instance
// without seeding a human admin. Used by the company-creation flow where
// the MSP manages the org on behalf of a client.
func (c *HTTPClient) AddOrganization(ctx context.Context, name string) (string, error) {
	var out addOrgWireResponse
	if err := c.post(ctx, "/admin/v1/orgs", "", addOrgWireRequest{Name: name}, &out); err != nil {
		return "", fmt.Errorf("zitadel AddOrganization: %w", err)
	}
	return out.ID, nil
}

// --- Management API (needs x-zitadel-orgid) ----------------------------

type addProjectWireRequest struct {
	Name string `json:"name"`
}

type addProjectWireResponse struct {
	ID string `json:"id"`
}

func (c *HTTPClient) AddProject(ctx context.Context, orgID, name string) (string, error) {
	var out addProjectWireResponse
	if err := c.post(ctx, "/management/v1/projects", orgID, addProjectWireRequest{Name: name}, &out); err != nil {
		return "", fmt.Errorf("zitadel AddProject: %w", err)
	}
	return out.ID, nil
}

type addOIDCAppWireRequest struct {
	Name            string   `json:"name"`
	RedirectUris    []string `json:"redirectUris,omitempty"`
	ResponseTypes   []string `json:"responseTypes,omitempty"`
	GrantTypes      []string `json:"grantTypes,omitempty"`
	AppType         string   `json:"appType,omitempty"`
	AuthMethodType  string   `json:"authMethodType,omitempty"`
	PostLogoutUris  []string `json:"postLogoutRedirectUris,omitempty"`
	Version         string   `json:"version,omitempty"`
	DevMode         bool     `json:"devMode,omitempty"`
	AccessTokenType string   `json:"accessTokenType,omitempty"`
}

type addOIDCAppWireResponse struct {
	AppID    string `json:"appId"`
	ClientID string `json:"clientId"`
}

func (c *HTTPClient) AddOIDCApp(ctx context.Context, orgID, projectID string, req AddOIDCAppRequest) (AddOIDCAppResponse, error) {
	wire := addOIDCAppWireRequest{
		Name:           req.Name,
		RedirectUris:   req.RedirectURIs,
		ResponseTypes:  []string{"OIDC_RESPONSE_TYPE_CODE"},
		GrantTypes:     []string{"OIDC_GRANT_TYPE_AUTHORIZATION_CODE", "OIDC_GRANT_TYPE_REFRESH_TOKEN"},
		AppType:        "OIDC_APP_TYPE_USER_AGENT",
		AuthMethodType: "OIDC_AUTH_METHOD_TYPE_NONE",
		PostLogoutUris: req.PostLogoutURIs,
		Version:        "OIDC_VERSION_1_0",
		DevMode:        req.DevMode,
		// JWT access tokens, not opaque. The Gospa gate validates
		// access tokens locally via the OIDC discovery JWKS path
		// (runtime/auth/verifier.go), which cannot decode opaque
		// tokens. ZITADEL's default for new OIDC apps is BEARER
		// (opaque); we override to JWT here so the audience scope
		// + signature + iss + exp checks all run against a real JWT.
		AccessTokenType: "OIDC_TOKEN_TYPE_JWT",
	}
	path := fmt.Sprintf("/management/v1/projects/%s/apps/oidc", projectID)
	var out addOIDCAppWireResponse
	if err := c.post(ctx, path, orgID, wire, &out); err != nil {
		return AddOIDCAppResponse{}, fmt.Errorf("zitadel AddOIDCApp: %w", err)
	}
	return AddOIDCAppResponse{AppID: out.AppID, ClientID: out.ClientID}, nil
}

// --- HTTP plumbing -----------------------------------------------------

func (c *HTTPClient) post(ctx context.Context, path, orgID string, body, out any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.pat)
	if orgID != "" {
		req.Header.Set("x-zitadel-orgid", orgID)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("zitadel returned %s: %s", resp.Status, string(body))
	}

	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
