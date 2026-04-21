import { createRoute } from "@tanstack/react-router";

import { RequireAuth } from "@/components/shell/require-auth";
import { Shell, useActiveNavKey } from "@/components/shell/shell";
import { SettingsRail } from "@/components/shell/settings-rail";
import { SettingsTeam } from "@/components/team/settings-team";
import { rootRoute } from "@/routes/__root";

export const settingsTeamRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/settings/team",
  component: SettingsTeamRoute,
});

function SettingsTeamRoute() {
  const activeKey = useActiveNavKey();
  return (
    <RequireAuth>
      <Shell activeKey={activeKey} breadcrumbs={["Configurações", "Time"]}>
        <div className="flex h-full">
          <SettingsRail active="team" />
          <SettingsTeam />
        </div>
      </Shell>
    </RequireAuth>
  );
}
