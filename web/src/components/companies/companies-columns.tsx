import { createColumnHelper } from "@tanstack/react-table";
import type * as React from "react";

import type { Company } from "@/lib/companies-client";
import { OwnerCell } from "@/components/shared/owner-cell";
import { cn } from "@/lib/utils";

// Table-level context injected via `useReactTable({ meta: ... })`.
// Declared here so consuming cells can `ctx.table.options.meta?.currentUserEmail`
// type-safely.
declare module "@tanstack/react-table" {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  interface TableMeta<TData extends unknown> {
    currentUserEmail: string | null;
    query: string;
  }
}

const ch = createColumnHelper<Company>();

export const companiesColumns = [
  ch.accessor("name", {
    header: "Empresa",
    cell: ({ row, table }) => {
      const archived = !!row.original.archivedAt;
      const query = table.options.meta?.query ?? "";
      return (
        <div className="flex min-w-0 items-center gap-2">
          <span
            className={cn(
              "h-1.5 w-1.5 flex-shrink-0 rounded-full",
              archived ? "bg-border-strong" : "bg-success",
            )}
          />
          <span
            className={cn(
              "min-w-0 truncate whitespace-nowrap text-[13px] font-medium",
              archived
                ? "text-fg-3 line-through decoration-border-default"
                : "text-fg-1",
            )}
          >
            {highlight(row.original.name, query)}
          </span>
        </div>
      );
    },
  }),
  ch.accessor(
    (row) => row.owner?.fullName ?? "",
    {
      id: "owner",
      header: "Dono",
      size: 220,
      cell: ({ row, table }) => (
        <OwnerCell
          owner={
            row.original.owner
              ? {
                  contactId: row.original.owner.contactId,
                  fullName: row.original.owner.fullName,
                }
              : null
          }
          currentUserEmail={table.options.meta?.currentUserEmail ?? null}
          // Company.owner carries only contactId + fullName today — no
          // email. We rely on the consuming page to pass the current
          // user's contactId match later if we want a strict self-check.
          // Meanwhile this cell falls through to "primeiro nome".
          ownerEmail={null}
        />
      ),
    },
  ),
];

function highlight(text: string, q: string): React.ReactNode {
  const needle = q.trim();
  if (needle.length < 2) return text;
  const i = text.toLowerCase().indexOf(needle.toLowerCase());
  if (i < 0) return text;
  return (
    <>
      {text.slice(0, i)}
      <mark className="rounded-sm bg-yellow-400/[0.28] px-px text-inherit">
        {text.slice(i, i + needle.length)}
      </mark>
      {text.slice(i + needle.length)}
    </>
  );
}
