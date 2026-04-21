import { AlertTriangle, ChevronDown, RotateCcw } from "lucide-react";

import { cn } from "@/lib/utils";

const SHIMMER_BG =
  "linear-gradient(90deg, var(--surface) 0%, var(--gray-300) 50%, var(--surface) 100%)";

function Shimmer({
  width,
  height = 10,
  className,
  rounded,
}: {
  width: number | string;
  height?: number;
  className?: string;
  rounded?: number | string;
}) {
  return (
    <div
      className={className}
      style={{
        width,
        height,
        borderRadius: rounded ?? 3,
        background: SHIMMER_BG,
        backgroundSize: "200% 100%",
        animation: "gospaShimmer 1.5s ease-in-out infinite",
      }}
    />
  );
}

// TableSkeletonRows renders `<tr>` rows wrapped in `<tbody>` so it
// slots cleanly into a shadcn `<Table>`. Each column is a `<Shimmer>`
// sized by the caller's `widths` array. Keeps the 44px row height the
// real table uses so the layout doesn't jump when data resolves.
export function TableSkeletonRows({
  count = 6,
  widths,
}: {
  count?: number;
  // One entry per column. Numbers render as fixed px widths; strings
  // render as CSS values (e.g., "60%").
  widths: Array<number | string>;
}) {
  return (
    <tbody>
      {Array.from({ length: count }).map((_, i) => (
        <tr key={i} className="border-b border-border-subtle">
          {widths.map((w, j) => (
            <td key={j} className="h-11 px-4 align-middle">
              <Shimmer width={w} />
            </td>
          ))}
        </tr>
      ))}
    </tbody>
  );
}

export function SkeletonRows({
  cols,
  count = 8,
}: {
  cols: string;
  count?: number;
}) {
  return (
    <div>
      {Array.from({ length: count }).map((_, i) => (
        <div
          key={i}
          className="grid h-11 items-center gap-3.5 border-b border-[#0f0f0f] px-2"
          style={{ gridTemplateColumns: cols }}
        >
          <div className="flex justify-center">
            <Shimmer width={24} height={16} />
          </div>
          <Shimmer width={6} height={6} rounded="50%" />
          <Shimmer width={72} />
          <Shimmer width={`${60 + ((i * 13) % 30)}%`} />
          <Shimmer width={`${50 + ((i * 7) % 30)}%`} />
          <Shimmer width={`${55 + ((i * 11) % 25)}%`} />
          <Shimmer width={22} height={22} rounded="50%" />
          <Shimmer width={40} />
        </div>
      ))}
    </div>
  );
}

export function ErrorState({ onRetry }: { onRetry: () => void }) {
  return (
    <div className="flex flex-col items-center justify-center gap-3.5 py-[72px] text-center">
      <div className="flex h-10 w-10 items-center justify-center rounded-[10px] border border-warning/25 bg-warning/[0.08] text-warning">
        <AlertTriangle className="h-4 w-4" strokeWidth={1.5} />
      </div>
      <div className="text-[15px] font-medium tracking-tight text-fg-1">
        Não conseguimos carregar os tickets.
      </div>
      <div className="max-w-[320px] text-[13px] leading-normal text-fg-3">
        Pode ser um problema temporário de conexão ou do servidor.
      </div>
      <div className="mt-1 flex items-center gap-2.5">
        <button
          type="button"
          onClick={onRetry}
          className="inline-flex h-[30px] items-center gap-1.5 rounded-md bg-fg-1 px-3 text-[12.5px] font-medium text-app"
        >
          <RotateCcw className="h-2.5 w-2.5" strokeWidth={1.75} />
          Tentar novamente
        </button>
        <a
          href="#"
          onClick={(e) => e.preventDefault()}
          className="border-0 text-[12px] text-accent-hover hover:no-underline"
        >
          Reportar problema
        </a>
      </div>
    </div>
  );
}

export function Spinner({
  size = 12,
  className,
}: {
  size?: number;
  className?: string;
}) {
  return (
    <span
      className={cn("inline-block flex-shrink-0", className)}
      style={{
        width: size,
        height: size,
        borderRadius: "50%",
        border: "1.5px solid color-mix(in oklab, currentColor 22%, transparent)",
        borderTopColor: "currentColor",
        animation: "gospaSpin 700ms linear infinite",
      }}
    />
  );
}

export type ListFooterMode =
  | "paginating"
  | "paginate-fallback"
  | "paginate-error"
  | "end";

export function ListFooter({
  mode,
  totalCount,
  onLoadMore,
  onRetry,
}: {
  mode: ListFooterMode;
  totalCount: number;
  onLoadMore: () => void;
  onRetry: () => void;
}) {
  const base =
    "flex items-center justify-center gap-2 px-2 py-3 text-[12px]";
  if (mode === "paginating") {
    return (
      <div className={cn(base, "text-fg-3")}>
        <Spinner size={12} />
        <span>Carregando mais…</span>
      </div>
    );
  }
  if (mode === "paginate-fallback") {
    return (
      <button
        type="button"
        onClick={onLoadMore}
        className={cn(
          base,
          "mt-1 w-full rounded-md border border-dashed border-border-default text-fg-2 hover:bg-elevated/40",
        )}
      >
        <ChevronDown className="h-3 w-3" strokeWidth={1.5} />
        <span>Carregar mais</span>
        <kbd className="ml-1 rounded border border-border-subtle bg-surface px-1.5 py-px font-mono text-[10px] text-fg-2">
          ⇧J
        </kbd>
      </button>
    );
  }
  if (mode === "paginate-error") {
    return (
      <div className={cn(base, "text-warning")}>
        <AlertTriangle className="h-3 w-3" strokeWidth={1.5} />
        <span className="text-fg-2">Erro ao carregar mais tickets.</span>
        <button
          type="button"
          onClick={onRetry}
          className="ml-1 rounded border border-border-default bg-transparent px-2 py-0.5 text-[11px] text-fg-1"
        >
          Tentar novamente
        </button>
      </div>
    );
  }
  return (
    <div className={cn(base, "text-fg-4")}>
      Você chegou ao fim —{" "}
      <span className="font-mono text-fg-3">{totalCount}</span> tickets.
    </div>
  );
}

export function RefreshingDot({ visible }: { visible: boolean }) {
  return (
    <span
      aria-hidden="true"
      className="ml-2.5 inline-block h-1.5 w-1.5 rounded-full bg-accent-hover align-middle"
      style={{
        opacity: visible ? 1 : 0,
        transform: visible ? "scale(1)" : "scale(0.6)",
        transition: "opacity 180ms, transform 180ms",
        animation: visible ? "gospaPulse 1.4s ease-in-out infinite" : "none",
      }}
    />
  );
}
