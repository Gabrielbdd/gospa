#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
project_dir=$(CDPATH= cd -- "$script_dir/.." && pwd)

cd "$project_dir"
. ./scripts/load-env.sh

host="${GOSPA_ZITADEL_EXTERNAL_DOMAIN:-localhost}"
port="${GOSPA_ZITADEL_PORT:-8081}"
url="http://${host}:${port}/debug/healthz"
attempts="${1:-120}"

poll() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsS --max-time 2 "$1" >/dev/null 2>&1
  elif command -v wget >/dev/null 2>&1; then
    wget -qO- --timeout=2 "$1" >/dev/null 2>&1
  else
    printf >&2 'wait-for-zitadel: neither curl nor wget found on host\n'
    return 2
  fi
}

i=1
while [ "$i" -le "$attempts" ]; do
  if poll "$url"; then
    printf 'zitadel is ready at %s\n' "$url"
    exit 0
  fi
  sleep 1
  i=$((i + 1))
done

printf >&2 'timed out waiting for zitadel at %s after %s seconds\n' "$url" "$attempts"
sh ./scripts/compose.sh logs zitadel >&2 || true
exit 1
