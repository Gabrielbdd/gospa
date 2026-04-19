// Hand-wired Connect RPC client for the InstallService. Gospa talks the
// Connect+JSON wire format directly with fetch; a proto-es pipeline would
// generate these types automatically, but for the MVP install flow the
// surface is small enough that the manual types pay for themselves.

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

// INSTALL_TOKEN_HEADER must mirror install.InstallTokenHeader on the
// Go side. Renaming one without the other silently breaks /install.
export const INSTALL_TOKEN_HEADER = "X-Install-Token";

async function callRPC<Req, Resp>(
  method: string,
  body: Req,
  extraHeaders?: Record<string, string>,
): Promise<Resp> {
  const res = await fetch(`${INSTALL_SERVICE}/${method}`, {
    method: "POST",
    headers: { "Content-Type": "application/json", ...(extraHeaders ?? {}) },
    body: JSON.stringify(body ?? {}),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`${method} ${res.status}: ${text || res.statusText}`);
  }
  return (await res.json()) as Resp;
}

export async function getStatus(): Promise<GetStatusResponse> {
  return callRPC<Record<string, never>, GetStatusResponse>("GetStatus", {});
}

export async function install(
  req: InstallRequest,
  installToken: string,
): Promise<InstallResponse> {
  return callRPC<InstallRequest, InstallResponse>("Install", req, {
    [INSTALL_TOKEN_HEADER]: installToken,
  });
}

export function isTerminal(state: InstallState): boolean {
  return state === "INSTALL_STATE_READY" || state === "INSTALL_STATE_FAILED";
}
