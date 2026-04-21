import { createColumnHelper } from "@tanstack/react-table";
import {
  Eye,
  Mail,
  MoreHorizontal,
  UserCheck,
  UserCog,
  UserMinus,
} from "lucide-react";

import { cn } from "@/lib/utils";
import type { MemberRole, MemberStatus, TeamMember } from "@/lib/team-client";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

// Meta injected by the Team page:
//   - onAction: fires when the kebab menu selects something
//   - currentUserEmail: lets the "Você" substitution bubble in later
//     (not used today — Team lists the caller by their own name)
declare module "@tanstack/react-table" {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  interface TableMeta<TData extends unknown> {
    onMemberAction?: (
      action: "open" | "role" | "resend" | "suspend" | "reactivate",
      member: TeamMember,
    ) => void;
  }
}

const ch = createColumnHelper<TeamMember>();

const AVATAR_TONES = ["#3291ff", "#a1a1a1", "#454545", "#2e2e2e", "#5a5a5a"];

function initials(name: string): string {
  return name
    .split(" ")
    .filter(Boolean)
    .map((w) => w[0])
    .slice(0, 2)
    .join("")
    .toUpperCase();
}

function avatarTone(id: string): string {
  let hash = 0;
  for (let i = 0; i < id.length; i++) hash = (hash * 31 + id.charCodeAt(i)) >>> 0;
  return AVATAR_TONES[hash % AVATAR_TONES.length];
}

function formatLastSeen(rfc: string): string {
  if (!rfc) return "—";
  const t = new Date(rfc).getTime();
  if (Number.isNaN(t)) return "—";
  const min = Math.max(1, Math.floor((Date.now() - t) / 60_000));
  if (min < 60) return `${min}min atrás`;
  const h = Math.floor(min / 60);
  if (h < 24) return `${h}h atrás`;
  const d = Math.floor(h / 24);
  if (d < 30) return `${d}d atrás`;
  return new Date(rfc).toLocaleDateString("pt-BR");
}

function Avatar({ name, id, suspended }: { name: string; id: string; suspended: boolean }) {
  return (
    <div
      className={cn(
        "flex h-[26px] w-[26px] flex-shrink-0 items-center justify-center rounded-full text-[11px] font-semibold text-white",
        suspended && "opacity-40 grayscale",
      )}
      style={{ background: avatarTone(id) }}
    >
      {initials(name)}
    </div>
  );
}

function RoleBadge({ role }: { role: MemberRole }) {
  if (role === "admin") {
    return (
      <span className="inline-flex items-center gap-1.5 rounded bg-success/[0.12] px-2 py-0.5 text-[11px] font-medium text-success">
        <span className="h-1 w-1 rounded-full bg-success" />
        Admin
      </span>
    );
  }
  return (
    <span className="inline-flex items-center rounded border border-border-default bg-surface px-2 py-0.5 text-[11px] font-medium text-fg-2">
      Técnico
    </span>
  );
}

function StatusBadge({ status }: { status: MemberStatus }) {
  if (status === "not_signed_in_yet") {
    return (
      <span className="inline-flex items-center gap-1.5 rounded bg-warning/10 px-2 py-0.5 text-[11px] font-medium text-warning">
        <span className="h-1 w-1 rounded-full bg-warning" />
        Ainda não acessou
      </span>
    );
  }
  if (status === "suspended") {
    return (
      <span className="inline-flex items-center gap-1.5 rounded border border-border-default bg-surface px-2 py-0.5 text-[11px] font-medium text-fg-3">
        <span className="h-1 w-1 rounded-full bg-fg-4" />
        Suspenso
      </span>
    );
  }
  return (
    <span className="inline-flex items-center gap-1.5 rounded border border-border-default px-2 py-0.5 text-[11px] font-medium text-fg-2">
      <span className="h-1 w-1 rounded-full bg-success" />
      Ativo
    </span>
  );
}

