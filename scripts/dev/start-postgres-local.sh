#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
source "$ROOT_DIR/scripts/dev/_common.sh"

SCRIPT_PATH="$(to_windows_path "$ROOT_DIR/back/scripts/postgres/init-local.ps1")"

powershell.exe -ExecutionPolicy Bypass -File "$SCRIPT_PATH"
