-- name: CreateCompany :one
INSERT INTO companies (
    name, zitadel_org_id, owner_contact_id,
    address_line1, address_line2, city, region, postal_code, country, timezone
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: CreateWorkspaceCompany :one
-- Materialised by the install orchestrator to give the MSP a
-- first-class companies row. The zitadel_org_id supplied here is the
-- workspace org id — no new ZITADEL organisation is created.
-- is_workspace_owner = TRUE is what the partial unique index in 00004
-- enforces, so a buggy code path can't accidentally insert a second
-- row of this kind. Address fields default to empty; the operator
-- fills them later via UpdateWorkspaceCompany.
INSERT INTO companies (name, zitadel_org_id, is_workspace_owner)
VALUES ($1, $2, TRUE)
RETURNING *;

-- name: UpdateCompany :one
-- Updates a non-workspace company. The WHERE clause guards against
-- accidentally editing the MSP row through the generic endpoint — the
-- workspace company uses UpdateWorkspaceCompany so admin-only vs
-- operator flows stay distinct in the app.
UPDATE companies
SET
    name             = $2,
    owner_contact_id = $3,
    address_line1    = $4,
    address_line2    = $5,
    city             = $6,
    region           = $7,
    postal_code      = $8,
    country          = $9,
    timezone         = $10
WHERE id = $1
  AND archived_at IS NULL
  AND is_workspace_owner = FALSE
RETURNING *;

-- name: UpdateWorkspaceCompany :one
-- Updates the singleton MSP row. The WHERE clause guards symmetrically
-- with UpdateCompany: only the is_workspace_owner = TRUE row is
-- eligible, so a stale id cannot accidentally update a customer.
UPDATE companies
SET
    name          = $1,
    address_line1 = $2,
    address_line2 = $3,
    city          = $4,
    region        = $5,
    postal_code   = $6,
    country       = $7,
    timezone      = $8
WHERE is_workspace_owner = TRUE
RETURNING *;

-- name: GetCompany :one
-- Non-archived lookup used by UpdateCompany / UI Detail. LEFT JOIN
-- contacts so the owner's full_name comes back denormalised — saves
-- the SPA a second round-trip.
SELECT
    c.*,
    o.full_name AS owner_full_name
FROM companies c
LEFT JOIN contacts o ON o.id = c.owner_contact_id
WHERE c.id = $1 AND c.archived_at IS NULL;

-- name: GetCompanyIncludingArchived :one
-- Same as GetCompany but includes archived rows. Used by the Detail
-- page so the operator can still see (and Restore) an archived
-- company via a direct link.
SELECT
    c.*,
    o.full_name AS owner_full_name
FROM companies c
LEFT JOIN contacts o ON o.id = c.owner_contact_id
WHERE c.id = $1;

-- name: GetWorkspaceCompany :one
-- Returns the singleton MSP row. Used by /settings/workspace (Slice 5).
SELECT *
FROM companies
WHERE is_workspace_owner = TRUE
LIMIT 1;

-- name: ListCompanies :many
-- Excludes the MSP row so the operator-facing companies list never
-- shows a customer-shaped record that isn't a customer. The owner's
-- full_name comes back denormalised via LEFT JOIN.
SELECT
    c.*,
    o.full_name AS owner_full_name
FROM companies c
LEFT JOIN contacts o ON o.id = c.owner_contact_id
WHERE c.is_workspace_owner = FALSE
ORDER BY c.created_at DESC;

-- name: ArchiveCompany :exec
UPDATE companies
SET archived_at = now()
WHERE id = $1
  AND archived_at IS NULL
  AND is_workspace_owner = FALSE;

-- name: RestoreCompany :one
-- Reverses ArchiveCompany. Rejects the MSP row symmetrically with
-- ArchiveCompany — the workspace row is never archived, so restoring
-- it must be a bug.
UPDATE companies
SET archived_at = NULL
WHERE id = $1
  AND archived_at IS NOT NULL
  AND is_workspace_owner = FALSE
RETURNING *;
