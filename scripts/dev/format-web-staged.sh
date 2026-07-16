#!/usr/bin/env bash
# format-web-staged.sh — roda Prettier --write nos arquivos web/ staged.
#
# Mesma motivação de lint-web-staged.sh: cwd em web/ + paths relativos.
set -euo pipefail

if [ "$#" -eq 0 ]; then
  exit 0
fi

repo_root="$(git rev-parse --show-toplevel)"
web_root="$repo_root/web"

rel_files=()
for arg in "$@"; do
  norm="${arg//\\//}"
  case "$norm" in
    "$web_root"/*) rel_files+=("${norm#$web_root/}") ;;
    web/*)         rel_files+=("${norm#web/}") ;;
    /*)            rel_files+=("$norm") ;;
    *)             rel_files+=("$norm") ;;
  esac
done

if [ "${#rel_files[@]}" -eq 0 ]; then
  exit 0
fi

cd "$web_root"
# Evita depender do wrapper em node_modules/.bin, que pode ter sido criado
# como symlink Linux pelo container e ficar quebrado no host Windows.
exec node node_modules/prettier/bin/prettier.cjs --write "${rel_files[@]}"
