# Gospa

Gospa is an open-source PSA (Professional Services Automation) for MSPs,
built on top of the [Gofra](https://github.com/Gabrielbdd/gofra) Go framework.

The product vision, market analysis, feature scope, and architectural
roadmap live in [`docs/blueprint/index.md`](docs/blueprint/index.md). This
README covers how to run, test, and build what exists today.

## Try it quickly (not production)

If you just want to see Gospa running, a self-contained Docker Compose
example lives at
[`docs/examples/deploy/compose/`](docs/examples/deploy/compose/):

```bash
mkdir gospa-quickrun && cd gospa-quickrun
wget https://raw.githubusercontent.com/Gabrielbdd/gospa/main/docs/examples/deploy/compose/compose.yaml
wget https://raw.githubusercontent.com/Gabrielbdd/gospa/main/docs/examples/deploy/compose/compose.env
docker compose --env-file compose.env up -d
```

Visit <http://localhost:3000>. See
[`docs/examples/deploy/compose/README.md`](docs/examples/deploy/compose/README.md)
for update, stop, tuning, caveats, and the `linux/arm64`-only note.

> **Not a production deployment.** Default credentials, no TLS, no
> secrets management, rolling `:edge` images. Use it to evaluate; do
> not run it as production. Real deployment guides will ship with the
> Gospa documentation site.

Gospa is still early bootstrap. <http://localhost:3000> today walks
you through the `/install` wizard (operator-supplied install token,
admin password, ZITADEL provisioning), then renders a tiny home page
with sign-in / sign-out and a "Companies probe" panel that exercises
an authenticated RPC end-to-end. The PSA product itself
(tickets, time tracking, billing, etc., described in the
[product blueprint](docs/blueprint/index.md)) is still future work
— the sections below document local development against the source.

## Current Scope

Gospa is still in early bootstrap. Today the app is the framework's canonical
starter applied with `--module github.com/Gabrielbdd/gospa` — enough to prove
the framework contract, not yet the product described in the blueprint.

Today the app provides:

- a runnable Go HTTP server in `cmd/app` using chi, with health check probes
  and graceful shutdown via Gofra's `runtime/health` and `runtime/serve`
- a proto-driven config schema in `proto/gospa/config/v1/config.proto`
  (Go + TS codegen via `mise run generate`)
- a `compose.yaml` with PostgreSQL and ZITADEL side-by-side, wired so
  `mise run infra` materialises a provisioner PAT under
  `./.secrets/zitadel-provisioner.pat` on first run
- a hard startup contract: Gospa refuses to start without a valid PAT
  file at `GOSPA_ZITADEL_PROVISIONER_PAT_FILE` (local dev reads from
  `./.secrets/...`, Kubernetes reads from a mounted Secret)
- workspace singleton + companies tables, with eager ZITADEL org
  creation per company
- a first-run `/install` wizard (React + TanStack Router/Query/Form,
  Tailwind v4, shadcn/ui) that provisions the MSP org, initial admin,
  project, and OIDC SPA application in ZITADEL
- OIDC login scoped to the workspace's ZITADEL org via
  `urn:zitadel:iam:org:id:<orgID>`
- health check endpoints at `/startupz`, `/livez`, `/readyz`
- a multi-stage `Dockerfile` (Node → Go → distroless) producing a
  static image with the SPA embedded
- `.github/workflows/ci.yml` running tests, build, and a local image
  build

Config fields, defaults, and descriptions are defined once in the proto file.
Run `mise run generate` after editing the proto to regenerate the Go code.

Remaining MVP debts (single PAT keeps IAM_OWNER on both bootstrap
and runtime paths even though rotation is now hot-reloaded, no authz
yet, Restate deferred for kernel-kill mid-install recovery,
single-replica install, companies handler still has no
compensation when its post-org INSERT fails) are recorded in
[`docs/blueprint/index.md` § Apêndice C](docs/blueprint/index.md#apêndice-c--mvp-debts-identidade--onboarding).

[`docs/operations.md`](docs/operations.md) enumerates every dev and
Kubernetes scenario explicitly — fresh clone, re-run, full reset,
stop-without-wipe, mid-install crash, PAT rotation, ZITADEL image
upgrade, multi-tenant — with the expected behaviour and the recovery
path for each. Read it before asking "why doesn't my state come
back" or "why is ZITADEL returning 401".

## Run

```bash
mise trust
mise run infra
mise run migrate
mise run dev
```

`mise run dev` depends on `mise run generate`, so config code is always
up-to-date before the server starts.

`mise run infra` starts PostgreSQL **and ZITADEL** through either
`docker compose` or `podman compose`, waits until Postgres accepts
connections and ZITADEL returns `/debug/healthz` 200, and materialises
the provisioner PAT at `./.secrets/zitadel-provisioner.pat` via the
ZITADEL `FirstInstance.PatPath` bootstrap. `mise run dev` will refuse
to start if that file is missing or empty.

The default database settings already line up across `compose.yaml`,
`gofra.yaml`, and the migration tasks, so no `.env` file is required for the
out-of-the-box setup. If you need to change the image, port, or credentials,
start from `.env.example`.

For a full clean rebuild of local database state:

```bash
mise run infra:reset
mise run infra
mise run migrate
```

## Tasks

The starter ships with these `mise` tasks:

| Task | Purpose |
| --- | --- |
| `mise run generate` | Regenerate config code from the proto schema. |
| `mise run test` | Run `go test ./...` after regenerating config code. |
| `mise run build` | Build the application binary to `bin/gospa`. |
| `mise run dev` | Start the backend locally (depends on `generate`). |
| `mise run infra` | Start local infrastructure (Postgres + ZITADEL) via Compose and materialise the provisioner PAT. |
| `mise run infra:stop` / `infra:reset` / `infra:logs` | Manage local infrastructure. `infra:reset` wipes the PAT, the install token, and the ZITADEL data volume so the next `infra` starts from scratch. |
| `mise run install:token` | Print the install token from `.secrets/install-token` so you can paste it into the `/install` wizard. |
| `mise run web:dev` | Start the Vite dev server on `:5173` standalone — only needed when iterating on the frontend without the Go backend. `mise run dev` already starts both together. |
| `mise run web:build` | Build the frontend into `web/dist` so the Go binary can embed it. |
| `mise run migrate` / `migrate:create` / `migrate:down` / `migrate:status` | Manage database migrations via `goose`. |
| `mise run seed` | Seed the database with development data. |

## Build a container image

The starter ships a multi-stage `Dockerfile` that produces a static,
distroless binary:

```bash
docker build -t gospa:dev .
```

The resulting image runs as the non-root `nonroot` user and exposes port
`3000`. Override the exposed port if you change `app.port` in `gofra.yaml`.

## CI

The starter also includes `.github/workflows/ci.yml`, which on every pull
request and push to `main`:

1. installs the pinned Go toolchain via `mise`
2. runs `mise run test`
3. runs `mise run build`
4. builds the Docker image locally (without pushing)

That workflow is intentionally quiet on publishing — pushing to a registry
is an opt-in concern, added per project when deployment actually needs it.
