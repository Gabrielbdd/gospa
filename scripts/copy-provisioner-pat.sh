#!/bin/sh
set -eu

# Waits for ZITADEL's FirstInstance bootstrap to write the provisioner PAT
# into the compose bind-mount, then copies it to the well-known path the
# app reads at startup (./.secrets/zitadel-provisioner.pat). Idempotent:
# exits early when the destination is already a non-empty regular file.

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
project_dir=$(CDPATH= cd -- "$script_dir/.." && pwd)

cd "$project_dir"
. ./scripts/load-env.sh

raw_dir="${project_dir}/.secrets/zitadel-provisioner-raw"
raw_file="${raw_dir}/zitadel-provisioner.pat"
dest="${GOSPA_ZITADEL_PROVISIONER_PAT_FILE}"
attempts="${1:-120}"

if [ -s "$dest" ]; then
  printf 'provisioner PAT already present at %s\n' "$dest"
  exit 0
fi

mkdir -p "$(dirname -- "$dest")"

i=1
while [ "$i" -le "$attempts" ]; do
  if [ -s "$raw_file" ]; then
    cp -f -- "$raw_file" "$dest"
    chmod 600 "$dest"
    printf 'provisioner PAT copied to %s\n' "$dest"
    exit 0
  fi
  sleep 1
  i=$((i + 1))
done

printf >&2 'timed out waiting for provisioner PAT at %s after %s seconds\n' "$raw_file" "$attempts"
printf >&2 'is ZITADEL healthy and configured with FirstInstance.Org.Machine in infra/zitadel/steps.yaml?\n'
exit 1
