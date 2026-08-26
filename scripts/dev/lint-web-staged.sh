#!/usr/bin/env bash
# lint-web-staged.sh — roda ESLint --fix nos arquivos web/ staged.
#
# Por que wrapper?
#   - lint-staged passa caminhos absolutos do Windows ("C:/...") como args.
#   - ESLint precisa do cwd em web/ para encontrar eslint.config.mjs.
#   - bash -c com aspas duplas dentro de aspas duplas vira escape hell no Windows.
#   - Este wrapper resolve isso: muda cwd para web/, converte paths absolutos
#     para relativos, e roda eslint --fix.
set -euo pipefail

if [ "$#" -eq 0 ]; then
  exit 0
fi

repo_root="$(git rev-parse --show-toplevel)"
web_root="$repo_root/web"

# Normaliza separadores Windows para Unix e converte para relativo a web/
rel_files=()
for arg in "$@"; do
  norm="${arg//\\//}"
  # Remove prefixo absoluto (Windows: C:/...) ou relativo
  case "$norm" in
    "$web_root"/*) rel_files+=("${norm#$web_root/}") ;;
    web/*)         rel_files+=("${norm#web/}") ;;
    /*)            rel_files+=("$norm") ;; # fora de web/ — ignora silenciosamente
    *)             rel_files+=("$norm") ;;
  esac
done

if [ "${#rel_files[@]}" -eq 0 ]; then
  exit 0
fi

cd "$web_root"
# A configuracao flat do projeto estende o arquivo gerado pelo modulo @nuxt/eslint.
# Caches podem ser removidos entre builds; regenere apenas quando ele estiver ausente.
if [ ! -f .nuxt/eslint.config.mjs ]; then
  echo "Preparing Nuxt ESLint config..."
  node node_modules/@nuxt/cli/bin/nuxi.mjs prepare
fi

# Nao use `npx eslint` aqui: quando node_modules foi criado no container,
# web/node_modules/.bin/eslint e um symlink Linux quebrado no host Windows.
# Chamar o entrypoint com Node funciona nos dois ambientes.
exec node node_modules/eslint/bin/eslint.js --fix --max-warnings=999 "${rel_files[@]}"
