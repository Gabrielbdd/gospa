import { useMutation, useQuery } from "@tanstack/react-query";
import { createRoute, redirect } from "@tanstack/react-router";
import { useForm } from "@tanstack/react-form";
import { useEffect, useState } from "react";

import { Button } from "@/components/ui/button";
import {
  getStatus,
  install,
  isTerminal,
  type InstallRequest,
  type InstallState,
} from "@/lib/install-client";
import { cn } from "@/lib/utils";
import { rootRoute } from "@/routes/__root";

export const installRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/install",
  loader: async () => {
    const status = await getStatus();
    if (status.state === "INSTALL_STATE_READY") {
      throw redirect({ to: "/" });
    }
    return { initialState: status.state, initialError: status.installError };
  },
  component: InstallPage,
});

const DEFAULT_TIMEZONE =
  typeof Intl !== "undefined"
    ? Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC"
    : "UTC";

function InstallPage() {
  const { initialState, initialError } = installRoute.useLoaderData();

  const alreadyProvisioning =
    initialState === "INSTALL_STATE_PROVISIONING";
  const [submitted, setSubmitted] = useState(alreadyProvisioning);

  const statusQuery = useQuery({
    queryKey: ["install-status"],
    queryFn: getStatus,
    enabled: submitted,
    refetchInterval: (q) => {
      const data = q.state.data;
      if (!data) return 1000;
      return isTerminal(data.state) ? false : 1000;
    },
    initialData: submitted
      ? { state: initialState, installError: initialError }
      : undefined,
  });

  // Install completion is a page-lifetime boundary: the server just
  // activated its JWT middleware and populated workspace.zitadel_org_id,
  // but /_gofra/config.js was loaded at boot with an empty orgId. A
  // full reload (not SPA navigation) forces the <script src=
  // "/_gofra/config.js"> tag to re-execute, so window.__GOFRA_CONFIG__
  // picks up the post-install values — including auth.orgId, which the
  // login button needs to build the org-scoped ZITADEL authorize URL.
  //
  // Using window.location.assign instead of replace so the wizard
  // stays in history — a deliberate back-button on the fresh home
  // page returns the operator to /install, which the loader then
  // redirects back to / (install_state is still READY). No stale state.
  useEffect(() => {
    if (statusQuery.data?.state === "INSTALL_STATE_READY") {
      window.location.assign("/");
    }
  }, [statusQuery.data?.state]);

  const installMutation = useMutation({
    mutationFn: (vars: { req: InstallRequest; token: string }) =>
      install(vars.req, vars.token),
    onSuccess: () => setSubmitted(true),
  });

  const form = useForm({
    defaultValues: {
      installToken: "",
      workspaceName: "",
      workspaceSlug: "",
      timezone: DEFAULT_TIMEZONE,
      currencyCode: "USD",
      email: "",
      givenName: "",
      familyName: "",
      password: "",
    },
    onSubmit: async ({ value }) => {
      installMutation.mutate({
        req: {
          workspaceName: value.workspaceName,
          workspaceSlug: value.workspaceSlug,
          timezone: value.timezone,
          currencyCode: value.currencyCode,
          initialUser: {
            email: value.email,
            givenName: value.givenName,
            familyName: value.familyName,
            password: value.password,
          },
        },
        token: value.installToken.trim(),
      });
    },
  });

  return (
    <main className="mx-auto max-w-2xl space-y-6 p-8">
      <SecurityBanner />
      <header className="space-y-1">
        <h1 className="text-3xl font-bold tracking-tight">Install Gospa</h1>
        <p className="text-muted-foreground">
          Create your MSP workspace and the first admin user.
        </p>
      </header>

      {submitted ? (
        <InstallStatus state={statusQuery.data?.state ?? initialState} error={statusQuery.data?.installError ?? initialError} />
      ) : (
        <form
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            e.stopPropagation();
            void form.handleSubmit();
          }}
        >
          <form.Field name="installToken">
            {(field) => (
              <Field
                label="Install token"
                hint="Local dev: cat .secrets/install-token. Production: copy from container logs (search for 'install token')."
              >
                <input
                  className={inputClass}
                  value={field.state.value}
                  onChange={(e) => field.handleChange(e.target.value)}
                  required
                  autoComplete="off"
                  spellCheck={false}
                  autoFocus
                />
              </Field>
            )}
          </form.Field>
          <form.Field name="workspaceName">
            {(field) => (
              <Field label="Workspace name">
                <input
                  className={inputClass}
                  value={field.state.value}
                  onChange={(e) => field.handleChange(e.target.value)}
                  required
                />
              </Field>
            )}
          </form.Field>
          <form.Field name="workspaceSlug">
            {(field) => (
              <Field label="Workspace slug" hint="lowercase, no spaces">
                <input
                  className={inputClass}
                  value={field.state.value}
                  onChange={(e) => field.handleChange(e.target.value)}
                  pattern="[a-z0-9-]+"
                  required
                />
              </Field>
            )}
          </form.Field>
          <div className="grid grid-cols-2 gap-3">
            <form.Field name="timezone">
              {(field) => (
                <Field label="Timezone">
                  <input
                    className={inputClass}
                    value={field.state.value}
                    onChange={(e) => field.handleChange(e.target.value)}
                    required
                  />
                </Field>
              )}
            </form.Field>
            <form.Field name="currencyCode">
              {(field) => (
                <Field label="Currency">
                  <input
                    className={inputClass}
                    value={field.state.value}
                    onChange={(e) =>
                      field.handleChange(e.target.value.toUpperCase())
                    }
                    maxLength={3}
                    required
                  />
                </Field>
              )}
            </form.Field>
          </div>

          <h2 className="pt-2 text-lg font-semibold">Initial admin</h2>
          <form.Field name="email">
            {(field) => (
              <Field label="Email">
                <input
                  type="email"
                  className={inputClass}
                  value={field.state.value}
                  onChange={(e) => field.handleChange(e.target.value)}
                  required
                />
              </Field>
            )}
          </form.Field>
          <div className="grid grid-cols-2 gap-3">
            <form.Field name="givenName">
              {(field) => (
                <Field label="First name">
                  <input
                    className={inputClass}
                    value={field.state.value}
                    onChange={(e) => field.handleChange(e.target.value)}
                    required
                  />
                </Field>
              )}
            </form.Field>
            <form.Field name="familyName">
              {(field) => (
                <Field label="Last name">
                  <input
                    className={inputClass}
                    value={field.state.value}
                    onChange={(e) => field.handleChange(e.target.value)}
                    required
                  />
                </Field>
              )}
            </form.Field>
          </div>
          <form.Field name="password">
            {(field) => (
              <Field
                label="Initial password"
                hint="Min 8 characters. Used for first login; ZITADEL applies its own policy on top (digit, case)."
              >
                <input
                  type="password"
                  className={inputClass}
                  value={field.state.value}
                  onChange={(e) => field.handleChange(e.target.value)}
                  minLength={8}
                  required
                  autoComplete="new-password"
                />
              </Field>
            )}
          </form.Field>

          {installMutation.isError && (
            <p className="text-sm text-destructive">
              {(installMutation.error as Error).message}
            </p>
          )}

          <div className="pt-2">
            <Button type="submit" disabled={installMutation.isPending}>
              {installMutation.isPending ? "Starting…" : "Install"}
            </Button>
          </div>
        </form>
      )}
    </main>
  );
}

