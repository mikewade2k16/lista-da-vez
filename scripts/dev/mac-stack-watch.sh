#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

compose=(
  docker compose
  -f docker-compose.yml
  -f compose.mac-local.yaml
  --profile automation
  --profile omnichannel
  --profile meta-ads-assistant
)
services=(
  postgres api web local-access redis n8n waha whisper
  evolution-db evolution meta-ads-assistant
)

case "${1:-}" in
  up)
    "${compose[@]}" up -d --build "${services[@]}"
    ;;
  start)
    "${compose[@]}" start "${services[@]}"
    ;;
  watch)
    ;;
  *)
    echo "Uso: $0 {up|start|watch}" >&2
    exit 2
    ;;
esac

echo "Compose Watch ativo para web. Mantenha este terminal aberto enquanto edita."
exec "${compose[@]}" watch --no-up web
