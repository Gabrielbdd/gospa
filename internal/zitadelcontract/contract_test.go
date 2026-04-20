package zitadelcontract_test

import (
	"testing"

	"github.com/Gabrielbdd/gospa/config"
	"github.com/Gabrielbdd/gospa/internal/zitadelcontract"
)

func TestDeriveFresh_UsesAuthIssuerWhenConfigured(t *testing.T) {
	cfg := &config.Config{
		Auth:    config.AuthConfig{Issuer: "https://issuer.example.com"},
		Zitadel: config.ZitadelConfig{AdminAPIURL: "https://admin.example.com"},
	}

	got := zitadelcontract.DeriveFresh(cfg, "project-42")

	if got.IssuerURL != "https://issuer.example.com" {
		t.Errorf("IssuerURL = %q; want explicit cfg.Auth.Issuer", got.IssuerURL)
	}
	if got.ManagementURL != "https://admin.example.com" {
		t.Errorf("ManagementURL = %q; want cfg.Zitadel.AdminAPIURL", got.ManagementURL)
	}
	if got.APIAudience != "project-42" {
		t.Errorf("APIAudience = %q; want project_id", got.APIAudience)
	}
}

func TestDeriveFresh_FallsBackToAdminAPIURLWhenIssuerEmpty(t *testing.T) {
	cfg := &config.Config{
		Auth:    config.AuthConfig{Issuer: ""},
		Zitadel: config.ZitadelConfig{AdminAPIURL: "https://admin.example.com"},
	}

	got := zitadelcontract.DeriveFresh(cfg, "project-42")

	if got.IssuerURL != "https://admin.example.com" {
		t.Errorf("IssuerURL = %q; want fallback to cfg.Zitadel.AdminAPIURL", got.IssuerURL)
	}
}

func TestOrgScope(t *testing.T) {
	if got := zitadelcontract.OrgScope("org-42"); got != "urn:zitadel:iam:org:id:org-42" {
		t.Errorf("OrgScope(org-42) = %q; want urn:zitadel:iam:org:id:org-42", got)
	}
	if got := zitadelcontract.OrgScope(""); got != "" {
		t.Errorf("OrgScope(empty) = %q; want empty", got)
	}
}

func TestAudienceScope(t *testing.T) {
	if got := zitadelcontract.AudienceScope("project-42"); got != "urn:zitadel:iam:org:project:id:project-42:aud" {
		t.Errorf("AudienceScope(project-42) = %q; want urn:zitadel:iam:org:project:id:project-42:aud", got)
	}
	if got := zitadelcontract.AudienceScope(""); got != "" {
		t.Errorf("AudienceScope(empty) = %q; want empty", got)
	}
}
