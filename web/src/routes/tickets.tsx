import { createRoute, useNavigate } from "@tanstack/react-router";
import * as React from "react";

import { RequireAuth } from "@/components/shell/require-auth";
import { Shell, useActiveNavKey } from "@/components/shell/shell";
import {
  TicketsPage,
  type TicketsPageSearch,
} from "@/components/tickets/tickets-page";
import { rootRoute } from "@/routes/__root";
import type { TicketView } from "@/lib/tickets-client";

const ALLOWED_VIEWS: TicketView[] = [
  "all",
  "unassigned",
  "mine",
  "risk",
  "closed",
];

export const ticketsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/tickets",
  validateSearch: (raw: Record<string, unknown>): TicketsPageSearch => {
    const view = ALLOWED_VIEWS.includes(raw.view as TicketView)
      ? (raw.view as TicketView)
      : "all";
    return {
      view,
      company: typeof raw.company === "string" ? raw.company : "",
      q: typeof raw.q === "string" ? raw.q : "",
    };
  },
  component: TicketsRoute,
});

function TicketsRoute() {
  const search = ticketsRoute.useSearch();
  const navigate = useNavigate({ from: ticketsRoute.fullPath });
  const activeKey = useActiveNavKey();

  const onSearchChange = React.useCallback(
    (next: Partial<TicketsPageSearch>) => {
      void navigate({
        search: (prev) => ({ ...prev, ...next }),
        replace: true,
      });
    },
    [navigate],
  );

  return (
    <RequireAuth>
      <Shell activeKey={activeKey} breadcrumbs={["Tickets"]}>
        <TicketsPage search={search} onSearchChange={onSearchChange} />
      </Shell>
    </RequireAuth>
  );
}
