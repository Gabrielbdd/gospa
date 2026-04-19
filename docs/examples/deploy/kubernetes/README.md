# Kubernetes deploy example for Gospa

A minimal set of manifests that shows the operational contract Gospa
expects in a Kubernetes cluster. **Not production-ready on its own** —
use it as a starting point for a real deployment (Ingress, TLS, cert
issuer, managed Postgres, monitoring, backups).

## Prerequisites

The manifests **do not deploy ZITADEL or Postgres**. You provide both:

| Dependency | What you need |
|---|---|
| ZITADEL | A reachable ZITADEL instance (Cloud or self-hosted). A **machine user** with the **IAM_OWNER** grant, and a **Personal Access Token** for that user. |
| Postgres | A Postgres database + user Gospa can write to. DSN lands in a `Secret` (not shown). |
| Install token | A high-entropy string (e.g. `openssl rand -hex 16`) that the operator pastes into the `/install` wizard. Lives in a `Secret`; the example `secret.yaml` ships a placeholder. |
| Ingress / LoadBalancer | A public HTTPS URL that routes to the `gospa` Service on port 3000. |

The PAT is the credential that lets Gospa call ZITADEL's Admin API to
create the MSP org, project, and OIDC application during `/install`.
Generate it in the ZITADEL console (Users → Service Users →
`gospa-provisioner` → Personal Access Tokens → Add) or via Terraform /
Pulumi / Crossplane against the ZITADEL provider.

The install token is a separate bootstrap secret. The `/install`
endpoint is intentionally unauthenticated (no user exists yet), so the
token is the only thing standing between a reachable Gospa URL and an
attacker hijacking the workspace into their own ZITADEL during the
install window. Keep it in a Secret; rotate it after install if you
care.

## Install flow in Kubernetes

1. **Create the secrets** (`secret.yaml`) with your real values:
   - `gospa-zitadel-provisioner`: paste the ZITADEL PAT under `pat`.
   - `gospa-install-token`: paste a high-entropy hex string under
     `token` (e.g. `openssl rand -hex 16`). Save a copy somewhere you
     can reach — you will paste it into the wizard.
2. **Create the Postgres DSN Secret** (not included — shape:
   `kubectl create secret generic gospa-database --from-literal=dsn="postgres://..."`).
3. **Edit `deployment.yaml`** env values to match your cluster:
   - `GOFRA_ZITADEL__ADMIN_API_URL`: the ZITADEL base URL.
   - `GOFRA_PUBLIC__API_BASE_URL`: the public Gospa URL (matters for
     the OIDC redirect URIs the install wizard writes to ZITADEL).
   - `GOFRA_PUBLIC__AUTH__ISSUER`: usually the same as the admin URL;
     override only when the browser reaches ZITADEL through a
     different host than the cluster does.
4. **`kubectl apply -f docs/examples/deploy/kubernetes/`**. The Pod
   starts; cmd/app verifies the PAT and install token are readable
   and the DB is reachable, runs auto-migrations, and serves. Auth is
   **disabled at this point** because workspace.install_state is
   `not_initialized` — the `/install` wizard is deliberately public.
5. **Protect the URL with an allowlist** (Ingress annotation, VPN,
   `kubectl port-forward`, etc.) until install completes. The install
   token blocks the actual provisioning POST, but defence in depth is
   cheap.
6. **Open the Gospa URL** → redirects to `/install`. Paste the install
   token into the first field, then fill the rest of the wizard
   (workspace name/slug/timezone/currency + first admin's name and
   email). Submit. The orchestrator calls ZITADEL through the
   provisioner PAT and creates the MSP org, project, and OIDC SPA
   application. Poll to `ready`. The orchestrator's `OnReady` hook
   activates the JWT middleware in-place — no Pod restart required.
7. **Remove the allowlist** once `install_state = ready`; auth is
   active and the Service is now safe to expose.

## Files

| File | Purpose |
| --- | --- |
| `secret.yaml`     | Two Secrets: `gospa-zitadel-provisioner.pat` (ZITADEL PAT) and `gospa-install-token.token` (bootstrap secret). Operator-filled. |
| `deployment.yaml` | Mounts the PAT Secret, injects the install token via env, sets the rest of the env, probes, single replica. |
| `service.yaml`    | `ClusterIP:3000` for the Gospa HTTP surface. |

Not included (you bring your own): Ingress / Gateway, certificate
issuer, Postgres StatefulSet or managed DSN Secret, HorizontalPodAutoscaler.

## Apply

```bash
kubectl apply --dry-run=client -f docs/examples/deploy/kubernetes/
# Review, then apply for real:
kubectl apply -f docs/examples/deploy/kubernetes/
```

## MVP debts relevant to K8s

- **Single PAT.** Reused for bootstrap and company-creation runtime
  calls. Rotation is manual: generate new PAT, update the Secret,
  `kubectl rollout restart deployment/gospa`.
- **One replica during install.** The orchestrator uses an in-process
  single-flight guard, so scaling up during the install window is
  unsafe. Scale up after `install_state = ready`. Restate will lift
  this restriction in a later slice.
- **Install token does not auto-rotate.** The same value stays valid
  until the operator updates the Secret. Once `install_state = ready`
  the endpoint refuses provisioning regardless, so the practical risk
  window is the install duration.
