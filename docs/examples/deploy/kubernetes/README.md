# Kubernetes deploy example for Gospa

A minimal set of manifests that shows the operational contract Gospa
expects in a Kubernetes cluster. **Not production-ready on its own** —
use it as a starting point for a real deployment (ingress, TLS,
database, monitoring).

## The operational contract

Gospa's hard startup invariant is that a ZITADEL provisioner Personal
Access Token must be present on disk at a path the operator chooses.
In Kubernetes this is a `Secret` mounted read-only as a file; the
`Deployment` points the `GOSPA_ZITADEL_PROVISIONER_PAT_FILE`
environment variable at that path.

The Gospa container does **not**:

- Contact ZITADEL at startup to resolve the PAT.
- Wait for a bootstrap Job.
- Watch the filesystem for the secret to appear.

If the file is missing or empty, the Pod exits `1` with an actionable
log line. Liveness and readiness probes therefore protect nothing
— the container fails fast on first start.

Provisioning the PAT itself is **out of scope for this manifest set**.
Operators typically do one of:

- Create a ZITADEL service account with IAM_OWNER, generate a PAT in
  the ZITADEL UI, and put it in the `Secret` directly
  (`kubectl create secret generic gospa-zitadel-provisioner
  --from-literal=pat=<PAT>`).
- Automate the same with Terraform / Pulumi / Crossplane against the
  ZITADEL provider.
- Run a one-shot Job that calls ZITADEL's AdminService to mint the
  PAT and writes it into the `Secret` before the Deployment rolls.

## Files

| File | Purpose |
| --- | --- |
| `secret.yaml`     | Holds the provisioner PAT under key `pat`. Created by the operator. |
| `deployment.yaml` | Mounts the Secret read-only, wires the env var, sets probes. |
| `service.yaml`    | `ClusterIP:3000` service for the Gospa HTTP surface. |

## Apply

```bash
kubectl apply --dry-run=client -f docs/examples/deploy/kubernetes/
# Review, then apply for real:
kubectl apply -f docs/examples/deploy/kubernetes/
```

## MVP debts

The install wizard has no install key in this MVP — anyone who can
reach the `/install` route can trigger provisioning. Gate the Service
behind a private Ingress, IP allowlist, or VPN until the workspace is
installed. After install, the Install RPC returns `FailedPrecondition`
for subsequent calls, but a determined attacker with network reach
could still abuse the polling status endpoint during provisioning.

The single PAT is reused for bootstrap and runtime provisioning
operations (org creation for companies). Rotation is a manual
operation — generate a new PAT, update the `Secret`, roll the
Deployment. A future slice will split bootstrap and runtime
credentials; until then, treat this PAT as a highly sensitive secret.

The Deployment is single-replica. Horizontal scaling is safe for
normal traffic but the install flow assumes a single process (it uses
a sync-group to serialise the orchestrator); scale up only after
install completes, or leave the replica count at 1 until the flow is
refactored to use Restate in a later slice.
