#!/usr/bin/env bash
# lint-go-staged.sh — wrapper para o pre-commit hook do golangci-lint.
#
# Por que existe?
#   Vários linters Go (unused, errcheck, staticcheck) precisam analisar o
#   PACOTE inteiro, não arquivos isolados. Se passarmos `golangci-lint run
#   arquivo1.go arquivo2.go`, geramos falsos positivos (unused que está usado
#   em outro arquivo do mesmo pacote) e falsos negativos.
#
# O que faz?
#   1. Recebe a lista de arquivos .go staged como argumentos (vinda do lint-staged).
#   2. Extrai os DIRETÓRIOS únicos (= pacotes Go).
#   3. Roda `golangci-lint run` em cada pacote único, usando a notação
#      `./caminho/...` (escopo de pacote, não de arquivo).
#
# Comportamento:
#   - Se nenhum arquivo .go vier, sai com 0.
#   - Se algum pacote tiver issue, falha com exit != 0 (Husky cancela o commit).
#   - golangci-lint usa o .golangci.yml de back/ automaticamente.
#
# Referência da decisão:
#   docs/PLANO_REFATORACAO.md — Fase 6.3 (Pre-commit hook)
#   back/AGENT.md — seção Lint
set -euo pipefail

if [ "$#" -eq 0 ]; then
  exit 0
fi

repo_root="$(git rev-parse --show-toplevel)"

# Extrai pacotes únicos dos arquivos passados. Cada arquivo vem como caminho
# absoluto ou relativo à raiz do repo. Normalizamos para relativo a back/.
declare -A seen
packages=()
for arg in "$@"; do
  # Normaliza para caminho relativo à raiz do repo
  rel="${arg#$repo_root/}"
  # Só nos interessam arquivos sob back/
  case "$rel" in
    back/*) ;;
    *) continue ;;
  esac

  # Diretório (= pacote) relativo a back/
  pkg_dir="$(dirname "${rel#back/}")"
  pkg_pattern="./$pkg_dir/..."

  if [ -z "${seen[$pkg_pattern]:-}" ]; then
    seen[$pkg_pattern]=1
    packages+=("$pkg_pattern")
  fi
done

if [ "${#packages[@]}" -eq 0 ]; then
  exit 0
fi

if ! command -v golangci-lint >/dev/null 2>&1; then
  echo "✖ golangci-lint não encontrado no PATH; o lint do backend não pôde ser executado." >&2
  echo "  Instale a ferramenta ou adicione \$(go env GOPATH)/bin ao PATH e tente novamente." >&2
  exit 127
fi

echo "→ golangci-lint nos pacotes alterados: ${packages[*]}"

cd "$repo_root/back"
# --new-from-rev=HEAD: só reporta issues NOVAS em relação ao último commit.
# Isso evita bloquear commits em pacotes que já tinham dívida documentada
# (baseline de 94 issues registrada em 2026-05-18). A dívida vai ser
# reduzida gradualmente nas Fases 7 e 8 do PLANO_REFATORACAO.
if golangci-lint run --new-from-rev=HEAD "${packages[@]}"; then
  echo "✓ golangci-lint concluído sem novos problemas."
else
  status=$?
  echo "✖ golangci-lint encontrou problemas nos pacotes alterados (código $status)." >&2
  exit "$status"
fi
