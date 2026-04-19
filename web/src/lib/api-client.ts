// Single fetch wrapper for Connect+JSON RPCs. Centralises the wire
// shape (POST, JSON in/out, error text on non-2xx) and the bearer
// vs install-token header logic so each service-specific client
// (install-client, companies-client, …) only owns its types.
//
// The bearer comes from react-oidc-context's auth.user.access_token
// and is supplied per-call via opts; this module deliberately stays
// hook-free so it can be invoked from non-React contexts (e.g.
// TanStack Router loaders) without dragging the AuthProvider into
// places it does not belong.

export interface ApiCallOptions {
  /** Bearer token attached as Authorization: Bearer <token>. */
  accessToken?: string;
  /** Install token attached as X-Install-Token (used only by /install). */
  installToken?: string;
}

export async function apiCall<Req, Resp>(
  service: string,
  method: string,
  body: Req,
  opts: ApiCallOptions = {},
): Promise<Resp> {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  if (opts.accessToken) {
    headers["Authorization"] = `Bearer ${opts.accessToken}`;
  }
  if (opts.installToken) {
    headers["X-Install-Token"] = opts.installToken;
  }
  const res = await fetch(`${service}/${method}`, {
    method: "POST",
    headers,
    body: JSON.stringify(body ?? {}),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`${method} ${res.status}: ${text || res.statusText}`);
  }
  return (await res.json()) as Resp;
}
