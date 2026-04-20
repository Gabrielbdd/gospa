// Package zitadelcontract derives the persisted ZITADEL auth contract
// values from the runtime config + the freshly-provisioned project ID.
//
// The orchestrator uses DeriveFresh after creating the OIDC SPA app in
// ZITADEL. There is no read-repair / backfill derivation: pre-v1, every
// schema or state change assumes a fresh install, not upgrade-over-
// existing-data. See repos/gospa/AGENTS.md "No repair before v1".
//
// ADR 0001 records why these values are explicit persisted state
// instead of runtime inference. v1 persists three of the eight fields;
// api_audience_scope stays as a derived helper for now.
package zitadelcontract

import (
	"github.com/Gabrielbdd/gospa/config"
)

// Contract is the auth contract derived for a workspace. Empty strings
// in any field mean "no safe derivation possible — caller should not
// overwrite a persisted value with empty".
type Contract struct {
	IssuerURL     string
	ManagementURL string
	APIAudience   string
}

// DeriveFresh returns the contract for a brand-new install. projectID
// is the ZITADEL project just created by the orchestrator (becomes the
// API audience). All three fields are guaranteed non-empty as long as
// projectID is non-empty and cfg.Zitadel.AdminAPIURL is configured —
// both invariants the orchestrator already enforces.
func DeriveFresh(cfg *config.Config, projectID string) Contract {
	return Contract{
		IssuerURL:     issuerOrFallback(cfg),
		ManagementURL: cfg.Zitadel.AdminAPIURL,
		APIAudience:   projectID,
	}
}

// issuerOrFallback returns cfg.Auth.Issuer when set, otherwise
// cfg.Zitadel.AdminAPIURL. The fallback is temporary while we still
// have deploys that haven't split issuer from management host; it is
// the same value the legacy code used implicitly.
func issuerOrFallback(cfg *config.Config) string {
	if cfg.Auth.Issuer != "" {
		return cfg.Auth.Issuer
	}
	return cfg.Zitadel.AdminAPIURL
}

// OrgScope returns the OIDC scope string that scopes a ZITADEL login
// to a specific organisation, e.g. urn:zitadel:iam:org:id:NNN. Returns
// an empty string when orgID is empty so the caller can decide not to
// inject anything pre-install. Kept as a server-side helper so the
// browser does not have to know how to compose ZITADEL scope URNs.
func OrgScope(orgID string) string {
	if orgID == "" {
		return ""
	}
	return "urn:zitadel:iam:org:id:" + orgID
}

// AudienceScope returns the OIDC scope string that asks ZITADEL to
// include the given audience value in the access token's `aud` claim,
// e.g. urn:zitadel:iam:org:project:id:NNN:aud. Returns an empty string
// when audience is empty.
//
// Callers should pass workspace.zitadel_api_audience — the same field
// the JWT verifier validates against. Anchoring the scope on the
// audience contract (not on project_id directly) guarantees the
// browser asks for exactly what the gate will accept, even if a
// future deploy persists an api_audience that differs from the
// project_id (cfg.Auth.Audience can override).
//
// The api_audience_scope intentionally is not a persisted column in
// this v1 round (decision in the provisioning hardening plan); it
// lives here as a deterministic derivation from the persisted
// audience so the browser can request it without learning the URN
// format.
func AudienceScope(audience string) string {
	if audience == "" {
		return ""
	}
	return "urn:zitadel:iam:org:project:id:" + audience + ":aud"
}
