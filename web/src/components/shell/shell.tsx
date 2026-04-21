import * as React from "react";
import { Link, useRouterState } from "@tanstack/react-router";
import {
  Building2,
  ChevronsLeft,
  ChevronsRight,
  type LucideIcon,
  Settings as SettingsIcon,
  Ticket as TicketIcon,
} from "lucide-react";

import { cn } from "@/lib/utils";

type NavKey = "tickets" | "companies" | "settings";

type NavItem = {
  k: NavKey;
  label: string;
  icon: LucideIcon;
  to: string;
  badge?: string;
};

type NavSection = { label: string; items: NavItem[] };

const SECTIONS: NavSection[] = [
  {
    label: "Workspace",
    items: [
      { k: "tickets", label: "Tickets", icon: TicketIcon, to: "/tickets" },
      { k: "companies", label: "Empresas", icon: Building2, to: "/companies" },
    ],
  },
  {
    label: "Conta",
    items: [
      {
        k: "settings",
        label: "Configurações",
        icon: SettingsIcon,
        to: "/settings",
      },
    ],
  },
];

const COLLAPSED_KEY = "gospa.sidebarCollapsed";

function useCollapsed(): [boolean, () => void] {
  const [collapsed, setCollapsed] = React.useState(() => {
    try {
      return localStorage.getItem(COLLAPSED_KEY) === "1";
    } catch {
      return false;
    }
  });
  const toggle = React.useCallback(() => {
    setCollapsed((c) => {
      const next = !c;
      try {
        localStorage.setItem(COLLAPSED_KEY, next ? "1" : "0");
      } catch {
        /* ignore */
      }
      return next;
    });
  }, []);
  return [collapsed, toggle];
}

type SidebarProps = {
  collapsed: boolean;
  onToggle: () => void;
  activeKey: NavKey | null;
};

function Sidebar({ collapsed, onToggle, activeKey }: SidebarProps) {
  return (
    <aside
      className={cn(
        "flex h-full flex-col border-r border-border-subtle bg-app transition-[width] duration-200 ease-out",
        collapsed ? "w-14" : "w-60",
      )}
      style={{ flexShrink: 0 }}
    >
      <div
        className={cn(
          "flex h-[57px] items-center border-b border-border-subtle",
          collapsed ? "justify-center" : "px-3.5",
        )}
      >
        <button
          type="button"
          onClick={onToggle}
          title={collapsed ? "Expandir (⌘\\)" : "Recolher (⌘\\)"}
          className="inline-flex h-7 w-7 items-center justify-center rounded-md text-fg-3 transition-colors hover:bg-elevated hover:text-fg-1"
        >
          {collapsed ? (
            <ChevronsRight className="h-4 w-4" strokeWidth={1.5} />
          ) : (
            <ChevronsLeft className="h-4 w-4" strokeWidth={1.5} />
          )}
        </button>
      </div>

      <nav
        className={cn(
          "flex flex-1 flex-col overflow-y-auto overflow-x-hidden px-2 py-1",
          collapsed ? "gap-2" : "gap-5",
        )}
      >
        {SECTIONS.map((section, si) => (
          <div key={section.label}>
            {!collapsed && (
              <div className="mb-1.5 px-2.5 text-[11px] font-medium uppercase tracking-[0.08em] text-fg-4">
                {section.label}
              </div>
            )}
            {collapsed && si > 0 && (
              <div className="mx-2 mb-2 h-px bg-border-subtle" />
            )}
            <ul className="flex flex-col gap-0.5">
              {section.items.map((item) => (
                <li key={item.k}>
                  <NavLink
                    item={item}
                    active={activeKey === item.k}
                    collapsed={collapsed}
                  />
                </li>
              ))}
            </ul>
          </div>
        ))}
      </nav>

      <div
        className={cn(
          "flex items-center gap-2.5 border-t border-border-subtle",
          collapsed ? "justify-center py-3" : "p-3",
        )}
      >
        <div
          title={collapsed ? "Você · admin" : ""}
          className="inline-flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-full bg-border-default text-xs font-semibold text-fg-1"
        >
          VC
        </div>
        {!collapsed && (
          <>
            <div className="flex min-w-0 flex-1 flex-col leading-tight">
              <span className="truncate text-[13px] font-medium text-fg-1">
                Você
              </span>
              <span className="text-[11px] text-fg-4">admin</span>
            </div>
            <SettingsIcon
              className="h-3.5 w-3.5 cursor-pointer text-fg-3"
              strokeWidth={1.5}
            />
          </>
        )}
      </div>
    </aside>
  );
}

function NavLink({
  item,
  active,
  collapsed,
}: {
  item: NavItem;
  active: boolean;
  collapsed: boolean;
}) {
  const Icon = item.icon;
  return (
    <Link
      to={item.to}
      title={collapsed ? item.label : undefined}
      className={cn(
        "relative flex items-center rounded-md border-0 text-[13px] font-medium no-underline transition-colors",
        collapsed ? "h-8 justify-center" : "h-8 gap-2.5 px-2.5",
        active
          ? "bg-elevated text-fg-1"
          : "text-fg-3 hover:bg-elevated hover:text-fg-1",
      )}
    >
      <Icon className="h-4 w-4 flex-shrink-0" strokeWidth={1.5} />
      {!collapsed && <span className="flex-1 truncate">{item.label}</span>}
      {!collapsed && item.badge && (
        <span className="font-mono text-[11px] text-fg-3">{item.badge}</span>
      )}
      {collapsed && item.badge && (
        <span className="absolute right-1.5 top-1 h-1.5 w-1.5 rounded-full bg-accent-hover" />
      )}
    </Link>
  );
}

function TopBar({ breadcrumbs }: { breadcrumbs: string[] }) {
  return (
    <header className="flex h-14 flex-shrink-0 items-center border-b border-border-subtle bg-app px-6">
      <div className="flex items-center gap-2">
        {breadcrumbs.map((crumb, i) => {
          const last = i === breadcrumbs.length - 1;
          return (
            <React.Fragment key={`${crumb}-${i}`}>
              <span
                className={cn(
                  "text-sm",
                  last
                    ? "font-medium text-fg-1"
                    : "font-normal text-fg-3",
                )}
              >
                {crumb}
              </span>
              {!last && <span className="text-fg-4">/</span>}
            </React.Fragment>
          );
        })}
      </div>
    </header>
  );
}

export type ShellProps = {
  activeKey?: NavKey | null;
  breadcrumbs: string[];
  children: React.ReactNode;
};

export function Shell({ activeKey = null, breadcrumbs, children }: ShellProps) {
  const [collapsed, toggle] = useCollapsed();

  React.useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "\\") {
        e.preventDefault();
        toggle();
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [toggle]);

  return (
    <div className="flex h-screen overflow-hidden bg-app text-fg-1">
      <Sidebar collapsed={collapsed} onToggle={toggle} activeKey={activeKey} />
      <div className="flex min-w-0 flex-1 flex-col overflow-hidden">
        <TopBar breadcrumbs={breadcrumbs} />
        <main className="flex-1 overflow-hidden">{children}</main>
      </div>
    </div>
  );
}

export function useActiveNavKey(): NavKey | null {
  const path = useRouterState({ select: (s) => s.location.pathname });
  if (path.startsWith("/tickets")) return "tickets";
  if (path.startsWith("/companies")) return "companies";
  if (path.startsWith("/settings")) return "settings";
  return null;
}
