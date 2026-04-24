#!/usr/bin/env bash
set -euo pipefail

to_windows_path() {
  local input="$1"

  if command -v cygpath >/dev/null 2>&1; then
    cygpath -w "$input"
    return 0
  fi

  if [[ "$input" =~ ^/mnt/([a-zA-Z])/(.*)$ ]]; then
    local drive="${BASH_REMATCH[1]}"
    local rest="${BASH_REMATCH[2]//\//\\}"
    printf '%s:\\%s\n' "${drive^^}" "$rest"
    return 0
  fi

  if [[ "$input" =~ ^/([a-zA-Z])/(.*)$ ]]; then
    local drive="${BASH_REMATCH[1]}"
    local rest="${BASH_REMATCH[2]//\//\\}"
    printf '%s:\\%s\n' "${drive^^}" "$rest"
    return 0
  fi

  printf '%s\n' "$input"
}

wait_for_http() {
  local url="$1"
  local attempts="${2:-40}"
  local delay_seconds="${3:-0.5}"

  local index=0
  while (( index < attempts )); do
    if curl.exe -fsS "$url" >/dev/null 2>&1; then
      return 0
    fi

    sleep "$delay_seconds"
    ((index += 1))
  done

  return 1
}
