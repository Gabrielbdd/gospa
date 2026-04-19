package zitadelcontract_test

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Gabrielbdd/gospa/config"
	"github.com/Gabrielbdd/gospa/db/sqlc"
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

func TestDeriveRepair_PrefersExplicitAuthAudience(t *testing.T) {
	cfg := &config.Config{
		Auth:    config.AuthConfig{Audience: "explicit-audience"},
		Zitadel: config.ZitadelConfig{AdminAPIURL: "https://admin.example.com"},
	}
	ws := sqlc.Workspace{
		ZitadelProjectID: pgtype.Text{String: "project-42", Valid: true},
	}

	got := zitadelcontract.DeriveRepair(cfg, ws)

	if got.APIAudience != "explicit-audience" {
		t.Errorf("APIAudience = %q; want cfg.Auth.Audience", got.APIAudience)
	}
}

func TestDeriveRepair_FallsBackToProjectIDWhenAudienceEmpty(t *testing.T) {
	cfg := &config.Config{
		Zitadel: config.ZitadelConfig{AdminAPIURL: "https://admin.example.com"},
	}
	ws := sqlc.Workspace{
		ZitadelProjectID: pgtype.Text{String: "project-42", Valid: true},
	}

	got := zitadelcontract.DeriveRepair(cfg, ws)

	if got.APIAudience != "project-42" {
		t.Errorf("APIAudience = %q; want fallback to workspace.zitadel_project_id", got.APIAudience)
	}
}

func TestDeriveRepair_LeavesAudienceEmptyWhenNoSafeSource(t *testing.T) {
	cfg := &config.Config{
		Zitadel: config.ZitadelConfig{AdminAPIURL: "https://admin.example.com"},
	}
	ws := sqlc.Workspace{
		ZitadelProjectID: pgtype.Text{Valid: false},
	}

	got := zitadelcontract.DeriveRepair(cfg, ws)

	if got.APIAudience != "" {
		t.Errorf("APIAudience = %q; want empty so caller can leave column NULL", got.APIAudience)
	}
	// Issuer + management still derived; only audience is unsafe.
	if got.IssuerURL == "" {
		t.Error("IssuerURL should still be derivable when audience is not")
	}
	if got.ManagementURL == "" {
		t.Error("ManagementURL should still be derivable when audience is not")
	}
}
