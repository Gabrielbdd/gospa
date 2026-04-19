// Hand-wired Connect RPC client for the InstallService. Gospa talks the
// Connect+JSON wire format directly with fetch via the shared apiCall
// helper; a proto-es pipeline would generate these types automatically,
// but for the MVP install flow the surface is small enough that the
// manual types pay for themselves.

import { apiCall } from "@/lib/api-client";

export type InstallState =
  | "INSTALL_STATE_UNSPECIFIED"
  | "INSTALL_STATE_NOT_INITIALIZED"
  | "INSTALL_STATE_PROVISIONING"
  | "INSTALL_STATE_READY"
  | "INSTALL_STATE_FAILED";

export interface InitialUser {
  email: string;
  givenName: string;
  familyName: string;
  password: string;
}

export interface InstallRequest {
  workspaceName: string;
  workspaceSlug: string;
  timezone: string;
  currencyCode: string;
  initialUser: InitialUser;
}

export interface GetStatusResponse {
  state: InstallState;
  installError?: string;
  zitadelOrgId?: string;
}

export interface InstallResponse {
  state: InstallState;
}

const INSTALL_SERVICE = "/gospa.install.v1.InstallService";

export async function getStatus(): Promise<GetStatusResponse> {
  return apiCall<Record<string, never>, GetStatusResponse>(INSTALL_SERVICE, "GetStatus", {});
}

export async function install(
  req: InstallRequest,
  installToken: string,
): Promise<InstallResponse> {
  return apiCall<InstallRequest, InstallResponse>(INSTALL_SERVICE, "Install", req, {
    installToken,
  });
}

export function isTerminal(state: InstallState): boolean {
  return state === "INSTALL_STATE_READY" || state === "INSTALL_STATE_FAILED";
}
