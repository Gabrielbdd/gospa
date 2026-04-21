import * as React from "react";
import {
  AlertTriangle,
  Check,
  ChevronDown,
  Search,
  Users,
  X,
} from "lucide-react";

import { cn } from "@/lib/utils";
import type { TeamMember } from "@/lib/team-client";
import { Spinner } from "@/components/tickets/async-states";
import { OwnerAvatar } from "@/components/companies/owner-avatar";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";

type Value = { contactId: string; fullName: string } | null;

export function OwnerPicker({
  value,
  onChange,
  members,
  loading = false,
  errorMsg = null,
  disabled,
  allowClear = true,
  placeholder = "Sem dono",
  compact = false,
}: {
  value: Value;
  onChange: (next: Value) => void;
  members: TeamMember[];
  loading?: boolean;
  errorMsg?: string | null;
  disabled?: boolean;
  allowClear?: boolean;
  placeholder?: string;
  compact?: boolean;
}) {
  const [open, setOpen] = React.useState(false);
  const [q, setQ] = React.useState("");
  const inputRef = React.useRef<HTMLInputElement | null>(null);

  React.useEffect(() => {
    if (open) {
      setQ("");
      setTimeout(() => inputRef.current?.focus(), 10);
    }
  }, [open]);

  const filtered = React.useMemo(() => {
    const needle = q.trim().toLowerCase();
    const active = members.filter((m) => m.status !== "suspended");
    if (!needle) return active;
    return active.filter(
      (m) =>
        m.fullName.toLowerCase().includes(needle) ||
        m.email.toLowerCase().includes(needle),
    );
  }, [members, q]);

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          disabled={disabled}
          className={cn(
            "flex w-full items-center gap-2 rounded-md border bg-transparent text-left text-[13px] text-fg-1 transition-colors",
            compact ? "h-8 px-2" : "h-9 px-2.5",
            disabled
              ? "cursor-not-allowed border-border-subtle opacity-60"
              : "border-border-default hover:bg-elevated/40",
          )}
        >
          <OwnerAvatar
            contactId={value?.contactId}
            fullName={value?.fullName}
            size={compact ? 18 : 20}
          />
          <span
            className={cn(
              "flex-1 truncate",
              value ? "text-fg-1" : "text-fg-3",
            )}
          >
            {value ? value.fullName : placeholder}
          </span>
          {value && allowClear && !disabled ? (
            <span
              role="button"
              tabIndex={0}
              onClick={(e) => {
                e.stopPropagation();
                onChange(null);
              }}
              className="inline-flex h-4 w-4 items-center justify-center rounded text-fg-3 hover:text-fg-1"
            >
              <X className="h-2.5 w-2.5" strokeWidth={1.5} />
            </span>
          ) : (
            <ChevronDown className="h-3 w-3 text-fg-4" strokeWidth={1.5} />
          )}
        </button>
      </PopoverTrigger>
      <PopoverContent
        align="start"
        className="w-(--radix-popover-trigger-width) p-0"
      >
        <div className="flex items-center gap-2 border-b border-border-subtle px-2.5 py-2">
          <Search className="h-3 w-3 text-fg-4" strokeWidth={1.5} />
          <input
            ref={inputRef}
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="Filtrar time…"
            className="flex-1 border-0 bg-transparent text-[13px] text-fg-1 outline-none placeholder:text-fg-4"
          />
        </div>
        <div className="max-h-[240px] overflow-y-auto p-1">
          {allowClear && (
            <button
              type="button"
              onClick={() => {
                onChange(null);
                setOpen(false);
              }}
              className={cn(
                "flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-[13px] text-fg-1 hover:bg-elevated",
                !value && "bg-elevated",
              )}
            >
              <Users className="h-3 w-3 text-fg-3" strokeWidth={1.5} />
              <span className="flex-1">Sem dono</span>
              {!value && <Check className="h-3 w-3 text-fg-3" strokeWidth={1.5} />}
            </button>
          )}
          {loading ? (
            <div className="flex items-center gap-2 px-2.5 py-3 text-[12px] text-fg-4">
              <Spinner size={10} />
              Carregando membros do time…
            </div>
          ) : errorMsg ? (
            <div className="flex items-start gap-2 px-2.5 py-3 text-[12px] text-danger">
              <AlertTriangle
                className="mt-0.5 h-3 w-3 flex-shrink-0"
                strokeWidth={1.5}
              />
              <span>Erro ao carregar time: {errorMsg}</span>
            </div>
          ) : filtered.length === 0 ? (
            <div className="px-2.5 py-3 text-[12px] text-fg-4">
              Nenhum membro do time.
            </div>
          ) : (
            filtered.map((m) => {
              const selected = value?.contactId === m.id;
              return (
                <button
                  key={m.id}
                  type="button"
                  onClick={() => {
                    onChange({ contactId: m.id, fullName: m.fullName });
                    setOpen(false);
                  }}
                  className={cn(
                    "flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-[13px] text-fg-1 hover:bg-elevated",
                    selected && "bg-elevated",
                  )}
                >
                  <OwnerAvatar
                    contactId={m.id}
                    fullName={m.fullName}
                    size={20}
                  />
                  <span className="flex-1 truncate">{m.fullName}</span>
                  {selected && (
                    <Check className="h-3 w-3 text-fg-3" strokeWidth={1.5} />
                  )}
                </button>
              );
            })
          )}
        </div>
      </PopoverContent>
    </Popover>
  );
}
