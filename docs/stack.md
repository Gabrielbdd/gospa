# Stack

The complete technology stack of Gospa today, with versions and the
file or package where each piece is wired in. Pinned to what actually
ships at the time of this doc — when versions move, this doc moves
too.

For the framework-side architecture (the runtime packages Gospa
imports from Gofra), see the
[Gofra design docs](../../gofra/docs/00-index.md).

## Toolchain

| Tool | Version | Role | Where |
|---|---|---|---|
| Go | `1.25` | server runtime | `mise.toml [tools]`, `go.mod` |
| Node | `22` | frontend build | `mise.toml [tools]` |
| `mise` | (system) | task runner + tool installer | `mise.toml` |
| `goose` | `3.27.0` | DB migrations CLI | `mise.toml [tools] "ubi:pressly/goose"` |
| `sqlc` | (system) | Go from SQL queries | `sqlc.yaml`, `mise run gen:sql` |
| `buf` | (system) | proto codegen (Go + TS) | `buf.gen.yaml`, `buf.yaml`, `mise run gen:proto` |
| Docker / Podman Compose | (system) | local infra | `compose.yaml`, `scripts/compose.sh` |

## Backend (Go)

| Module | Version | Role | Where |
|---|---|---|---|
| `github.com/Gabrielbdd/gofra` | `v0.1.3-0.20260419...` | framework runtime (config, health, serve, database, auth, zitadel/secret, errors) | `cmd/app/main.go` |
| `github.com/go-chi/chi/v5` | `v5.2.5` | HTTP router + middleware mount | `cmd/app/main.go` |
| `connectrpc.com/connect` | `v1.18.1` | typed RPC server | generated handlers in `gen/gospa/**` |
| `github.com/jackc/pgx/v5` | `v5.9.1` | Postgres driver + pool | via `runtime/database` |
| `github.com/pressly/goose/v3` | `v3.27.0` (indirect) | migration library used by `runtime/database` (auto-migrate path) | `db/migrations/` |
| `github.com/zitadel/oidc/v3` | `v3.47.2` (indirect) | OIDC discovery + JWT verify (used by `runtime/auth`) | `internal/authgate` consumes |
| `github.com/fsnotify/fsnotify` | `v1.9.0` | secret-file rotation watcher | `internal/patwatch/patwatch.go` |
| `github.com/spf13/pflag` | `v1.0.10` (indirect) | CLI flag parsing (via `runtime/config`) | `config/load_gen.go` |

### Gospa-owned internal packages

| Package | Role |
|---|---|
| `internal/authgate` | dynamic gate: inactive (public Connect through, private 401) → JWT-validating after `Activate` |
| `internal/installtoken` | loads + validates the operator-supplied bootstrap secret |
| `internal/patwatch` | observes the PAT file with last-known-good semantics, K8s-symlink-swap aware |
| `internal/install` | orchestrator (6-step ZITADEL provisioning) + opportunistic cleanup on mid-install failure |
| `internal/zitadel` | hand-wired ZITADEL Admin/Management API client |
| `internal/zitadelcontract` | derives issuer/management/audience + OIDC scope URNs from cfg + workspace row |
| `internal/publicconfig` | mutator that injects workspace-scoped values into `/_gofra/config.js` |
| `internal/companies` | MSP customer-org CRUD; eager `AddOrganization` per company |

## API contract

| Tech | Role | Where |
|---|---|---|
| Protobuf 3 | source of truth for RPC + runtime config schemas | `proto/gospa/**/*.proto` |
| Connect RPC | wire format + Go/TS bindings | `gen/gospa/**` (Go), `web/src/gen/` (TS) |
| `gofra generate config` | proto → typed Go `PublicConfig` + TS runtime config | `mise run generate` |

## Database

