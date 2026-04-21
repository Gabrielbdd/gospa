import * as React from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
} from "@tanstack/react-table";
import { useAuth } from "react-oidc-context";
import { AlertTriangle, Check, Copy, Plus, Search } from "lucide-react";
import { toast } from "sonner";

import { cn } from "@/lib/utils";
import {
  changeRole,
  inviteMember,
  type InviteMemberInput,
  type InviteMemberResult,
  listMembers,
  type MemberRole,
  type MemberStatus,
  reactivateMember,
  suspendMember,
  type TeamMember,
} from "@/lib/team-client";
import {
  ErrorState,
  RefreshingDot,
  Spinner,
  TableSkeletonRows,
} from "@/components/tickets/async-states";
import { teamColumns } from "@/components/team/team-columns";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

// -----------------------------------------------------------
// Mini stats
// -----------------------------------------------------------

function MiniStats({
  total,
  active,
  pending,
}: {
  total: number;
  active: number;
  pending: number;
}) {
  const items = [
    { label: "Total", value: total },
    { label: "Ativos", value: active, tone: "var(--success)" },
    { label: "Não acessaram ainda", value: pending, tone: "var(--warning)" },
  ];
  return (
    <div className="flex gap-10 border-b border-t border-border-subtle py-3.5">
      {items.map((s) => (
        <div key={s.label} className="flex flex-col gap-1">
          <div className="text-[11px] font-medium uppercase tracking-[0.08em] text-fg-4">
            {s.label}
          </div>
          <div className="flex items-baseline gap-1.5">
            <span className="font-mono text-xl font-medium tracking-tight text-fg-1">
              {s.value}
            </span>
            {s.tone && (
              <span
                className="-translate-y-0.5 inline-block h-1.5 w-1.5 rounded-full"
                style={{ background: s.tone }}
              />
            )}
          </div>
        </div>
      ))}
    </div>
  );
}

// -----------------------------------------------------------
// Search bar
// -----------------------------------------------------------

const SearchBar = React.forwardRef<
  HTMLInputElement,
  { value: string; onChange: (v: string) => void }
>(function SearchBar({ value, onChange }, ref) {
  return (
    <div className="flex h-9 items-center gap-2.5 rounded-lg border border-border-subtle bg-transparent px-3 transition-colors focus-within:border-border-strong focus-within:bg-subtle">
      <Search className="h-3.5 w-3.5 text-fg-4" strokeWidth={1.5} />
      <input
        ref={ref}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder="Buscar por nome ou email…"
        className="h-full flex-1 border-0 bg-transparent text-[13px] text-fg-1 outline-none placeholder:text-fg-4"
      />
      <kbd className="rounded border border-border-default border-b-2 bg-surface px-1.5 py-px font-mono text-[11px] leading-snug text-fg-3">
        /
      </kbd>
    </div>
  );
});

type KebabAction = "open" | "role" | "resend" | "suspend" | "reactivate";

// -----------------------------------------------------------
// Invite modal
// -----------------------------------------------------------

