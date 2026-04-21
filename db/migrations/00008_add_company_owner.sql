-- Slice 5 (frontend) — add a company owner FK so the operator UI can
-- show who on the team is responsible for each customer account.
--
-- Owner is a contact, not a workspace_grant. In practice only contacts
-- that also have a workspace_grant will show up in the picker, but the
-- column FKs to contacts(id) so it survives role changes and keeps the
-- schema orthogonal — workspace_grants lives alongside contacts, not
-- underneath it.
--
-- ON DELETE SET NULL so archiving a contact (soft) or physically
-- deleting one later does not cascade-orphan companies.
-- +goose Up
ALTER TABLE companies
    ADD COLUMN owner_contact_id UUID NULL
    REFERENCES contacts(id) ON DELETE SET NULL;

-- Partial index so "my companies" queries stay fast as the table grows
-- and the index stays small (non-archived rows only).
CREATE INDEX companies_owner_contact_id_active
    ON companies (owner_contact_id)
    WHERE archived_at IS NULL AND owner_contact_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS companies_owner_contact_id_active;
ALTER TABLE companies DROP COLUMN IF EXISTS owner_contact_id;
