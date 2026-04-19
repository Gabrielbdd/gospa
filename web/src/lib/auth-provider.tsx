// Wires react-oidc-context to the runtime config served by
// /_gofra/config.js. The publicconfig mutator is the source of truth
// for issuer, clientId, and the full scope set (defaults + org scope +
// audience scope, derived server-side from the persisted workspace
// row). The browser does not learn how to compose ZITADEL scope URNs.
//
// sessionStorage backs the user store so a tab close clears the
// session — closer to "log in per browser session" than localStorage,
// and keeps a stale token out of long-lived browser profiles.
//
// onSigninCallback strips the ?code=... query string from the URL and
// sends the user back to "/" so the address bar matches what the SPA
// renders. handleSigninCallback (the actual code exchange) is handled
// internally by react-oidc-context when AuthProvider mounts on the
// callback URL.

import { ReactNode } from "react";
import { AuthProvider, AuthProviderProps } from "react-oidc-context";
import { WebStorageStateStore } from "oidc-client-ts";

import { runtimeConfig } from "@/lib/runtime-config";

function buildOidcConfig(): AuthProviderProps {
  const cfg = runtimeConfig.auth;
  return {
    authority: cfg.issuer,
    client_id: cfg.clientId,
    redirect_uri: window.location.origin + cfg.redirectPath,
    post_logout_redirect_uri:
      window.location.origin + (cfg.postLogoutRedirectPath || "/"),
    response_type: "code",
    // The server already merged defaults + org scope + audience scope
    // for the installed workspace; we just space-join.
    scope: (cfg.scopes ?? []).join(" "),
    userStore: new WebStorageStateStore({ store: window.sessionStorage }),
    stateStore: new WebStorageStateStore({ store: window.sessionStorage }),
    automaticSilentRenew: true,
    onSigninCallback: () => {
      window.history.replaceState({}, document.title, "/");
    },
  };
}

export function GospaAuthProvider({ children }: { children: ReactNode }) {
  return <AuthProvider {...buildOidcConfig()}>{children}</AuthProvider>;
}
