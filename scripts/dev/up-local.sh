#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

bash "$ROOT_DIR/scripts/dev/start-postgres-local.sh"
bash "$ROOT_DIR/scripts/dev/start-api-local.sh"
bash "$ROOT_DIR/scripts/dev/start-web-local.sh"
