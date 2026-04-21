import type { SortingFn } from "@tanstack/react-table";
import { createColumnHelper } from "@tanstack/react-table";
import type * as React from "react";

import type { Ticket, TicketPriority } from "@/lib/tickets-client";
import { TEAM } from "@/lib/tickets-client";
import { OwnerAvatar } from "@/components/companies/owner-avatar";

const ch = createColumnHelper<Ticket>();

// SLA sort puts paused/closed rows (slaSeconds === null) at the bottom
// regardless of direction, since they represent "no clock ticking" —
// natural tail for both overdue-first (asc) and longest-first (desc).
const sortSLA: SortingFn<Ticket> = (a, b) => {
  const av = a.original.slaSeconds;
  const bv = b.original.slaSeconds;
  if (av === null && bv === null) return 0;
  if (av === null) return 1;
  if (bv === null) return -1;
  return av - bv;
};

// Priority sort ranks P1 first. Column stores strings "P1".."P4"; we
// parse the numeric suffix so the comparison is numeric, not lexical.
const sortPriority: SortingFn<Ticket> = (a, b) => {
  return priorityRank(a.original.priority) - priorityRank(b.original.priority);
};

function priorityRank(p: TicketPriority): number {
  return parseInt(p.slice(1), 10);
}

export const ticketsColumns = [
  ch.accessor("id", {
    header: "ID",
    size: 64,
    cell: ({ row, table }) => (
      <span className="inline-block min-w-0 truncate font-mono text-[11.5px] text-fg-3">
        {highlight(row.original.id, table.options.meta?.query ?? "")}
      </span>
    ),
  }),
  ch.accessor("priority", {
    header: "Prioridade",
    size: 96,
    sortingFn: sortPriority,
    cell: ({ row }) => <PriorityBadge p={row.original.priority} />,
  }),
  ch.accessor("slaSeconds", {
    header: "SLA",
    size: 120,
    sortingFn: sortSLA,
    cell: ({ row }) =>
      row.original.closed ? (
        <span className="font-mono text-[12px] text-fg-4">—</span>
      ) : (
        <SLACell secs={row.original.slaSeconds} />
      ),
  }),
  ch.accessor("subject", {
    header: "Assunto",
    size: 320,
    cell: ({ row, table }) => (
      <span className="block min-w-0 truncate text-[13px] font-medium text-fg-1">
        {highlight(row.original.subject, table.options.meta?.query ?? "")}
      </span>
    ),
  }),
  ch.accessor("clientName", {
    header: "Cliente",
    size: 200,
    cell: ({ row, table }) => (
      <div className="flex min-w-0 flex-col gap-px">
        <span className="truncate text-[12px] text-fg-1">
          {highlight(row.original.clientName, table.options.meta?.query ?? "")}
        </span>
        <span className="truncate text-[11px] text-fg-4">
          {highlight(
            row.original.requesterName,
            table.options.meta?.query ?? "",
          )}
        </span>
      </div>
    ),
  }),
  ch.accessor("assigneeHandle", {
    header: "Dono",
    size: 140,
    cell: ({ row }) => <AssigneeCell handle={row.original.assigneeHandle} />,
  }),
  ch.accessor("status", {
    header: "Status",
    size: 120,
    cell: ({ row }) => (
      <StatusChip label={row.original.status} tone={row.original.statusTone} />
    ),
  }),
];

// -----------------------------------------------------------
// Cell primitives — kept here so column defs read bottom-up as
// declarative rendering and the pieces are not scattered across the
// page file. Each piece stays local to this file because it encodes
// ticket-specific visual rules (priority colours, SLA thresholds,
// assignee handle mock) that only this column set uses.
// -----------------------------------------------------------

function PriorityBadge({ p }: { p: TicketPriority }) {
  const styles: Record<
    TicketPriority,
    { bg: string; color: string; border: string }
  > = {
    P1: { bg: "#ff4d4f", color: "#fff", border: "transparent" },
    P2: {
      bg: "rgba(245,166,35,0.16)",
      color: "#f5a623",
      border: "rgba(245,166,35,0.35)",
    },
    P3: { bg: "transparent", color: "#a1a1a1", border: "#242424" },
    P4: { bg: "transparent", color: "#666", border: "#1a1a1a" },
  };
  const s = styles[p];
  return (
    <span
      className="inline-flex items-center justify-center rounded font-mono font-semibold tracking-wide"
      style={{
        width: 26,
        height: 18,
        fontSize: 10.5,
        background: s.bg,
        color: s.color,
        border: `1px solid ${s.border}`,
      }}
    >
      {p}
    </span>
  );
}

