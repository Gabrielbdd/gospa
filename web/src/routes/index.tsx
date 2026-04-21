import { createRoute, redirect, useNavigate } from "@tanstack/react-router";
import { useEffect } from "react";
import { useAuth } from "react-oidc-context";

import { Button } from "@/components/ui/button";
import { getStatus } from "@/lib/install-client";
import { getLastLoginEmail, setLastLoginEmail } from "@/lib/login-hint";
import { runtimeConfig } from "@/lib/runtime-config";
import { rootRoute } from "@/routes/__root";

// "/" is the pre-login landing page. Once the operator has a session
// the Tickets list becomes the home screen — the effect below pushes
// them there as soon as auth resolves.
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
  const navigate = useNavigate();

  useEffect(() => {
    if (auth.isAuthenticated) {
      const email = auth.user?.profile.email;
      if (typeof email === "string") {
        setLastLoginEmail(email);
      }
      void navigate({
        to: "/tickets",
        search: { view: "all", company: "", q: "" },
        replace: true,
      });
    }
  }, [auth.isAuthenticated, auth.user?.profile.email, navigate]);

  const effectiveOrgId = zitadelOrgId || configOrgId;
  const configIsStale = !!zitadelOrgId && configOrgId !== zitadelOrgId;

  useEffect(() => {
    if (configIsStale) {
      window.location.reload();
    }
  }, [configIsStale]);

  if (configIsStale) {
    return (
      <main className="flex h-screen items-center justify-center bg-app text-[13px] text-fg-3">
        Atualizando configuração…
      </main>
    );
  }

  if (auth.isLoading) {
    return (
      <main className="flex h-screen items-center justify-center bg-app text-[13px] text-fg-3">
        Carregando sessão…
      </main>
    );
  }

  return (
    <main className="flex h-screen items-center justify-center bg-app">
      <div className="w-[360px] rounded-xl border border-border-default bg-subtle p-6 shadow-md">
        <h1 className="mb-1.5 text-xl font-semibold tracking-tight text-fg-1">
          {runtimeConfig.appName ?? "Gospa"}
        </h1>
        <p className="mb-5 text-[13px] text-fg-3">
          Entre para gerenciar seu workspace.
        </p>
        <Button
          className="w-full"
          onClick={() => {
            const hint = getLastLoginEmail();
            void auth.signinRedirect(hint ? { login_hint: hint } : undefined);
          }}
          disabled={!effectiveOrgId}
        >
          Entrar
        </Button>
        {!effectiveOrgId && (
          <p className="mt-3 text-[11px] text-fg-4">
            Login indisponível — configuração do workspace ainda não
            resolvida.
          </p>
        )}
        {auth.error && (
          <p className="mt-3 text-[11px] text-danger">{auth.error.message}</p>
        )}
      </div>
    </main>
  );
}
