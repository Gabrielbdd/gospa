#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
project_dir=$(CDPATH= cd -- "$script_dir/.." && pwd)
compose_file="$project_dir/compose.yaml"

if docker compose version >/dev/null 2>&1; then
  exec docker compose --file "$compose_file" "$@"
fi

if podman compose version >/dev/null 2>&1; then
  export PODMAN_COMPOSE_WARNING_LOGS=false
  exec podman compose --file "$compose_file" "$@"
fi

if docker-compose version >/dev/null 2>&1; then
  exec docker-compose --file "$compose_file" "$@"
fi

if podman-compose version >/dev/null 2>&1; then
  exec podman-compose --file "$compose_file" "$@"
fi

cat >&2 <<'EOF'
no Compose provider found.

Install one of:
  - Docker with the Compose plugin (`docker compose`)
  - Podman with compose support (`podman compose`)
  - docker-compose
  - podman-compose
EOF
exit 1
