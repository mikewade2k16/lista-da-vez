#!/usr/bin/env bash
# lint-migrations-staged.sh — lint de arquivos SQL de migration (pre-commit)
#
# Verifica se DDL novo no diff staged usa nomes qualificados com schema.
# Rodado via lint-staged quando .sql de migration é staged.
# Linhas legadas que não foram alteradas não podem bloquear um commit sem DDL novo.
#
# Falha: ALTER TABLE consultants
# OK:    ALTER TABLE queue.consultants
set -euo pipefail

if [ "$#" -eq 0 ]; then
  exit 0
fi

fail=0
repo_root="$(git rev-parse --show-toplevel)"

if command -v cygpath >/dev/null 2>&1; then
    repo_root_match="$(cygpath -u "$repo_root")"
else
    repo_root_match="$repo_root"
fi

for file in "$@"; do
    case "$file" in
        *migrations/*.sql) ;;
        *) continue ;;
    esac

    if command -v cygpath >/dev/null 2>&1; then
        file_match="$(cygpath -u "$file")"
    else
        file_match="$file"
    fi

    case "$file_match" in
        "$repo_root_match"/*) repo_file="${file_match#"$repo_root_match"/}" ;;
        *) repo_file="${file_match#./}" ;;
    esac

    # DDL TABLE novo sem schema: a linha bate no padrão, mas não tem
    # "palavra.palavra" depois de TABLE. O diff staged evita reprovar DDL
    # legado já presente no arquivo antes da alteração atual.
    while IFS= read -r line; do
        echo "  ERRO [$file]: DDL sem schema qualificado: $line"
        echo "  → Use 'schema.tabela' (ex: 'queue.consultants', não 'consultants')"
        fail=1
    done < <(
        git -C "$repo_root" diff --cached --unified=0 --no-color -- "$repo_file" \
        | awk '/^\+\+\+ / { next } /^\+/ { sub(/^\+/, ""); print }' \
        | grep -iE '^\s*(alter|create|drop)\s+table(\s+if(\s+not)?\s+exists)?\s+[a-z_"]' \
        | grep -v '^\s*--' \
        | grep -vE '[a-zA-Z_"][.][a-zA-Z_"]' \
        || true
    )
done

if [[ $fail -ne 0 ]]; then
    echo ""
    echo "Migration linter falhou. DDL sem schema bloqueia o commit."
    echo "Referência: back/internal/platform/database/migrations/0104_queue_schema_foundation.sql"
    exit 1
fi
