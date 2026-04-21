import * as React from "react";
import {
  type UseMutationResult,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
} from "@tanstack/react-table";
import { useAuth } from "react-oidc-context";
import { AlertTriangle, Building2, Plus, Search, X } from "lucide-react";
import { toast } from "sonner";

import { cn } from "@/lib/utils";
import {
  type Company,
  type CreateCompanyInput,
  createCompany,
  listCompanies,
} from "@/lib/companies-client";
import { listMembers, type TeamMember } from "@/lib/team-client";
import { useCurrentUserEmail } from "@/lib/use-current-user";
import {
  ErrorState,
  RefreshingDot,
  Spinner,
  TableSkeletonRows,
} from "@/components/tickets/async-states";
import { companiesColumns } from "@/components/companies/companies-columns";
import { OwnerPicker } from "@/components/companies/owner-picker";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

// -----------------------------------------------------------
// View definitions
// -----------------------------------------------------------

type CompanyView = "active" | "inactive" | "unowned" | "all";

const VIEWS: {
  k: CompanyView;
  label: string;
  hint: string;
  tone?: "warning";
}[] = [
  { k: "active", label: "Ativas", hint: "1" },
  { k: "inactive", label: "Desativadas", hint: "2" },
  { k: "unowned", label: "Sem dono", hint: "3", tone: "warning" },
  { k: "all", label: "Todas", hint: "4" },
];

// -----------------------------------------------------------
// Segmented views
// -----------------------------------------------------------

function SegmentedViews({
  active,
  counts,
  onChange,
}: {
  active: CompanyView;
  counts: Record<CompanyView, number>;
  onChange: (v: CompanyView) => void;
}) {
  return (
    <div className="inline-flex gap-0.5 rounded-lg border border-border-subtle bg-subtle p-[3px]">
      {VIEWS.map((v) => {
        const isActive = active === v.k;
        const count = counts[v.k];
        const warn = !isActive && count > 0 && v.tone === "warning";
        return (
          <button
            key={v.k}
            type="button"
            onClick={() => onChange(v.k)}
            title={`${v.label} · tecla ${v.hint}`}
            className={cn(
              "inline-flex h-7 items-center gap-1.5 whitespace-nowrap rounded-md px-2.5 text-[12.5px] font-medium transition-colors",
              isActive
                ? "bg-elevated text-fg-1 shadow-[inset_0_0_0_1px_var(--border-default)]"
                : warn
                  ? "text-warning"
                  : "text-fg-3",
            )}
          >
            <span>{v.label}</span>
            <span
              className={cn(
                "font-mono text-[11px] font-medium",
                isActive ? "text-fg-3" : warn ? "text-warning" : "text-fg-4",
              )}
            >
              ({count})
            </span>
          </button>
        );
      })}
    </div>
  );
}

// -----------------------------------------------------------
// Search
// -----------------------------------------------------------

const SearchInput = React.forwardRef<
  HTMLInputElement,
  { value: string; onChange: (v: string) => void }
>(function SearchInput({ value, onChange }, ref) {
  return (
    <div className="inline-flex h-8 w-[280px] flex-shrink-0 items-center gap-2 rounded-lg border border-border-subtle bg-subtle px-2.5">
      <Search className="h-3 w-3 text-fg-4" strokeWidth={1.5} />
      <input
        ref={ref}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder="Buscar por nome…"
        className="min-w-0 flex-1 border-0 bg-transparent text-[13px] text-fg-1 outline-none placeholder:text-fg-4"
      />
      {value ? (
        <button
          type="button"
          onClick={() => onChange("")}
          title="Limpar busca (Esc)"
          className="inline-flex h-4 w-4 flex-shrink-0 items-center justify-center rounded bg-elevated text-fg-2"
        >
          <X className="h-2.5 w-2.5" strokeWidth={1.5} />
        </button>
      ) : (
        <kbd className="flex-shrink-0 rounded border border-border-subtle bg-surface px-1.5 py-px font-mono text-[10px] text-fg-3">
          /
        </kbd>
      )}
    </div>
  );
});

// -----------------------------------------------------------
// Empty state
// -----------------------------------------------------------

