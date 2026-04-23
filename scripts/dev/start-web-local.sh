#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PORT="${PORT:-3003}"

cd "$ROOT_DIR/web"
export NUXT_PUBLIC_API_BASE="${NUXT_PUBLIC_API_BASE:-http://localhost:8080}"
export NUXT_DEVTOOLS="${NUXT_DEVTOOLS:-false}"
export CHOKIDAR_USEPOLLING="${CHOKIDAR_USEPOLLING:-true}"
export WATCHPACK_POLLING="${WATCHPACK_POLLING:-true}"
export CHOKIDAR_INTERVAL="${CHOKIDAR_INTERVAL:-350}"
npm run dev -- --port "$PORT"
