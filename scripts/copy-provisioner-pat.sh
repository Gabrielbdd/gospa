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
# Detection strategy: ZITADEL ships a distroless image (no shell, no
# test, no cat), so probing the file from inside the container is not
# possible. Instead we try `compose cp` on every tick — it returns
# non-zero when the source path does not yet exist, and writes the
# file to the host when it does. Exit code does the work.
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
  if sh ./scripts/compose.sh cp "zitadel:$container_path" "$dest" >/dev/null 2>&1; then
    if [ -s "$dest" ]; then
      chmod 600 "$dest"
      printf 'provisioner PAT copied to %s\n' "$dest"
      exit 0
    fi
    # cp returned 0 but left an empty file. Unexpected; drop and retry.
    rm -f "$dest"
  fi
  sleep 1
  i=$((i + 1))
done

printf >&2 'timed out waiting for provisioner PAT inside the zitadel_secrets volume after %s seconds\n' "$attempts"
printf >&2 'is ZITADEL healthy and configured with FirstInstance.Org.Machine + PatPath in infra/zitadel/steps.yaml?\n'
printf >&2 'last ZITADEL logs:\n'
sh ./scripts/compose.sh logs --tail=40 zitadel >&2 || true
exit 1
