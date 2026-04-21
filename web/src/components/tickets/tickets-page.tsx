import * as React from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  type SortingState,
  useReactTable,
} from "@tanstack/react-table";
import {
  Archive,
  Building2,
  Check,
  CheckCircle,
  ChevronDown,
  ChevronUp,
  Coffee,
  Inbox,
  Layers,
  Plus,
  RotateCcw,
  Search,
  SearchX,
  ShieldCheck,
  SlidersHorizontal,
  X,
} from "lucide-react";
import { toast } from "sonner";

import { cn } from "@/lib/utils";
import {
  type CreateTicketRequest,
  type ListTicketsResponse,
  type Ticket,
  type TicketCounts,
  type TicketPriority,
  type TicketView,
  createTicket,
  getTicketCounts,
  listClientNames,
  listTickets,
} from "@/lib/tickets-client";
import { useCurrentUserEmail } from "@/lib/use-current-user";
import {
  ErrorState,
  ListFooter,
  type ListFooterMode,
  RefreshingDot,
  Spinner,
  TableSkeletonRows,
} from "@/components/tickets/async-states";
import { ticketsColumns } from "@/components/tickets/tickets-columns";
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
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";

// -----------------------------------------------------------
// View definitions, async state machine, format helpers
// -----------------------------------------------------------

const VIEWS: {
  k: TicketView;
  label: string;
  hint: string;
  tone: "neutral" | "warning" | "danger";
  history?: boolean;
}[] = [
  { k: "all", label: "Todos abertos", hint: "1", tone: "neutral" },
  { k: "unassigned", label: "Não atribuídos", hint: "2", tone: "warning" },
  { k: "mine", label: "Meus", hint: "3", tone: "neutral" },
  { k: "risk", label: "Em risco", hint: "4", tone: "danger" },
  {
    k: "closed",
    label: "Fechados recentes",
    hint: "5",
    tone: "neutral",
    history: true,
  },
];

export type AsyncState =
  | "idle"
  | "loading"
  | "error"
  | "refreshing"
  | "paginating"
  | "paginate-fallback"
  | "paginate-error"
  | "end";

// -----------------------------------------------------------
// Segmented views (pill bar)
// -----------------------------------------------------------

function SegmentedViews({
  active,
  counts,
  onChange,
  countersLoading,
}: {
  active: TicketView;
  counts: TicketCounts;
  onChange: (v: TicketView) => void;
  countersLoading: boolean;
}) {
  return (
    <div className="inline-flex gap-0.5 rounded-lg border border-border-subtle bg-subtle p-[3px]">
      {VIEWS.map((v) => {
        const isActive = active === v.k;
        const count = counts[v.k];
        const highlight =
          !isActive && count > 0 && (v.tone === "danger" || v.tone === "warning");
        const toneFg =
          v.tone === "danger"
            ? "text-danger"
            : v.tone === "warning"
              ? "text-warning"
              : "text-fg-3";
        return (
          <button
            key={v.k}
            type="button"
            onClick={() => onChange(v.k)}
            title={`${v.label} · tecla ${v.hint}`}
            className={cn(
              "inline-flex h-7 flex-shrink-0 items-center gap-1.5 whitespace-nowrap rounded-md px-2.5 text-[12.5px] font-medium transition-colors",
              isActive
                ? "bg-elevated text-fg-1 shadow-[inset_0_0_0_1px_var(--border-default)]"
                : highlight
                  ? toneFg
                  : "text-fg-3",
              v.history && !isActive && "opacity-70",
            )}
          >
            <span>{v.label}</span>
            <span
              className={cn(
                "font-mono text-[11px] font-medium",
                isActive ? "text-fg-3" : highlight ? toneFg : "text-fg-4",
              )}
            >
              ({countersLoading ? "—" : count})
            </span>
          </button>
        );
      })}
    </div>
  );
}

// -----------------------------------------------------------
// Search input + Company filter + Active filters bar
// -----------------------------------------------------------

