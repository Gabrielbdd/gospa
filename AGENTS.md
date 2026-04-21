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

Short list — full table with versions and roles is in
[`docs/stack.md`](docs/stack.md).

- **Framework**: Gofra (`github.com/Gabrielbdd/gofra`) — `runtime/config`,
  `runtime/health`, `runtime/serve`, `runtime/database`, `runtime/auth`,
  `runtime/zitadel/secret`, `runtime/errors` packages imported directly.
- **API**: Connect RPC + protobuf contracts under `proto/gospa/**`.
- **Database**: PostgreSQL + `goose` migrations + `sqlc` generated queries.
- **Frontend**: React 19 + TanStack (Router/Query/Form) + shadcn/ui +
  Tailwind v4 + Vite. OIDC login via `react-oidc-context`.
- **Auth**: ZITADEL via OIDC. Per-workspace persisted contract
  (issuer/management/audience) plus an operator-supplied install token
  for the bootstrap wizard. JWT access tokens validated locally.
- **Local infra**: Docker/Podman Compose runs Postgres + ZITADEL;
  `mise run infra` materialises the provisioner PAT and the install
  token under `.secrets/`.
- **Durable execution (planned)**: Restate. Will replace the in-process
  install orchestrator and close the kernel-kill mid-install gap.

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

### No repair before v1 (hard rule)

**Pre-v1, every schema or state change assumes the operator runs a fresh
install.** There is no read-repair, no backfill, no migration data shim, no
mid-install crash recovery, no "the app self-heals on boot" code. Period.

When a feature appears to need "what about workspaces installed before this
change?" thinking, the answer is: `mise run infra:reset` + reinstall. That
is the contract. Document it; do not work around it.

**Why this is a rule, not a guideline:**

- Pre-v1 there are no production users. There is no persistent data we
  cannot drop. The only "users" are the developers themselves, and the
  developer workflow is destroy-and-recreate constantly.
- Backfill / repair code carries permanent maintenance cost (it must keep
  working as the schema evolves; it adds risk to every later change; it
  needs deprecation cycles). For pre-v1, that cost buys nothing.
- The repair pattern is contagious — once one read-repair exists, the
  next agent (human or AI) will follow the precedent without questioning
  it. This rule exists to break the chain.

**What to do when you spot the pattern proposed:**

- In a design / spec / proposal: reject it. Cite this rule. Suggest the
  fresh-install path instead.
- In a code review: reject the PR. The author should remove the
  repair/backfill/recovery code and document the reset path instead.
- In your own implementation: stop. Re-read this rule. Replace the
  "self-heal" path with a clear log message + documented operator
  action.

**Examples of the pattern (all banned pre-v1):**

- Startup hook that backfills missing rows by querying an external system
  (e.g. ZITADEL Management API to reconstruct lost local state).
- Startup hook that flips a workspace stuck in a transient state
  (`provisioning`, `migrating`, etc.) to a recoverable state.
- Migration that does INSERT/UPDATE on existing rows beyond strict schema
  evolution (column add/drop, index add/drop, constraint change).
- Code that reads a NULL column and infers what it "should have been".
- Any code path whose only justification is "in case the operator
  upgraded from a pre-X version".

**When does this rule relax?** Only when v1 ships and there is a
production deploy with persistent data we cannot drop. At that point the
discussion reopens — and the bar for re-introducing repair-style patterns
becomes "operator runs an explicit `gospa migrate` (or equivalent)
command", not "app silently self-heals on every boot".

### Framework vs product boundary

If a task touches generic behavior that any Gofra-based app would want
(config, health, serve, error shape, auth, database lifecycle, scaffold), the
fix belongs in the Gofra repository, not here. Gospa imports a new version of
Gofra; it does not reimplement framework concerns locally.

If a task touches PSA-specific domain logic (tickets, time entries, billing,
companies, etc.), it belongs here. Do not push product-specific concerns into
the framework.

## Frontend UI primitives (hard rule)

