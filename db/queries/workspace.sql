-- name: GetWorkspace :one
-- Column order intentionally matches the workspace table's column order
-- (base columns from 00001, then the auth-contract columns added in
-- 00003) so sqlc returns the canonical Workspace model rather than a
-- query-specific row type.
SELECT
    id,
    name,
    slug,
    timezone,
    currency_code,
    install_state,
    install_error,
    zitadel_org_id,
    zitadel_project_id,
    zitadel_spa_app_id,
    zitadel_spa_client_id,
    created_at,
    initialized_at,
    zitadel_issuer_url,
    zitadel_management_url,
    zitadel_api_audience
FROM workspace
WHERE id = 1;

-- name: MarkWorkspaceProvisioning :exec
UPDATE workspace
SET
    name          = $1,
    slug          = $2,
    timezone      = $3,
    currency_code = $4,
    install_state = 'provisioning',
    install_error = NULL
WHERE id = 1;

-- name: PersistZitadelIDs :exec
UPDATE workspace
SET
    zitadel_org_id         = $1,
    zitadel_project_id     = $2,
    zitadel_spa_app_id     = $3,
    zitadel_spa_client_id  = $4,
    zitadel_issuer_url     = $5,
    zitadel_management_url = $6,
    zitadel_api_audience   = $7
WHERE id = 1;

-- name: RepairWorkspaceAuthContract :exec
-- Idempotent fill-in for already-installed workspaces that pre-date the
-- explicit auth contract columns. COALESCE keeps any persisted value
-- and only writes the supplied default when the column is currently
-- NULL. Pass pgtype.Text{Valid: false} for fields the caller cannot
-- safely derive (e.g. audience when both cfg.Auth.Audience and
-- workspace.zitadel_project_id are empty) and they will be left NULL.
UPDATE workspace
SET
    zitadel_issuer_url     = COALESCE(zitadel_issuer_url, $1),
    zitadel_management_url = COALESCE(zitadel_management_url, $2),
    zitadel_api_audience   = COALESCE(zitadel_api_audience, $3)
WHERE id = 1;

-- name: MarkWorkspaceReady :exec
UPDATE workspace
SET
    install_state  = 'ready',
    install_error  = NULL,
    initialized_at = now()
WHERE id = 1;

-- name: MarkWorkspaceFailed :exec
UPDATE workspace
SET
    install_state = 'failed',
    install_error = $1
WHERE id = 1;
