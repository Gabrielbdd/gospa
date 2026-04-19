#!/bin/sh
set -eu

# Materialise a random install token at the path Gospa reads at startup
# (./.secrets/install-token). Idempotent: keeps the existing token when
# present so a `mise run infra` rerun does not invalidate the operator's
# already-pasted bootstrap secret.
#
# `mise run infra:reset` deletes the file so the next infra run produces
# a fresh token alongside the new ZITADEL bootstrap.

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
project_dir=$(CDPATH= cd -- "$script_dir/.." && pwd)

cd "$project_dir"
. ./scripts/load-env.sh

dest="${GOSPA_INSTALL_TOKEN_FILE}"
mkdir -p "$(dirname -- "$dest")"

if [ -s "$dest" ]; then
  printf 'install token already present at %s (kept)\n' "$dest"
  exit 0
fi

# 16 random bytes -> 32 hex chars. od + tr is portable across BSD/GNU.
token=$(LC_ALL=C od -An -N16 -tx1 /dev/urandom | tr -d ' \n')

printf '%s\n' "$token" > "$dest"
chmod 600 "$dest"
printf 'install token generated at %s\n' "$dest"
