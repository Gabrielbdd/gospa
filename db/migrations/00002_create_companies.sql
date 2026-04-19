-- +goose Up
-- +goose StatementBegin

CREATE TABLE companies (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name             TEXT        NOT NULL,
    slug             TEXT        NOT NULL,
    zitadel_org_id   TEXT        NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    archived_at      TIMESTAMPTZ
);

-- Slug is unique among active (non-archived) companies. Archived rows
-- can keep a historical slug without blocking future reuses.
CREATE UNIQUE INDEX companies_slug_active_unique
    ON companies (slug)
    WHERE archived_at IS NULL;

-- zitadel_org_id is unique because every company maps to exactly one
-- ZITADEL organization in the MVP.
CREATE UNIQUE INDEX companies_zitadel_org_id_unique
    ON companies (zitadel_org_id);

CREATE INDEX companies_active_created_at
    ON companies (created_at DESC)
    WHERE archived_at IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS companies;
-- +goose StatementEnd
