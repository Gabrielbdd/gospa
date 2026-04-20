import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { createRouter, RouterProvider } from "@tanstack/react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

import { GospaAuthProvider } from "@/lib/auth-provider";
import { rootRoute } from "@/routes/__root";
import { authCallbackRoute } from "@/routes/auth-callback";
import { indexRoute } from "@/routes/index";
import { installRoute } from "@/routes/install";
import "@/styles/index.css";

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1 } },
});

const routeTree = rootRoute.addChildren([
  indexRoute,
  installRoute,
  authCallbackRoute,
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
      </GospaAuthProvider>
    </QueryClientProvider>
  </StrictMode>,
);
