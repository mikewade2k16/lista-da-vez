#!/bin/sh
# AC-04: cria a role de runtime da api no postgres do compose dev.
# Roda automatico no primeiro init do volume (docker-entrypoint-initdb.d)
# e manualmente em volume existente:
#   docker compose exec -T postgres sh /docker-entrypoint-initdb.d/10-app-role.sh
set -eu
: "${APP_DB_ROLE:=omni_app}"
: "${APP_DB_ROLE_PASSWORD:?APP_DB_ROLE_PASSWORD obrigatoria}"
psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
  -v role="$APP_DB_ROLE" -v pw="$APP_DB_ROLE_PASSWORD" \
  -f /scripts/db/create-app-role.sql
