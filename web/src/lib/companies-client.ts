// Hand-wired Connect+JSON client for CompaniesService. Mirrors the
// team-client pattern; will be replaced when the proto-es codegen
// pipeline lands.

import { apiCall } from "@/lib/api-client";

const SERVICE = "/gospa.companies.v1.CompaniesService";

export interface Address {
  line1?: string;
  line2?: string;
  city?: string;
  region?: string;
  postalCode?: string;
  country?: string;
  timezone?: string;
}

export interface Owner {
  contactId: string;
  fullName: string;
}

export interface Company {
  id: string;
  name: string;
  zitadelOrgId: string;
  createdAt: string;
  archivedAt?: string;
  address?: Address;
  isWorkspaceOwner?: boolean;
  owner?: Owner;
}

interface WireCompany {
  id: string;
  name?: string;
  zitadelOrgId?: string;
  createdAt?: string;
  archivedAt?: string;
  address?: Address;
  isWorkspaceOwner?: boolean;
  owner?: WireOwner;
}

interface WireOwner {
  contactId?: string;
  fullName?: string;
}

function companyFromWire(w: WireCompany): Company {
  return {
    id: w.id,
    name: w.name ?? "",
    zitadelOrgId: w.zitadelOrgId ?? "",
    createdAt: w.createdAt ?? "",
    archivedAt: w.archivedAt || undefined,
    address: w.address,
    isWorkspaceOwner: w.isWorkspaceOwner ?? false,
    owner: w.owner?.contactId
      ? { contactId: w.owner.contactId, fullName: w.owner.fullName ?? "" }
      : undefined,
  };
}

export interface ListCompaniesResponse {
  companies?: Company[];
}

export async function listCompanies(
  accessToken: string,
): Promise<ListCompaniesResponse> {
  const resp = await apiCall<
    Record<string, never>,
    { companies?: WireCompany[] }
  >(SERVICE, "ListCompanies", {}, { accessToken });
  return { companies: (resp.companies ?? []).map(companyFromWire) };
}

export async function getCompany(
  id: string,
  accessToken: string,
): Promise<Company> {
  const resp = await apiCall<{ id: string }, { company: WireCompany }>(
    SERVICE,
    "GetCompany",
    { id },
    { accessToken },
  );
  return companyFromWire(resp.company);
}

export interface CreateCompanyInput {
  name: string;
  ownerContactId?: string;
}

export async function createCompany(
  input: CreateCompanyInput,
  accessToken: string,
): Promise<Company> {
  const resp = await apiCall<
    { name: string; ownerContactId: string },
    { company: WireCompany }
  >(
    SERVICE,
    "CreateCompany",
    { name: input.name, ownerContactId: input.ownerContactId ?? "" },
    { accessToken },
  );
  return companyFromWire(resp.company);
}

export interface UpdateCompanyInput {
  id: string;
  name: string;
  ownerContactId?: string;
}

export async function updateCompany(
  input: UpdateCompanyInput,
  accessToken: string,
): Promise<Company> {
  const resp = await apiCall<
    { id: string; name: string; ownerContactId: string },
    { company: WireCompany }
  >(
    SERVICE,
    "UpdateCompany",
    {
      id: input.id,
      name: input.name,
      ownerContactId: input.ownerContactId ?? "",
    },
    { accessToken },
  );
  return companyFromWire(resp.company);
}

export async function archiveCompany(
  id: string,
  accessToken: string,
): Promise<void> {
  await apiCall<{ id: string }, Record<string, never>>(
    SERVICE,
    "ArchiveCompany",
    { id },
    { accessToken },
  );
}

export async function restoreCompany(
  id: string,
  accessToken: string,
): Promise<Company> {
  const resp = await apiCall<{ id: string }, { company: WireCompany }>(
    SERVICE,
    "RestoreCompany",
    { id },
    { accessToken },
  );
  return companyFromWire(resp.company);
}
