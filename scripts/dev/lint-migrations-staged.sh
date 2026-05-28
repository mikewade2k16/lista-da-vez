#!/usr/bin/env bash
# lint-migrations-staged.sh — lint de arquivos SQL de migration (pre-commit)
#
# Verifica se DDL usa nomes qualificados com schema.
# Rodado via lint-staged quando .sql de migration é staged.
#
# Falha: ALTER TABLE consultants
# OK:    ALTER TABLE queue.consultants
set -euo pipefail

if [ "$#" -eq 0 ]; then
  exit 0
fi

fail=0

for file in "$@"; do
    case "$file" in
        *migrations/*.sql) ;;
        *) continue ;;
    esac

    # DDL TABLE sem schema: linha bate no padrão mas não tem "palavra.palavra" depois do TABLE
    while IFS= read -r line; do
        echo "  ERRO [$file]: DDL sem schema qualificado: $line"
        echo "  → Use 'schema.tabela' (ex: 'queue.consultants', não 'consultants')"
        fail=1
    done < <(
        grep -inE '^\s*(alter|create|drop)\s+table(\s+if(\s+not)?\s+exists)?\s+[a-z_"]' "$file" \
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
