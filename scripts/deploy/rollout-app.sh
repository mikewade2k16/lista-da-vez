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
@IMAGE_PULL@
"${compose[@]}" up -d --no-deps --no-build --pull never --wait --wait-timeout 180 @FORCE_RECREATE@ api web
"${compose[@]}" ps api web
curl --fail --silent --show-error --retry 5 --retry-delay 3 --retry-all-errors @PUBLIC_URL@/healthz >/dev/null
curl --fail --silent --show-error --retry 5 --retry-delay 3 --retry-all-errors @PUBLIC_URL@/ >/dev/null
docker ps --format '{{.Names}} {{.Image}}' > "$release/images.after.txt"
touch "$release/app.success"
trap - ERR
