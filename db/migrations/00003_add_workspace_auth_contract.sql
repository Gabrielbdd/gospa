-- +goose Up
-- +goose StatementBegin

-- Persist the explicit auth contract alongside the existing ZITADEL
-- identifiers. ADR 0001 records the rationale; in v1 the install
-- orchestrator derives these from cfg.Auth.Issuer / cfg.Zitadel.AdminAPIURL
-- / project_id, with a startup read-repair filling them in for
-- already-installed workspaces. api_audience_scope stays as a
-- server-side helper for now and is intentionally not persisted.

ALTER TABLE workspace
    ADD COLUMN zitadel_issuer_url     TEXT,
    ADD COLUMN zitadel_management_url TEXT,
    ADD COLUMN zitadel_api_audience   TEXT;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE workspace
    DROP COLUMN IF EXISTS zitadel_issuer_url,
    DROP COLUMN IF EXISTS zitadel_management_url,
    DROP COLUMN IF EXISTS zitadel_api_audience;

-- +goose StatementEnd
