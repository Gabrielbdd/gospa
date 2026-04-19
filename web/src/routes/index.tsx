import { createRoute, redirect } from "@tanstack/react-router";
import { useEffect } from "react";

import { Button } from "@/components/ui/button";
import { startLogin } from "@/lib/auth";
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

  // Authoritative source: GetStatus, which the loader just called.
  // Runtime config (from /_gofra/config.js) is a page-load snapshot
  // and may be stale if the operator somehow landed on / via SPA
  // navigation before the install flow's full reload took effect.
  const effectiveOrgId = zitadelOrgId || configOrgId;

  // Belt-and-suspenders: if config.js snapshot disagrees with the live
  // workspace (install just completed, page was not reloaded), force
  // one reload now so /_gofra/config.js is re-executed and the rest
  // of the app (including callers that read runtimeConfig directly)
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

  return (
    <main className="mx-auto max-w-3xl space-y-6 p-8">
      <header className="space-y-2">
        <h1 className="text-3xl font-bold tracking-tight">
          {runtimeConfig.appName ?? "Gospa"}
        </h1>
        <p className="text-muted-foreground">
          Workspace installed. Sign in to manage your MSP.
        </p>
      </header>
      <div className="flex gap-3">
        <Button
          onClick={() => {
            void startLogin({ orgIdOverride: effectiveOrgId });
          }}
          disabled={!effectiveOrgId}
        >
          Log in
        </Button>
        <Button variant="outline">Documentation</Button>
      </div>
      {!effectiveOrgId && (
        <p className="text-xs text-muted-foreground">
          Login disabled — workspace auth.orgId has not been resolved yet.
        </p>
      )}
    </main>
  );
}
