// Minimal OIDC login URL builder. The MVP only needs to redirect the user
// to ZITADEL's authorize endpoint with the right client + scopes + org
// context; callback handling, token storage, and refresh loops are left
// for a follow-up slice that will adopt react-oidc-context (per decision
// #124 in the Gofra docs).

import { loadRuntimeConfig } from "@/lib/runtime-config";

function base64url(bytes: Uint8Array): string {
  let s = "";
  for (let i = 0; i < bytes.byteLength; i++) s += String.fromCharCode(bytes[i]!);
  return btoa(s).replaceAll("+", "-").replaceAll("/", "_").replaceAll("=", "");
}

async function sha256(input: string): Promise<Uint8Array> {
  const buf = new TextEncoder().encode(input);
  const digest = await crypto.subtle.digest("SHA-256", buf);
  return new Uint8Array(digest);
}

function randomVerifier(): string {
  const arr = new Uint8Array(32);
  crypto.getRandomValues(arr);
  return base64url(arr);
}

/**
 * Builds the ZITADEL authorize URL and appends the
 * urn:zitadel:iam:org:id:<orgId> scope so the login consent is bound to
 * the MSP organisation.
 *
 * orgId source: an explicit override wins over
 * window.__GOFRA_CONFIG__.auth.orgId. Callers that have a fresher
 * orgId (e.g., from a loader that just called GetStatus) should pass
 * it so login works even when /_gofra/config.js is a pre-install
 * snapshot. Pre-install both are empty and the function throws.
 */
export async function buildLoginURL(opts?: {
  orgIdOverride?: string;
}): Promise<{ url: string; state: string; codeVerifier: string }> {
  const cfg = loadRuntimeConfig();
  const orgId = opts?.orgIdOverride || cfg.auth.orgId;
  if (!orgId) {
    throw new Error("cannot build login URL: workspace not installed yet");
  }

  const redirectURI = window.location.origin + cfg.auth.redirectPath;
  const state = base64url(crypto.getRandomValues(new Uint8Array(16)));
  const codeVerifier = randomVerifier();
  const codeChallenge = base64url(await sha256(codeVerifier));

  const scopes = [
    ...cfg.auth.scopes,
    "offline_access",
    `urn:zitadel:iam:org:id:${orgId}`,
  ];

  const params = new URLSearchParams({
    client_id: cfg.auth.clientId,
    redirect_uri: redirectURI,
    response_type: "code",
    scope: scopes.join(" "),
    state,
    code_challenge: codeChallenge,
    code_challenge_method: "S256",
  });

  return {
    url: `${cfg.auth.issuer}/oauth/v2/authorize?${params.toString()}`,
    state,
    codeVerifier,
  };
}

/**
 * Kicks off the OIDC login redirect. Stashes the PKCE verifier + state
 * in sessionStorage so the eventual /auth/callback route can complete
 * the code exchange. Callback handling is deferred to the next slice.
 *
 * Pass orgIdOverride when a caller has a fresher orgId than
 * window.__GOFRA_CONFIG__ (e.g., a loader reading GetStatus on every
 * navigation). This keeps login working even if the config script was
 * loaded before install completed and has not yet been refreshed by
 * a page reload.
 */
export async function startLogin(opts?: { orgIdOverride?: string }): Promise<void> {
  const { url, state, codeVerifier } = await buildLoginURL(opts);
  sessionStorage.setItem("oidc:state", state);
  sessionStorage.setItem("oidc:code_verifier", codeVerifier);
  window.location.assign(url);
}
