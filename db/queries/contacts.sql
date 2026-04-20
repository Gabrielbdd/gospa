-- name: CreateContact :one
-- Inserts a contact in the given company. Identity columns
-- (zitadel_user_id, identity_source, external_id) are accepted as
-- explicit parameters so the install orchestrator + invite flow can
-- supply them; the contacts handler defaults identity_source to
-- 'manual' app-side when omitted.
INSERT INTO contacts (
    company_id,
    full_name,
    job_title,
    email,
    phone,
    mobile,
    notes,
    zitadel_user_id,
    identity_source,
    external_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetContact :one
-- Returns the active contact by id. Used by handlers that accept a
-- contact_id from the wire (TeamService change-role/suspend/archive,
-- ContactsService single-record operations).
SELECT *
FROM contacts
WHERE id = $1 AND archived_at IS NULL;

-- name: ContactExistsByCompanyEmail :one
-- Used by InviteMember to fail fast on a duplicate email before
-- creating the ZITADEL user. Matches the case-insensitive uniqueness
-- predicate enforced by the contacts_company_email_active_unique
-- partial index so the app-level pre-check stays consistent with the
-- DB-level guarantee.
SELECT EXISTS(
    SELECT 1 FROM contacts
    WHERE company_id = $1
      AND email IS NOT NULL
      AND lower(email) = lower($2)
      AND archived_at IS NULL
) AS found;
