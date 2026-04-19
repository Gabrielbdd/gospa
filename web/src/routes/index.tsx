import { createRoute, redirect } from "@tanstack/react-router";

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
    return { zitadelOrgId: status.zitadelOrgId };
  },
  component: HomePage,
});

function HomePage() {
  const orgId = runtimeConfig.auth?.orgId;
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
            void startLogin();
          }}
          disabled={!orgId}
        >
          Log in
        </Button>
        <Button variant="outline">Documentation</Button>
      </div>
      {!orgId && (
        <p className="text-xs text-muted-foreground">
          Login disabled — workspace auth.orgId has not been resolved yet.
        </p>
      )}
    </main>
  );
}
