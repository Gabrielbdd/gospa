-- +goose Up
-- +goose StatementBegin

-- contacts is the directory primitive shared by team members (MSP
-- staff, contacts of the workspace-owner company) and customer-side
-- contacts (people at client companies). Identity (zitadel_user_id)
-- is nullable so that contacts who do not log in remain valid.
--
-- identity_source + external_id ship now even though no code reads
-- them in this slice. The cost is two columns and a default; the
-- benefit is that future Microsoft 365 / SCIM sync can populate them
-- without a migration on a populated table.

CREATE TABLE contacts (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id       UUID        NOT NULL REFERENCES companies(id) ON DELETE RESTRICT,
    full_name        TEXT        NOT NULL,
    job_title        TEXT,
    email            TEXT,
    phone            TEXT,
    mobile           TEXT,
    notes            TEXT,
    zitadel_user_id  TEXT,
    identity_source  TEXT        NOT NULL DEFAULT 'manual',
    external_id      TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    archived_at      TIMESTAMPTZ
);

-- Email is unique per (company, lower(email)) when both are present
-- and the contact is active. Two rules combined:
--   * Email may be NULL (contact with only name/phone/notes).
--   * Same email may exist at different companies (a consultant who
--     services multiple clients).
--   * Within one company, two non-archived contacts may not share
--     an email.
-- Archived rows are excluded so re-using an email after archive is
-- allowed (operator workflow: archive a stale contact, add a fresh
-- one with the same address).
CREATE UNIQUE INDEX contacts_company_email_active_unique
    ON contacts (company_id, lower(email))
    WHERE email IS NOT NULL AND archived_at IS NULL;

-- Hot lookup: list active contacts for a company.
CREATE INDEX contacts_company_active
    ON contacts (company_id)
    WHERE archived_at IS NULL;

-- Hot lookup for the authz middleware: resolve caller by JWT subject.
-- Globally unique because one ZITADEL user maps to at most one
-- contact in this product (the MSP user is one contact; future
-- client-portal users are also one contact each, in their own
-- company). Partial so contacts without identity don't trip the
-- constraint.
CREATE UNIQUE INDEX contacts_zitadel_user_id_unique
    ON contacts (zitadel_user_id)
    WHERE zitadel_user_id IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS contacts;

-- +goose StatementEnd
