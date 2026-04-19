import { createRoute, redirect } from "@tanstack/react-router";
import { useEffect } from "react";
import { useAuth } from "react-oidc-context";

import { Button } from "@/components/ui/button";
import { getStatus } from "@/lib/install-client";
import { runtimeConfig } from "@/lib/runtime-config";
import { rootRoute } from "@/routes/__root";

export const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  loader: async () => {
    const status = await getStatus();
    if (status.state !== "INSTALL_STATE_READY") {
      throw redirect({ to: "/install" });
    }
    return { zitadelOrgId: status.zitadelOrgId ?? "" };
  },
  component: HomePage,
});

function HomePage() {
  const { zitadelOrgId } = indexRoute.useLoaderData();
  const configOrgId = runtimeConfig.auth?.orgId ?? "";
  const auth = useAuth();

  // Authoritative source: GetStatus, which the loader just called.
  // Runtime config (from /_gofra/config.js) is a page-load snapshot
  // and may be stale if the operator somehow landed on / via SPA
  // navigation before the install flow's full reload took effect.
  const effectiveOrgId = zitadelOrgId || configOrgId;

  // Belt-and-suspenders: if config.js snapshot disagrees with the live
  // workspace (install just completed, page was not reloaded), force
  // one reload now so /_gofra/config.js is re-executed and the rest
  // of the app (including react-oidc-context, which read the issuer +
  // client_id + scopes from runtimeConfig at provider construction)
  // sees the post-install state.
  const configIsStale = !!zitadelOrgId && configOrgId !== zitadelOrgId;
  useEffect(() => {
    if (configIsStale) {
      window.location.reload();
    }
  }, [configIsStale]);

  if (configIsStale) {
    return (
      <main className="mx-auto max-w-3xl p-8 text-muted-foreground">
        Refreshing runtime config…
      </main>
    );
  }

  // While the auth library is still bootstrapping (e.g. silent renewal
  // on first paint) we render a tiny placeholder so the login button
  // doesn't flicker between "Log in" and "Log out".
  if (auth.isLoading) {
    return (
      <main className="mx-auto max-w-3xl p-8 text-muted-foreground">
        Loading session…
      </main>
    );
  }

  return (
    <main className="mx-auto max-w-3xl space-y-6 p-8">
      <header className="space-y-2">
        <h1 className="text-3xl font-bold tracking-tight">
          {runtimeConfig.appName ?? "Gospa"}
        </h1>
        <p className="text-muted-foreground">
          {auth.isAuthenticated
            ? `Signed in as ${auth.user?.profile.email ?? auth.user?.profile.sub ?? "unknown"}.`
            : "Workspace installed. Sign in to manage your MSP."}
        </p>
      </header>
      <div className="flex gap-3">
        {auth.isAuthenticated ? (
          <Button
            onClick={() => {
              // Clear local state first so a slow ZITADEL logout does
              // not leave a half-authenticated session on screen.
              void auth.removeUser().then(() =>
                auth.signoutRedirect({
                  post_logout_redirect_uri:
                    window.location.origin +
                    (runtimeConfig.auth?.postLogoutRedirectPath || "/"),
                }),
              );
            }}
          >
            Log out
          </Button>
        ) : (
          <Button
            onClick={() => {
              void auth.signinRedirect();
            }}
            disabled={!effectiveOrgId}
          >
            Log in
          </Button>
        )}
        <Button variant="outline">Documentation</Button>
      </div>
      {!effectiveOrgId && !auth.isAuthenticated && (
        <p className="text-xs text-muted-foreground">
          Login disabled — workspace auth.orgId has not been resolved yet.
        </p>
      )}
      {auth.error && (
        <p className="text-xs text-destructive">{auth.error.message}</p>
      )}
    </main>
  );
}