function EmptyCompanies({
  view,
  hasQuery,
  onNew,
  onClearFilters,
}: {
  view: CompanyView;
  hasQuery: boolean;
  onNew: () => void;
  onClearFilters: () => void;
}) {
  if (hasQuery) {
    return (
      <div className="flex flex-col items-center gap-2 py-14 text-center">
        <Building2 className="h-6 w-6 text-border-default" strokeWidth={1.5} />
        <div className="text-[14px] font-medium text-fg-1">
          Nenhuma empresa encontrada.
        </div>
        <div className="max-w-[360px] text-[12.5px] text-fg-3">
          Tente ajustar a busca ou trocar de view.
        </div>
        <button
          type="button"
          onClick={onClearFilters}
          className="mt-2.5 h-[30px] rounded-md border border-accent-hover/35 bg-transparent px-3 text-[12.5px] font-medium text-accent-hover"
        >
          Limpar filtros
        </button>
      </div>
    );
  }
  const copy: Record<CompanyView, { title: string; body: string }> = {
    active: {
      title: "Nenhuma empresa ativa.",
      body: "Crie a primeira para começar a operar.",
    },
    inactive: {
      title: "Nenhuma empresa desativada.",
      body: "Empresas arquivadas aparecem aqui.",
    },
    unowned: {
      title: "Todas têm dono.",
      body: "Nenhuma empresa está órfã no momento.",
    },
    all: {
      title: "Nenhuma empresa ainda.",
      body: "Crie a primeira para começar.",
    },
  };
  const c = copy[view];
  return (
    <div className="flex flex-col items-center gap-2 py-14 text-center">
      <Building2 className="h-6 w-6 text-border-default" strokeWidth={1.5} />
      <div className="text-[14px] font-medium text-fg-1">{c.title}</div>
      <div className="max-w-[360px] text-[12.5px] text-fg-3">{c.body}</div>
      {(view === "active" || view === "all") && (
        <button
          type="button"
          onClick={onNew}
          className="mt-2.5 inline-flex h-[30px] items-center gap-1.5 rounded-md bg-fg-1 px-3 text-[13px] font-medium text-app"
        >
          <Plus className="h-3 w-3" strokeWidth={1.75} />
          Nova empresa
        </button>
      )}
    </div>
  );
}

// -----------------------------------------------------------
// New company modal
// -----------------------------------------------------------

