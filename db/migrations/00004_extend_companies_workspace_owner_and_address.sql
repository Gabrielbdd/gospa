-- +goose Up
-- +goose StatementBegin

-- Mark exactly one companies row as the MSP itself (the workspace
-- owner). The MSP shares the companies table with customer companies
-- so that team members can be modelled as contacts of a company —
-- avoiding the polymorphic "ticket requester is a workspace user OR
-- a contact" model legacy PSAs carry. Materialised by the install
-- orchestrator in the same transaction that flips install_state to
-- ready. Pre-v1 there is no backfill for workspaces installed under
-- an earlier schema — drop the DB and reinstall.

ALTER TABLE companies
    ADD COLUMN is_workspace_owner BOOLEAN NOT NULL DEFAULT FALSE;

-- Enforce "exactly one workspace-owner row" at the database layer so
-- a buggy code path can't accidentally create a second MSP row. The
-- partial index allows any number of FALSE rows (customer companies)
-- and at most one TRUE row.
CREATE UNIQUE INDEX companies_one_workspace_owner
    ON companies (is_workspace_owner)
    WHERE is_workspace_owner = TRUE;

-- Address fields used by the operator-facing companies UI and by
-- /settings/workspace for the MSP itself. Defaults are empty strings
-- so insert paths that don't supply them stay valid; the MVP UI
-- treats empty as "not set". timezone defaults to UTC for the same
-- reason workspace.timezone does.
ALTER TABLE companies
    ADD COLUMN address_line1 TEXT NOT NULL DEFAULT '',
    ADD COLUMN address_line2 TEXT NOT NULL DEFAULT '',
    ADD COLUMN city          TEXT NOT NULL DEFAULT '',
    ADD COLUMN region        TEXT NOT NULL DEFAULT '',
    ADD COLUMN postal_code   TEXT NOT NULL DEFAULT '',
    ADD COLUMN country       TEXT NOT NULL DEFAULT '',
    ADD COLUMN timezone      TEXT NOT NULL DEFAULT 'UTC';

-- Tighten the slug uniqueness predicate so concurrent inserts of an
-- empty slug (which start happening once Slice 2 stops writing
-- meaningful slug values) do not collide. The original index was
-- "WHERE archived_at IS NULL"; it now also excludes empty strings.
-- Drop+create is one statement-block in goose so it stays atomic.
DROP INDEX IF EXISTS companies_slug_active_unique;
CREATE UNIQUE INDEX companies_slug_active_unique
    ON companies (slug)
    WHERE archived_at IS NULL AND slug <> '';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Reverse the slug index tightening so the rollback restores the
-- original predicate. Existing rows continue to satisfy the looser
-- predicate.
DROP INDEX IF EXISTS companies_slug_active_unique;
CREATE UNIQUE INDEX companies_slug_active_unique
    ON companies (slug)
    WHERE archived_at IS NULL;

ALTER TABLE companies
    DROP COLUMN IF EXISTS timezone,
    DROP COLUMN IF EXISTS country,
    DROP COLUMN IF EXISTS postal_code,
    DROP COLUMN IF EXISTS region,
    DROP COLUMN IF EXISTS city,
    DROP COLUMN IF EXISTS address_line2,
    DROP COLUMN IF EXISTS address_line1;

DROP INDEX IF EXISTS companies_one_workspace_owner;

ALTER TABLE companies
    DROP COLUMN IF EXISTS is_workspace_owner;

-- +goose StatementEnd
