# Briefing — Claude Code (Trilha A: backend, infra, scripts, docs)

> Você é o Claude Code. Sua zona é o backend Go + infraestrutura + docs + CI. Ver [PARALELIZACAO.md](../PARALELIZACAO.md) para visão geral. Ver [README.md](README.md) para regras comuns.

## Sua zona — pode editar

- `back/**` (todo o backend Go)
- `.github/**` (workflows CI/CD)
- `docs/**` exceto `docs/agents/CODEX.md` e `docs/agents/COPILOT.md`
- `scripts/**`
- `.husky/**`
- `.gitignore`, `.golangci.yml`
- `package.json` raiz
- `README.md`, `AGENT.md` raiz

## NÃO toca

- `web/**` (zona dos outros 2 agentes)
- `web/package.json`

---

## ONDA 1 — Tarefas

### Tarefa A1 — Fase 3.1: Remover código morto no backend ✅ (2026-05-21)

Itens já identificados como mortos (validados em 2026-05-16) e o que de fato foi feito:

- [x] Removida pasta vazia `back/cmd/debuginvite/`.
- [x] **`StreamCSV` foi mantido** em `back/internal/modules/erp/csv_parser.go:185`. Apesar de a produção só chamar `StreamCSVWithLimit`, há 10+ callers no `csv_parser_test.go` (mesmo pacote) que usam o wrapper sem limite. Como tests contam como callers legítimos, não é morto.
- [x] Removido `back/internal/modules/auth/store_memory.go` inteiro (`MemoryUserStore` + `NewMemoryUserStore` + `FindByEmail`/`FindByID`/`LoadUserForAuth` + `SeedDemoUsers`). Confirmado zero callers externos via Grep antes de deletar; a persistência roda 100% no Postgres e o seed demo vive em `migrations/0002_seed_demo_auth.sql`.
- [x] Removidas as constantes `DemoTenantID`, `StoreRiomarID`, `StoreJardinsID` em `model.go` — só `store_memory.go` consumia (testes usam os literais direto).
- [x] `go vet ./...` + `go build ./...` + `go test ./...` em `back/` → tudo verde.
- [x] `golangci-lint run ./...` → 104 issues, todos pré-existentes (nenhum em `auth/`). `unused` ficou em 19 — o "esperado 14" do briefing era estimativa otimista; o que importa é não ter introduzido issue nova.
- [x] `back/internal/modules/auth/AGENT.md` atualizado: removido `store_memory.go` da lista + nota explicando a remoção.

Sugestão de commit (para Mike): `refactor(back): remover código morto identificado na Fase 3`

### Tarefa A2 — Fase 5: Renomear `Controlle10 - ftp/` → `erp-source-local/` ✅ (2026-05-21)

