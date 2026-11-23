#!/usr/bin/env bash
set -euo pipefail
cd @REMOTE_PATH@
compose=(docker compose --env-file @ENV_FILE@ -f docker-compose.prod.yml)
release=@RELEASE_DIR@
rollback() {
  local status=$?
  trap - ERR
  echo 'Falha na atualizacao; restaurando Compose e IMAGE_TAG anteriores.' >&2
  cp "$release/env.before" @ENV_FILE@
  cp "$release/compose.before.yml" docker-compose.prod.yml
  "${compose[@]}" up -d --no-deps --no-build --pull never --wait --wait-timeout 180 api web || true
  exit "$status"
}
trap rollback ERR
@REGISTRY_AUTH@
omni_timed_step remote.images.pull bash -c @IMAGE_PULL_QUOTED@
omni_timed_step remote.containers.health "${compose[@]}" up -d --no-deps --no-build --pull never --wait --wait-timeout 180 @FORCE_RECREATE@ api web
"${compose[@]}" ps api web
omni_timed_step remote.http.health curl --fail --silent --show-error --retry 5 --retry-delay 3 --retry-all-errors --output /dev/null @PUBLIC_URL@/healthz
omni_timed_step remote.http.web curl --fail --silent --show-error --retry 5 --retry-delay 3 --retry-all-errors --output /dev/null @PUBLIC_URL@/
# Agente de observabilidade (mesma imagem da API): opcional, nunca bloqueia nem
# reverte o deploy de api/web.
"${compose[@]}" up -d --no-deps --no-build --pull never obs-agent \
  || echo 'Aviso: obs-agent nao subiu; api/web seguem no ar.' >&2
docker ps --format '{{.Names}} {{.Image}}' > "$release/images.after.txt"
touch "$release/app.success"
trap - ERR
