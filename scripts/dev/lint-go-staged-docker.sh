#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -eq 0 ]; then
  exit 0
fi

repo_root="$(git rev-parse --show-toplevel)"
files=()
for arg in "$@"; do
  norm="${arg//\\//}"
  case "$norm" in
    "$repo_root"/*) files+=("back/${norm#"$repo_root"/back/}") ;;
    back/*) files+=("$norm") ;;
  esac
done

if [ "${#files[@]}" -gt 0 ]; then
  bash "$repo_root/scripts/dev/go-tools-docker.sh" run \
    bash scripts/dev/lint-go-staged.sh "${files[@]}"
fi
