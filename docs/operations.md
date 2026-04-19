# Operations & scenarios

Explicit list of the scenarios the local dev flow and the container/
Kubernetes flow cover, with the expected behaviour and recovery paths
for each. Read this before wondering "it worked yesterday, why not
now" — the answer is almost always in one of the scenarios below.

State lives in three places:

- **Postgres** (named volume `gospa_postgres_data`): Gospa's workspace
  and companies rows, plus ZITADEL's own database. Wiped by
  `mise run infra:reset` in dev; by the operator in K8s.
- **ZITADEL secret volume** (named volume `gospa_zitadel_secrets`):
  the provisioner PAT materialised by `FirstInstance.PatPath`. Wiped
  alongside the Postgres volume on reset.
- **Host file** `./.secrets/zitadel-provisioner.pat`: the app reads
  this via `GOSPA_ZITADEL_PROVISIONER_PAT_FILE`. Always re-copied
  from the named volume on `mise run infra`.

The container image has **no state**. Everything survives or dies
with the three locations above.

---

## Local dev scenarios

The three-command loop is the same every time; these scenarios
describe what each command actually does under different starting
conditions.

    mise run infra     # (re)bring up services + materialise PAT
    mise run dev       # backend + frontend in parallel
    # Ctrl+C stops both

### Design note — install completion is a page-lifetime boundary

The moment `install_state` transitions to `ready` three things change
at once:

1. **Workspace row** now carries `zitadel_org_id`,
   `zitadel_project_id`, `zitadel_spa_app_id`, `zitadel_spa_client_id`.
2. **Auth gate** (server-side middleware) flips from pass-through to
   JWT-validating via `OnReady → gate.Activate`. Protected RPCs now
   require a Bearer token.
3. **Public runtime config** served at `/_gofra/config.js` — which
   the browser already has loaded as a snapshot from page boot — is
   now *semantically* different: `auth.orgId` would return the new
   org id if re-fetched. But the snapshot in `window.__GOFRA_CONFIG__`
   is stale.

The SPA handles this by triggering a **full page reload** (not SPA
navigation) the instant `GetStatus` reports `READY`:

```tsx
// web/src/routes/install.tsx
useEffect(() => {
  if (statusQuery.data?.state === "INSTALL_STATE_READY") {
    window.location.assign("/");
  }
}, [statusQuery.data?.state]);
```

`window.location.assign` re-executes every `<script>` tag, so the
fresh `/_gofra/config.js` overwrites `window.__GOFRA_CONFIG__` with
post-install values including `auth.orgId`. React remounts. Home page
reads the live orgId. Login button is active.

**Three defensive layers keep login working even if the reload path
is bypassed:**

1. The install wizard's `window.location.assign("/")` on READY — the
   primary mechanism.
2. The `/` route loader always calls `GetStatus` and carries
   `zitadelOrgId` in loader data. The home page compares that against
   `runtimeConfig.auth?.orgId`. If they disagree (stale snapshot),
   the home page forces `window.location.reload()` to re-sync.
3. `startLogin({ orgIdOverride })` accepts an explicit orgId so the
   home page can pass the loader's live value directly to the OIDC
   URL builder. Even if both reload paths fail (extensions, edge
   cases), login still builds correctly.

Backend side: every `GET /_gofra/config.js` re-runs the
`publicconfig.Handler` mutator which re-reads the workspace row. The
response is never cached at the framework level, so "reload to
refresh" is authoritative. See `internal/publicconfig/resolver.go`
and its tests.

No SPA state is lost across the install-completion reload because the
operator just completed a one-shot wizard; nothing worth preserving
is in flight.

### Scenario A — Fresh clone, never run before

Starting state: repo just cloned, no Docker volumes, no `.secrets/`.

    mise run infra
    mise run dev

**What happens, step by step:**

1. `mise run infra`:
   - Compose creates `gospa_postgres_data` and `gospa_zitadel_secrets`.
   - Postgres boots; Zitadel runs `start-from-init --steps steps.yaml`
     and writes the provisioner PAT inside the container at
     `/zitadel-secrets/zitadel-provisioner.pat`.
   - `wait-for-postgres.sh` + `wait-for-zitadel.sh` hold until both
     services report healthy.
   - `copy-provisioner-pat.sh` uses `compose cp` to extract the PAT
     into `./.secrets/zitadel-provisioner.pat` on the host (`chmod 600`).

