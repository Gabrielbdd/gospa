import { createRoute } from "@tanstack/react-router";

import { RequireAuth } from "@/components/shell/require-auth";
import { Shell, useActiveNavKey } from "@/components/shell/shell";
import { CompanyDetail } from "@/components/companies/company-detail";
import { rootRoute } from "@/routes/__root";

export const companyDetailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/companies/$companyId",
  component: CompanyDetailRoute,
});

function CompanyDetailRoute() {
  const { companyId } = companyDetailRoute.useParams();
  const activeKey = useActiveNavKey();
  return (
    <RequireAuth>
      <Shell activeKey={activeKey} breadcrumbs={["Empresas", companyId.slice(0, 8)]}>
        <CompanyDetail companyId={companyId} />
      </Shell>
    </RequireAuth>
  );
}
