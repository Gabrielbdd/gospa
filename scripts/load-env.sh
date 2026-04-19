#!/bin/sh
set -eu

# Load optional local overrides and derive one database URL for goose and the app.
if [ -f ./.env ]; then
  set -a
  # shellcheck disable=SC1091
  . ./.env
  set +a
fi

: "${GOFRA_POSTGRES_IMAGE:=postgres:18.3-alpine3.23}"
: "${GOFRA_DB_HOST:=localhost}"
: "${GOFRA_DB_PORT:=5432}"
: "${GOFRA_DB_USER:=postgres}"
: "${GOFRA_DB_PASSWORD:=postgres}"
: "${GOFRA_DB_NAME:=gospa}"
: "${GOFRA_DB_SSLMODE:=disable}"

: "${GOSPA_ZITADEL_IMAGE:=ghcr.io/zitadel/zitadel:stable}"
: "${GOSPA_ZITADEL_PORT:=8081}"
: "${GOSPA_ZITADEL_EXTERNAL_DOMAIN:=localhost}"
: "${GOSPA_ZITADEL_MASTERKEY:=MasterkeyNeedsToHave32Characters}"

# Use $PWD — not $0 — because this script is sourced (not executed) and
# $0 in a sourced script is the calling shell, not the script path.
# Mise tasks always run from the project root, so $PWD is the right anchor.
: "${GOSPA_ZITADEL_PROVISIONER_PAT_FILE:=$PWD/.secrets/zitadel-provisioner.pat}"
: "${GOSPA_INSTALL_TOKEN_FILE:=$PWD/.secrets/install-token}"

default_database_url="postgres://${GOFRA_DB_USER}:${GOFRA_DB_PASSWORD}@${GOFRA_DB_HOST}:${GOFRA_DB_PORT}/${GOFRA_DB_NAME}?sslmode=${GOFRA_DB_SSLMODE}"
: "${DATABASE_URL:=$default_database_url}"
: "${GOFRA_DATABASE__DSN:=$DATABASE_URL}"

export GOFRA_POSTGRES_IMAGE
export GOFRA_DB_HOST
export GOFRA_DB_PORT
export GOFRA_DB_USER
export GOFRA_DB_PASSWORD
export GOFRA_DB_NAME
export GOFRA_DB_SSLMODE
export GOSPA_ZITADEL_IMAGE
export GOSPA_ZITADEL_PORT
export GOSPA_ZITADEL_EXTERNAL_DOMAIN
export GOSPA_ZITADEL_MASTERKEY
export GOSPA_ZITADEL_PROVISIONER_PAT_FILE
export GOSPA_INSTALL_TOKEN_FILE
export DATABASE_URL
export GOFRA_DATABASE__DSN
