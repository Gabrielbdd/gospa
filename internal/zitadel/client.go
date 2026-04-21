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
	// RemoveOrg deletes an organisation (cascades to its projects + apps).
	// Used by the install orchestrator's opportunistic cleanup when a
	// post-SetUpOrg step fails. Idempotent: a 404 from ZITADEL is
	// treated as success so retries are safe.
	RemoveOrg(ctx context.Context, orgID string) error
	// AddHumanUser creates a human user inside the given organisation
	// with a fixed password. Used by the team-invite flow to
	// bootstrap a new member whose first sign-in must change the
	// password. Returns the ZITADEL user id.
	AddHumanUser(ctx context.Context, orgID string, req AddHumanUserRequest) (string, error)
	// RemoveUser deletes a user. Used by the invite-failure cleanup
	// path (mirrors the RemoveOrg pattern in S15). 404 is treated as
	// success so retries are safe.
	RemoveUser(ctx context.Context, orgID, userID string) error
	// RenameOrg updates an organisation's display name. Used by
	// UpdateWorkspaceCompany to propagate a Gospa-side workspace
	// rename into ZITADEL so the two names stay in sync. Best-effort
	// — the caller treats failure as a warning, not a fatal.
	RenameOrg(ctx context.Context, orgID, name string) error
}

// AddHumanUserRequest is the narrow subset of the ZITADEL human-user
// creation API that Gospa consumes. Email is verified server-side
// (IsEmailVerified=true) because the invite flow hands the credentials
// off to the admin, not the invitee — the email is attested by the
// admin, not by a click on a link.
type AddHumanUserRequest struct {
	Email                  string
	FirstName              string
	LastName               string
	// Password is the server-generated one-time secret. ZITADEL hashes
	// it on the server side. The invitee must change it on first
	// sign-in (see PasswordChangeRequired).
	Password               string
	PasswordChangeRequired bool
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
	baseURL     string
	patProvider func() string
	http        *http.Client
}

// NewHTTPClient constructs an HTTPClient. baseURL is the ZITADEL
// instance root (e.g. "http://localhost:8081"). patProvider returns
// the current provisioner Personal Access Token (IAM_OWNER) at the
// moment a request is built — this is what lets cmd/app rotate the
// PAT in place via internal/patwatch without rebuilding the client.
func NewHTTPClient(baseURL string, patProvider func() string, httpClient *http.Client) *HTTPClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	if patProvider == nil {
		patProvider = func() string { return "" }
	}
	return &HTTPClient{baseURL: baseURL, patProvider: patProvider, http: httpClient}
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
	OrganizationID string `json:"organizationId"`
}

// AddOrganization creates a new organization inside the ZITADEL instance
// without seeding a human admin. Used by the company-creation flow where
// the MSP manages the org on behalf of a client.
//
// Uses /v2beta/organizations — the legacy /admin/v1/orgs endpoint was
// removed in ZITADEL v3. The v2 path expects {"name": "..."} and
// returns {"organizationId": "...", "details": {...}}. Once ZITADEL
// promotes v2 out of beta the URL will drop the "beta" suffix; until
// then v2beta is the only working create-org endpoint.
func (c *HTTPClient) AddOrganization(ctx context.Context, name string) (string, error) {
	var out addOrgWireResponse
	if err := c.post(ctx, "/v2beta/organizations", "", addOrgWireRequest{Name: name}, &out); err != nil {
		return "", fmt.Errorf("zitadel AddOrganization: %w", err)
	}
	return out.OrganizationID, nil
}

