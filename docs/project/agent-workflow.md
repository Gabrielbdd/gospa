# Agent Workflow — Long-Running Work

Some tasks are too large for a single session: blueprint-driven feature
builds, multi-phase domain refactors, investigations that span proto,
migrations, and runtime code. This document defines how agents (and
humans) hand work off between sessions without losing context.

## When to use a progress file

Create a progress file when any of the following is true:

- The task is expected to span more than one working session.
- The task is an investigation whose outcome is not yet a single clear
  implementation step.
- The task is a feature rollout staged in phases (proto → migration →
  runtime → UI), where each phase ships independently.
- You are blocked and need to resume later with the same context.

Isolated bug fixes, small doc edits, and single-session feature work do
not need a progress file. The commit message is enough.

## Location and naming

Progress files live under:

```
docs/project/progress/<kebab-slug>.md
```

The slug is short, descriptive, and kebab-cased. Examples:

- `docs/project/progress/ticket-sla-engine.md`
- `docs/project/progress/time-entry-billing-roundtrip.md`
- `docs/project/progress/companies-contacts-seed.md`

Archived (completed) progress files move to:

```
docs/project/progress/done/<kebab-slug>.md
```

## Required content

Every progress file must include the following sections, in this order:

1. **Objective** — what this work achieves for the MSP and why it matters
   for the product.
2. **Context consulted** — specific docs, files, and external sources
   that informed the current state of the plan. Always reference the
   relevant section(s) of `docs/blueprint/index.md`.
3. **Decisions already taken** — concrete choices that are locked in.
   Include the blueprint rule or product principle each decision is
   anchored to.
4. **Next steps** — the immediate upcoming actions, in order.
5. **Blockers** — anything preventing forward progress, and what would
   unblock it. If the blocker is a missing capability in the framework,
   call that out explicitly — it belongs upstream, not worked around
   here.
6. **Pending validations** — tests, builds, smoke checks, and manual
   scenarios that must pass before the task is considered done.
7. **Current state** — a single dated line summarizing where things
   stand. Example:
   ```
   2026-04-18 — proto and migration landed; runtime handlers drafted;
   UI slice not started.
   ```

## When to update

Update the file at the end of each working session or the moment you
become blocked. The goal is that a new agent starting from scratch
tomorrow can pick up your work by reading only this file and the
referenced blueprint sections.

Do not batch updates to "when I'm done" — that defeats the purpose. A
stale progress file is worse than no progress file.

## When to archive

When the task is fully resolved — code merged, docs updated, validations
green — move the file from `docs/project/progress/` to
`docs/project/progress/done/` and remove its entry from any index. Keep
the file; the history is useful for future work that revisits the same
area.
