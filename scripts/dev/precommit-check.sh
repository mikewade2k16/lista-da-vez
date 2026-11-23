#!/usr/bin/env bash
if [ -z "${BASH_VERSION:-}" ] || shopt -oq posix; then exec bash "$0" "$@"; fi
# Pre-flight do commit — Omni (npm run precommit:check)
#
# Roda as MESMAS checagens do .husky/pre-commit, mas somente leitura: nao corrige,
# nao faz stash e nao mexe no indice. Cobre tudo que um `git add .` levaria
# (staged + modificados + nao rastreados), para achar o problema ANTES do commit.
#
#   npm run precommit:check            # conjunto completo
#   npm run precommit:check -- --staged  # apenas o que ja esta staged
#
# Corrigir com: prettier --write / eslint --fix (em web/), gofmt -w (via Docker).
set -uo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

mode="all"
[ "${1:-}" = "--staged" ] && mode="staged"

files=()
if [ "$mode" = "staged" ]; then
  while IFS= read -r -d '' f; do files+=("$f"); done < <(git diff --cached --name-only --diff-filter=ACMR -z)
else
  while IFS= read -r -d '' f; do files+=("$f"); done < <(
    { git diff --cached --name-only --diff-filter=ACMR -z
      git diff --name-only --diff-filter=ACMR -z
      git ls-files --others --exclude-standard -z; } | sort -zu
  )
fi

failures=()
step() { printf '\n== %s\n' "$1"; }
fail() { failures+=("$1"); }

step "Metadados do sistema operacional (.DS_Store, ._*)"
meta=$(git ls-files | grep -E '(^|/)(\.DS_Store|\._[^/]*)$' || true)
if [ -n "$meta" ]; then
  echo "$meta"
  echo "Rastreados no Git: o Finder reescreve esses arquivos e o lint-staged aborta"
  echo "com 'merge conflict'. Remover do indice: git rm --cached <arquivo>"
  fail "metadados do macOS rastreados"
else
  echo "ok"
fi

step "Skills sincronizadas (.claude -> .agents)"
node scripts/dev/sync-agent-skills.mjs --check || fail "skills fora de sincronia (npm run agents:sync)"

web_code=() web_fmt=() go_files=() sql_files=()
for f in "${files[@]}"; do
  [ -f "$f" ] || continue
  case "$f" in
    web/*.ts | web/*.vue | web/*.js | web/*.mjs) web_code+=("${f#web/}"); web_fmt+=("${f#web/}") ;;
    web/*.json | web/*.md | web/*.css | web/*.scss) web_fmt+=("${f#web/}") ;;
    back/internal/platform/database/migrations/*.sql) sql_files+=("$f") ;;
  esac
  case "$f" in back/*.go) go_files+=("$f") ;; esac
done

step "Prettier (web, ${#web_fmt[@]} arquivos)"
if [ "${#web_fmt[@]}" -gt 0 ]; then
  (cd web && node node_modules/prettier/bin/prettier.cjs --check --log-level warn "${web_fmt[@]}") \
    && echo "ok" || fail "prettier (cd web && npx prettier --write <arquivos>)"
else
  echo "nada a checar"
fi

step "ESLint (web, ${#web_code[@]} arquivos)"
if [ "${#web_code[@]}" -gt 0 ]; then
  (cd web && { [ -f .nuxt/eslint.config.mjs ] || node node_modules/@nuxt/cli/bin/nuxi.mjs prepare >/dev/null; } \
    && node node_modules/eslint/bin/eslint.js --max-warnings=999 "${web_code[@]}") \
    && echo "ok" || fail "eslint (cd web && npx eslint --fix <arquivos>)"
else
  echo "nada a checar"
fi

step "Migrations (${#sql_files[@]} arquivos)"
if [ "${#sql_files[@]}" -gt 0 ]; then
  bash scripts/dev/lint-migrations-staged.sh "${sql_files[@]}" && echo "ok" || fail "migrations sem schema qualificado"
else
  echo "nada a checar"
fi

step "Go: gofmt + golangci-lint (${#go_files[@]} arquivos)"
if [ "${#go_files[@]}" -gt 0 ]; then
  if bash scripts/dev/go-tools-docker.sh ensure; then
    container_files=()
    for f in "${go_files[@]}"; do container_files+=("/workspace/$f"); done
    unformatted=$(bash scripts/dev/go-tools-docker.sh run gofmt -l "${container_files[@]}")
    if [ -n "$unformatted" ]; then
      echo "$unformatted"
      fail "gofmt (bash scripts/dev/gofmt-staged-docker.sh <arquivos>)"
    fi
    bash scripts/dev/lint-go-staged-docker.sh "${go_files[@]}" || fail "golangci-lint"
  else
    fail "Docker indisponivel para checar Go"
  fi
else
  echo "nada a checar"
fi

printf '\n'
if [ "${#failures[@]}" -gt 0 ]; then
  echo "FALHOU (${#failures[@]}):"
  printf '  - %s\n' "${failures[@]}"
  exit 1
fi
echo "Pronto para commit: todas as checagens do pre-commit passaram."
