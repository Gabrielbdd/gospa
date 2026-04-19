// Hand-wired Connect RPC client for the CompaniesService. Same
// pattern as install-client; the goal at this stage is just to prove
// the bearer-token flow end-to-end on a real private endpoint.
// Generated client types will replace this when the proto-es pipeline
// lands.

import { apiCall } from "@/lib/api-client";

export interface Company {
  id: string;
  name: string;
  slug: string;
  zitadelOrgId: string;
  createdAt: string;
  archivedAt?: string;
}

export interface ListCompaniesResponse {
  companies?: Company[];
}

const COMPANIES_SERVICE = "/gospa.companies.v1.CompaniesService";

export async function listCompanies(accessToken: string): Promise<ListCompaniesResponse> {
  return apiCall<Record<string, never>, ListCompaniesResponse>(
    COMPANIES_SERVICE,
    "ListCompanies",
    {},
    { accessToken },
  );
}
