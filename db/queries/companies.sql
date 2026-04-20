-- name: CreateCompany :one
-- slug column defaults to ''. Wave 2 of the slug removal plan drops
-- the column entirely; until then the handler never writes a
-- meaningful slug value.
INSERT INTO companies (
    name, zitadel_org_id,
    address_line1, address_line2, city, region, postal_code, country, timezone
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
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
    name          = $2,
    address_line1 = $3,
    address_line2 = $4,
    city          = $5,
    region        = $6,
    postal_code   = $7,
    country       = $8,
    timezone      = $9
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
SELECT *
FROM companies
WHERE id = $1 AND archived_at IS NULL;

-- name: GetWorkspaceCompany :one
-- Returns the singleton MSP row. Used by /settings/workspace (Slice 5).
SELECT *
FROM companies
WHERE is_workspace_owner = TRUE
LIMIT 1;

-- name: ListCompanies :many
-- Excludes the MSP row so the operator-facing companies list never
-- shows a customer-shaped record that isn't a customer.
SELECT *
FROM companies
WHERE archived_at IS NULL
  AND is_workspace_owner = FALSE
ORDER BY created_at DESC;

-- name: ArchiveCompany :exec
UPDATE companies
SET archived_at = now()
WHERE id = $1
  AND archived_at IS NULL
  AND is_workspace_owner = FALSE;