function SLACell({ secs }: { secs: number | null }) {
  const f = formatSLA(secs);
  if (f.state === "paused") {
    return (
      <span className="whitespace-nowrap font-mono text-[12px] text-fg-3">
        Aguardando
      </span>
    );
  }
  if (f.state === "overdue") {
    return (
      <span className="inline-flex items-center whitespace-nowrap rounded bg-danger/10 px-1.5 py-0.5 font-mono text-[11.5px] font-medium text-danger">
        {f.text}
      </span>
    );
  }
  if (f.state === "warn") {
    return (
      <span className="whitespace-nowrap font-mono text-[12px] font-medium text-warning">
        {f.text}
      </span>
    );
  }
  return (
    <span className="whitespace-nowrap font-mono text-[12px] text-fg-3">
      {f.text}
    </span>
  );
}

function AssigneeCell({ handle }: { handle: string | null }) {
  if (!handle) {
    return (
      <span className="inline-flex items-center gap-1.5 text-[12px] italic text-fg-4">
        <OwnerAvatar size={20} />
        <span className="truncate">Sem dono</span>
      </span>
    );
  }
  const m = TEAM.find((t) => t.handle === handle);
  const name = m?.name ?? handle;
  const short = handle === "you" ? "Você" : name.split(" ")[0];
  return (
    <span
      title={name}
      className="inline-flex min-w-0 items-center gap-1.5 text-[12.5px] text-fg-1"
    >
      <OwnerAvatar contactId={handle} fullName={name} size={20} />
      <span className="truncate">{short}</span>
    </span>
  );
}

function StatusChip({ label, tone }: { label: string; tone: string }) {
  return (
    <span
      title={label}
      className="inline-flex max-w-full items-center gap-1.5 overflow-hidden rounded px-1.5 py-0.5 text-[11px] font-medium"
      style={{ background: `${tone}18`, color: tone }}
    >
      <span
        className="h-1 w-1 flex-shrink-0 rounded-full"
        style={{ background: tone }}
      />
      <span className="truncate">{label}</span>
    </span>
  );
}

// -----------------------------------------------------------
// Helpers
// -----------------------------------------------------------

function formatSLA(secs: number | null): {
  text: string;
  state: "ok" | "warn" | "overdue" | "paused";
} {
  if (secs === null) return { text: "Aguardando cliente", state: "paused" };
  const abs = Math.abs(secs);
  const h = Math.floor(abs / 3600);
  const m = Math.floor((abs % 3600) / 60);
  const d = Math.floor(h / 24);

  let text: string;
  if (d >= 1) text = `${d}d ${h % 24}h`;
  else if (h >= 1) text = `${h}h ${m.toString().padStart(2, "0")}m`;
  else text = `${m}m`;

  if (secs < 0) {
    if (d >= 1) return { text: `Vencido há ${d}d ${h % 24}h`, state: "overdue" };
    if (h >= 1) return { text: `Vencido há ${h}h ${m}m`, state: "overdue" };
    return { text: `Vencido há ${m}m`, state: "overdue" };
  }
  const hoursLeft = secs / 3600;
  if (hoursLeft < 2) return { text: `${text} restantes`, state: "warn" };
  return { text, state: "ok" };
}

function highlight(text: string, q: string): React.ReactNode {
  const needle = q.trim();
  if (!needle) return text;
  const lower = text.toLowerCase();
  const low = needle.toLowerCase();
  const parts: React.ReactNode[] = [];
  let i = 0;
  while (i < text.length) {
    const idx = lower.indexOf(low, i);
    if (idx === -1) {
      parts.push(text.slice(i));
      break;
    }
    if (idx > i) parts.push(text.slice(i, idx));
    parts.push(
      <mark
        key={idx}
        className="rounded-sm bg-yellow-400/[0.28] px-px text-inherit"
      >
        {text.slice(idx, idx + needle.length)}
      </mark>,
    );
    i = idx + needle.length;
  }
  return parts;
}