function NewCompanyModal({
  open,
  onClose,
  members,
  membersLoading,
  membersError,
  createMut,
}: {
  open: boolean;
  onClose: () => void;
  members: TeamMember[];
  membersLoading: boolean;
  membersError: string | null;
  createMut: UseMutationResult<Company, Error, CreateCompanyInput>;
}) {
  const [name, setName] = React.useState("");
  const [owner, setOwner] = React.useState<
    { contactId: string; fullName: string } | null
  >(null);
  const [errorMsg, setErrorMsg] = React.useState<string | null>(null);

  React.useEffect(() => {
    if (open) {
      setName("");
      setOwner(null);
      setErrorMsg(null);
    }
  }, [open]);

  const valid = name.trim().length >= 2;
  const submit = () => {
    if (!valid || createMut.isPending) return;
    setErrorMsg(null);
    createMut.mutate(
      { name: name.trim(), ownerContactId: owner?.contactId },
      {
        onSuccess: () => onClose(),
        onError: (err) => setErrorMsg(err.message),
      },
    );
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (!o && !createMut.isPending) onClose();
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
            Nova empresa
          </DialogTitle>
          <DialogDescription className="mt-0.5 text-[12px] text-fg-3">
            Adicione uma nova empresa ao seu workspace.
          </DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-3.5 p-5">
          <div>
            <div className="mb-1.5 text-[11px] font-medium uppercase tracking-[0.08em] text-fg-3">
              Nome
            </div>
            <input
              autoFocus
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Ex: Acme Mfg"
              className="h-9 w-full rounded-lg border border-border-default bg-app px-3 text-[13px] text-fg-1 outline-none focus:border-border-strong"
            />
          </div>
          <div>
            <div className="mb-1.5 text-[11px] font-medium uppercase tracking-[0.08em] text-fg-3">
              Dono
            </div>
            <OwnerPicker
              value={owner}
              onChange={setOwner}
              members={members}
              loading={membersLoading}
              errorMsg={membersError}
            />
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
            <span className="ml-2">para criar</span>
          </div>
          <div className="flex gap-2">
            <button
              type="button"
              onClick={onClose}
              disabled={createMut.isPending}
              className="h-[30px] rounded-md border border-border-default bg-transparent px-3 text-[13px] font-medium text-fg-1 disabled:opacity-50"
            >
              Cancelar
            </button>
            <button
              type="button"
              onClick={submit}
              disabled={!valid || createMut.isPending}
              className={cn(
                "h-[30px] rounded-md px-3.5 text-[13px] font-medium",
                valid && !createMut.isPending
                  ? "bg-fg-1 text-app"
                  : "cursor-not-allowed bg-border-default text-fg-4",
              )}
            >
              {createMut.isPending ? (
                <span className="inline-flex items-center gap-2">
                  <Spinner size={10} />
                  Criando…
                </span>
              ) : (
                "Criar empresa"
              )}
            </button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// -----------------------------------------------------------
// KbdLegend
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
// Main
// -----------------------------------------------------------

export type CompaniesPageSearch = {
  view: CompanyView;
  q: string;
};

export function CompaniesPage({
  search,
  onSearchChange,
}: {
  search: CompaniesPageSearch;
  onSearchChange: (next: Partial<CompaniesPageSearch>) => void;
}) {
  const auth = useAuth();
  const accessToken = auth.user?.access_token;
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const [rawQuery, setRawQuery] = React.useState(search.q);
  const [debouncedQuery, setDebouncedQuery] = React.useState(search.q);
  const [focusIdx, setFocusIdx] = React.useState(0);
  const [newOpen, setNewOpen] = React.useState(false);
  const searchRef = React.useRef<HTMLInputElement | null>(null);

  React.useEffect(() => {
    setRawQuery(search.q);
  }, [search.q]);

  React.useEffect(() => {
    const t = setTimeout(() => setDebouncedQuery(rawQuery), 180);
    return () => clearTimeout(t);
  }, [rawQuery]);

  React.useEffect(() => {
    if (debouncedQuery !== search.q) onSearchChange({ q: debouncedQuery });
  }, [debouncedQuery, search.q, onSearchChange]);

  const companiesQuery = useQuery({
    queryKey: ["companies"],
    queryFn: () => listCompanies(accessToken!),
    enabled: !!accessToken,
  });

  const membersQuery = useQuery({
    queryKey: ["team", "members"],
    queryFn: () => listMembers(accessToken!),
    enabled: !!accessToken,
  });

  const createMut = useMutation({
    mutationFn: (input: CreateCompanyInput) =>
      createCompany(input, accessToken!),
    onSuccess: (c) => {
      queryClient.invalidateQueries({ queryKey: ["companies"] });
      toast.success(`${c.name} criada.`);
    },
  });

  const all = companiesQuery.data?.companies ?? [];

  const filtered = React.useMemo(() => {
    let list = all.slice();
    if (search.view === "active") list = list.filter((c) => !c.archivedAt);
    else if (search.view === "inactive")
      list = list.filter((c) => !!c.archivedAt);
    else if (search.view === "unowned")
      list = list.filter((c) => !c.archivedAt && !c.owner);
    const needle = debouncedQuery.trim().toLowerCase();
    if (needle) {
      list = list.filter((c) => c.name.toLowerCase().includes(needle));
    }
    return list;
  }, [all, search.view, debouncedQuery]);

  const counts = React.useMemo(
    () => ({
      active: all.filter((c) => !c.archivedAt).length,
      inactive: all.filter((c) => !!c.archivedAt).length,
      unowned: all.filter((c) => !c.archivedAt && !c.owner).length,
      all: all.length,
    }),
    [all],
  );

  const currentUserEmail = useCurrentUserEmail();
  const table = useReactTable({
    data: filtered,
    columns: companiesColumns,
    getCoreRowModel: getCoreRowModel(),
    meta: { currentUserEmail, query: debouncedQuery },
  });

  React.useEffect(() => {
    setFocusIdx(0);
  }, [search.view, debouncedQuery]);

  React.useEffect(() => {
    if (focusIdx >= filtered.length) {
      setFocusIdx(Math.max(0, filtered.length - 1));
    }
  }, [filtered.length, focusIdx]);

  const openCompany = React.useCallback(
    (id: string) => {
      void navigate({ to: "/companies/$companyId", params: { companyId: id } });
    },
    [navigate],
  );

  React.useEffect(() => {
    const h = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement | null;
      const tag = target?.tagName;
      const inInput =
        tag === "INPUT" || tag === "TEXTAREA" || target?.isContentEditable;
      if (newOpen) return;

      if (e.key === "/" && !inInput) {
        e.preventDefault();
        searchRef.current?.focus();
        searchRef.current?.select();
        return;
      }
      if (
        e.key === "Escape" &&
        inInput &&
        tag === "INPUT" &&
        rawQuery &&
        document.activeElement === searchRef.current
      ) {
        setRawQuery("");
        return;
      }
      if (e.key === "n" && !inInput) {
        e.preventDefault();
        setNewOpen(true);
        return;
      }
      if (inInput) return;
      if (e.key >= "1" && e.key <= "4") {
        const v = VIEWS[parseInt(e.key, 10) - 1]?.k;
        if (v) onSearchChange({ view: v });
        return;
      }
      if (e.key === "j" || e.key === "ArrowDown") {
        e.preventDefault();
        setFocusIdx((i) => Math.min(filtered.length - 1, i + 1));
      } else if (e.key === "k" || e.key === "ArrowUp") {
        e.preventDefault();
        setFocusIdx((i) => Math.max(0, i - 1));
      } else if (e.key === "Enter") {
        const c = filtered[focusIdx];
        if (c) openCompany(c.id);
      }
    };
    window.addEventListener("keydown", h);
    return () => window.removeEventListener("keydown", h);
  }, [filtered, focusIdx, newOpen, rawQuery, onSearchChange, openCompany]);

  const clearFilters = () => {
    setRawQuery("");
    setDebouncedQuery("");
    onSearchChange({ q: "", view: "active" });
  };

  const refreshing =
    companiesQuery.isFetching && !companiesQuery.isLoading;

  return (
    <div className="flex h-full flex-col overflow-hidden bg-app">
      <div className="flex items-center justify-between gap-3 px-6 pb-2.5 pt-4">
        <h1 className="m-0 flex-shrink-0 text-xl font-semibold leading-tight tracking-tight text-fg-1">
          Companies
          <RefreshingDot visible={refreshing} />
        </h1>
        <div className="flex min-w-0 flex-1 items-center justify-end gap-2">
          <SearchInput
            ref={searchRef}
            value={rawQuery}
            onChange={setRawQuery}
          />
          <button
            type="button"
            onClick={() => setNewOpen(true)}
            className="inline-flex h-8 items-center gap-2 whitespace-nowrap rounded-lg bg-fg-1 py-0 pl-3 pr-1.5 text-[13px] font-medium text-app transition-colors hover:opacity-90"
          >
            <Plus className="h-3 w-3" strokeWidth={1.75} />
            <span>Nova empresa</span>
            <kbd className="ml-0.5 rounded border border-black/10 bg-black/5 px-1.5 py-px font-mono text-[10px] leading-snug text-app">
              n
            </kbd>
          </button>
        </div>
      </div>

      <div className="flex items-center gap-3 overflow-x-auto border-b border-[#0f0f0f] px-6 pb-3.5">
        <SegmentedViews
          active={search.view}
          counts={counts}
          onChange={(v) => onSearchChange({ view: v })}
        />
      </div>

      <div className="flex-1 overflow-auto px-6 pb-6">
        {companiesQuery.isLoading ? (
          <div className="pt-3.5">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Empresa</TableHead>
                  <TableHead style={{ width: 220 }}>Dono</TableHead>
                </TableRow>
              </TableHeader>
              <TableSkeletonRows count={6} widths={["60%", "50%"]} />
            </Table>
          </div>
        ) : companiesQuery.isError ? (
          <ErrorState onRetry={() => companiesQuery.refetch()} />
        ) : (
          <div
            className="pt-3.5 transition-opacity"
            style={{
              opacity: debouncedQuery !== rawQuery ? 0.7 : 1,
              transitionDuration: "120ms",
            }}
          >
            {filtered.length > 0 ? (
              <>
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
                            {flexRender(
                              h.column.columnDef.header,
                              h.getContext(),
                            )}
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
                        onClick={() => openCompany(row.original.id)}
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
                <div className="flex items-center justify-between px-1 pt-3">
                  <div className="text-[11.5px] text-fg-4">
                    <span className="font-mono text-fg-2">
                      {filtered.length}
                    </span>{" "}
                    de{" "}
                    <span className="font-mono text-fg-2">{all.length}</span>{" "}
                    empresa{all.length === 1 ? "" : "s"}
                  </div>
                  <div className="flex items-center gap-3.5 text-[11px] text-fg-4">
                    <KbdLegend keys={["j", "k"]} label="navegar" />
                    <KbdLegend keys={["↵"]} label="abrir" />
                    <KbdLegend keys={["/"]} label="buscar" />
                    <KbdLegend keys={["n"]} label="nova" />
                  </div>
                </div>
              </>
            ) : (
              <EmptyCompanies
                view={search.view}
                hasQuery={!!debouncedQuery}
                onNew={() => setNewOpen(true)}
                onClearFilters={clearFilters}
              />
            )}
          </div>
        )}
      </div>

      <NewCompanyModal
        open={newOpen}
        onClose={() => setNewOpen(false)}
        members={membersQuery.data ?? []}
        membersLoading={membersQuery.isLoading}
        membersError={
          membersQuery.error ? (membersQuery.error as Error).message : null
        }
        createMut={createMut}
      />

    </div>
  );
}

// Re-export the Link primitive from TanStack Router for the tiny nav
// helper on the empty states; kept here to keep the file self-contained.
export { Link };
