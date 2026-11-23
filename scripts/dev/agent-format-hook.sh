#!/usr/bin/env bash
# Hook PostToolUse (Write|Edit) do Claude Code — Omni.
#
# Deixa cada arquivo que o agente cria/edita ja no padrao do pre-commit, para o
# commit nunca travar por formatacao/lint:
#   web/*.{ts,vue,js,mjs}          -> prettier --write + eslint --fix
#   web/*.{json,md,css,scss,html}  -> prettier --write
#   back/**/*.go                   -> gofmt -w (Docker, mesma imagem do hook)
#   .claude/skills/**              -> agents:sync
# Se sobrar erro de ESLint, sai com codigo 2: o Claude Code devolve a mensagem ao
# agente, que corrige antes de seguir. Falha de ferramenta nao bloqueia a edicao.
set -uo pipefail

repo_root="$(git rev-parse --show-toplevel 2>/dev/null)" || exit 0
file="$(jq -r '.tool_input.file_path // .tool_response.filePath // empty')"
[ -n "$file" ] && [ -f "$file" ] || exit 0

rel="${file#"$repo_root"/}"
[ "$rel" = "$file" ] && exit 0 # fora do repositorio

case "$(basename "$rel")" in
  .DS_Store | ._*) exit 0 ;;
esac

case "$rel" in
  web/node_modules/* | web/.nuxt/* | web/.output/*) exit 0 ;;
  web/*.ts | web/*.vue | web/*.js | web/*.mjs)
    cd "$repo_root/web" || exit 0
    web_rel="${rel#web/}"
    node node_modules/prettier/bin/prettier.cjs --write --log-level warn "$web_rel" >/dev/null 2>&1
    if ! out="$(node node_modules/eslint/bin/eslint.js --fix --max-warnings=999 --no-warn-ignored "$web_rel" 2>&1)"; then
      {
        echo "ESLint ainda acusa erro em $rel (o pre-commit vai bloquear). Corrija antes de continuar:"
        echo "$out" | grep -E 'error|✖' | head -20
      } >&2
      exit 2
    fi
    ;;
  web/*.json | web/*.md | web/*.css | web/*.scss | web/*.html)
    cd "$repo_root/web" || exit 0
    node node_modules/prettier/bin/prettier.cjs --write --log-level warn "${rel#web/}" >/dev/null 2>&1
    ;;
  back/*.go)
    if curl -s -m 2 --unix-socket "$HOME/.docker/run/docker.sock" http://localhost/_ping >/dev/null 2>&1; then
      bash "$repo_root/scripts/dev/go-tools-docker.sh" run gofmt -w "/workspace/$rel" >/dev/null 2>&1
    else
      echo "Docker indisponivel: gofmt nao rodou em $rel (rode npm run precommit:check antes do commit)." >&2
    fi
    ;;
  .claude/skills/*)
    node "$repo_root/scripts/dev/sync-agent-skills.mjs" >/dev/null 2>&1
    ;;
esac
exit 0
