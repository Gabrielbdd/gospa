import { Link } from "@tanstack/react-router";
import { Building, CreditCard, type LucideIcon, Users } from "lucide-react";

import { cn } from "@/lib/utils";

type RailKey = "team" | "workspace" | "billing";

type RailItem = {
  k: RailKey;
  label: string;
  icon: LucideIcon;
  to?: string;
  disabled?: boolean;
  hint?: string;
};

const ITEMS: RailItem[] = [
  { k: "team", label: "Time", icon: Users, to: "/settings/team" },
  {
    k: "workspace",
    label: "Workspace",
    icon: Building,
    disabled: true,
    hint: "Em breve",
  },
  {
    k: "billing",
    label: "Faturamento",
    icon: CreditCard,
    disabled: true,
    hint: "Em breve",
  },
];

export function SettingsRail({ active }: { active: RailKey }) {
  return (
    <aside
      className="flex w-[220px] flex-shrink-0 flex-col gap-0.5 border-r border-border-subtle bg-app px-3 py-6"
    >
      <div className="px-2.5 pb-2 text-[11px] font-medium uppercase tracking-[0.08em] text-fg-4">
        Configurações
      </div>
      {ITEMS.map((it) => {
        const isActive = active === it.k;
        const Icon = it.icon;
        const className = cn(
          "flex items-center gap-2.5 rounded-md border-0 px-2.5 py-1.5 text-[13px] font-medium no-underline transition-colors",
          it.disabled
            ? "cursor-not-allowed text-fg-4"
            : isActive
              ? "bg-elevated text-fg-1"
              : "text-fg-3 hover:bg-elevated hover:text-fg-1",
        );
        const inner = (
          <>
            <Icon className="h-3.5 w-3.5" strokeWidth={1.5} />
            <span className="flex-1">{it.label}</span>
            {it.hint && (
              <span className="rounded-sm border border-border-subtle bg-surface px-1 py-px font-mono text-[10px] uppercase tracking-[0.06em] text-fg-4">
                {it.hint}
              </span>
            )}
          </>
        );
        if (it.disabled || !it.to) {
          return (
            <div key={it.k} className={className} aria-disabled>
              {inner}
            </div>
          );
        }
        return (
          <Link key={it.k} to={it.to} className={className}>
            {inner}
          </Link>
        );
      })}
    </aside>
  );
}
