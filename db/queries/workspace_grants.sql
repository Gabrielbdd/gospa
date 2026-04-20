-- name: CreateWorkspaceGrant :one
-- Inserts the grant for a contact. Status defaults to 'active' for the
-- install-time admin grant; the team invite flow will pass
-- 'not_signed_in_yet' for new members.
INSERT INTO workspace_grants (
    contact_id,
    role,
    status,
    granted_by_contact_id
)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetGrantByContactID :one
SELECT *
FROM workspace_grants
WHERE contact_id = $1;

-- name: ResolveTeamCallerByZitadelUserID :one
-- Single-query authz resolution used by internal/authz/middleware.
-- Joins contacts to its (optional) workspace_grants row, returning
-- everything the middleware needs in one round-trip:
--   * contact_id      — request context
--   * role            — policy enforcement
--   * grant_status    — suspended / not_signed_in_yet / active branches
--   * last_seen_at    — throttled update decision
-- LEFT JOIN keeps a contact-without-grant valid; the middleware then
-- decides 401 vs 403 based on whether the grant exists.
SELECT
    c.id            AS contact_id,
    c.archived_at   AS contact_archived_at,
    g.role          AS grant_role,
    g.status        AS grant_status,
    g.last_seen_at  AS grant_last_seen_at
FROM contacts AS c
LEFT JOIN workspace_grants AS g ON g.contact_id = c.id
WHERE c.zitadel_user_id = $1
LIMIT 1;

-- name: CountActiveAdmins :one
-- Used by the last-admin invariant to short-circuit demote/suspend/
-- archive operations that would leave zero active admins. The partial
-- index workspace_grants_active_admins from 00007 makes this a
-- micro-cheap lookup.
SELECT COUNT(*)::BIGINT AS count
FROM workspace_grants
WHERE role = 'admin'
  AND status = 'active';

-- name: UpdateGrantRole :exec
UPDATE workspace_grants
SET role = $2
WHERE contact_id = $1;

-- name: UpdateGrantStatus :exec
UPDATE workspace_grants
SET status = $2
WHERE contact_id = $1;

-- name: ActivatePendingGrant :exec
-- Idempotent first-sign-in transition. Only flips when the grant is
-- currently 'not_signed_in_yet', so concurrent activations cannot
-- accidentally clobber a later 'suspended' state. Called by the authz
-- middleware on every authenticated request whose grant is pending —
-- the WHERE clause makes it a no-op for already-active rows.
UPDATE workspace_grants
SET status = 'active'
WHERE contact_id = $1
  AND status = 'not_signed_in_yet';

-- name: TouchLastSeen :exec
-- Throttled in-memory by the middleware; this query is fired only when
-- the in-memory check decides the persisted value is stale. Setting
-- last_seen_at = now() unconditionally is intentional: the throttle
-- already absorbed the high-frequency case.
UPDATE workspace_grants
SET last_seen_at = now()
WHERE contact_id = $1;
