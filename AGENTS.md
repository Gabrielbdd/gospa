# Repository Guidelines

## Project Scope

This repository is **Gospa** — an open-source PSA (Professional Services
Automation) for MSPs, built on top of the
[Gofra](https://github.com/Gabrielbdd/gofra) Go framework.

Gospa is an independent product repository with its own `go.mod`, CI,
releases, and contributors. It depends on Gofra as a normal Go module. Agents
working in this repo should treat Gofra as a published dependency: if
something is missing in the framework, raise it in the Gofra repository; do
not work around it by forking framework behavior inside Gospa.

## Sources of Truth

| Document | What it tells you |
|----------|-------------------|
| `docs/blueprint/index.md` | Product blueprint — market analysis, competitor research, product rules, feature scope, architecture, roadmap, business model |
| `README.md` | How to run, test, build, and develop Gospa locally |
| `proto/gospa/**` | Authoritative API contract for the product |

## Stack

- **Framework**: Gofra (`github.com/Gabrielbdd/gofra`) — `runtime/config`,
  `runtime/health`, `runtime/serve`, `runtime/database`, `runtime/auth`,
  `runtime/errors` packages imported directly.
- **API**: Connect RPC + protobuf contracts under `proto/gospa/**`.
- **Database**: PostgreSQL + `goose` migrations + `sqlc` generated queries.
- **Frontend (planned)**: React + TanStack + shadcn/ui + Vite.
- **Durable execution (planned)**: Restate.
- **Auth (planned)**: Zitadel via OIDC.
- **Local infra**: Docker/Podman Compose (Postgres today; Restate + Zitadel
  as they land in the framework).

## Build, Test, and Development Commands

```bash
mise trust
mise run infra        # start local Postgres
mise run migrate      # run pending migrations
mise run dev          # generate + run the app
mise run test         # go test ./...
mise run build        # build the binary to bin/gospa
docker build -t gospa:dev .   # build the production image
```

CI (`.github/workflows/ci.yml`) runs `mise run test`, `mise run build`, and a
local `docker build` on every pull request and push to `main`.

## Work Protocol — Plan Before You Build

### Agent posture: consultative partner, not passive executor

You are a product-engineering partner, not a command runner. This means:

- **Question premises.** If a request conflicts with the blueprint, the
  existing architecture, or documented decisions, surface the conflict and
  propose an alternative. Do not silently comply.
- **Propose improvements.** If during investigation you find a simpler or
  better approach, suggest it proactively.
- **Clarify ambiguity.** If a request is vague, ask before assuming.
- **Verify, don't trust blindly.** Check the blueprint and the code before
  accepting claims about current state.

### Protocol steps

Every non-trivial task follows this sequence:

1. **Investigate** — read the relevant blueprint section(s), the
   corresponding proto(s), the current Go code, and tests.

   **Deep research for non-trivial changes.** "Non-trivial" here means any
   change that touches a proto contract, introduces a migration, adds a new
   product feature, or shifts the boundary with the framework. Isolated bug
   fixes and pure doc edits are exempt.

   For non-trivial changes, before moving on, produce:

   - Known architectural patterns for the domain in play (ticketing,
     time tracking, billing, RMM integration, companies/contacts,
     invoicing, etc.).
   - How mature PSAs solve the same problem — ConnectWise Manage,
     Autotask, HaloPSA / Halo, SuperOps, Atera, Syncro. Include the
     open-source references ITFlow and Alga PSA when applicable.
   - What users praise, complain about, and ask for, with the source
     cited (competitor docs, G2 / Capterra reviews, r/msp threads,
     MSP community forums).
   - Explicit statements of business, UX, and operational impact — not
     "we'll see", but the concrete consequences you expect.
   - At least two alternatives beyond the one you plan to propose, with
     a one-line reason each was or was not chosen.

2. **Clarify** — confirm the problem and scope with the user if anything is
   uncertain.

3. **Plan and present** — describe what will change and where, before
   touching files. Include a confidence breakdown using this template:

   ```
   Confidence:
     - Architectural:    NN%  (alignment with the blueprint and product rules)
     - Business / PO:    NN%  (alignment with MSP value and the roadmap in docs/blueprint)
     - User perspective: NN% — persona: <named persona>
                         (default: an MSP technician; switch to
                          "MSP's end client" or "MSP owner" when the
                          change targets a different role)
     - Solution:         NN%  (correctness and completeness of the proposed plan)
   ```

   Any axis below 70% means you must state what you are unsure about and
   what would raise your confidence. Silence on a low score is not
   acceptable.

4. **Gate** — wait for user approval before implementing.

5. **Implement** — follow the approved plan; re-present if you deviate
   meaningfully.

6. **Validate** — `mise run test`, `mise run build`, and, for anything
   touching the image, `docker build`. Changes that affect deployment or
   day-to-day operation (`Dockerfile`, `compose.yaml`, `mise.toml`,
   `docs/examples/deploy/**`) require reviewing and, when necessary,
   updating the corresponding runbooks in `docs/` and the deploy
   examples. Realistic validation is part of the contract:

   - run `mise run test` and `mise run build`;
   - run `docker build -t gospa:dev .` when the image, Dockerfile, or
     `compose.yaml` changed;
   - for UI-visible changes, open the app in a browser and confirm it
     works before declaring the task done.

   For long-running work that will span sessions, follow the convention
   in [`docs/project/agent-workflow.md`](docs/project/agent-workflow.md)
   and keep a progress file current.

### Framework vs product boundary

If a task touches generic behavior that any Gofra-based app would want
(config, health, serve, error shape, auth, database lifecycle, scaffold), the
fix belongs in the Gofra repository, not here. Gospa imports a new version of
Gofra; it does not reimplement framework concerns locally.

If a task touches PSA-specific domain logic (tickets, time entries, billing,
companies, etc.), it belongs here. Do not push product-specific concerns into
the framework.

## Repository Layout

- `cmd/app/` — application entrypoint (generated by the starter).
- `runtime/` (consumed, not owned) — framework packages imported from Gofra.
- `proto/gospa/` — protobuf definitions owned by Gospa (API contract).
- `config/` — generated config code (post `mise run generate`).
- `db/migrations/`, `db/queries/`, `db/seeds/` — Postgres schema + queries.
- `web/` — embedded frontend (currently a minimal HTML shell; React stack
  lands when the framework wires it).
- `docs/blueprint/` — product blueprint.

## Commit & Pull Request Guidelines

Follow Conventional Commit style: `feat:`, `fix:`, `docs:`, `chore:`,
`refactor:`, `test:`. Commit when a logical change is done, not once at the
end of a session. Do not commit with broken tests or a red CI.

PR descriptions should explain:

- what product behavior changed (user-visible)
- which proto/db/runtime surfaces moved
- which blueprint section the change advances
- how it was tested