| Piece | Role | Where |
|---|---|---|
| PostgreSQL | persistence | `compose.yaml` image `postgres:18.3-alpine3.23`; managed Postgres in deploys |
| Singleton `workspace` table | install state machine + persisted ZITADEL auth contract (org / project / spa app / spa client / issuer / management / api_audience) | `db/migrations/00001_create_workspace.sql` + `00003_add_workspace_auth_contract.sql` |
| `companies` table | MSP customer rows, each backed by a ZITADEL org | `db/migrations/00002_create_companies.sql` |
| Migrations | `goose` SQL files; `runtime/database.Migrate` runs at startup when `database.auto_migrate = true` | `db/migrations/` |
| Queries | `sqlc` generates typed Go from SQL files | `db/queries/`, `db/sqlc/` |
| Seeds | `goose --no-versioning` for dev data | `db/seeds/` |

## Identity / Auth

| Piece | Role | Where |
|---|---|---|
| ZITADEL | OIDC IdP, source of users + access tokens | `compose.yaml` image `ghcr.io/zitadel/zitadel:stable` (local); operator-managed in deploys |
| Provisioner PAT | IAM_OWNER token Gospa uses for Admin + Management API calls | `.secrets/zitadel-provisioner.pat` (local) or K8s Secret, hot-reloaded by `internal/patwatch` |
| Install token | operator-supplied bootstrap secret validated on `POST /install` | `.secrets/install-token` (local), env / K8s Secret in deploys |
| OIDC SPA app | created by the install orchestrator, configured for JWT access tokens (`OIDC_TOKEN_TYPE_JWT`), `AppType=USER_AGENT`, `AuthMethodType=NONE` | `internal/install/orchestrator.go` step 4 |
| Browser PKCE flow | `react-oidc-context` + `oidc-client-ts`; sessionStorage user/state store; `automaticSilentRenew` enabled | `web/src/lib/auth-provider.tsx` + `web/src/routes/auth-callback.tsx` |
| Local JWT verify | `runtime/auth.NewJWTVerifier` does OIDC discovery + JWKS at `Activate` time, validates `iss`, `aud`, `exp` per request | wrapped by `internal/authgate.Gate` |
| Audience scope | `urn:zitadel:iam:org:project:id:{api_audience}:aud`, derived server-side and merged into published scopes | `internal/zitadelcontract.AudienceScope` |
| Org scope | `urn:zitadel:iam:org:id:{org_id}`, derived server-side | `internal/zitadelcontract.OrgScope` |

ADR 0001 in the workspace (`docs/adr/0001-explicit-zitadel-api-audience.md`)
records the persisted-contract decision; the v1 implementation refinement
(7 columns + 1 server-side helper) is in the addendum at the end of that
ADR.

## Frontend

| Tech | Version | Role | Where |
|---|---|---|---|
| React | `^19` | UI runtime | `web/src/main.tsx` |
| TanStack Router | `^1.100` | client routing + loaders | `web/src/main.tsx`, `web/src/routes/` |
| TanStack Query | `^5.66` | RPC cache for Connect + install polling | `web/src/routes/index.tsx`, `routes/install.tsx` |
| TanStack Form | `^1.0` | install wizard form state | `web/src/routes/install.tsx` |
| TanStack Table | `^8.21` | column defs + sorting for every list screen; pair with shadcn `Table` primitive | `web/src/components/{tickets,companies,team}/*-columns.tsx` |
| `react-oidc-context` | `^3.3.1` | React wrapper around `oidc-client-ts` for sign-in/out + session | `web/src/lib/auth-provider.tsx` |
| `oidc-client-ts` | `^3.5.0` | OIDC code exchange + storage | indirect via `react-oidc-context` |
| Tailwind v4 | `^4.0` | utility CSS | `web/src/styles.css`, `@tailwindcss/vite` |
| shadcn/ui (Radix via `radix-ui` umbrella + `class-variance-authority` + `clsx` + `tailwind-merge`) | various | accessible component primitives (Dialog, Popover, DropdownMenu, Sonner, Table…) | `web/src/components/ui/` |
| `sonner` | `^2.0` | toast queue, mounted once in `main.tsx` as `<Toaster />` | `web/src/components/ui/sonner.tsx` |
| `lucide-react` | `^0.474` | icons | components |
| Vite | `^6.1` | dev server (port `:5173`) + production build into `web/dist` | `web/vite.config.ts` |

