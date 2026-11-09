#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
image="omni-go-tools:1.26.5-v2.4.0"
back_root="$repo_root/back"

case "${1:-}" in
  ensure)
    if ! docker info >/dev/null 2>&1; then
      echo "Docker Desktop precisa estar ativo para validar arquivos Go no commit." >&2
      exit 1
    fi
    if docker image inspect "$image" >/dev/null 2>&1; then
      exit 0
    fi
    echo "Preparando imagem Docker das ferramentas Go (primeira execução)..."
    docker build \
      --file "$back_root/Dockerfile.devtools" \
      --tag "$image" \
      /tmp
    ;;
  run)
    shift
    mkdir -p "$back_root/.gocache" "$back_root/.gomodcache"
    exec docker run --rm \
      --user "$(id -u):$(id -g)" \
      --env GOCACHE=/go-cache/build \
      --env GOMODCACHE=/go-cache/modules \
      --env XDG_CACHE_HOME=/go-cache/build/xdg \
      --env GOLANGCI_LINT_CACHE=/go-cache/build/golangci-lint \
      --env GIT_CONFIG_COUNT=1 \
      --env GIT_CONFIG_KEY_0=safe.directory \
      --env GIT_CONFIG_VALUE_0=/workspace \
      --volume "$repo_root:/workspace" \
      --volume "$back_root/.gocache:/go-cache/build" \
      --volume "$back_root/.gomodcache:/go-cache/modules" \
      --workdir /workspace \
      "$image" "$@"
    ;;
  *)
    echo "Uso: $0 {ensure|run comando [argumentos...]}" >&2
    exit 2
    ;;
esac