// RemoveOrg deletes an organisation. ZITADEL cascades the delete to the
// org's projects and OIDC apps, so callers compensating for a partial
// install only need to remove the org.
//
// 404 is treated as success — the org is already gone, which is exactly
// what the caller wanted. Other 4xx/5xx are propagated so the caller can
// record the failure in install_error and let the operator investigate.
func (c *HTTPClient) RemoveOrg(ctx context.Context, orgID string) error {
	if orgID == "" {
		return fmt.Errorf("zitadel RemoveOrg: empty org id")
	}
	pat := c.patProvider()
	if pat == "" {
		return fmt.Errorf("zitadel RemoveOrg: provisioner PAT not available")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/admin/v1/orgs/"+orgID, nil)
	if err != nil {
		return fmt.Errorf("zitadel RemoveOrg: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+pat)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("zitadel RemoveOrg: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("zitadel RemoveOrg: %s: %s", resp.Status, string(body))
	}
	return nil
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

// --- Management API: org rename ---------------------------------------

type renameOrgWireRequest struct {
	Name string `json:"name"`
}

// RenameOrg issues PUT /management/v1/orgs/me scoped to the given org
// via the `x-zitadel-orgid` header. ZITADEL returns 200 with the
// updated org payload; we discard it.
func (c *HTTPClient) RenameOrg(ctx context.Context, orgID, name string) error {
	if orgID == "" {
		return fmt.Errorf("zitadel RenameOrg: empty org id")
	}
	if name == "" {
		return fmt.Errorf("zitadel RenameOrg: empty name")
	}
	pat := c.patProvider()
	if pat == "" {
		return fmt.Errorf("zitadel RenameOrg: provisioner PAT not available")
	}
	buf, err := json.Marshal(renameOrgWireRequest{Name: name})
	if err != nil {
		return fmt.Errorf("zitadel RenameOrg: marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+"/management/v1/orgs/me", bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("zitadel RenameOrg: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+pat)
	req.Header.Set("x-zitadel-orgid", orgID)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("zitadel RenameOrg: do request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("zitadel RenameOrg: %s: %s", resp.Status, string(body))
	}
	return nil
}

// --- Management API: human user creation + removal --------------------

type addHumanUserWireRequest struct {
	UserName string                       `json:"userName"`
	Profile  setUpOrgProfileWire          `json:"profile"`
	Email    setUpOrgEmailWire            `json:"email"`
	// InitialPassword is what ZITADEL names this field in the v1
	// Management API. The key differs from the admin/_setup flow
	// (which uses "password") — keep the wire types distinct.
	InitialPassword        string `json:"initialPassword,omitempty"`
	PasswordChangeRequired bool   `json:"passwordChangeRequired,omitempty"`
}

type addHumanUserWireResponse struct {
	UserID string `json:"userId"`
}

// AddHumanUser issues POST /management/v1/users/human/_import scoped
// to the given organisation. The invitee logs in with the temporary
// password and is forced to change it on first sign-in when
// PasswordChangeRequired is true.
func (c *HTTPClient) AddHumanUser(ctx context.Context, orgID string, req AddHumanUserRequest) (string, error) {
	if orgID == "" {
		return "", fmt.Errorf("zitadel AddHumanUser: empty org id")
	}
	wire := addHumanUserWireRequest{
		UserName: req.Email,
		Profile: setUpOrgProfileWire{
			FirstName: req.FirstName,
			LastName:  req.LastName,
		},
		Email: setUpOrgEmailWire{
			Email:           req.Email,
			IsEmailVerified: true,
		},
		InitialPassword:        req.Password,
		PasswordChangeRequired: req.PasswordChangeRequired,
	}
	var out addHumanUserWireResponse
	if err := c.post(ctx, "/management/v1/users/human/_import", orgID, wire, &out); err != nil {
		return "", fmt.Errorf("zitadel AddHumanUser: %w", err)
	}
	return out.UserID, nil
}

// RemoveUser deletes a user by id. 404 is treated as success — if the
// user is already gone (previous cleanup succeeded, manual delete in
// ZITADEL console), the caller's intent is satisfied.
func (c *HTTPClient) RemoveUser(ctx context.Context, orgID, userID string) error {
	if orgID == "" {
		return fmt.Errorf("zitadel RemoveUser: empty org id")
	}
	if userID == "" {
		return fmt.Errorf("zitadel RemoveUser: empty user id")
	}
	pat := c.patProvider()
	if pat == "" {
		return fmt.Errorf("zitadel RemoveUser: provisioner PAT not available")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/management/v1/users/"+userID, nil)
	if err != nil {
		return fmt.Errorf("zitadel RemoveUser: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+pat)
	req.Header.Set("x-zitadel-orgid", orgID)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("zitadel RemoveUser: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("zitadel RemoveUser: %s: %s", resp.Status, string(body))
	}
	return nil
}

// --- HTTP plumbing -----------------------------------------------------

func (c *HTTPClient) post(ctx context.Context, path, orgID string, body, out any) error {
	pat := c.patProvider()
	if pat == "" {
		// Should never happen at runtime: the patwatch is fail-closed
		// at startup and last-known-good after that. Surface it
		// loudly if it ever does, instead of issuing an unauthenticated
		// request that ZITADEL would 401.
		return fmt.Errorf("zitadel: provisioner PAT not available")
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+pat)
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
