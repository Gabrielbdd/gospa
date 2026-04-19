# Quick-run Gospa with Docker Compose

Run Gospa + PostgreSQL with two downloaded files and one command.

> **This is not a production deployment.** It exists so you can try
> Gospa in a minute without cloning the repo or building anything. It
> uses default credentials, no TLS, no secrets management, no backups,
> and rolling `:edge` images. Treat it as an evaluation harness for
> your own laptop, nothing more.
>
> **Identity is not configured here.** The quickrun compose starts
> only PostgreSQL and the Gospa binary — it does NOT launch ZITADEL
> and does NOT materialise the provisioner PAT file that Gospa
> requires at startup. The container will exit `1` on first boot
> with a clear error about a missing PAT file. For a runnable local
> setup, clone the repo and run `mise run infra` (which brings up
> ZITADEL alongside Postgres and produces the PAT via the
> `FirstInstance.PatPath` bootstrap). A future quickrun variant
> that bundles ZITADEL will ship with the Gospa documentation site.
>
> This directory is a placeholder until Gospa has its own documentation
> site with real deployment guides.

## Run

```bash
mkdir gospa-quickrun && cd gospa-quickrun

wget https://raw.githubusercontent.com/Gabrielbdd/gospa/main/docs/examples/deploy/compose/compose.yaml
wget https://raw.githubusercontent.com/Gabrielbdd/gospa/main/docs/examples/deploy/compose/compose.env

docker compose --env-file compose.env up -d
```

Follow the startup:

```bash
docker compose --env-file compose.env logs -f app
```

Check it is responding:

```bash
curl http://localhost:3000/readyz              # 200 when Postgres is reachable
curl http://localhost:3000/livez               # 200 while the process runs
curl http://localhost:3000/_gofra/config.js    # browser-safe runtime config
```

Visit <http://localhost:3000>.

## Update

```bash
docker compose --env-file compose.env pull
docker compose --env-file compose.env up -d
```

## Stop

```bash
docker compose --env-file compose.env down              # stop, keep Postgres data
docker compose --env-file compose.env down --volumes    # stop, wipe Postgres data
```

## Tune

Edit `compose.env` to override any of:

| Variable | Default | Purpose |
| --- | --- | --- |
| `GOSPA_IMAGE` | `ghcr.io/gabrielbdd/gospa:edge` | Image tag. Pin to `:v0.1.0`, `:sha-...` from the [Packages page](https://github.com/Gabrielbdd/gospa/pkgs/container/gospa). |
| `POSTGRES_IMAGE` | `postgres:18.3-alpine3.23` | PostgreSQL image. |
| `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` | `postgres` / `postgres` / `gospa` | Database credentials. Defaults are insecure. |
| `APP_PORT` | `3000` | Host port for the Gospa app. |

The compose.yaml in this directory is self-contained — no other files
are needed.

## Platform

`ghcr.io/gabrielbdd/gospa` is published for `linux/arm64` only in this
iteration. Hosts running on `linux/amd64` will fail with
"no matching manifest". For amd64, build from source with
`docker build -t gospa .` in a gospa checkout; multi-arch publish is a
follow-up.

## First-time caveat

The `edge` tag exists only after the publish workflow runs on `main`
at least once. If `docker compose` fails with "manifest not found",
pick a specific tag from the [Packages page] and set `GOSPA_IMAGE` in
`compose.env` accordingly.

## Status

Gospa is in early bootstrap. <http://localhost:3000> today shows only
the framework starter's placeholder page, not the PSA product
described in the [product blueprint](../../../blueprint/index.md). Use
this quickrun to validate the runtime contract (HTTP server, health
probes, Postgres, auto-migrations, runtime config), not to evaluate
product features.

## What replaces this

When Gospa ships its own documentation site, real deployment guides
will live there — covering TLS, secrets handling, auth, observability,
backup, and minimal production-safe manifests. This directory will
then go away or point at those guides.

[Packages page]: https://github.com/Gabrielbdd/gospa/pkgs/container/gospa