function SearchInput({
  value,
  onChange,
  inputRef,
}: {
  value: string;
  onChange: (v: string) => void;
  inputRef: React.RefObject<HTMLInputElement | null>;
}) {
  return (
    <div className="inline-flex h-8 w-[260px] flex-shrink-0 items-center gap-2 rounded-lg border border-border-subtle bg-subtle px-2.5">
      <Search className="h-3 w-3 text-fg-4" strokeWidth={1.5} />
      <input
        ref={inputRef}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder="Buscar ticket…"
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
}

type CompanyFilterHandle = { open: () => void };

const CompanyFilter = React.forwardRef<
  CompanyFilterHandle,
  {
    value: string;
    onChange: (v: string) => void;
    clients: { name: string; ticketCount: number }[];
  }
>(function CompanyFilter({ value, onChange, clients }, ref) {
  const [open, setOpen] = React.useState(false);
  const [q, setQ] = React.useState("");
  const inputRef = React.useRef<HTMLInputElement | null>(null);

  React.useImperativeHandle(
    ref,
    () => ({
      open: () => setOpen(true),
    }),
    [],
  );

  React.useEffect(() => {
    if (open) {
      setQ("");
      setTimeout(() => inputRef.current?.focus(), 10);
    }
  }, [open]);

  const filteredClients = React.useMemo(() => {
    const needle = q.trim().toLowerCase();
    const list = clients.slice();
    if (!needle) return list;
    return list.filter((c) => c.name.toLowerCase().includes(needle));
  }, [clients, q]);

  const hasFilter = !!value;

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          title="Filtrar por empresa (F)"
          className={cn(
            "inline-flex h-8 flex-shrink-0 items-center gap-1.5 whitespace-nowrap rounded-lg border text-[13px] font-medium transition-colors",
            hasFilter
              ? "border-accent-hover/35 bg-accent-hover/10 pl-2.5 pr-1 text-accent-hover"
              : "border-border-subtle bg-subtle px-2.5 text-fg-2",
          )}
        >
          <Building2 className="h-3 w-3" strokeWidth={1.5} />
          <span>{hasFilter ? `Empresa: ${value}` : "Todas empresas"}</span>
          {hasFilter ? (
            <span
              role="button"
              tabIndex={0}
              onClick={(e) => {
                e.stopPropagation();
                onChange("");
              }}
              title="Limpar empresa"
              className="ml-0.5 inline-flex h-4 w-4 cursor-pointer items-center justify-center rounded bg-accent-hover/20"
            >
              <X className="h-2.5 w-2.5" strokeWidth={1.5} />
            </span>
          ) : (
            <ChevronDown
              className="ml-0.5 h-3 w-3 text-fg-4"
              strokeWidth={1.5}
            />
          )}
        </button>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-[280px] p-0">
        <div className="flex items-center gap-2 border-b border-border-subtle px-2.5 py-2">
          <Search className="h-3 w-3 text-fg-4" strokeWidth={1.5} />
          <input
            ref={inputRef}
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="Filtrar empresa…"
            className="flex-1 border-0 bg-transparent text-[13px] text-fg-1 outline-none placeholder:text-fg-4"
          />
        </div>
        <div className="max-h-[280px] overflow-y-auto p-1">
          <button
            type="button"
            onClick={() => {
              onChange("");
              setOpen(false);
            }}
            className={cn(
              "flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left text-[13px] text-fg-1 hover:bg-elevated",
              !value && "bg-elevated",
            )}
          >
            <Layers className="h-3 w-3 text-fg-3" strokeWidth={1.5} />
            <span className="flex-1">Todas empresas</span>
            {!value && <Check className="h-3 w-3 text-fg-3" strokeWidth={1.5} />}
          </button>
          {filteredClients.length === 0 && (
            <div className="px-2.5 py-3 text-[12px] text-fg-4">
              Nenhuma empresa corresponde.
            </div>
          )}
          {filteredClients.map((c) => (
            <button
              key={c.name}
              type="button"
              onClick={() => {
                onChange(c.name);
                setOpen(false);
              }}
              className={cn(
                "flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left text-[13px] text-fg-1 hover:bg-elevated",
                value === c.name && "bg-elevated",
              )}
            >
              <span className="inline-flex h-4 w-4 flex-shrink-0 items-center justify-center rounded bg-border-default text-[9px] font-semibold text-fg-1">
                {c.name
                  .split(" ")
                  .map((w) => w[0])
                  .slice(0, 2)
                  .join("")
                  .toUpperCase()}
              </span>
              <span className="flex-1 truncate">{c.name}</span>
              <span className="font-mono text-[11px] text-fg-4">
                {c.ticketCount}
              </span>
            </button>
          ))}
        </div>
      </PopoverContent>
    </Popover>
  );
});