function InviteModal({
  open,
  onClose,
  onInvite,
  pending,
  errorMsg,
}: {
  open: boolean;
  onClose: () => void;
  onInvite: (input: InviteMemberInput) => void;
  pending: boolean;
  errorMsg: string | null;
}) {
  const [fullName, setFullName] = React.useState("");
  const [email, setEmail] = React.useState("");
  const [role, setRole] = React.useState<MemberRole>("technician");

  React.useEffect(() => {
    if (open) {
      setFullName("");
      setEmail("");
      setRole("technician");
    }
  }, [open]);

  const validEmail = /^\S+@\S+\.\S+$/.test(email);
  const validName = fullName.trim().length >= 2;
  const valid = validEmail && validName;

  const submit = () => {
    if (!valid || pending) return;
    onInvite({ fullName: fullName.trim(), email: email.trim(), role });
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (!o && !pending) onClose();
      }}
    >
      <DialogContent
        showCloseButton={false}
        className="w-[460px] max-w-[460px] p-0 sm:max-w-[460px]"
        onKeyDown={(e) => {
          if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) submit();
        }}
      >
        <DialogHeader className="rounded-t-xl border-b border-border-subtle px-5 pb-3.5 pt-4 text-left">
          <DialogTitle className="text-[15px] font-semibold text-fg-1">
            Convidar membro
          </DialogTitle>
          <DialogDescription className="mt-0.5 text-[12px] text-fg-3">
            Geraremos uma senha temporária e mostraremos a você uma única vez.
          </DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-3.5 p-5">
          <div>
            <div className="mb-1.5 text-[11px] font-medium uppercase tracking-[0.08em] text-fg-3">
              Nome completo
            </div>
            <input
              autoFocus
              value={fullName}
              onChange={(e) => setFullName(e.target.value)}
              placeholder="Maria Silva"
              className="h-9 w-full rounded-lg border border-border-default bg-app px-3 text-[13px] text-fg-1 outline-none focus:border-border-strong"
            />
          </div>
          <div>
            <div className="mb-1.5 text-[11px] font-medium uppercase tracking-[0.08em] text-fg-3">
              Email
            </div>
            <input
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="pessoa@msp.com.br"
              className="h-9 w-full rounded-lg border border-border-default bg-app px-3 font-mono text-[13px] text-fg-1 outline-none focus:border-border-strong"
            />
          </div>
          <div>
            <div className="mb-1.5 text-[11px] font-medium uppercase tracking-[0.08em] text-fg-3">
              Papel
            </div>
            <div className="flex gap-1.5">
              {(["admin", "technician"] as MemberRole[]).map((r) => (
                <button
                  key={r}
                  type="button"
                  onClick={() => setRole(r)}
                  className={cn(
                    "h-9 flex-1 rounded-lg border text-[13px] font-medium",
                    role === r
                      ? "border-fg-1 bg-fg-1 text-app"
                      : "border-border-default bg-transparent text-fg-1",
                  )}
                >
                  {r === "admin" ? "Admin" : "Técnico"}
                </button>
              ))}
            </div>
          </div>
          {errorMsg && (
            <div className="flex items-start gap-2 rounded-md border border-danger/30 bg-danger/10 p-2.5 text-[12px] text-danger">
              <AlertTriangle
                className="mt-0.5 h-3 w-3 flex-shrink-0"
                strokeWidth={1.5}
              />
              <span>{errorMsg}</span>
            </div>
          )}
        </div>
        <DialogFooter className="flex-row items-center justify-between rounded-b-xl border-t border-border-subtle bg-[#070707] px-5 py-3 sm:justify-between">
          <div className="text-[11px] text-fg-4">
            <kbd className="rounded border border-border-default bg-surface px-1.5 py-px font-mono text-[10px] text-fg-3">
              ⌘ enter
            </kbd>
            <span className="ml-2">para enviar</span>
          </div>
          <div className="flex gap-2">
            <button
              type="button"
              onClick={onClose}
              disabled={pending}
              className="h-[30px] rounded-md border border-border-default bg-transparent px-3 text-[13px] font-medium text-fg-1 disabled:opacity-50"
            >
              Cancelar
            </button>
            <button
              type="button"
              onClick={submit}
              disabled={!valid || pending}
              className={cn(
                "h-[30px] rounded-md px-3.5 text-[13px] font-medium",
                valid && !pending
                  ? "bg-fg-1 text-app"
                  : "cursor-not-allowed bg-border-default text-fg-4",
              )}
            >
              {pending ? (
                <span className="inline-flex items-center gap-2">
                  <Spinner size={10} />
                  Enviando…
                </span>
              ) : (
                "Enviar convite"
              )}
            </button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// -----------------------------------------------------------