Gospa's web UI is React 19 + Tailwind v4 + **shadcn/ui** (which owns Radix
UI under the hood). Before you write any new interactive widget, check
whether shadcn already has it. If it does, use it — don't reinvent.

### When shadcn is required

Any widget with non-trivial interaction, focus management, keyboard
semantics, or accessibility requirements. If in doubt, default to shadcn.

| Widget | shadcn primitive |
|--------|-------------------|
| Modal / confirmation | `Dialog` |
| Floating panel over a trigger (combobox, owner picker, filter) | `Popover` (+ `Command` for searchable lists) |
| Context / kebab / actions menu | `DropdownMenu` |
| Transient notification | `Sonner` (toast) |
| Data tables / lists with rows and columns | `Table` **+ TanStack Table v8** (see below) |
| Command palette, fuzzy search | `Command` |
| Form controls | `Input`, `Label`, `Textarea`, `Select`, `Checkbox`, `RadioGroup`, `Switch`, `Slider` |
| Disclosure / toggle content | `Collapsible`, `Accordion`, `Tabs` |
| Hover text | `Tooltip` |
| Inline status / scroll area / layout | `Badge`, `Separator`, `ScrollArea` |

### Tables (tickets, companies, team members, future lists)

Use **TanStack Table v8** (`@tanstack/react-table`) paired with the
shadcn `Table` primitive. TanStack owns column defs, sorting, and
(future) pagination/filtering/selection; shadcn `Table` provides the
semantic `<table>`/`<thead>`/`<tbody>` chrome with Gospa tokens.

Rules:

- Column definitions go in a sibling file `*-columns.tsx` next to the
  page that consumes them. Co-located, not extracted globally — we
  don't have enough repetition to warrant a shared layer.
- Do **not** wrap TanStack behind a generic `<DataTable>` component.
  Use `useReactTable` + `flexRender` directly in the page. Premature
  abstraction is explicitly banned by `.claude/rules/principles.md`.
- Sorting is client-side (`getCoreRowModel` + `getSortedRowModel`)
  until a backend owns a list. Flip `manualSorting: true` on the page
  when the RPC takes a `sort_by` param.
- Keyboard nav (`j/k`, `Enter`) stays in the page component's own
  useEffect. TanStack does not manage row focus.
- Density is fixed by `web/src/components/ui/table.tsx`: 44px rows,
  `px-4 gap-3`, `text-[13px]` cells, `text-[11px] uppercase
  tracking-[0.06em] text-fg-4` headers, `border-border-subtle`.
  Don't override these on a per-page basis.
- Loading state: `TableSkeletonRows` from
  `components/tickets/async-states.tsx`. Pass a `widths` array that
  matches the column widths of the real header.

### When custom is OK

- **Pure presentational** with no interaction: status dots, custom badges
  that don't fit shadcn `Badge`'s variants, decorative avatars.
- **Domain-specific layout** that doesn't map to any shadcn primitive:
  segmented view pills with counts, SLA cells, priority badges, the
  ticket row grid.
- **Skeletons and spinners** — trivial and we already have them.

If the design calls for a widget that *almost* matches a shadcn
primitive, edit the local `components/ui/*.tsx` file directly (you own
that code — that's the whole point of shadcn) rather than writing a
parallel custom component.

### How to add a primitive

```bash
cd web
npx shadcn@latest add <name>
```

The CLI drops the component at `web/src/components/ui/<name>.tsx` and
installs the Radix runtime dep. After adding, update the catalog in
`web/src/components/ui/README.md` so future agents see it's available.

### What not to do

1. Do not hand-roll focus trap, scroll lock, Escape handling, or portal
   positioning logic. If you find yourself calling `getBoundingClientRect`
   to place a floating panel, stop — you want `Popover`.
2. Do not ship a modal without `Dialog` wrapping it.
3. Do not build another toast component. Call `toast()` from `sonner`.
4. Do not pull in competing headless libraries (React Aria Components,
   Ark UI, Base UI, Headless UI). The project standardized on shadcn
   (Radix) — mixing libraries fragments a11y behavior and bundle size.

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
