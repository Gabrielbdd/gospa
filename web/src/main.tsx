import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { createRouter, RouterProvider } from "@tanstack/react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

import { Toaster } from "@/components/ui/sonner";
import { GospaAuthProvider } from "@/lib/auth-provider";
import { rootRoute } from "@/routes/__root";
import { authCallbackRoute } from "@/routes/auth-callback";
import { companyDetailRoute } from "@/routes/companies.$companyId";
import { companiesRoute } from "@/routes/companies";
import { indexRoute } from "@/routes/index";
import { installRoute } from "@/routes/install";
import { settingsIndexRoute } from "@/routes/settings";
import { settingsTeamRoute } from "@/routes/settings.team";
import { ticketsRoute } from "@/routes/tickets";
import "@/styles/index.css";

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1 } },
});

const routeTree = rootRoute.addChildren([
  indexRoute,
  installRoute,
  authCallbackRoute,
  ticketsRoute,
  companiesRoute,
  companyDetailRoute,
  settingsIndexRoute,
  settingsTeamRoute,
]);

const router = createRouter({
  routeTree,
  context: { queryClient },
  defaultPreload: "intent",
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

const rootElement = document.getElementById("root");
if (!rootElement) {
  throw new Error("missing #root element in index.html");
}

createRoot(rootElement).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <GospaAuthProvider>
        <RouterProvider router={router} />
        <Toaster position="bottom-center" richColors closeButton />
      </GospaAuthProvider>
    </QueryClientProvider>
  </StrictMode>,
);
