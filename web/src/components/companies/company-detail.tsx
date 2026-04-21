import * as React from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useAuth } from "react-oidc-context";
import {
  AlertTriangle,
  Archive,
  ArrowLeft,
  Check,
  RotateCcw,
} from "lucide-react";
import { toast } from "sonner";

import { cn } from "@/lib/utils";
import {
  archiveCompany,
  type Company,
  getCompany,
  restoreCompany,
  updateCompany,
} from "@/lib/companies-client";
import { listMembers } from "@/lib/team-client";
import { ErrorState, Spinner } from "@/components/tickets/async-states";
import { OwnerPicker } from "@/components/companies/owner-picker";

// -----------------------------------------------------------
// Status dot
// -----------------------------------------------------------

function StatusPill({ active }: { active: boolean }) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded border px-2 py-0.5 text-[11px] font-medium",
        active
          ? "border-border-default text-fg-2"
          : "border-border-default bg-surface text-fg-3",
      )}
    >
      <span
        className={cn(
          "h-1 w-1 rounded-full",
          active ? "bg-success" : "bg-border-strong",
        )}
      />
      {active ? "Ativa" : "Desativada"}
    </span>
  );
}

// -----------------------------------------------------------
// Sidebar field wrapper
// -----------------------------------------------------------

function SideField({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div>
      <div className="mb-1.5 text-[10.5px] font-medium uppercase tracking-[0.08em] text-fg-4">
        {label}
      </div>
      {children}
    </div>
  );
}

// -----------------------------------------------------------
// Detail page
// -----------------------------------------------------------

