import { createRoute, useNavigate } from "@tanstack/react-router";
import * as React from "react";

import { RequireAuth } from "@/components/shell/require-auth";
import { Shell, useActiveNavKey } from "@/components/shell/shell";
import {
  CompaniesPage,
  type CompaniesPageSearch,
} from "@/components/companies/companies-page";
import { rootRoute } from "@/routes/__root";

const VIEWS = ["active", "inactive", "unowned", "all"] as const;

export const companiesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/companies",
  validateSearch: (raw: Record<string, unknown>): CompaniesPageSearch => {
    const view = VIEWS.includes(raw.view as (typeof VIEWS)[number])
      ? (raw.view as CompaniesPageSearch["view"])
      : "active";
    return {
      view,
      q: typeof raw.q === "string" ? raw.q : "",
    };
  },
  component: CompaniesRoute,
});

function CompaniesRoute() {
  const search = companiesRoute.useSearch();
  const navigate = useNavigate({ from: companiesRoute.fullPath });
  const activeKey = useActiveNavKey();

  const onSearchChange = React.useCallback(
    (next: Partial<CompaniesPageSearch>) => {
      void navigate({
        search: (prev) => ({ ...prev, ...next }),
        replace: true,
      });
    },
    [navigate],
  );

  return (
    <RequireAuth>
      <Shell activeKey={activeKey} breadcrumbs={["Empresas"]}>
        <CompaniesPage search={search} onSearchChange={onSearchChange} />
      </Shell>
    </RequireAuth>
  );
}
