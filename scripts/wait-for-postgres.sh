#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
project_dir=$(CDPATH= cd -- "$script_dir/.." && pwd)

cd "$project_dir"
. ./scripts/load-env.sh

attempts="${1:-30}"
i=1
while [ "$i" -le "$attempts" ]; do
  if sh ./scripts/compose.sh exec -T postgres \
    pg_isready --dbname "$GOFRA_DB_NAME" --username "$GOFRA_DB_USER" >/dev/null 2>&1; then
    printf 'postgres is ready on %s:%s\n' "$GOFRA_DB_HOST" "$GOFRA_DB_PORT"
    exit 0
  fi
  sleep 1
  i=$((i + 1))
done

printf >&2 'timed out waiting for postgres after %s seconds\n' "$attempts"
sh ./scripts/compose.sh logs postgres >&2 || true
exit 1
