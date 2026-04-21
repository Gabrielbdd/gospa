// /auth/callback is the OIDC redirect target. react-oidc-context's
// AuthProvider auto-detects the ?code=... query string when it mounts
// and runs the code exchange against the issuer; this route only
// renders a placeholder while that happens. The configured
// onSigninCallback (auth-provider.tsx) replaces the URL with "/" once
// the exchange succeeds, at which point the user is redirected by the
// state-aware index route.
//
// The route is intentionally public: react-oidc-context needs to run
// before any protected RPC happens, and the install-token-protected
// /install RPC is unrelated to this flow. There is no backend RPC
// invoked from this page.

import { createRoute } from "@tanstack/react-router";
import { useAuth } from "react-oidc-context";

import { rootRoute } from "@/routes/__root";

export const authCallbackRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/auth/callback",
  component: AuthCallbackPage,
});

function AuthCallbackPage() {
  const auth = useAuth();

  if (auth.error) {
    return (
      <main className="mx-auto max-w-2xl space-y-3 p-8">
        <h1 className="text-2xl font-bold tracking-tight">Falha no login</h1>
        <p className="text-sm text-destructive">{auth.error.message}</p>
        <p className="text-sm text-muted-foreground">
          Volte para a <a className="underline" href="/">página inicial</a> e
          tente de novo. Se continuar, contate o administrador do workspace.
        </p>
      </main>
    );
  }

  return (
    <main className="mx-auto max-w-2xl p-8 text-muted-foreground">
      Finalizando login…
    </main>
  );
}
