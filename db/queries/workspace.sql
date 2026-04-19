-- name: GetWorkspace :one
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
    initialized_at
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
    zitadel_org_id        = $1,
    zitadel_project_id    = $2,
    zitadel_spa_app_id    = $3,
    zitadel_spa_client_id = $4
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
