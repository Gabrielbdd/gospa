# Quick-run Docker Compose example for Gospa

> **Status today: this example does not run standalone.**
>
> The `compose.yaml` in this directory only brings up PostgreSQL and
> the Gospa binary. It does **not** bring up ZITADEL and does **not**
> materialise the provisioner PAT file that Gospa requires at
> startup. The container will exit `1` on first boot with a clear
> error about a missing PAT file:
>
>     ERROR zitadel provisioner PAT unavailable; refusing to start
>
> A self-contained quickrun that bundles ZITADEL + bootstraps the
> PAT inside Compose is future work; it will land alongside the
> Gospa documentation site.

## What to run instead, today

For a runnable local Gospa, clone the repository and use the
in-repo `mise` workflow — which brings up Postgres **and** ZITADEL,
materialises the provisioner PAT, generates the install token, and
runs the app pointing at the right paths:

```bash
git clone https://github.com/Gabrielbdd/gospa.git
cd gospa
mise trust
mise run infra            # Postgres + ZITADEL + PAT + install token
mise run install:token    # prints the token to paste into the wizard
mise run dev              # backend + Vite dev server
```

Then visit <http://localhost:3000>. See the top-level
[`README.md`](../../../../README.md) and [`docs/operations.md`](../../../operations.md)
for the rest of the local-dev surface (reset, logs, scenarios for
restart and rotation, and the `/install` wizard walk-through).

## Why the artifact is kept in the repo

The `compose.yaml` and `compose.env` in this directory are checked
in so that a future quickrun variant — once it is honest — can drop
in here and the documentation links in the rest of the project keep
pointing at the same path. The current files are not deleted to
preserve that path stability and to make the gap obvious to anyone
opening the directory.

## What replaces this

When Gospa ships its own documentation site, real deployment guides
will live there — covering TLS, secrets handling, auth, observability,
backup, and minimal production-safe manifests. For Kubernetes today,
[`../kubernetes/README.md`](../kubernetes/README.md) is the
already-coherent example: it documents the install token Secret,
the patwatch-based PAT rotation contract, and the in-place auth gate
activation that does not need `kubectl rollout restart`.
