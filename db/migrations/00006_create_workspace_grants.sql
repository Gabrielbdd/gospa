-- +goose Up
-- +goose StatementBegin

-- workspace_grants is the authz primitive for the MSP workspace.
-- One row per team member; the contact backing the grant must live
-- at the workspace-owner company (enforced app-side, not via DB
-- check). Authz lives entirely in this table — there are no ZITADEL
-- project roles or user grants.
--
-- The role enum is intentionally small (admin, technician). Adding
-- a third role would be a forward-compatible ALTER TYPE; widening
-- to ABAC would require a join table.
--
-- The status enum tracks the operational lifecycle of the grant:
--   * not_signed_in_yet — fresh invite, password issued, member
--     hasn't completed first sign-in
--   * active            — normal operating state
--   * suspended         — admin revoked workspace access (the
--     ZITADEL user remains valid; the authz middleware rejects)
-- The middleware flips not_signed_in_yet → active on the first
-- successful authenticated request.

CREATE TYPE workspace_role AS ENUM ('admin', 'technician');

CREATE TYPE grant_status AS ENUM ('active', 'not_signed_in_yet', 'suspended');

CREATE TABLE workspace_grants (
    contact_id              UUID        PRIMARY KEY REFERENCES contacts(id) ON DELETE CASCADE,
    role                    workspace_role NOT NULL,
    status                  grant_status   NOT NULL DEFAULT 'active',
    last_seen_at            TIMESTAMPTZ,
    granted_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- granted_by is the contact_id of the admin who issued this grant.
    -- SET NULL on delete keeps the grant intact if the granting admin
    -- is later removed; the audit value is best-effort.
    granted_by_contact_id   UUID        REFERENCES contacts(id) ON DELETE SET NULL
);

-- Hot lookup: count active admins for the last-admin invariant.
-- Partial index keeps it cheap even as the team grows.
CREATE INDEX workspace_grants_active_admins
    ON workspace_grants (contact_id)
    WHERE role = 'admin' AND status = 'active';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS workspace_grants;
DROP TYPE IF EXISTS grant_status;
DROP TYPE IF EXISTS workspace_role;

-- +goose StatementEnd