function ActiveFiltersBar({
  viewLabel,
  company,
  query,
  resultCount,
  onClear,
}: {
  viewLabel: string;
  company: string;
  query: string;
  resultCount: number;
  onClear: () => void;
}) {
  if (!company && !query) return null;
  return (
    <div className="flex items-center justify-between gap-2.5 border-b border-[#0f0f0f] bg-[#060606] px-6 py-2 text-[12px] text-fg-3">
      <div className="inline-flex min-w-0 items-center gap-2 truncate whitespace-nowrap">
        <span className="flex-shrink-0">{viewLabel}</span>
        {company && (
          <>
            <span className="flex-shrink-0 text-border-default">•</span>
            <span className="flex-shrink-0">{company}</span>
          </>
        )}
        {query && (
          <>
            <span className="flex-shrink-0 text-border-default">•</span>
            <span className="min-w-0 truncate text-fg-1">“{query}”</span>
          </>
        )}
        <span className="flex-shrink-0 text-border-default">—</span>
        <span className="flex-shrink-0">
          <span className="font-mono text-fg-2">{resultCount}</span>{" "}
          resultado{resultCount === 1 ? "" : "s"}
        </span>
      </div>
      <button
        type="button"
        onClick={onClear}
        className="border-0 bg-transparent text-[12px] text-accent-hover hover:underline"
      >
        Limpar filtros
      </button>
    </div>
  );
}

// -----------------------------------------------------------
// Empty states
// -----------------------------------------------------------

const EMPTY_COPY: Record<
  TicketView,
  { icon: React.ComponentType<{ className?: string; strokeWidth?: number }>; title: string; body: string; cta?: string }
> = {
  all: {
    icon: Inbox,
    title: "Nenhum ticket aberto.",
    body: "Quando chegar um, ele aparece aqui.",
    cta: "Criar ticket",
  },
  unassigned: {
    icon: CheckCircle,
    title: "Todos atribuídos.",
    body: "Não há tickets órfãos no momento.",
  },
  mine: {
    icon: Coffee,
    title: "Nada atribuído a você.",
    body: "Bom momento pra pegar da fila de não atribuídos.",
  },
  risk: {
    icon: ShieldCheck,
    title: "Nenhum SLA em risco.",
    body: "Mantenha o ritmo — tudo sob controle.",
  },
  closed: {
    icon: Archive,
    title: "Nenhum ticket fechado nos últimos 30 dias.",
    body: "Quando tickets forem resolvidos, aparecem aqui por 30 dias.",
  },
};

