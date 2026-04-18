# Gospa

Gospa is an open-source PSA (Professional Services Automation) for MSPs,
built on top of the [Gofra](https://github.com/Gabrielbdd/gofra) Go framework.

The product vision, market analysis, feature scope, and architectural
roadmap live in [`docs/blueprint/index.md`](docs/blueprint/index.md). This
README covers how to run, test, and build what exists today.

## Run with Docker Compose

The fastest way to try Gospa locally:

```bash
docker compose --profile app up -d
docker compose --profile app logs -f app   # follow startup logs
```

This pulls `ghcr.io/gabrielbdd/gospa:edge`, starts it alongside a local
PostgreSQL, applies migrations automatically on startup, and exposes the
app at <http://localhost:3000>.

**What to expect right now.** Gospa is still early bootstrap. Landing at
<http://localhost:3000> today shows a placeholder page from the framework
starter, not the full PSA product described in
[`docs/blueprint/index.md`](docs/blueprint/index.md). If you want to see
the scaffold wired up end-to-end — HTTP server, health probes, Postgres,
migrations, config handler — this is the fastest way. If you want to see
product features (tickets, billing, etc.), those are not implemented yet.

Check the app is responding:

```bash
curl http://localhost:3000/readyz              # 200 when Postgres is reachable
curl http://localhost:3000/livez               # 200 while the process runs
curl http://localhost:3000/_gofra/config.js    # browser-safe runtime config
```

To pin a specific version, edit the `image:` line in `compose.yaml`
(for example `ghcr.io/gabrielbdd/gospa:v0.1.0` or
`ghcr.io/gabrielbdd/gospa:sha-abc1234`). The Compose file is the source
of truth — no environment variable indirection is exposed for this.

Update to the latest `edge`:

```bash
docker compose --profile app pull
docker compose --profile app up -d
```

Stop it:

```bash
docker compose --profile app down              # stop + keep Postgres data
docker compose --profile app down --volumes    # stop + wipe Postgres volume
```

**Platform note.** Images are published for `linux/arm64` only in this
iteration. If you need `linux/amd64`, build from source with
`docker build -t gospa:dev .` for now; multi-arch publishing is a
follow-up.

**First-time caveat.** The `edge` tag only exists after the publish
workflow runs on `main` for the first time. If you clone before that and
`docker compose --profile app up -d` fails with a "manifest not found"
error, pick a specific tag from the
[Packages page](https://github.com/Gabrielbdd/gospa/pkgs/container/gospa)
and edit the `image:` line in `compose.yaml` to use it.

Available tags:

- `edge` — rolling latest on `main`
- `sha-<short>` — immutable per commit
- `vX.Y.Z`, `vX.Y`, `latest` — on GitHub releases

The "Run" section below covers the development workflow (app on host,
Postgres via Compose) — use that when you want to modify Gospa, not just
run it.

## Current Scope

Gospa is still in early bootstrap. Today the app is the framework's canonical
starter applied with `--module github.com/Gabrielbdd/gospa` — enough to prove
the framework contract, not yet the product described in the blueprint.

Today the app provides:

- a runnable Go HTTP server in `cmd/app` using chi, with health check probes
  and graceful shutdown via Gofra's `runtime/health` and `runtime/serve`
- a proto-driven config schema in `proto/gospa/config/v1/config.proto`
- config code generation via `mise run generate` (produces `config/*_gen.go`)
- optional YAML overrides in `gofra.yaml`
- a `compose.yaml` file for local PostgreSQL with a named volume and healthcheck
- `mise run infra` tasks that work with either Docker Compose or Podman Compose
- a minimal embedded web shell in `web/`
- health check endpoints at `/startupz`, `/livez`, `/readyz` (Kubernetes convention)
- a multi-stage `Dockerfile` producing a static distroless image
- `.github/workflows/ci.yml` running tests, build, and a local image build

Config fields, defaults, and descriptions are defined once in the proto file.
Run `mise run generate` after editing the proto to regenerate the Go code.

## Run

```bash
mise trust
mise run infra
mise run migrate
mise run dev
```

`mise run dev` depends on `mise run generate`, so config code is always
up-to-date before the server starts.

`mise run infra` starts PostgreSQL through either `docker compose` or
`podman compose`, then waits until the database accepts connections.

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
| `mise run infra` | Start local infrastructure (Postgres) via Compose. |
| `mise run infra:stop` / `infra:reset` / `infra:logs` | Manage local infrastructure. |
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
