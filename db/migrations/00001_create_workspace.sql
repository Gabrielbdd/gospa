-- +goose Up
-- +goose StatementBegin

-- pgcrypto provides gen_random_uuid() used by later migrations.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TYPE workspace_install_state AS ENUM (
    'not_initialized',
    'provisioning',
    'ready',
    'failed'
);

-- Singleton table: one deploy, one workspace. The CHECK constraint
-- enforces structural uniqueness at the database layer so the code path
-- can always load `id = 1` without guarding against mistakes.
CREATE TABLE workspace (
    id               SMALLINT PRIMARY KEY CHECK (id = 1),
    name             TEXT        NOT NULL DEFAULT '',
    slug             TEXT        NOT NULL DEFAULT '',
    timezone         TEXT        NOT NULL DEFAULT 'UTC',
    currency_code    TEXT        NOT NULL DEFAULT 'USD',
    install_state    workspace_install_state NOT NULL DEFAULT 'not_initialized',
    install_error    TEXT,
    zitadel_org_id         TEXT,
    zitadel_project_id     TEXT,
    zitadel_spa_app_id     TEXT,
    zitadel_spa_client_id  TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    initialized_at   TIMESTAMPTZ
);

-- Seed the singleton. The install wizard at POST /install fills the rest.
INSERT INTO workspace (id) VALUES (1) ON CONFLICT (id) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS workspace;
DROP TYPE IF EXISTS workspace_install_state;
-- +goose StatementEnd