function EmptyState({
  view,
  query,
  company,
  viewLabel,
  onNew,
  onGoTo,
  onClearFilters,
}: {
  view: TicketView;
  query: string;
  company: string;
  viewLabel: string;
  onNew: () => void;
  onGoTo: (v: TicketView) => void;
  onClearFilters: () => void;
}) {
  const hasFilters = !!query || !!company;
  const c = EMPTY_COPY[view];
  const Icon = c.icon;

  if (hasFilters) {
    return (
      <div className="flex flex-col items-center gap-2 py-14 text-center">
        <div className="mb-1 text-border-default">
          <SearchX className="h-6 w-6" strokeWidth={1.5} />
        </div>
        <div className="text-[14px] font-medium text-fg-1">
          Nenhum ticket encontrado{" "}
          {query && (
            <>
              para “<span className="text-fg-1">{query}</span>”
            </>
          )}
          {query && company && " "}
          {company && (
            <>
              em <span className="text-fg-1">{company}</span>
            </>
          )}{" "}
          em {viewLabel.toLowerCase()}.
        </div>
        <div className="max-w-[420px] text-[12.5px] text-fg-3">
          Tente ajustar a busca, trocar de view, ou limpar os filtros ativos.
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

  return (
    <div className="flex flex-col items-center gap-2 py-14 text-center">
      <div className="mb-1 text-border-default">
        <Icon className="h-6 w-6" strokeWidth={1.5} />
      </div>
      <div className="text-[14px] font-medium text-fg-1">{c.title}</div>
      <div className="max-w-[360px] text-[12.5px] text-fg-3">{c.body}</div>
      {view === "all" && c.cta && (
        <button
          type="button"
          onClick={onNew}
          className="mt-2.5 inline-flex h-[30px] items-center gap-1.5 rounded-md bg-fg-1 px-3 text-[13px] font-medium text-app"
        >
          <Plus className="h-3 w-3" strokeWidth={1.75} />
          {c.cta}
        </button>
      )}
      {view === "mine" && (
        <button
          type="button"
          onClick={() => onGoTo("unassigned")}
          className="mt-2.5 h-7 rounded-md border border-border-default bg-transparent px-2.5 text-[12.5px] font-medium text-fg-1"
        >
          Ver não atribuídos
        </button>
      )}
    </div>
  );
}

// -----------------------------------------------------------
// New-ticket modal
// -----------------------------------------------------------

const CLIENT_OPTIONS = [
  "Acme Mfg",
  "Northside Dental",
  "Viridis Labs",
  "Halden Legal",
  "Kronos Holdings",
  "Stellar Group",
  "Marlow Architects",
  "Baltic Freight",
];

function NewTicketModal({
  open,
  onClose,
  onCreate,
  pending,
}: {
  open: boolean;
  onClose: () => void;
  onCreate: (req: CreateTicketRequest) => void;
  pending: boolean;
}) {
  const [subject, setSubject] = React.useState("");
  const [priority, setPriority] = React.useState<TicketPriority>("P3");
  const [client, setClient] = React.useState("Acme Mfg");

  React.useEffect(() => {
    if (open) {
      setSubject("");
      setPriority("P3");
      setClient("Acme Mfg");
    }
  }, [open]);

  const valid = subject.trim().length >= 3;
  const submit = () => {
    if (!valid) return;
    onCreate({ subject: subject.trim(), priority, clientName: client });
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (!o) onClose();
      }}
    >
      <DialogContent
        showCloseButton={false}
        className="w-[520px] max-w-[520px] p-0 sm:max-w-[520px]"
        onKeyDown={(e) => {
          if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) submit();
        }}
      >
        <DialogHeader className="rounded-t-xl border-b border-border-subtle px-5 pb-3.5 pt-4 text-left">
          <DialogTitle className="text-[15px] font-semibold text-fg-1">
            Novo ticket
          </DialogTitle>
          <DialogDescription className="mt-0.5 text-[12px] text-fg-3">
            Comece pelo assunto — você refina depois.
          </DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-3.5 p-5">
          <div>
            <div className="mb-1.5 text-[11px] font-medium uppercase tracking-[0.08em] text-fg-3">
              Assunto
            </div>
            <input
              autoFocus
              value={subject}
              onChange={(e) => setSubject(e.target.value)}
              placeholder="Ex: VPN cai ao suspender notebook"
              className="h-9 w-full rounded-lg border border-border-default bg-app px-3 text-[13px] text-fg-1 outline-none focus:border-border-strong"
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <div className="mb-1.5 text-[11px] font-medium uppercase tracking-[0.08em] text-fg-3">
                Cliente
              </div>
              <select
                value={client}
                onChange={(e) => setClient(e.target.value)}
                className="h-9 w-full rounded-lg border border-border-default bg-app px-2.5 text-[13px] text-fg-1 outline-none focus:border-border-strong"
              >
                {CLIENT_OPTIONS.map((c) => (
                  <option key={c}>{c}</option>
                ))}
              </select>
            </div>
            <div>
              <div className="mb-1.5 text-[11px] font-medium uppercase tracking-[0.08em] text-fg-3">
                Prioridade
              </div>
              <div className="flex gap-1">
                {(["P1", "P2", "P3", "P4"] as TicketPriority[]).map((p) => (
                  <button
                    key={p}
                    type="button"
                    onClick={() => setPriority(p)}
                    className={cn(
                      "h-9 flex-1 rounded-lg border font-mono text-[12.5px] font-medium transition-colors",
                      priority === p
                        ? "border-fg-1 bg-fg-1 text-app"
                        : "border-border-default bg-transparent text-fg-1",
                    )}
                  >
                    {p}
                  </button>
                ))}
              </div>
            </div>
          </div>
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
              className="h-[30px] rounded-md border border-border-default bg-transparent px-3 text-[13px] font-medium text-fg-1"
            >
              Cancelar
            </button>
            <button
              type="button"
              onClick={submit}
              disabled={!valid || pending}
              className={cn(
                "h-[30px] rounded-md px-3.5 text-[13px] font-medium transition-colors",
                valid && !pending
                  ? "bg-fg-1 text-app"
                  : "cursor-not-allowed bg-border-default text-fg-4",
              )}
            >
              {pending ? (
                <span className="inline-flex items-center gap-2">
                  <Spinner size={10} />
                  Criando…
                </span>
              ) : (
                "Criar ticket"
              )}
            </button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// -----------------------------------------------------------
// Tweaks panel — demo harness for the 8 async states
// -----------------------------------------------------------

const ASYNC_STATES: { k: AsyncState; label: string; hint: string }[] = [
  { k: "idle", label: "Carregado", hint: "dados presentes, estado nominal" },
  { k: "loading", label: "Carregando", hint: "primeira carga — skeleton rows" },
  {
    k: "refreshing",
    label: "Atualizando",
    hint: "refetch silencioso — dot no título",
  },
  { k: "error", label: "Erro", hint: "falha ao carregar — tentar novamente" },
  { k: "paginating", label: "Paginando", hint: "carregando próxima página" },
  {
    k: "paginate-fallback",
    label: "Carregar mais",
    hint: "botão fallback no fim",
  },
  {
    k: "paginate-error",
    label: "Erro paginação",
    hint: "falha ao carregar mais",
  },
  { k: "end", label: "Fim da lista", hint: "não há mais páginas" },
];

function TweaksPanel({
  asyncState,
  onAsyncState,
  countersLoading,
  onCountersLoading,
  onClose,
}: {
  asyncState: AsyncState;
  onAsyncState: (s: AsyncState) => void;
  countersLoading: boolean;
  onCountersLoading: (v: boolean) => void;
  onClose: () => void;
}) {
  return (
    <div className="fixed bottom-5 right-5 z-[300] w-[280px] rounded-[10px] border border-border-default bg-subtle p-3.5 shadow-[0_16px_48px_rgba(0,0,0,0.7)]">
      <div className="mb-2.5 flex items-center gap-2 border-b border-border-subtle pb-2.5">
        <SlidersHorizontal className="h-3 w-3 text-fg-3" strokeWidth={1.5} />
        <span className="text-[12px] font-semibold tracking-tight text-fg-1">
          Tweaks
        </span>
        <span className="ml-auto text-[10.5px] text-fg-4">demo harness</span>
        <button
          type="button"
          onClick={onClose}
          className="rounded p-0.5 text-fg-4 hover:bg-elevated hover:text-fg-1"
        >
          <X className="h-3 w-3" strokeWidth={1.5} />
        </button>
      </div>
      <div className="mb-1.5 text-[10.5px] font-medium uppercase tracking-[0.08em] text-fg-4">
        Estado assíncrono
      </div>
      <div className="mb-3 flex flex-col gap-0.5">
        {ASYNC_STATES.map((s) => {
          const active = asyncState === s.k;
          return (
            <button
              key={s.k}
              type="button"
              onClick={() => onAsyncState(s.k)}
              className={cn(
                "flex items-center gap-2 rounded border px-2 py-1.5 text-left text-[12px] font-medium",
                active
                  ? "border-border-default bg-surface text-fg-1"
                  : "border-transparent bg-transparent text-fg-2",
              )}
            >
              <span
                className={cn(
                  "h-1.5 w-1.5 flex-shrink-0 rounded-full",
                  active ? "bg-accent-hover" : "bg-border-strong",
                )}
              />
              <span className="flex-1">{s.label}</span>
              <span className="text-[10.5px] font-normal text-fg-4">
                {s.hint}
              </span>
            </button>
          );
        })}
      </div>
      <div className="mb-1.5 text-[10.5px] font-medium uppercase tracking-[0.08em] text-fg-4">
        Outros
      </div>
      <label
        className={cn(
          "flex cursor-pointer items-center gap-2 rounded border px-2 py-1.5",
          countersLoading
            ? "border-border-default bg-surface"
            : "border-transparent bg-transparent",
        )}
      >
        <input
          type="checkbox"
          checked={countersLoading}
          onChange={(e) => onCountersLoading(e.target.checked)}
          className="m-0 accent-accent-hover"
        />
        <span className="flex-1 text-[12px] text-fg-1">
          Contadores carregando
        </span>
        <span className="text-[10.5px] text-fg-4">mostra —</span>
      </label>
    </div>
  );
}

// -----------------------------------------------------------
// KbdLegend — bottom-right keyboard hints
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
// TicketsPage — main composed view
// -----------------------------------------------------------

export type TicketsPageSearch = {
  view: TicketView;
  company: string;
  q: string;
};

export type TicketsPageProps = {
  search: TicketsPageSearch;
  onSearchChange: (next: Partial<TicketsPageSearch>) => void;
};

export function TicketsPage({ search, onSearchChange }: TicketsPageProps) {
  const queryClient = useQueryClient();

  const [rawQuery, setRawQuery] = React.useState(search.q);
  const [debouncedQuery, setDebouncedQuery] = React.useState(search.q);
  const [focusIdx, setFocusIdx] = React.useState(0);
  const [sorting, setSorting] = React.useState<SortingState>([]);
  const [newOpen, setNewOpen] = React.useState(false);
  const [asyncState, setAsyncState] = React.useState<AsyncState>("idle");
  const [countersLoading, setCountersLoading] = React.useState(false);
  const [tweaksOpen, setTweaksOpen] = React.useState(false);

  const searchInputRef = React.useRef<HTMLInputElement | null>(null);
  const companyRef = React.useRef<CompanyFilterHandle | null>(null);

  const view = search.view;
  const company = search.company;
  const isHistory = view === "closed";

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

  const ticketsQuery = useQuery<ListTicketsResponse>({
    queryKey: ["tickets", view, company, debouncedQuery],
    queryFn: () =>
      listTickets({
        view,
        companyName: company || undefined,
        query: debouncedQuery || undefined,
      }),
  });

  const countsQuery = useQuery<TicketCounts>({
    queryKey: ["tickets", "counts"],
    queryFn: getTicketCounts,
  });

  const clientsQuery = useQuery({
    queryKey: ["tickets", "clients"],
    queryFn: listClientNames,
  });

  const createMutation = useMutation({
    mutationFn: createTicket,
    onSuccess: ({ ticket }) => {
      queryClient.invalidateQueries({ queryKey: ["tickets"] });
      setNewOpen(false);
      toast.success(`Ticket ${ticket.id} criado.`);
    },
  });

  const currentUserEmail = useCurrentUserEmail();
  const table = useReactTable({
    data: ticketsQuery.data?.tickets ?? [],
    columns: ticketsColumns,
    state: { sorting },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    meta: { currentUserEmail, query: debouncedQuery },
  });

  const sortedTickets = table.getRowModel().rows.map((r) => r.original);

  const counts: TicketCounts = countsQuery.data ?? {
    all: 0,
    unassigned: 0,
    mine: 0,
    risk: 0,
    closed: 0,
  };

  React.useEffect(() => {
    setFocusIdx(0);
  }, [view, sorting, debouncedQuery, company]);

  React.useEffect(() => {
    if (focusIdx >= sortedTickets.length) {
      setFocusIdx(Math.max(0, sortedTickets.length - 1));
    }
  }, [sortedTickets.length, focusIdx]);

  // Keyboard
  React.useEffect(() => {
    const h = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement | null;
      const tag = target?.tagName;
      const inInput =
        tag === "INPUT" || tag === "TEXTAREA" || target?.isContentEditable;
      if (newOpen) return;

      if (e.key === "/" && !inInput) {
        e.preventDefault();
        searchInputRef.current?.focus();
        searchInputRef.current?.select();
        return;
      }
      if (
        e.key === "Escape" &&
        inInput &&
        tag === "INPUT" &&
        rawQuery &&
        document.activeElement === searchInputRef.current
      ) {
        setRawQuery("");
        return;
      }
      if (e.key === "n" && !inInput) {
        e.preventDefault();
        setNewOpen(true);
        return;
      }
      if (e.key === "f" && !inInput) {
        e.preventDefault();
        companyRef.current?.open();
        return;
      }
      if (inInput) return;

      if (e.key >= "1" && e.key <= "5") {
        const idx = parseInt(e.key, 10) - 1;
        const next = VIEWS[idx]?.k;
        if (next) onSearchChange({ view: next });
        return;
      }
      if (e.key === "J" && e.shiftKey) {
        if (
          asyncState === "paginate-fallback" ||
          asyncState === "idle" ||
          asyncState === "end"
        ) {
          e.preventDefault();
          setAsyncState("paginating");
          return;
        }
      }
      if (e.key === "j" || e.key === "ArrowDown") {
        e.preventDefault();
        setFocusIdx((i) => Math.min(sortedTickets.length - 1, i + 1));
      } else if (e.key === "k" || e.key === "ArrowUp") {
        e.preventDefault();
        setFocusIdx((i) => Math.max(0, i - 1));
      } else if (e.key === "Enter") {
        const t = sortedTickets[focusIdx];
        if (t) toast.info(`Abrir ${t.id} — ${t.subject}`);
      }
    };
    window.addEventListener("keydown", h);
    return () => window.removeEventListener("keydown", h);
  }, [
    sortedTickets,
    focusIdx,
    newOpen,
    rawQuery,
    asyncState,
    onSearchChange,
  ]);

  const resetSort = () => setSorting([]);

  const clearFilters = () => {
    setRawQuery("");
    setDebouncedQuery("");
    onSearchChange({ q: "", company: "" });
  };

  const viewLabel = VIEWS.find((v) => v.k === view)?.label ?? "";
  const refreshing =
    asyncState === "refreshing" ||
    (ticketsQuery.isFetching && !ticketsQuery.isLoading);

  const showLoading = asyncState === "loading" || ticketsQuery.isLoading;
  const showError = asyncState === "error" || ticketsQuery.isError;

  return (
    <div className="flex h-full flex-col overflow-hidden bg-app">
      <div className="flex items-center justify-between gap-3 px-6 pb-2.5 pt-4">
        <h1 className="m-0 flex-shrink-0 text-xl font-semibold leading-tight tracking-tight text-fg-1">
          Tickets
          <RefreshingDot visible={refreshing} />
        </h1>
        <div className="flex min-w-0 flex-1 items-center justify-end gap-2">
          <SearchInput
            value={rawQuery}
            onChange={setRawQuery}
            inputRef={searchInputRef}
          />
          <CompanyFilter
            ref={companyRef}
            value={company}
            onChange={(c) => onSearchChange({ company: c })}
            clients={clientsQuery.data ?? []}
          />
          {sorting.length > 0 && (
            <button
              type="button"
              onClick={resetSort}
              className="inline-flex h-[30px] items-center gap-1.5 whitespace-nowrap rounded-md border border-border-default bg-transparent px-2.5 text-[12px] font-medium text-fg-2"
            >
              <RotateCcw className="h-2.5 w-2.5" strokeWidth={1.5} />
              Padrão
            </button>
          )}
          <button
            type="button"
            onClick={() => setTweaksOpen((o) => !o)}
            title="Demo: estados assíncronos"
            className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-border-subtle text-fg-3 hover:text-fg-1"
          >
            <SlidersHorizontal className="h-3.5 w-3.5" strokeWidth={1.5} />
          </button>
          <button
            type="button"
            onClick={() => setNewOpen(true)}
            className="inline-flex h-8 items-center gap-2 whitespace-nowrap rounded-lg bg-fg-1 py-0 pl-3 pr-1.5 text-[13px] font-medium text-app transition-colors hover:opacity-90"
          >
            <Plus className="h-3 w-3" strokeWidth={1.75} />
            <span>Novo ticket</span>
            <kbd className="ml-0.5 rounded border border-black/10 bg-black/5 px-1.5 py-px font-mono text-[10px] leading-snug text-app">
              n
            </kbd>
          </button>
        </div>
      </div>

      <div className="flex items-center gap-3 overflow-x-auto border-b border-[#0f0f0f] px-6 pb-3.5">
        <SegmentedViews
          active={view}
          counts={counts}
          onChange={(v) => onSearchChange({ view: v })}
          countersLoading={countersLoading || countsQuery.isLoading}
        />
      </div>

      <ActiveFiltersBar
        viewLabel={viewLabel}
        company={company}
        query={debouncedQuery}
        resultCount={sortedTickets.length}
        onClear={clearFilters}
      />

      <div className="flex-1 overflow-auto px-6 pb-6">
        {showLoading ? (
          <div className="pt-3.5">
            <Table>
              <TableHeader>
                <TableRow>
                  {table.getHeaderGroups()[0]?.headers.map((h) => (
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
              </TableHeader>
              <TableSkeletonRows
                count={8}
                widths={[
                  64,
                  26,
                  80,
                  "60%",
                  "40%",
                  "50%",
                  70,
                ]}
              />
            </Table>
          </div>
        ) : showError ? (
          <ErrorState
            onRetry={() => {
              setAsyncState("loading");
              queryClient.invalidateQueries({ queryKey: ["tickets"] });
              setTimeout(() => setAsyncState("idle"), 600);
            }}
          />
        ) : (
          <div
            className="pt-3.5 transition-opacity"
            style={{
              opacity: debouncedQuery !== rawQuery ? 0.7 : 1,
              transitionDuration: "120ms",
            }}
          >
            {sortedTickets.length > 0 ? (
              <>
                <Table>
                  <TableHeader>
                    {table.getHeaderGroups().map((hg) => (
                      <TableRow key={hg.id}>
                        {hg.headers.map((h) => {
                          const sort = h.column.getIsSorted();
                          const canSort = h.column.getCanSort();
                          return (
                            <TableHead
                              key={h.id}
                              style={
                                h.column.columnDef.size
                                  ? { width: h.column.columnDef.size }
                                  : undefined
                              }
                            >
                              <button
                                type="button"
                                onClick={h.column.getToggleSortingHandler()}
                                className={cn(
                                  "inline-flex items-center gap-1 border-0 bg-transparent text-inherit",
                                  canSort && "cursor-pointer",
                                  sort && "text-fg-1",
                                )}
                              >
                                {flexRender(
                                  h.column.columnDef.header,
                                  h.getContext(),
                                )}
                                {sort === "asc" && (
                                  <ChevronUp
                                    className="h-2.5 w-2.5"
                                    strokeWidth={1.5}
                                  />
                                )}
                                {sort === "desc" && (
                                  <ChevronDown
                                    className="h-2.5 w-2.5"
                                    strokeWidth={1.5}
                                  />
                                )}
                              </button>
                            </TableHead>
                          );
                        })}
                      </TableRow>
                    ))}
                  </TableHeader>
                  <TableBody>
                    {table.getRowModel().rows.map((row, i) => (
                      <TableRow
                        key={row.id}
                        onMouseEnter={() => setFocusIdx(i)}
                        onClick={() =>
                          toast.info(
                            `Abrir ${row.original.id} — ${row.original.subject}`,
                          )
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
                {(asyncState === "paginating" ||
                  asyncState === "paginate-error" ||
                  asyncState === "paginate-fallback" ||
                  asyncState === "end") && (
                  <ListFooter
                    mode={asyncState as ListFooterMode}
                    totalCount={sortedTickets.length}
                    onLoadMore={() => setAsyncState("paginating")}
                    onRetry={() => setAsyncState("paginating")}
                  />
                )}
                <div className="flex items-center justify-between px-1 pt-3">
                  <div className="text-[11.5px] text-fg-4">
                    <span className="font-mono text-fg-2">
                      {sortedTickets.length}
                    </span>{" "}
                    de{" "}
                    <span className="font-mono text-fg-2">
                      {ticketsQuery.data?.totalCount ?? sortedTickets.length}
                    </span>{" "}
                    tickets
                    {sorting.length === 0 && !isHistory && (
                      <span className="ml-2 text-border-default">
                        · ordenação inteligente
                      </span>
                    )}
                    {isHistory && (
                      <span className="ml-2 text-border-default">
                        · últimos 30 dias
                      </span>
                    )}
                  </div>
                  <div className="flex items-center gap-3.5 text-[11px] text-fg-4">
                    <KbdLegend keys={["j", "k"]} label="navegar" />
                    <KbdLegend keys={["↵"]} label="abrir" />
                    <KbdLegend keys={["/"]} label="buscar" />
                    <KbdLegend keys={["f"]} label="empresa" />
                    <KbdLegend keys={["n"]} label="novo" />
                  </div>
                </div>
              </>
            ) : (
              <EmptyState
                view={view}
                query={debouncedQuery}
                company={company}
                viewLabel={viewLabel}
                onNew={() => setNewOpen(true)}
                onGoTo={(v) => onSearchChange({ view: v })}
                onClearFilters={clearFilters}
              />
            )}
          </div>
        )}
      </div>

      <NewTicketModal
        open={newOpen}
        onClose={() => setNewOpen(false)}
        onCreate={(req) => createMutation.mutate(req)}
        pending={createMutation.isPending}
      />

      {tweaksOpen && (
        <TweaksPanel
          asyncState={asyncState}
          onAsyncState={setAsyncState}
          countersLoading={countersLoading}
          onCountersLoading={setCountersLoading}
          onClose={() => setTweaksOpen(false)}
        />
      )}

    </div>
  );
}
