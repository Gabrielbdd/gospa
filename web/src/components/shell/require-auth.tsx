import * as React from "react";
import { useNavigate } from "@tanstack/react-router";
import { useAuth } from "react-oidc-context";

import { Spinner } from "@/components/tickets/async-states";

// RequireAuth gates a route behind an authenticated session. Unauth
// users are redirected to "/", where the operator sees the login
// button. While react-oidc-context is still bootstrapping (silent
// renewal on first paint) we render a tiny spinner to avoid a
// login-button flash before the session resolves.
export function RequireAuth({ children }: { children: React.ReactNode }) {
  const auth = useAuth();
  const navigate = useNavigate();

  React.useEffect(() => {
    if (!auth.isLoading && !auth.isAuthenticated) {
      void navigate({ to: "/", replace: true });
    }
  }, [auth.isLoading, auth.isAuthenticated, navigate]);

  if (auth.isLoading) {
    return (
      <div className="flex h-screen items-center justify-center bg-app">
        <Spinner size={16} />
      </div>
    );
  }
  if (!auth.isAuthenticated) {
    return (
      <div className="flex h-screen items-center justify-center bg-app text-[13px] text-fg-3">
        Redirecionando para o login…
      </div>
    );
  }
  return <>{children}</>;
}