function KebabCell({ member, onAction }: { member: TeamMember; onAction?: (a: "open" | "role" | "resend" | "suspend" | "reactivate", m: TeamMember) => void }) {
  const suspended = member.status === "suspended";
  const invited = member.status === "not_signed_in_yet";
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        className="inline-flex h-7 w-7 items-center justify-center rounded-md border border-transparent text-fg-3 transition-colors outline-none hover:border-border-default hover:bg-elevated focus-visible:border-border-default data-[state=open]:border-border-default data-[state=open]:bg-elevated"
        onClick={(e) => e.stopPropagation()}
        aria-label={`Abrir ações para ${member.fullName}`}
      >
        <MoreHorizontal className="h-3.5 w-3.5" strokeWidth={1.5} />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-[180px]">
        <DropdownMenuItem onSelect={() => onAction?.("open", member)}>
          <Eye className="text-fg-3" strokeWidth={1.5} />
          <span className="flex-1">Abrir detalhes</span>
          <DropdownMenuShortcut>enter</DropdownMenuShortcut>
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => onAction?.("role", member)}>
          <UserCog className="text-fg-3" strokeWidth={1.5} />
          <span className="flex-1">Trocar papel</span>
          <DropdownMenuShortcut>r</DropdownMenuShortcut>
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => onAction?.("resend", member)}>
          <Mail className="text-fg-3" strokeWidth={1.5} />
          <span className="flex-1">
            {invited ? "Reenviar convite" : "Enviar redefinição de senha"}
          </span>
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        {suspended ? (
          <DropdownMenuItem onSelect={() => onAction?.("reactivate", member)}>
            <UserCheck className="text-fg-3" strokeWidth={1.5} />
            <span className="flex-1">Reativar</span>
          </DropdownMenuItem>
        ) : (
          <DropdownMenuItem onSelect={() => onAction?.("suspend", member)}>
            <UserMinus className="text-fg-3" strokeWidth={1.5} />
            <span className="flex-1">Suspender acesso</span>
            <DropdownMenuShortcut>s</DropdownMenuShortcut>
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export const teamColumns = [
  ch.display({
    id: "avatar",
    size: 40,
    header: "",
    cell: ({ row }) => (
      <Avatar
        name={row.original.fullName}
        id={row.original.id}
        suspended={row.original.status === "suspended"}
      />
    ),
  }),
  ch.accessor("fullName", {
    header: "Nome",
    cell: ({ row }) => {
      const suspended = row.original.status === "suspended";
      return (
        <span
          className={cn(
            "truncate whitespace-nowrap text-[13px] font-medium",
            suspended
              ? "text-fg-4 line-through decoration-border-default"
              : "text-fg-1",
          )}
        >
          {row.original.fullName}
        </span>
      );
    },
  }),
  ch.accessor("email", {
    header: "Email",
    cell: ({ row }) => {
      const suspended = row.original.status === "suspended";
      return (
        <span
          className={cn(
            "truncate whitespace-nowrap font-mono text-[12px]",
            suspended ? "text-border-default" : "text-fg-2",
          )}
        >
          {row.original.email}
        </span>
      );
    },
  }),
  ch.accessor("role", {
    header: "Papel",
    size: 112,
    cell: ({ row }) => <RoleBadge role={row.original.role} />,
  }),
  ch.accessor("status", {
    header: "Status",
    size: 148,
    cell: ({ row }) => <StatusBadge status={row.original.status} />,
  }),
  ch.accessor("lastSeenAt", {
    header: "Visto por último",
    size: 128,
    cell: ({ row }) => (
      <span
        className={cn(
          "font-mono text-[12px]",
          row.original.lastSeenAt ? "text-fg-3" : "text-border-default",
        )}
      >
        {formatLastSeen(row.original.lastSeenAt)}
      </span>
    ),
  }),
  ch.display({
    id: "kebab",
    size: 40,
    header: "",
    cell: ({ row, table }) => (
      <KebabCell
        member={row.original}
        onAction={table.options.meta?.onMemberAction}
      />
    ),
  }),
];
