#!/usr/bin/env bash
# Backup pre-deploy executado em SEGUNDO PLANO na VPS (iniciado pela fase prepare,
# antes do build local). A fase publish espera `backup.status` = 0 antes de trocar
# IMAGE_TAG ou recriar containers; qualquer falha aborta o deploy sem mexer na stack.
#
# Formato inalterado (SQL plano .sql.gz + .sha256): o runbook de restore continua
# valendo. Ganhos: dumps do Postgres e do Evolution em paralelo, compressao rapida
# (pigz quando existir, senao gzip -1) e escrita em .part com rename so apos gzip -t.
set -euo pipefail
cd @REMOTE_PATH@
umask 077
mkdir -p backups
release=@RELEASE_DIR@
status_file="$release/backup.status"
rm -f "$status_file"
trap 'printf "%s\n" "$?" > "$status_file"' EXIT

compress() {
  if command -v pigz >/dev/null 2>&1; then pigz -1; else gzip -1; fi
}

# $1 = nome do arquivo final, $2.. = argumentos do compose antes de exec.
dump() {
  local target=$1
  shift
  if ! { "$@" exec -T "$service" sh -lc 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB"' </dev/null | compress > "$target.part" \
    && gzip -t "$target.part"; }; then
    rm -f -- "$target.part"
    echo "ERRO: dump de $service falhou." >&2
    return 1
  fi
  mv -- "$target.part" "$target"
  sha256sum "$target" > "$target.sha256"
  printf '%s\n' "@REMOTE_PATH_RAW@/$target"
}

stamp=$(date +%Y%m%d_%H%M%S)
compose=(docker compose --env-file @ENV_FILE@ -f docker-compose.prod.yml)

service=postgres dump "backups/backup_@TAG@_$stamp.sql.gz" "${compose[@]}" &
main_pid=$!
evolution_pid=
if [ "@WITH_EVOLUTION@" = "1" ] && [ -n "$("${compose[@]}" --profile omnichannel ps -q evolution-db)" ]; then
  service=evolution-db dump "backups/evolution_@TAG@_$stamp.sql.gz" "${compose[@]}" --profile omnichannel &
  evolution_pid=$!
fi
wait "$main_pid"
if [ -n "$evolution_pid" ]; then wait "$evolution_pid"; fi
