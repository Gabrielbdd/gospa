-- name: CreateCompany :one
INSERT INTO companies (name, slug, zitadel_org_id)
VALUES ($1, $2, $3)
RETURNING *;

-- name: CreateWorkspaceCompany :one
-- Materialised by the install orchestrator to give the MSP a
-- first-class companies row. The zitadel_org_id supplied here is the
-- workspace org id — no new ZITADEL organisation is created.
-- is_workspace_owner = TRUE is what the partial unique index in 00004
-- enforces, so a buggy code path can't accidentally insert a second
-- row of this kind.
INSERT INTO companies (name, slug, zitadel_org_id, is_workspace_owner)
VALUES ($1, $2, $3, TRUE)
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