- [x] Renomeada a pasta local via `mv` (494 MB de conteúdo preservado, está no `.gitignore`).
- [x] Atualizado bind em [docker-compose.yml:93](../../docker-compose.yml#L93) para `"./erp-source-local:/app/data/erp/source:ro"`.
- [x] Atualizado [.gitignore](../../.gitignore): adicionado `erp-source-local/`, mantido `Controlle10 - ftp/` como defesa em profundidade por 1 ciclo.
- [x] Atualizados excludes em [.github/workflows/deploy-vps.yml](../../.github/workflows/deploy-vps.yml) e [scripts/deploy/deploy-vps-fast.ps1](../../scripts/deploy/deploy-vps-fast.ps1) — novo `--exclude='erp-source-local'` adicionado, antigo mantido.
- [x] Atualizadas refs em [docs/ESTADO_ATUAL.md](../ESTADO_ATUAL.md) (seções 2.4 / 6.1 / pontos de atenção), [docs/estado-atual.html](../estado-atual.html) (rótulo do gráfico de diretórios), [docs/ERP_CONSOLIDATED_PIPELINE.md](../ERP_CONSOLIDATED_PIPELINE.md), [docs/DEPLOY_VPS.md](../DEPLOY_VPS.md) e [docs/DEPLOY_CHECKLIST.md](../DEPLOY_CHECKLIST.md).
- [x] [docs/ERP_FTP_INGESTION.md](../ERP_FTP_INGESTION.md) verificado — não referencia o nome antigo, nada a fazer.
- [x] `docker compose config` verde.

Sugestão de commit (para Mike): `chore(infra): renomear Controlle10 - ftp/ para erp-source-local/`

### Tarefa A3 — Fase 7.4: Fatiar 8 arquivos Go > 1000 linhas — ✅ completo (2026-05-21)

Resultado:

- [x] **analytics/service.go** 1003 → 230 linhas (orchestrator) + 4 arquivos novos (`service_ranking.go`, `service_data.go`, `service_intelligence.go`, `helpers.go`). `go test` verde.
- [x] **alerts/store_postgres.go** 1466 → 140 linhas (struct + shared helpers) + 3 arquivos novos (`store_postgres_instances.go`, `store_postgres_rules.go`, `store_postgres_signals.go`). `go test` verde.
- [x] **settings/service.go** 1320 → 123 linhas (orchestrator) + 6 arquivos novos (`service_bundle.go`, `service_sections.go`, `service_modal.go`, `service_options.go`, `service_products.go`, `helpers.go`). `go test` verde.
- [x] **reports/service.go** 1217 → 353 linhas (4 endpoints + multi-store) + 4 arquivos novos (`service_loading.go`, `service_metrics.go`, `service_charts.go`, `helpers.go`). `go build` verde (módulo sem testes).
- [x] **operations/service.go** 1968 → 428 linhas (Service + Snapshot + Overview + auth) + 6 arquivos novos (`service_queue.go`, `service_pause.go`, `service_finish.go`, `service_alerts.go`, `snapshot.go`, `helpers.go`). `go test` verde.
- [x] **tasks/repository_postgres.go** 1535 → 98 linhas (struct + auth helpers) + 5 arquivos novos (`repository_postgres_boards.go`, `repository_postgres_tasks.go`, `repository_postgres_collab.go`, `repository_postgres_tracking.go`, `repository_helpers.go`). `go test` verde.
- [x] **erp/service.go** 1366 → 309 linhas (Service + 7 endpoints query/overview) + 2 arquivos novos (`service_sync.go` com BootstrapItems/Bootstrap/IngestStore/IngestAllStores/streamAndImport/loadCSVBatch/importBatch/manualSyncAllowed/normalizeIngest*, `service_helpers.go` com normalizers + sourceCandidate + permission gates + small utils). `go test` verde.
- [x] **erp/repository_postgres.go** 2942 → 11 linhas (struct + construtor) + 17 arquivos novos por responsabilidade (`repository_scope.go`, `repository_status.go`, `repository_items.go`, `repository_raw_records.go`, `repository_records_stats.go`, `repository_crm*.go`, `repository_import_*.go`, `repository_sync_*.go`, `repository_raw_mirror.go`). `go test ./...` verde.

**Para cada fatiamento (procedimento aplicado em todos):**
1. Antes de mexer: rodar `go test ./internal/modules/<modulo>/...` baseline (verde).
2. Extrair funções em arquivos novos no mesmo pacote (mesmo `package`).
3. Não exportar nada que não estava exportado antes — todos os privates ficaram privates.
4. Rodar `go vet` + `go build` + `go test` por módulo, todos verdes.
5. `golangci-lint run ./internal/modules/erp/...` foi rodado após o último corte: restaram 17 issues pré-existentes em `service_sync.go`/`ftp_client.go` (gocritic/gosec/ineffassign), sem issue nos novos `repository_*`.

Sugestão de commit por módulo (para Mike): `refactor(<modulo>): fatiar <arquivo>.go em N arquivos por responsabilidade`

### Tarefa A4 — Fase 8.6: Guard contra secret default em produção ✅ (2026-05-21)

`Load()` retorna `Config` sem `error` e tem muitos callers — mexer na assinatura era ruído. Solução: método `(Config).Validate()` que é no-op em dev/docker e aborta em production, chamado de `main.go` logo após `Load()`.

- [x] Em [back/internal/platform/config/config.go](../../back/internal/platform/config/config.go): adicionado `(Config).Validate() error` + constantes `devTokenSecretDefault` e `productionMinBcrypt`. Reprova `AUTH_TOKEN_SECRET` vazio ou igual ao default de dev, e `AUTH_BCRYPT_COST < 10`.
- [x] Em [back/cmd/api/main.go](../../back/cmd/api/main.go): logo após `config.Load()` + criação do logger, chama `cfg.Validate()`; em erro, loga `config_invalid` e `os.Exit(1)`.
- [x] Em [back/internal/platform/config/config_test.go](../../back/internal/platform/config/config_test.go): 5 testes novos cobrindo dev no-op, secret vazio em prod, secret default em prod, bcrypt baixo em prod, e config válida em prod. Todos passando.
- [x] [back/internal/platform/config/AGENT.md](../../back/internal/platform/config/AGENT.md) atualizado documentando os guards.

Sugestão de commit (para Mike): `feat(config): abortar boot em production com secret default ou bcrypt fraco`

### Tarefa A5 — Fase 9 setup: Vitest funcional + 1 teste por store crítica ✅ (2026-05-21)

- [x] `npm --prefix web test` rodado: **13 testes passam** em 2 arquivos (`text.test.ts` 9 testes + `content.test.ts` 4 testes). Vitest 2.1.9 operacional, baseline verde.
- [~] **Criação de pastas `web/app/stores/__tests__/` e `web/app/composables/__tests__/` punted**: ambos os caminhos estão fora da minha zona (Claude = back/infra/docs/scripts). `web/app/stores/**` também está bloqueado pra Codex até a Fase 9. A pasta nascerá organicamente quando o primeiro teste de store/composable for escrito (Codex Onda 2 tarefa B6, ou agente da Onda 2 que pegar Fase 9 completa).
- [~] **Exemplo mínimo no CODEX.md**: o briefing do Codex (Tarefa B6) já tem um snippet pronto de teste de store Pinia com `setActivePinia(createPinia())` + uma assertion — sem necessidade de duplicar aqui. CODEX.md também está fora da minha zona, então não posso editar mesmo.

Sugestão de commit (para Mike): nada a commitar aqui (apenas validação + atualização de status). Inclui o `PARALELIZACAO.md` e este briefing no commit do A4 ou agrupa.

---

## ONDA 2 — Tarefas (sequenciais)

### Tarefa A6 — Fase 8.4: HTTP security headers em prod (via Caddy)

- [ ] Em [docs/DEPLOY_VPS.md](../DEPLOY_VPS.md), adicionar seção "Headers de segurança" com snippet Caddy:
  ```caddyfile
  header {
      Strict-Transport-Security "max-age=31536000; includeSubDomains"
      X-Content-Type-Options "nosniff"
      X-Frame-Options "SAMEORIGIN"
      Referrer-Policy "strict-origin-when-cross-origin"
      Permissions-Policy "geolocation=(), microphone=(), camera=()"
      Content-Security-Policy "default-src 'self'; img-src 'self' data: https:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self' wss://lista.whenthelightsdie.com;"
  }
  ```
- [ ] Documentar como validar pós-deploy: [securityheaders.com](https://securityheaders.com/) ou `curl -I` checando os headers.

Sugestão de commit (para Mike): `docs(deploy): adicionar matriz de HTTP security headers para o Caddy`

### Tarefa A7 — Fase 8.5: CI audits (`npm audit` + `govulncheck`)

- [ ] Em [.github/workflows/deploy-vps.yml](../../.github/workflows/deploy-vps.yml), adicionar steps **antes** do deploy:
  ```yaml
  - name: Audit npm dependencies
    run: npm --prefix web audit --audit-level=high
    continue-on-error: false

  - name: Install govulncheck
    run: go install golang.org/x/vuln/cmd/govulncheck@latest

  - name: Audit Go dependencies
    working-directory: back
    run: govulncheck ./...
  ```
- [ ] Decidir política: `continue-on-error: true` (warn) ou `false` (bloqueia deploy). Recomendação: `false` para `npm audit --audit-level=high` (não bloqueia em moderate/low) e `false` para `govulncheck` (vulnerabilidade Go = sério).
- [ ] Documentar em [README.md](../../README.md) seção "Validação".

Sugestão de commit (para Mike): `ci(security): adicionar npm audit e govulncheck no workflow de deploy`

### Tarefa A8 — Fase 8.8: Script gerador de COMPONENT_INVENTORY

- [ ] Criar [scripts/dev/gen-component-inventory.mjs](../../scripts/dev/) que:
  - Lista arquivos `.vue` em `web/app/components/`, `web/app/features/` (não deve existir mais), `web/layers/*/components/`
  - Conta linhas
  - Detecta `<style scoped>` (true/false)
  - Detecta uso de TipTap, Pinia, composables externos
  - Gera tabela markdown em `docs/COMPONENT_INVENTORY_AUTO.md` (NÃO sobrescrever o `COMPONENT_INVENTORY.md` que é planejamento humano do `web-reference/`)
- [ ] Adicionar comando em `package.json` raiz: `"inventory": "node scripts/dev/gen-component-inventory.mjs"`.
- [ ] Rodar uma vez e commitar o output inicial.

Sugestão de commit (para Mike): `chore(scripts): adicionar gerador de inventário de componentes`

### Tarefa A9 — Fase 9 completar: testes de stores back-relevantes

Você não toca testes do front (zona dos outros agentes), mas se notar que algum módulo Go crítico está sem teste, **criar 1 teste mínimo cobrindo o caminho feliz**:

- [ ] Avaliar se `back/internal/modules/{analytics,consultants,core,notifications,reports,tenants}` precisam de teste mínimo (estão na lista de "sem teste" da Seção 4.6 do ESTADO_ATUAL).
- [ ] Priorizar `core` (RBAC v2) e `notifications` (segurança e correção crítica).
- [ ] 1 arquivo de teste por módulo, cobrindo só o construtor + 1 método público.

Sugestão de commit (para Mike): `test(<modulo>): adicionar teste mínimo para <Modulo>`

---

## ONDA 3 — Fase 4 (rename Omni) — TODOS OS OUTROS PAUSAM

**Antes de começar**: confirmar com o usuário que Codex e Copilot terminaram tudo da Onda 1+2.

Procedimento completo na [Fase 4 do PLANO_REFATORACAO.md](../PLANO_REFATORACAO.md). Resumo:

- [ ] 4.1 — `package.json` raiz e web (`name`)
- [ ] 4.2 — `docker-compose.yml` + `docker-compose.prod.yml` (`name`, `APP_NAME`, `COMPOSE_PROJECT_NAME`)
- [ ] 4.3 — `.env.docker.example` + `.env.production.example` (POSTGRES_*, APP_NAME, SMTP_FROM_NAME)
- [ ] 4.4 — `back/internal/platform/config/config.go:68` + `back/internal/modules/auth/password_reset_delivery.go:136`
- [ ] 4.5 — `web/nuxt.config.ts:105` (head.title)
- [ ] 4.6 — DB prod com `ALTER DATABASE listaatendimento RENAME TO omni` em janela
- [ ] 4.7 — README, AGENT.md (todos), back/README.md, back/PLAN.md, back/START_LOCAL.md
- [ ] 4.8 — (opcional) renomear diretório do repo local

Sugestão para Mike: commit por sub-tarefa, ou tudo em 1 commit grande `chore: renomear lista-da-vez/fila-atendimento para Omni`.

---

## ONDA 4 — Validação e deploy

- [ ] Rodar a Validação Final do [PARALELIZACAO.md](../PARALELIZACAO.md).
- [ ] Smoke local com usuário.
- [ ] Atualizar [ESTADO_ATUAL.md](../ESTADO_ATUAL.md) com o estado pós-deploy.
- [ ] Atualizar [plano-refatoracao.html](../plano-refatoracao.html) marcando todas as fases como ✅.

Sugestão de commit final (para Mike): `docs: atualizar ESTADO_ATUAL e plano após Ondas 1-3`

---

## Critério de "tarefa concluída" (checklist por tarefa)

Para considerar `[x]`:

1. Código mudado no working tree (sem rodar git — Mike commita).
2. `go test ./...` em `back/` passa (se mexeu back).
3. `golangci-lint run ./internal/modules/<modulo>/...` não introduziu issue nova.
4. AGENT.md do módulo tocado atualizado.
5. Linha sua em [PARALELIZACAO.md](../PARALELIZACAO.md) atualizada para 🟢.
6. Aviso no chat dizendo "Tarefa AX pronta, sugestão de commit: ..." para Mike disparar o git.
