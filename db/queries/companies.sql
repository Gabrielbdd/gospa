-- name: CreateCompany :one
INSERT INTO companies (name, slug, zitadel_org_id)
VALUES ($1, $2, $3)
RETURNING
    id,
    name,
    slug,
    zitadel_org_id,
    created_at,
    archived_at;

-- name: GetCompany :one
SELECT
    id,
    name,
    slug,
    zitadel_org_id,
    created_at,
    archived_at
FROM companies
WHERE id = $1 AND archived_at IS NULL;

-- name: ListCompanies :many
SELECT
    id,
    name,
    slug,
    zitadel_org_id,
    created_at,
    archived_at
FROM companies
WHERE archived_at IS NULL
ORDER BY created_at DESC;

-- name: ArchiveCompany :exec
UPDATE companies
SET archived_at = now()
WHERE id = $1 AND archived_at IS NULL;
