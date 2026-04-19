#!/bin/sh
set -eu

# Waits for ZITADEL's FirstInstance bootstrap to write the provisioner
# PAT inside the zitadel_secrets named volume, then `compose cp`s it
# to the well-known path the app reads at startup
# (./.secrets/zitadel-provisioner.pat).
#
# A named volume is used instead of a bind-mount because the ZITADEL
# container runs as a non-root user whose UID rarely matches the host
# user; bind-mounts therefore produce permission-denied errors when
# ZITADEL tries to write the PAT. Named volumes live inside the Docker
# VM and do not inherit host-side UIDs.
#
# Idempotent: exits early when the destination is already a non-empty
# regular file.

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
project_dir=$(CDPATH= cd -- "$script_dir/.." && pwd)

cd "$project_dir"
. ./scripts/load-env.sh

dest="${GOSPA_ZITADEL_PROVISIONER_PAT_FILE}"
container_path="/zitadel-secrets/zitadel-provisioner.pat"
attempts="${1:-120}"

if [ -s "$dest" ]; then
  printf 'provisioner PAT already present at %s\n' "$dest"
  exit 0
fi

mkdir -p "$(dirname -- "$dest")"

i=1
while [ "$i" -le "$attempts" ]; do
  # `compose exec -T zitadel test -s <path>` checks the file exists and
  # is non-empty inside the volume without needing a shell. ZITADEL's
  # image is distroless-ish, so `test` is the most portable probe.
  if sh ./scripts/compose.sh exec -T zitadel test -s "$container_path" >/dev/null 2>&1; then
    # `compose cp <service>:<path> <local>` works for Docker Compose v2,
    # Podman Compose (recent), and docker-compose 1.x.
    if sh ./scripts/compose.sh cp "zitadel:$container_path" "$dest"; then
      chmod 600 "$dest"
      printf 'provisioner PAT copied to %s\n' "$dest"
      exit 0
    fi
  fi
  sleep 1
  i=$((i + 1))
done

printf >&2 'timed out waiting for provisioner PAT at %s inside the zitadel_secrets volume after %s seconds\n' "$container_path" "$attempts"
printf >&2 'is ZITADEL healthy and configured with FirstInstance.Org.Machine + PatPath in infra/zitadel/steps.yaml?\n'
printf >&2 'try: mise run infra:reset && mise run infra\n'
exit 1