export function CompanyDetail({ companyId }: { companyId: string }) {
  const auth = useAuth();
  const accessToken = auth.user?.access_token;
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const companyQuery = useQuery({
    queryKey: ["companies", companyId],
    queryFn: () => getCompany(companyId, accessToken!),
    enabled: !!accessToken,
  });

  const membersQuery = useQuery({
    queryKey: ["team", "members"],
    queryFn: () => listMembers(accessToken!),
    enabled: !!accessToken,
  });

  const [name, setName] = React.useState("");
  const [owner, setOwner] = React.useState<
    { contactId: string; fullName: string } | null
  >(null);
  const [errorMsg, setErrorMsg] = React.useState<string | null>(null);

  // Seed the form state from the fetched company. Re-seeding when the
  // fetched row changes keeps server-side updates (e.g. another
  // operator renaming) in sync, but only when the form is clean —
  // otherwise we'd clobber the user's in-progress edit.
  const serverName = companyQuery.data?.name ?? "";
  const serverOwner = companyQuery.data?.owner
    ? {
        contactId: companyQuery.data.owner.contactId,
        fullName: companyQuery.data.owner.fullName,
      }
    : null;

  const [initialSeedKey, setInitialSeedKey] = React.useState<string | null>(
    null,
  );
  const seedKey = companyQuery.data
    ? `${companyQuery.data.id}:${serverName}:${serverOwner?.contactId ?? ""}`
    : null;

  React.useEffect(() => {
    if (!companyQuery.data) return;
    if (seedKey && seedKey !== initialSeedKey) {
      setName(serverName);
      setOwner(serverOwner);
      setInitialSeedKey(seedKey);
      setErrorMsg(null);
    }
  }, [companyQuery.data, seedKey, initialSeedKey, serverName, serverOwner]);

  const dirty =
    (companyQuery.data && name.trim() !== serverName) ||
    (owner?.contactId ?? "") !== (serverOwner?.contactId ?? "");

  const updateMut = useMutation({
    mutationFn: () =>
      updateCompany(
        {
          id: companyId,
          name: name.trim(),
          ownerContactId: owner?.contactId,
        },
        accessToken!,
      ),
    onSuccess: (c: Company) => {
      queryClient.setQueryData(["companies", companyId], c);
      queryClient.invalidateQueries({ queryKey: ["companies"] });
      setErrorMsg(null);
      toast.success("Alterações salvas.");
    },
    onError: (err: Error) => setErrorMsg(err.message),
  });

  const archiveMut = useMutation({
    mutationFn: () => archiveCompany(companyId, accessToken!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["companies"] });
      queryClient.invalidateQueries({ queryKey: ["companies", companyId] });
      toast("Empresa desativada.", {
        icon: <Archive className="size-4" strokeWidth={1.5} />,
      });
    },
    onError: (err: Error) => toast.error(err.message),
  });

  const restoreMut = useMutation({
    mutationFn: () => restoreCompany(companyId, accessToken!),
    onSuccess: (c: Company) => {
      queryClient.setQueryData(["companies", companyId], c);
      queryClient.invalidateQueries({ queryKey: ["companies"] });
      toast.success("Empresa reativada.");
    },
    onError: (err: Error) => toast.error(err.message),
  });

  const back = React.useCallback(() => {
    void navigate({ to: "/companies", search: { view: "active", q: "" } });
  }, [navigate]);

  // Keyboard: Esc goes back (only when nothing focused); ⌘+S saves
  React.useEffect(() => {
    const h = (e: KeyboardEvent) => {
      const tag = (e.target as HTMLElement | null)?.tagName;
      const inInput = tag === "INPUT" || tag === "TEXTAREA";
      if ((e.metaKey || e.ctrlKey) && e.key === "s") {
        if (dirty && !updateMut.isPending) {
          e.preventDefault();
          updateMut.mutate();
        }
        return;
      }
      if (e.key === "Escape" && !inInput && !dirty) {
        back();
      }
    };
    window.addEventListener("keydown", h);
    return () => window.removeEventListener("keydown", h);
  }, [dirty, updateMut, back]);

  if (companyQuery.isLoading) {
    return (
      <div className="flex h-full items-center justify-center bg-app">
        <Spinner size={16} />
      </div>
    );
  }
  if (companyQuery.isError || !companyQuery.data) {
    return (
      <div className="flex h-full items-center justify-center bg-app">
        <ErrorState onRetry={() => companyQuery.refetch()} />
      </div>
    );
  }

  const company = companyQuery.data;
  const archived = !!company.archivedAt;
  const valid = name.trim().length >= 2;

  return (
    <div className="flex h-full flex-col overflow-hidden bg-app">
      <header className="flex items-center justify-between gap-3 border-b border-[#0f0f0f] px-6 py-3.5">
        <div className="flex items-center gap-2.5">
          <button
            type="button"
            onClick={back}
            className="inline-flex h-7 items-center gap-1 rounded-md border border-border-subtle bg-transparent pl-1.5 pr-2 text-[12px] text-fg-2 hover:text-fg-1"
          >
            <ArrowLeft className="h-3 w-3" strokeWidth={1.5} />
            Companies
          </button>
          <span className="text-border-default">/</span>
          <span className="font-mono text-[12.5px] text-fg-2">
            {company.id.slice(0, 8)}
          </span>
          <StatusPill active={!archived} />
        </div>
        <div className="flex items-center gap-2">
          {dirty && (
            <button
              type="button"
              onClick={() => {
                setName(serverName);
                setOwner(serverOwner);
                setErrorMsg(null);
              }}
              disabled={updateMut.isPending}
              className="h-[30px] rounded-md border border-border-default bg-transparent px-3 text-[12.5px] font-medium text-fg-2 hover:text-fg-1 disabled:opacity-50"
            >
              Descartar
            </button>
          )}
          <button
            type="button"
            onClick={() => updateMut.mutate()}
            disabled={!dirty || !valid || updateMut.isPending}
            className={cn(
              "inline-flex h-[30px] items-center gap-2 rounded-md px-3 text-[12.5px] font-medium transition-colors",
              dirty && valid && !updateMut.isPending
                ? "bg-fg-1 text-app hover:opacity-90"
                : "cursor-not-allowed bg-border-default text-fg-4",
            )}
          >
            {updateMut.isPending ? (
              <>
                <Spinner size={10} />
                Salvando…
              </>
            ) : (
              <>
                <Check className="h-3 w-3" strokeWidth={1.75} />
                Salvar
                <kbd
                  className={cn(
                    "rounded border px-1.5 py-px font-mono text-[10px] leading-snug",
                    dirty && valid
                      ? "border-black/10 bg-black/5 text-app"
                      : "border-border-subtle text-fg-4",
                  )}
                >
                  ⌘S
                </kbd>
              </>
            )}
          </button>
          {archived ? (
            <button
              type="button"
              onClick={() => restoreMut.mutate()}
              disabled={restoreMut.isPending}
              className="inline-flex h-[30px] items-center gap-1.5 rounded-md border border-success/30 bg-success/[0.08] px-2.5 text-[12px] font-medium text-success disabled:opacity-50"
            >
              {restoreMut.isPending ? (
                <Spinner size={10} />
              ) : (
                <RotateCcw className="h-3 w-3" strokeWidth={1.5} />
              )}
              Reativar
            </button>
          ) : (
            <button
              type="button"
              onClick={() => archiveMut.mutate()}
              disabled={archiveMut.isPending}
              className="inline-flex h-[30px] items-center gap-1.5 rounded-md border border-border-default bg-transparent px-2.5 text-[12px] font-medium text-fg-2 hover:text-fg-1 disabled:opacity-50"
            >
              {archiveMut.isPending ? (
                <Spinner size={10} />
              ) : (
                <Archive className="h-3 w-3" strokeWidth={1.5} />
              )}
              Desativar
            </button>
          )}
        </div>
      </header>

      <div className="flex-1 overflow-auto">
        <div className="grid h-full" style={{ gridTemplateColumns: "1fr 320px" }}>
          <div className="min-w-0 px-8 pb-14 pt-7">
            <div className="max-w-[720px]">
              <div className="mb-1.5 text-[11px] font-medium uppercase tracking-[0.08em] text-fg-4">
                Nome
              </div>
              <input
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Nome da empresa"
                className={cn(
                  "w-full rounded-md border bg-transparent px-3 py-2 text-2xl font-semibold tracking-tight text-fg-1 outline-none transition-colors",
                  archived && "text-fg-3",
                  "border-transparent hover:border-border-default focus:border-accent-hover",
                )}
              />
              {!valid && name.length > 0 && (
                <div className="mt-1.5 text-[12px] text-danger">
                  Mínimo 2 caracteres.
                </div>
              )}
              {errorMsg && (
                <div className="mt-3 flex items-start gap-2 rounded-md border border-danger/30 bg-danger/10 p-2.5 text-[12px] text-danger">
                  <AlertTriangle
                    className="mt-0.5 h-3 w-3 flex-shrink-0"
                    strokeWidth={1.5}
                  />
                  <span>{errorMsg}</span>
                </div>
              )}
            </div>

            <div className="mt-10 max-w-[720px] rounded-lg border border-dashed border-border-subtle bg-subtle p-4">
              <div className="text-[12.5px] font-medium text-fg-2">
                Contatos
                <span className="ml-2 font-mono text-[10.5px] text-fg-4">
                  em breve
                </span>
              </div>
              <div className="mt-1 text-[12px] text-fg-3">
                Lista de contatos vinculados a esta empresa aparece aqui.
              </div>
            </div>
          </div>

          <aside className="flex flex-col gap-5 border-l border-[#0f0f0f] px-6 pb-14 pt-7">
            <SideField label="Dono">
              <OwnerPicker
                value={owner}
                onChange={setOwner}
                members={membersQuery.data ?? []}
                loading={membersQuery.isLoading}
                errorMsg={
                  membersQuery.error
                    ? (membersQuery.error as Error).message
                    : null
                }
                compact
              />
            </SideField>
            <SideField label="Status">
              <StatusPill active={!archived} />
            </SideField>
            <SideField label="Criada em">
              <div className="font-mono text-[12px] text-fg-2">
                {company.createdAt
                  ? new Date(company.createdAt).toLocaleString("pt-BR", {
                      dateStyle: "short",
                      timeStyle: "short",
                    })
                  : "—"}
              </div>
            </SideField>
            <div className="mt-2 h-px bg-border-subtle" />
            <div className="text-[11px] leading-relaxed text-fg-4">
              Edite os campos e clique em Salvar (ou{" "}
              <kbd className="rounded border border-border-default bg-surface px-1 py-px font-mono text-[10px]">
                ⌘S
              </kbd>
              ). <kbd className="rounded border border-border-default bg-surface px-1 py-px font-mono text-[10px]">Esc</kbd> volta para a lista.
            </div>
          </aside>
        </div>
      </div>

    </div>
  );
}