// One-time password modal — shown once after a successful invite.
// No esc / clickaway: the password is shown exactly one time, so we
// require explicit "I copied this" + Done.
// -----------------------------------------------------------

function OneTimePasswordModal({
  invite,
  onDone,
}: {
  invite: InviteMemberResult | null;
  onDone: () => void;
}) {
  const [confirmed, setConfirmed] = React.useState(false);
  const [copied, setCopied] = React.useState(false);

  React.useEffect(() => {
    if (invite) {
      setConfirmed(false);
      setCopied(false);
    }
  }, [invite]);

  const copy = async () => {
    if (!invite) return;
    try {
      await navigator.clipboard.writeText(invite.temporaryPassword);
      setCopied(true);
      setTimeout(() => setCopied(false), 1800);
    } catch {
      /* clipboard blocked — user must select manually */
    }
  };

  return (
    <Dialog open={!!invite}>
      <DialogContent
        showCloseButton={false}
        // No esc / clickaway: the password is shown exactly one time,
        // so we require an explicit "I copied this" + Done.
        onEscapeKeyDown={(e) => e.preventDefault()}
        onPointerDownOutside={(e) => e.preventDefault()}
        onInteractOutside={(e) => e.preventDefault()}
        className="w-[480px] max-w-[480px] p-0 sm:max-w-[480px]"
      >
        {invite && (
          <>
            <DialogHeader className="rounded-t-xl border-b border-border-subtle px-6 pb-4 pt-5 text-left">
              <DialogTitle className="text-[16px] font-semibold text-fg-1">
                Senha temporária de {invite.member.fullName}
              </DialogTitle>
              <DialogDescription className="mt-1 text-[12.5px] text-fg-3">
                Compartilhe com {invite.member.email}. Não vamos mostrar de novo.
              </DialogDescription>
            </DialogHeader>
            <div className="px-6 py-5">
              <div className="flex items-center gap-2 rounded-lg border border-border-default bg-app px-3 py-3.5">
                <code className="flex-1 select-all break-all font-mono text-[15px] tracking-wider text-fg-1">
                  {invite.temporaryPassword}
                </code>
                <button
                  type="button"
                  onClick={copy}
                  className={cn(
                    "inline-flex h-8 items-center gap-1.5 rounded-md border border-border-default px-2.5 text-[12px] font-medium transition-colors",
                    copied ? "text-success" : "text-fg-1 hover:bg-elevated",
                  )}
                >
                  {copied ? (
                    <>
                      <Check className="h-3 w-3" strokeWidth={1.75} />
                      Copiada
                    </>
                  ) : (
                    <>
                      <Copy className="h-3 w-3" strokeWidth={1.5} />
                      Copiar
                    </>
                  )}
                </button>
              </div>
              <label className="mt-4 flex cursor-pointer items-start gap-2.5 text-[13px] text-fg-2">
                <input
                  type="checkbox"
                  checked={confirmed}
                  onChange={(e) => setConfirmed(e.target.checked)}
                  className="mt-0.5 accent-accent-hover"
                />
                <span>
                  Copiei a senha em um lugar seguro e vou compartilhar com o
                  novo membro. No primeiro acesso, ele precisa trocá-la.
                </span>
              </label>
            </div>
            <DialogFooter className="flex-row items-center justify-end gap-2 rounded-b-xl border-t border-border-subtle bg-[#070707] px-6 py-3 sm:justify-end">
              <button
                type="button"
                onClick={onDone}
                disabled={!confirmed}
                className={cn(
                  "h-[30px] rounded-md px-4 text-[13px] font-medium",
                  confirmed
                    ? "bg-fg-1 text-app"
                    : "cursor-not-allowed bg-border-default text-fg-4",
                )}
              >
                Concluir
              </button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}

// -----------------------------------------------------------
// KbdLegend / Toast
// -----------------------------------------------------------

function KbdLegend({ keys, label }: { keys: string[]; label: string }) {
  return (
    <span className="inline-flex items-center gap-1">
      {keys.map((k, i) => (
        <kbd
          key={i}
          className="rounded border border-border-default border-b-2 bg-surface px-1.5 py-px font-mono text-[10px] leading-snug text-fg-3"
        >
          {k}
        </kbd>
      ))}
      <span className="ml-0.5 text-fg-4">{label}</span>
    </span>
  );
}

// -----------------------------------------------------------
// Main page
// -----------------------------------------------------------

export function SettingsTeam() {
  const auth = useAuth();
  const accessToken = auth.user?.access_token;
  const queryClient = useQueryClient();

  const [query, setQuery] = React.useState("");
  const [focusIdx, setFocusIdx] = React.useState(0);
  const [inviteOpen, setInviteOpen] = React.useState(false);
  const [inviteResult, setInviteResult] =
    React.useState<InviteMemberResult | null>(null);
  const [inviteError, setInviteError] = React.useState<string | null>(null);
  const searchRef = React.useRef<HTMLInputElement | null>(null);

  const membersQuery = useQuery({
    queryKey: ["team", "members"],
    queryFn: () => listMembers(accessToken!),
    enabled: !!accessToken,
  });


  const inviteMutation = useMutation({
    mutationFn: (input: InviteMemberInput) =>
      inviteMember(input, accessToken!),
    onSuccess: (result) => {
      setInviteOpen(false);
      setInviteError(null);
      setInviteResult(result);
      void queryClient.invalidateQueries({ queryKey: ["team", "members"] });
    },
    onError: (err: Error) => {
      setInviteError(err.message);
    },
  });

  const changeRoleMutation = useMutation({
    mutationFn: ({
      contactId,
      role,
    }: {
      contactId: string;
      role: MemberRole;
    }) => changeRole(contactId, role, accessToken!),
    onSuccess: (member) => {
      void queryClient.invalidateQueries({ queryKey: ["team", "members"] });
      toast.success(
        `${member.fullName} agora é ${member.role === "admin" ? "Admin" : "Técnico"}.`,
      );
    },
    onError: (err: Error) => toast.error(err.message),
  });

  const suspendMutation = useMutation({
    mutationFn: (contactId: string) =>
      suspendMember(contactId, accessToken!),
    onSuccess: (member) => {
      void queryClient.invalidateQueries({ queryKey: ["team", "members"] });
      toast(`${member.fullName} suspenso.`);
    },
    onError: (err: Error) => toast.error(err.message),
  });

  const reactivateMutation = useMutation({
    mutationFn: (contactId: string) =>
      reactivateMember(contactId, accessToken!),
    onSuccess: (member) => {
      void queryClient.invalidateQueries({ queryKey: ["team", "members"] });
      toast.success(`${member.fullName} reativado.`);
    },
    onError: (err: Error) => toast.error(err.message),
  });

  const members = membersQuery.data ?? [];
  const filtered = React.useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return members;
    return members.filter(
      (m) =>
        m.fullName.toLowerCase().includes(q) ||
        m.email.toLowerCase().includes(q),
    );
  }, [members, query]);

  const stats = React.useMemo(
    () => ({
      total: members.length,
      active: members.filter((m) => m.status === "active").length,
      pending: members.filter((m) => m.status === "not_signed_in_yet").length,
    }),
    [members],
  );

  React.useEffect(() => {
    if (focusIdx >= filtered.length) {
      setFocusIdx(Math.max(0, filtered.length - 1));
    }
  }, [filtered.length, focusIdx]);

  const cycleRole = React.useCallback(
    (m: TeamMember) => {
      changeRoleMutation.mutate({
        contactId: m.id,
        role: m.role === "admin" ? "technician" : "admin",
      });
    },
    [changeRoleMutation],
  );

  const handleAction = React.useCallback(
    (action: KebabAction, m: TeamMember) => {
      if (action === "open") {
        toast.info(`Abriria ${m.fullName}.`);
      } else if (action === "role") {
        cycleRole(m);
      } else if (action === "resend") {
        toast.info("Reenvio ainda não implementado.");
      } else if (action === "suspend") {
        suspendMutation.mutate(m.id);
      } else if (action === "reactivate") {
        reactivateMutation.mutate(m.id);
      }
    },
    [cycleRole, suspendMutation, reactivateMutation],
  );

  const table = useReactTable({
    data: filtered,
    columns: teamColumns,
    getCoreRowModel: getCoreRowModel(),
    meta: { onMemberAction: handleAction, currentUserEmail: null, query: "" },
  });

  // Keyboard
  React.useEffect(() => {
    const h = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement | null;
      const tag = target?.tagName;
      const inInput =
        tag === "INPUT" || tag === "TEXTAREA" || target?.isContentEditable;

      if (inviteOpen || inviteResult) return;

      if (e.key === "/" && !inInput) {
        e.preventDefault();
        searchRef.current?.focus();
        return;
      }
      if (e.key === "Escape") {
        if (document.activeElement === searchRef.current) {
          searchRef.current?.blur();
        }
        return;
      }
      if (e.key === "n" && !inInput) {
        e.preventDefault();
        setInviteOpen(true);
        return;
      }
      if (inInput) return;

      if (e.key === "j" || e.key === "ArrowDown") {
        e.preventDefault();
        setFocusIdx((i) => Math.min(filtered.length - 1, i + 1));
      } else if (e.key === "k" || e.key === "ArrowUp") {
        e.preventDefault();
        setFocusIdx((i) => Math.max(0, i - 1));
      } else if (e.key === "Enter") {
        const m = filtered[focusIdx];
        if (m) toast.info(`Abriria ${m.fullName}.`);
      } else if (e.key === "s") {
        const m = filtered[focusIdx];
        if (m && m.status !== "suspended") suspendMutation.mutate(m.id);
      } else if (e.key === "r") {
        const m = filtered[focusIdx];
        if (m) cycleRole(m);
      }
    };
    window.addEventListener("keydown", h);
    return () => window.removeEventListener("keydown", h);
  }, [
    filtered,
    focusIdx,
    inviteOpen,
    inviteResult,
    suspendMutation,
    cycleRole,
  ]);

  const refreshing =
    membersQuery.isFetching && !membersQuery.isLoading;

  return (
    <div className="flex h-full bg-app">
      <div className="flex-1 overflow-auto">
        <div className="mx-auto flex max-w-[1040px] flex-col gap-5 px-6 pb-14 pt-7">
          <header className="flex items-start justify-between gap-6">
            <div>
              <h1 className="m-0 text-2xl font-semibold leading-tight tracking-tight text-fg-1">
                Time
                <RefreshingDot visible={refreshing} />
              </h1>
              <p className="mt-1 text-[13px] text-fg-3">
                Membros do seu workspace.
              </p>
            </div>
            <button
              type="button"
              onClick={() => setInviteOpen(true)}
              className="inline-flex h-8 items-center gap-2 rounded-lg bg-fg-1 py-0 pl-3 pr-1.5 text-[13px] font-medium text-app transition-colors hover:opacity-90"
            >
              <Plus className="h-3 w-3" strokeWidth={1.75} />
              <span>Convidar membro</span>
              <kbd className="ml-0.5 rounded border border-black/10 bg-black/5 px-1.5 py-px font-mono text-[10px] leading-snug text-app">
                n
              </kbd>
            </button>
          </header>

          <MiniStats
            total={stats.total}
            active={stats.active}
            pending={stats.pending}
          />

          <SearchBar ref={searchRef} value={query} onChange={setQuery} />

          <section className="overflow-visible rounded-[10px] border border-border-subtle bg-subtle">
            {membersQuery.isLoading ? (
              <Table>
                <TableHeader>
                  <TableRow>
                    {table.getHeaderGroups()[0]?.headers.map((h) => (
                      <TableHead key={h.id}>
                        {flexRender(h.column.columnDef.header, h.getContext())}
                      </TableHead>
                    ))}
                  </TableRow>
                </TableHeader>
                <TableSkeletonRows
                  count={4}
                  widths={[26, "40%", "60%", 80, 110, 80, 26]}
                />
              </Table>
            ) : membersQuery.isError ? (
              <ErrorState onRetry={() => membersQuery.refetch()} />
            ) : filtered.length === 0 ? (
              <div className="px-4 py-12 text-center text-[13px] text-fg-4">
                Nenhum membro corresponde.{" "}
                <button
                  type="button"
                  onClick={() => setQuery("")}
                  className="border-b border-border-strong text-fg-1"
                >
                  Limpar filtros
                </button>
                .
              </div>
            ) : (
              <Table>
                <TableHeader>
                  {table.getHeaderGroups().map((hg) => (
                    <TableRow key={hg.id}>
                      {hg.headers.map((h) => (
                        <TableHead
                          key={h.id}
                          style={
                            h.column.columnDef.size
                              ? { width: h.column.columnDef.size }
                              : undefined
                          }
                        >
                          {flexRender(h.column.columnDef.header, h.getContext())}
                        </TableHead>
                      ))}
                    </TableRow>
                  ))}
                </TableHeader>
                <TableBody>
                  {table.getRowModel().rows.map((row, i) => (
                    <TableRow
                      key={row.id}
                      onMouseEnter={() => setFocusIdx(i)}
                      onClick={() =>
                        toast.info(`Abriria ${row.original.fullName}.`)
                      }
                      data-focused={i === focusIdx || undefined}
                      className="relative cursor-pointer data-[focused]:bg-[#0f0f0f]"
                    >
                      {i === focusIdx && (
                        <td
                          aria-hidden
                          className="absolute bottom-0 left-0 top-0 w-0.5 bg-accent-hover"
                        />
                      )}
                      {row.getVisibleCells().map((cell) => (
                        <TableCell key={cell.id}>
                          {flexRender(
                            cell.column.columnDef.cell,
                            cell.getContext(),
                          )}
                        </TableCell>
                      ))}
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </section>

          <div className="flex items-center justify-between pt-1">
            <div className="text-[12px] text-fg-4">
              Mostrando{" "}
              <span className="font-mono text-fg-2">{filtered.length}</span> de{" "}
              <span className="font-mono text-fg-2">{members.length}</span>{" "}
              membro{members.length === 1 ? "" : "s"}
            </div>
            <div className="flex items-center gap-3.5 text-[11px] text-fg-4">
              <KbdLegend keys={["j", "k"]} label="navegar" />
              <KbdLegend keys={["enter"]} label="abrir" />
              <KbdLegend keys={["n"]} label="convidar" />
              <KbdLegend keys={["s"]} label="suspender" />
              <KbdLegend keys={["r"]} label="papel" />
            </div>
          </div>
        </div>
      </div>

      <InviteModal
        open={inviteOpen}
        onClose={() => {
          if (!inviteMutation.isPending) {
            setInviteOpen(false);
            setInviteError(null);
          }
        }}
        onInvite={(input) => inviteMutation.mutate(input)}
        pending={inviteMutation.isPending}
        errorMsg={inviteError}
      />

      <OneTimePasswordModal
        invite={inviteResult}
        onDone={() => {
          const email = inviteResult?.member.email ?? "";
          setInviteResult(null);
          toast.success(`Convite enviado para ${email}.`);
        }}
      />
    </div>
  );
}

// `MemberStatus` is exported via team-client; re-exported here for
// nothing — kept for type completeness if a parent needs it.
export type { MemberStatus };
