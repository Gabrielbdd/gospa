# UI primitives

Components in this directory are **shadcn/ui** components owned by
Gospa. They wrap [Radix UI](https://www.radix-ui.com/) primitives with
Tailwind classes and project-specific tokens.

Rule (also in `/AGENTS.md`): before writing a new interactive widget,
check whether a shadcn primitive exists. If it does, use it. Don't
hand-roll focus trap, floating panel positioning, Escape handling, or
toast queues — those are solved problems.

## Installed

| File | Wraps | Used by |
|------|-------|---------|
| `button.tsx` | — (custom, cva-based) | Install wizard, `/`, misc call-to-actions |
| `dialog.tsx` | `radix-ui` Dialog | `NewTicketModal`, `NewCompanyModal`, `InviteModal`, `OneTimePasswordModal` |
| `popover.tsx` | `radix-ui` Popover | `OwnerPicker`, `CompanyFilter` |
| `dropdown-menu.tsx` | `radix-ui` DropdownMenu | Team row Kebab menu |
| `sonner.tsx` | `sonner` | Global toast; mounted once via `<Toaster />` in `main.tsx` |
| `table.tsx` | — (semantic `<table>` + Gospa density tokens) | Tickets list, Companies list, Team list (each via TanStack Table v8) |

## Adding a new primitive

```bash
cd web
npx shadcn@latest add <name>
```

Component lands here as `<name>.tsx`. The CLI also installs the Radix
runtime dep. After adding, update the table above so future agents see
it's available.

## Modifying a primitive

Since the file is local, you can edit it. Common reasons:

- Tighten Tailwind classes to match the Gospa dark-first token scale
  (`bg-subtle`, `border-border-default`, etc.) instead of the defaults
  shadcn ships with (`bg-popover`, `bg-background`, etc.).
- Shrink padding / heights to the Linear-grade density the design uses.
- Remove animations if they feel too heavy.

Do not rename exports — consumers import by the shadcn-canonical name.

## Data tables (TanStack Table + `table.tsx`)

Every list screen uses **TanStack Table v8** paired with this
directory's `table.tsx` semantic primitive. The pattern:

1. Define `ColumnDef<Row>[]` in `components/<feature>/<feature>-columns.tsx`.
   Use `createColumnHelper<Row>()` for type safety. Let TS infer the
   exported type — do **not** annotate as `ColumnDef<Row>[]` or you hit
   the variance issue on `unknown` accessor values.
2. In the page, drive the table with
   `useReactTable({ data, columns, getCoreRowModel: getCoreRowModel(), meta })`.
   Add `getSortedRowModel()` only when the column can sort.
3. Render with `flexRender` inside the shadcn `Table`, `TableHeader`,
   `TableRow`, `TableHead`, `TableBody`, `TableCell` primitives.
4. Keyboard nav (`j/k`, `Enter`) stays in the page's own `useEffect`.
   TanStack does not manage focus.
5. Loading state: `TableSkeletonRows` from
   `components/tickets/async-states.tsx`. Widths array must match
   column count.
6. Cross-cell context (current user's email, search query) goes through
   `useReactTable({ meta: {...} })` and is read in cells via
   `ctx.table.options.meta`. The shape is declared in `team-columns.tsx`
   and `companies-columns.tsx` via `declare module "@tanstack/react-table"`.

Do **not** build a `<GospaDataTable>` wrapper. We own each page's
`useReactTable` call directly.

## Tokens these components consume

- **Backgrounds**: `bg-app`, `bg-subtle`, `bg-surface`, `bg-elevated`,
  `bg-overlay` (scrim).
- **Borders**: `border-border-default`, `border-border-subtle`,
  `border-border-strong`.
- **Foregrounds**: `text-fg-1` (primary), `text-fg-2`, `text-fg-3`,
  `text-fg-4` (muted/placeholder).
- **Accents**: `bg-accent`, `bg-accent-hover`, `text-accent`.
- **Status**: `text-success`, `text-warning`, `text-danger`.

Full scale: `web/src/styles/tokens.css`. Tailwind alias map:
`web/src/styles/index.css` `@theme` block.

## Shadcn legacy aliases

`index.css` also maps shadcn's legacy names (`--color-background`,
`--color-primary`, `--color-destructive`, etc.) to our tokens, so the
stock shadcn generator output works without a rewrite on first landing.
You may still want to replace those aliases with our semantic names
(`text-fg-1` instead of `text-foreground`) when editing for the second
time — our names carry more intent.
