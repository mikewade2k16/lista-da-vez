// lint-staged config — Omni
//
// Estratégia: cada categoria de arquivo dispara um wrapper shell pequeno
// em scripts/dev/. Wrappers são robustos contra paths absolutos do Windows
// e escape hell de aspas duplas no Git Bash.
//
// Wrappers:
//   - scripts/dev/lint-web-staged.sh    → eslint --fix com cwd em web/
//   - scripts/dev/format-web-staged.sh  → prettier --write com cwd em web/
// O golangci-lint roda uma única vez no pre-commit, depois deste arquivo. Assim,
// o chunking interno do lint-staged não abre vários runners concorrentes.
//
// Por que os wrappers em vez de bash -c inline?
//   - lint-staged passa paths absolutos do Windows ("C:/...") como args.
//   - ESLint/Prettier precisam de cwd em web/ para achar configs.
//   - bash -c "cd web && cmd "$path1" "$path2"" no Windows quebra com aspas duplas
//     dentro de aspas duplas.
//   - Wrapper resolve isso de forma limpa e auditável.

export default {
  // Web — código (ESLint + Prettier)
  'web/**/*.{ts,vue,js,mjs}': [
    'scripts/dev/lint-web-staged.sh',
    'scripts/dev/format-web-staged.sh',
  ],

  // Web — apenas Prettier (configs, docs, estilos)
  'web/**/*.{json,md,css,scss}': ['scripts/dev/format-web-staged.sh'],

  // Back — gofmt file-level. O lint de pacote roda uma vez no pre-commit.
  'back/**/*.go': ['gofmt -w'],

  // Migrations — DDL deve usar schema qualificado (ex: queue.consultants, não consultants)
  'back/internal/platform/database/migrations/*.sql': ['scripts/dev/lint-migrations-staged.sh'],
}