2. `mise run dev`:
   - `scripts/load-env.sh` sets `GOSPA_ZITADEL_PROVISIONER_PAT_FILE`
     to the host PAT path.
   - The Go binary reads the PAT, validates it is non-empty, runs
     migrations (`workspace` + `companies` created), loads the
     singleton workspace row (state = `not_initialized`), mounts the
     auth gate in pass-through mode. Vite starts in parallel.

3. Browser to `http://localhost:3000`:
   - TanStack Router calls `GetStatus`, sees `not_initialized`,
     redirects to `/install`.
   - Fill the wizard (workspace name/slug/timezone/currency +
     initial admin's email and name). Submit.
   - Orchestrator calls ZITADEL: `SetUpOrg` → `AddProject` →
     `AddOIDCApp` → `PersistZitadelIDs` → `MarkWorkspaceReady` →
     `OnReady` flips the auth gate to authenticated.
   - Status polling sees `ready`, redirects to `/`.
   - "Log in" button is active (auth.orgId is populated).

**Expected duration:** ~30 seconds from `mise run infra` to SPA
rendering `/install`; install itself is 2–5 seconds depending on
ZITADEL latency.

### Scenario B — Second run, everything persisted

Starting state: Scenario A completed. Containers maybe stopped
(`mise run infra:stop`) or running.

    mise run infra
    mise run dev

**What happens:**

1. `mise run infra`:
   - Compose sees the volumes and (re)starts the containers. Postgres
     keeps the workspace + companies rows. Zitadel skips its init
     because the instance already exists; the PAT file inside the
     volume is the same one from Scenario A.
   - `copy-provisioner-pat.sh` runs anyway (no early-exit) and copies
     the same PAT content to `./.secrets/zitadel-provisioner.pat`.

2. `mise run dev`:
   - Same PAT, same workspace row with `install_state = ready`,
     same `zitadel_project_id`.
   - Startup reads the row and **activates the gate eagerly** — auth
     is on from the first request.

3. Browser to `http://localhost:3000`:
   - `GetStatus` returns `ready`, router stays on `/`. Login works.

### Scenario C — Full reset (`mise run infra:reset`)

Starting state: working setup you want to wipe (bad state, changing
ZITADEL version, just experimenting).

    mise run infra:reset
    mise run infra
    mise run dev

**What `infra:reset` does:**

- `docker compose down --volumes --remove-orphans` → deletes
  `gospa_postgres_data` AND `gospa_zitadel_secrets`. All rows gone.
- `rm -f .secrets/zitadel-provisioner.pat` → stale host PAT gone.
- `rm -rf .secrets/zitadel-provisioner-raw` → legacy bind-mount
  leftover from older checkouts (harmless when absent).

From there the next `mise run infra` is equivalent to Scenario A:
Zitadel re-bootstraps, new PAT, `install_state` starts at
`not_initialized` again.

### Scenario D — Stop without wipe (`mise run infra:stop`)

    mise run infra:stop       # stops containers, volumes intact
    mise run infra            # brings them back up

Volumes persist, so this is Scenario B. Useful when you need the host
resources back briefly.

### Scenario E — Process crashed mid-install

Starting state: you were at `/install`, hit submit, and killed
`mise run dev` (or it crashed) before the orchestrator reached
`MarkWorkspaceReady`. The workspace row is stuck in `provisioning`.

    mise run dev        # just restart

**What happens:**

- Startup detects `install_state = provisioning`, automatically flips
  it to `failed` with `previous process exited during provisioning;
  retry from /install`, and logs:

      level=WARN msg="workspace was stuck in provisioning;
      transitioned to failed so /install can retry"

- The install wizard accepts the workspace again and lets you submit.

**Caveat:** if the previous attempt had already called `AddOrganization`
in ZITADEL, that org is orphaned in ZITADEL — the retry creates a new
one. Clean up manually in the ZITADEL console if it matters. This is
documented as an MVP debt; a Restate saga would eliminate it.

### Scenario F — PAT rotated manually in the ZITADEL console

Starting state: you generated a new PAT in ZITADEL's UI and revoked
the old one. The app is still holding the old token.

Symptoms:

    create_org: zitadel SetUpOrg: zitadel returned 401 Unauthorized
    # or similar on company creation

**Recovery paths:**

- **Easy:** `mise run infra:reset && mise run infra && mise run dev`
  — lose everything, start over. Fine in dev.
- **Targeted:** write the new token directly to the host PAT file:

      echo -n "$NEW_PAT" > .secrets/zitadel-provisioner.pat
      chmod 600 .secrets/zitadel-provisioner.pat

  Then restart `mise run dev`. ZITADEL's container still has the
  FirstInstance PAT in its volume, so the next `mise run infra` would
  overwrite with that — avoid running `infra` until you're done with
  the manual PAT.

### Scenario G — ZITADEL image version change

Starting state: you bumped `GOSPA_ZITADEL_IMAGE` in `compose.yaml`
(or ZITADEL's `:stable` tag floated to a new major that doesn't read
the existing volume format).

**Symptoms:** ZITADEL container crash-loops with schema errors.

**Recovery:** `mise run infra:reset` and re-run. Workspace state is
lost. For real deployments, follow the ZITADEL upgrade guide before
touching the image.

### Scenario H — Stale PAT file at a wrong path (legacy)

If you cloned Gospa before the `load-env.sh` path fix (commit
`3712700`), the PAT may have been written one directory above its
intended location, e.g. `<workspace>/repos/.secrets/...`. Delete it
once:

    rm -f ../.secrets/zitadel-provisioner.pat
    rmdir ../.secrets 2>/dev/null || true
    mise run infra:reset
    mise run infra

---

## Kubernetes deployment scenarios

The manifest set at `docs/examples/deploy/kubernetes/` deliberately
delegates ZITADEL and Postgres to the operator. Only the Gospa
container and its two Secrets (provisioner PAT + DB DSN) are in
scope.

### Scenario K1 — First deploy

Starting state: brand-new cluster namespace, no workspace row yet.

1. Operator creates Secrets:

       kubectl create secret generic gospa-zitadel-provisioner \
         --from-literal=pat="<IAM_OWNER PAT>"
       kubectl create secret generic gospa-database \
         --from-literal=dsn="postgres://..."

2. Operator sets env values in `deployment.yaml` to match the cluster:
   `GOFRA_ZITADEL__ADMIN_API_URL`, `GOFRA_PUBLIC__API_BASE_URL`,
   `GOFRA_PUBLIC__AUTH__ISSUER`.
3. `kubectl apply -f docs/examples/deploy/kubernetes/`. Pod starts,
   validates the PAT file, runs migrations on the managed DB,
   workspace row starts `not_initialized`, auth gate is
   pass-through. Probes flip to ready.
4. Operator **restricts Ingress** to their own IP / VPN / port-forward.
5. Operator opens the URL, fills the wizard. Orchestrator provisions
   ZITADEL resources. `OnReady` activates the auth gate **in-place**
   — the same pod now authenticates requests. No `kubectl rollout
   restart`.
6. Operator removes the Ingress restriction.

### Scenario K2 — Pod crash / rolling update

Starting state: workspace already `ready`. Image update,
`kubectl rollout restart`, or a liveness-probe failure.

- New pod boots, reads PAT, reads workspace row, sees `ready` +
  populated `zitadel_project_id`, activates the auth gate eagerly at
  startup. Probes flip to ready.
- No manual steps. Login continues to work throughout (tokens issued
  by ZITADEL survive the pod restart).

### Scenario K3 — Pod restart during install

Starting state: operator submitted the wizard, the pod died before
`MarkWorkspaceReady` (node failure, OOMKill, whatever).

- New pod boots, detects `install_state = provisioning`, auto-
  transitions to `failed` with a recovery note, and continues
  serving in pass-through.
- Operator retries the wizard. Same caveat as Scenario E about
  orphaned ZITADEL orgs.

### Scenario K4 — PAT rotation

Starting state: PAT compromised or age-policy triggered rotation.

1. Operator generates a new PAT in ZITADEL (same machine user keeps
   its IAM_OWNER grant).
2. `kubectl patch secret gospa-zitadel-provisioner --type=json \
      -p='[{"op":"replace","path":"/data/pat","value":"<base64>"}]'`
3. The kubelet refreshes the projected volume; `internal/patwatch`
   in cmd/app sees the basename change inside the parent directory
   (Kubernetes rotates Secrets via atomic symlink swap) and reloads
   the value into memory. The next ZITADEL request — provisioning,
   company creation — uses the new PAT. No restart, no dropped
   request, no manual step beyond the `kubectl patch` above.

If the new file is briefly empty or unreadable mid-rotation, the
watcher keeps the previous PAT (last-known-good) and logs a
warning; runtime never sees an empty bearer.

`kubectl rollout restart deployment/gospa` still works as a fallback
when the operator wants to force-reset the process for unrelated
reasons, but it is no longer required for rotation.

### Scenario K5 — Scaling up after install

    kubectl scale deployment/gospa --replicas=3

Each replica independently reads the workspace row at startup,
activates its gate eagerly, and serves authenticated traffic. No
coordination needed — the gate is process-local and the workspace
row is the shared source of truth.

**Do not** scale up during install. The orchestrator's in-process
single-flight guard is per-pod; parallel pods could each call
`AddOrganization` and create duplicate ZITADEL state. Wait until
`install_state = ready`.

### Scenario K6 — Migrating to a new ZITADEL instance

Starting state: you stood up a different ZITADEL (upgrade,
cloud-to-self-host, org split). The old workspace's
`zitadel_project_id` does not exist in the new instance.

This is **not a supported upgrade path in the MVP**. The workspace
row is tightly coupled to the original ZITADEL's identifiers.
Options:

- Nuke the workspace row and the companies rows, update the Secret
  with a PAT on the new ZITADEL, redeploy. The SPA goes through
  `/install` again.
- Script your own migration: create an equivalent org/project/app on
  the new ZITADEL, update the four `zitadel_*_id` columns on the
  workspace row, rotate the PAT, restart. Untested — you are on
  your own until a migration story lands.

### Scenario K7 — Multi-tenant / multi-deploy

Starting state: you want several Gospa instances (one per MSP) in
the same cluster.

Each Gospa deploys is **one MSP ↔ one deploy** — that's the entire
product direction. Run a separate Namespace (or separate cluster)
per MSP, each with its own ZITADEL org, provisioner PAT, Postgres,
and Ingress. The manifests in `docs/examples/deploy/kubernetes/`
work as the per-tenant starting point.

Do not try to share a Gospa pod across MSPs; the workspace is a
singleton-by-design (`CHECK id = 1` on the workspace table).

---

## Troubleshooting reference

### `zitadel provisioner PAT unavailable; refusing to start`

cmd/app's fail-fast hit. Either:

- Path mismatch: check `GOSPA_ZITADEL_PROVISIONER_PAT_FILE` env
  matches where the file actually lives.
- File empty: ZITADEL didn't write the PAT. Check
  `sh ./scripts/compose.sh logs zitadel` for errors.
- Permission denied reading: common on K8s when the Secret volume's
  `defaultMode` is too restrictive for the container user; set
  `mode: 0400` explicitly in the volume item spec.

### `401 Unauthorized` on /install from ZITADEL

The PAT the app holds is not recognised by ZITADEL. Causes (ranked
by frequency):

- Stale PAT file (Scenario C was skipped after a reset). Fix:
  `mise run infra:reset && mise run infra && mise run dev`.
- PAT rotated/revoked in ZITADEL UI (Scenario F). Re-copy or reset.
- Copy script's idempotency short-circuit (fixed in commit
  `3712700`). If you're on an older checkout, pull.

### Install stuck in `provisioning`

As of commit `3712700`, startup auto-transitions this to `failed`
with a recovery note. If you're on an older binary, manually:

    psql "$DATABASE_URL" -c "UPDATE workspace SET install_state = 'failed',
      install_error = 'manual recovery' WHERE id = 1;"

Then retry via the `/install` UI.

### "Permission denied" writing the PAT (legacy bind-mount error)

You are on a pre-fix checkout where the compose bind-mounted
`./.secrets/zitadel-provisioner-raw:/zitadel-secrets`. Pull the fix
(named volume + `compose cp`, commit `9c504ee`), then
`mise run infra:reset && mise run infra`.

---

## Related docs

- [`README.md`](../README.md) — quick-start + task reference.
- [`docs/blueprint/index.md`](blueprint/index.md) — product direction
  and MVP debts (Apêndice C).
- [`docs/examples/deploy/kubernetes/README.md`](examples/deploy/kubernetes/README.md)
  — K8s manifest walkthrough.
- [`docs/examples/deploy/compose/README.md`](examples/deploy/compose/README.md)
  — evaluation-only Compose quickrun (does NOT include ZITADEL).
