-- name: GetWorkspace :one
-- Column order intentionally matches the workspace table's column order
-- (base columns from 00001, the auth-contract columns added in 00003)
-- so sqlc returns the canonical Workspace model rather than a
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
-- slug column is no longer written by the install flow (Wave 1 of
-- slug removal). Wave 2 drops the column entirely.
UPDATE workspace
SET
    name          = $1,
    timezone      = $2,
    currency_code = $3,
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