function SecurityBanner() {
  return (
    <div className="rounded-md border border-warning bg-warning/10 p-3 text-sm text-warning-foreground">
      <strong className="font-semibold">Bootstrap-only flow.</strong>{" "}
      The install endpoint requires an operator-supplied token and runs
      before any user exists — keep it behind a private ingress until
      <code className="mx-1 rounded bg-background/50 px-1">install_state = ready</code>
      anyway. The token field below proves you control this Gospa
      instance.
    </div>
  );
}

function InstallStatus({ state, error }: { state: InstallState; error?: string }) {
  const label: Record<InstallState, string> = {
    INSTALL_STATE_UNSPECIFIED: "Unknown state",
    INSTALL_STATE_NOT_INITIALIZED: "Ready to install",
    INSTALL_STATE_PROVISIONING: "Provisioning ZITADEL org, project, and app…",
    INSTALL_STATE_READY: "Install complete. Redirecting…",
    INSTALL_STATE_FAILED: "Install failed.",
  };
  return (
    <section className="rounded-md border border-border bg-card p-4">
      <p className="font-medium">{label[state]}</p>
      {state === "INSTALL_STATE_FAILED" && error && (
        <pre className="mt-3 whitespace-pre-wrap rounded bg-muted p-3 text-sm text-muted-foreground">
          {error}
        </pre>
      )}
      {state === "INSTALL_STATE_PROVISIONING" && (
        <p className="mt-2 text-sm text-muted-foreground">
          Polling once per second. This usually takes a few seconds.
        </p>
      )}
    </section>
  );
}

const inputClass = cn(
  "flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm",
  "placeholder:text-muted-foreground",
  "focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring",
);

function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <label className="flex flex-col gap-1 text-sm">
      <span className="font-medium">{label}</span>
      {children}
      {hint && <span className="text-xs text-muted-foreground">{hint}</span>}
    </label>
  );
}
