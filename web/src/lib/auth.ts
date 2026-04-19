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
 * the MSP organisation. Pre-install (auth.orgId is empty) the function
 * throws — the `/` route should already have redirected the user to
 * `/install` before they could reach the login button.
 */
export async function buildLoginURL(): Promise<{ url: string; state: string; codeVerifier: string }> {
  const cfg = loadRuntimeConfig();
  if (!cfg.auth.orgId) {
    throw new Error("cannot build login URL: workspace not installed yet");
  }

  const redirectURI = window.location.origin + cfg.auth.redirectPath;
  const state = base64url(crypto.getRandomValues(new Uint8Array(16)));
  const codeVerifier = randomVerifier();
  const codeChallenge = base64url(await sha256(codeVerifier));

  const scopes = [
    ...cfg.auth.scopes,
    "offline_access",
    `urn:zitadel:iam:org:id:${cfg.auth.orgId}`,
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
 * Kicks off the OIDC login redirect. Stashes the PKCE verifier + state in
 * sessionStorage so the eventual /auth/callback route can complete the
 * code exchange. Callback handling is deferred to the next slice.
 */
export async function startLogin(): Promise<void> {
  const { url, state, codeVerifier } = await buildLoginURL();
  sessionStorage.setItem("oidc:state", state);
  sessionStorage.setItem("oidc:code_verifier", codeVerifier);
  window.location.assign(url);
}
