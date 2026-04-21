import { createRoute, redirect } from "@tanstack/react-router";

import { rootRoute } from "@/routes/__root";

export const settingsIndexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/settings",
  beforeLoad: () => {
    throw redirect({ to: "/settings/team" });
  },
});
