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
