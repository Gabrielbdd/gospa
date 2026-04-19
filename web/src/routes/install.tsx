import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createRoute, redirect, useNavigate } from "@tanstack/react-router";
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
  beforeLoad: async () => {
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
  const navigate = useNavigate();
  const queryClient = useQueryClient();

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

  useEffect(() => {
    if (statusQuery.data?.state === "INSTALL_STATE_READY") {
      void queryClient.invalidateQueries({ queryKey: ["install-status"] });
      void navigate({ to: "/" });
    }
  }, [statusQuery.data?.state, navigate, queryClient]);

  const installMutation = useMutation({
    mutationFn: (req: InstallRequest) => install(req),
    onSuccess: () => setSubmitted(true),
  });

  const form = useForm({
    defaultValues: {
      workspaceName: "",
      workspaceSlug: "",
      timezone: DEFAULT_TIMEZONE,
      currencyCode: "USD",
      email: "",
      givenName: "",
      familyName: "",
    },
    onSubmit: async ({ value }) => {
      installMutation.mutate({
        workspaceName: value.workspaceName,
        workspaceSlug: value.workspaceSlug,
        timezone: value.timezone,
        currencyCode: value.currencyCode,
        initialUser: {
          email: value.email,
          givenName: value.givenName,
          familyName: value.familyName,
        },
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
          <form.Field name="workspaceName">
            {(field) => (
              <Field label="Workspace name">
                <input
                  className={inputClass}
                  value={field.state.value}
                  onChange={(e) => field.handleChange(e.target.value)}
                  required
                  autoFocus
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
      <strong className="font-semibold">This flow is not authenticated.</strong>{" "}
      Confirm the Gospa URL is not publicly reachable before completing
      install — this MVP has no install key. Put the service behind a
      private ingress or VPN during setup.
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
