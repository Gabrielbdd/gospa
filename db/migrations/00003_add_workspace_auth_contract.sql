-- +goose Up
-- +goose StatementBegin

-- Persist the explicit auth contract alongside the existing ZITADEL
-- identifiers. ADR 0001 records the rationale; the install orchestrator
-- derives these from cfg.Auth.Issuer / cfg.Zitadel.AdminAPIURL /
-- project_id. api_audience_scope stays as a server-side helper for now
-- and is intentionally not persisted.
-- Pre-v1 there is no backfill path for workspaces installed before
-- these columns existed — drop the DB and reinstall.

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