The Go binary serves the embedded `web/dist` in production
(`web/embed.go`, `//go:build !dev`); a `-tags dev` build instead reverse-
proxies to the Vite dev server (`web/embed_dev.go`). `mise run dev`
starts both Go and Vite together but currently does NOT compile with
`-tags dev` — that wiring is a future starter improvement (S12 in
the provisioning hardening plan).

## Local infrastructure

| Piece | Role | Where |
|---|---|---|
| `compose.yaml` (project root) | brings up PostgreSQL + ZITADEL with the bootstrap PAT path mounted | run via `mise run infra` |
| `scripts/compose.sh` | detects `docker compose` vs `podman compose` so tasks have one surface | shell helper |
| `scripts/wait-for-postgres.sh` | gates `mise run infra` on `pg_isready` | shell helper |
| `scripts/wait-for-zitadel.sh` | gates on `GET /debug/healthz` 200 | shell helper |
| `scripts/copy-provisioner-pat.sh` | exports the PAT from the ZITADEL bootstrap volume to the host path the app reads | shell helper |
| `scripts/generate-install-token.sh` | materialises `.secrets/install-token` (idempotent across reruns) | shell helper |
| `scripts/load-env.sh` | exports `GOSPA_ZITADEL_PROVISIONER_PAT_FILE`, `GOSPA_INSTALL_TOKEN_FILE`, `DATABASE_URL`, etc. | sourced by every `mise` task that touches infra |

## Build + deploy

| Piece | Role | Where |
|---|---|---|
| Multi-stage `Dockerfile` (Node → Go → distroless) | produces a static image with the SPA embedded | project root |
| `.github/workflows/ci.yml` | `mise run test`, `mise run build`, local `docker build` on PRs and main | CI |
| K8s example | runnable manifests (Secret with PAT + install token, Deployment with patwatch volume mount, Service) | `docs/examples/deploy/kubernetes/` |
| Compose example | placeholder; does not run standalone today (no ZITADEL service in the file). See [its README](examples/deploy/compose/README.md) for the honest disclaimer | `docs/examples/deploy/compose/` |

## What is *not* in the stack today

Tracked here so nobody assumes a missing piece is hiding somewhere.

- **Restate.** Planned. Will replace the in-process install
  orchestrator (`internal/install/orchestrator.go`) and close the
  kernel-kill mid-install gap that operations.md Scenario K3 still
  documents.
- **Authz.** ZITADEL handles authn only today; any authenticated
  user passes the gate. The MSP role × tenant model lands in a
  later slice on top of `runtime/authz` from Gofra.
- **Image publish workflow.** CI builds the Docker image but does
  not push it. Pushing is per-project, opt-in.
- **Production Kubernetes guide.** The `docs/examples/deploy/kubernetes/`
  example is a starting point, not a production guide. TLS, cert
  issuer, Ingress, managed Postgres, observability, backup, and
  HPA are explicitly the operator's job.

## Cross-references

- Per-component design and behavior — Gofra docs at
  `repos/gofra/docs/00-index.md` (architecture, decision log,
  reference). Each numbered design doc maps to a `runtime/*`
  package Gospa imports.
- Operational scenarios (fresh clone, re-run, mid-install crash,
  PAT rotation, K8s deploy) — `docs/operations.md`.
- Product blueprint (market, rules, MVP debts, roadmap) —
  `docs/blueprint/index.md`.
- Workspace-level decision records — `../../docs/adr/`.
- Implementation progress (closed and open slices) —
  `../../docs/progress/`.
