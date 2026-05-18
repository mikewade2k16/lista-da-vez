# Plano de Refatoração — Limpeza e Renomeação

> Documento de execução. Cada bloco é uma **Tarefa** com **Subtarefas** marcáveis. Pré-requisito: ler [ESTADO_ATUAL.md](ESTADO_ATUAL.md) primeiro.
>
> **Versão visual com timeline e gráficos:** [plano-refatoracao.html](plano-refatoracao.html)
>
> Convenções:
> - `[ ]` = pendente · `[~]` = em andamento · `[x]` = concluído
> - Todas as ações devem rodar **localmente**. Commits/push ficam por conta do usuário (instrução durável).
> - Sempre que tocar um módulo do back ou um layer do front, atualizar o respectivo `AGENT.md`/`AGENTS.md`.
> - Mudanças em diretórios/nomes que mexem com Docker exigem `npm run dev:down:volumes` (perda do banco local) **ou** uma estratégia explícita de rename do banco.

---

## Fase 0 — Decisões fechadas

Decisões que travam o restante do plano. **Concluídas em 16/05/2026.**

### Tarefa 0.1 — Nome oficial do produto ✅

| | |
|---|---|
| **Display** | `Omni` |
| **Slug** | `omni` |
| **Renomeação do DB em prod** | Sim, padronizar com `omni` (planejar janela de manutenção). |

Subtarefas:
- [x] Escolher nome canônico → `Omni`.
- [x] Escolher slug técnico → `omni`.
- [x] Decidir sobre o banco em prod → renomear junto.
- [ ] Registrar a decisão no topo do [ESTADO_ATUAL.md](ESTADO_ATUAL.md) (concluído neste commit).
- [ ] Atualizar [AGENT.md](../AGENT.md) com o nome novo (fica para a Fase 4).

**Cuidados específicos do nome `omni`:**
- Não confundir com `omnichannel` (módulo de chat — continua existindo).
- Não confundir com `omnichannel-mvp` / `omnichannel-mvp_default` (stack VPS de outro projeto — preservar).
- Várias coisas já usam o prefixo `omni` (`OmniEditor.vue`, `OmniDataTable.vue`, `useOmniTheme`, `omni-design-system.css`, `acesso.omni.local`) — não precisam mudar, **a renomeação só formaliza**.

### Tarefa 0.2 — Destino dos arquivos da raiz ✅

Subtarefas:
- [x] Aprovar a tabela "manter / mover / remover" da Seção 2 de [ESTADO_ATUAL.md](ESTADO_ATUAL.md) (decisão do usuário).
- [ ] Criar `docs/historico/` (fica para a Fase 1).
- [ ] Renomear `Controlle10 - ftp/` → `erp-source-local/` (fica para a Fase 5).

### Tarefa 0.3 — Layer `queue` esqueleto ✅

| | |
|---|---|
| **Decisão** | Manter como placeholder e documentar. |

Subtarefas:
- [x] Decisão fechada.
- [ ] Adicionar `web/layers/queue/AGENT.md` explicando o estado atual (fica para a Fase 1).

---

## Fase 1 — Limpeza da raiz ✅ (16/05/2026)

Trabalho de baixo risco, mas alto retorno visual e cognitivo. **Concluída.**

### Tarefa 1.1 — Mover histórico para `docs/historico/` ✅

Subtarefas:
- [x] Criar `docs/historico/`.
- [x] Mover `diagnostico.md` → [historico/diagnostico-2026-03-23.md](historico/diagnostico-2026-03-23.md).
- [x] Mover `ERP-auto.md` → [historico/erp-auto-2026-05-05.md](historico/erp-auto-2026-05-05.md).
- [x] Mover `PLANO_PROGRESSO.md` → [historico/plano-alertas-concluido.md](historico/plano-alertas-concluido.md).
- [x] Mover `TESTE_ALERTAS.md` → [historico/teste-alertas.md](historico/teste-alertas.md).
- [x] Mover `todo-alertas.md` → [historico/todo-alertas.md](historico/todo-alertas.md).
- [x] Mover `todo-reuniao.md` → [historico/todo-reuniao.md](historico/todo-reuniao.md).
- [x] Adicionar [historico/README.md](historico/README.md) listando os arquivos.

### Tarefa 1.2 — Triagem do `todo.md` ativo ⚠ parcial

Subtarefas:
- [x] Arquivar `todo.md` raiz → [historico/todo-settings.md](historico/todo-settings.md).
- [x] Adicionar nota no topo de [BACKLOG.md](BACKLOG.md) sinalizando os 170 itens pendentes.
- [ ] **Pendente**: triagem item-a-item dos 170 pendentes — adia para sessão dedicada.

### Tarefa 1.3 — Mover o roadmap para `docs/` ✅

Subtarefas:
- [x] Mover `roadmap.md` → [ROADMAP.md](ROADMAP.md).
- [x] Atualizar link em [COMPONENT_INVENTORY.md](COMPONENT_INVENTORY.md) (linha 225) e em [ESTADO_ATUAL.md](ESTADO_ATUAL.md).
- [x] [README.md](../README.md) e [AGENT.md](../AGENT.md) não referenciavam — sem mudança.

### Tarefa 1.4 — Remover artefatos avulsos da raiz ✅

Subtarefas:
- [x] `git rm` em `editor-page-check.png`, `tasks-3004.png`, `gif-indeva.gif`, `logo.png`, `logo.webp`, `logo.avif`, `back/tmp_api_8081.log`.
- [x] `rm` em `test-perola-api.html` (era untracked).
- [x] Confirmado: componentes Vue (`DashboardHeader`, `DashboardSidebarNav`, `DashboardUnifiedHeader`, `AdminAuthShell`) usam `/logo.*` que resolvem em [web/public/](../web/public/) — nada quebrou.

### Tarefa 1.5 — Mover os helpers de token para fora da raiz ✅

Subtarefas:
- [x] Criar `scripts/dev/token-helpers/`.
- [x] Mover `gen_token.js`, `token_gen.js`, `token_gen_real.js`, `verify.sh` para lá.
- [x] Mover `token.txt`, `full_token.txt`, `payload.b64`, `secret.key` para `tmp/`.
- [x] Atualizar [.gitignore](../.gitignore): adicionar `tmp/`, `scripts/dev/token-helpers/`, `.codex-logs/` e manter as regras antigas como defesa em profundidade.

### Tarefa 1.6 — Limpar `tmp/` raiz ✅

Subtarefas:
- [x] Apagar todos os `back-*.log`, `web-*.log`, `nuxt-*.log` em `tmp/` (datados de mar/abr 2026).
- [x] Remover `tmp/write_sections.py` e `tmp/genhash.go` (throwaway antigos — bcrypt one-off e gerador de boilerplate de Settings).
- [x] `tmp/` agora cai no padrão `tmp/` do `.gitignore`.

### Tarefa 1.7 — Arquivar `dev-compose-perfis.md` ✅

> O documento descreve serviços (`painel-web`, `plataforma-api`, `redis`) que **não existem** neste repositório. É lixo de outro projeto.

Subtarefas:
- [x] Arquivado em [historico/dev-compose-perfis.md](historico/dev-compose-perfis.md) (em vez de removido, para preservar contexto).

---

## Fase 2 — Reorganizar a documentação

### Tarefa 2.1 — Renomear `docs_depoy/` → consolidar em `docs/deploy/`

Subtarefas:
- [ ] Criar `docs/deploy/`.
- [ ] Comparar [docs_depoy/deploy-vps.md](../docs_depoy/deploy-vps.md) com [docs/DEPLOY_VPS.md](DEPLOY_VPS.md) — se for redundante (auto-declarado "arquivado"), descartar.
- [ ] Avaliar [docs_depoy/deploy-main-vps-auto.md](../docs_depoy/deploy-main-vps-auto.md) e [docs_depoy/deploy-producao-checklist.md](../docs_depoy/deploy-producao-checklist.md): integrar ao [docs/DEPLOY_VPS.md](DEPLOY_VPS.md) ou mover como `docs/deploy/{auto,checklist}.md`.
- [ ] Remover `docs_depoy/`.

### Tarefa 2.2 — Tirar o `.md` errante de `pages/`

Subtarefas:
- [ ] Mover [web/app/pages/operacao/operations.md](../web/app/pages/operacao/operations.md) → `docs/operacao/operations.md`.
- [ ] Atualizar referências em [web/AGENTS.md](../web/AGENTS.md) e em [back/PLAN.md](../back/PLAN.md) (que referencia este `.md`).

### Tarefa 2.3 — Decidir o futuro do HTML estático

Subtarefas:
- [ ] Avaliar utilidade de [docs/tasks-orquestrador-plano.html](tasks-orquestrador-plano.html) (100 KB).
- [ ] Se for somente visualização do `.md` correspondente, remover.
- [ ] Se for usado, mover para `docs/assets/`.

### Tarefa 2.4 — Auditar e atualizar `COMPONENT_INVENTORY.md`

> Provavelmente desatualizado. Decidir.

Subtarefas:
- [ ] Comparar [docs/COMPONENT_INVENTORY.md](COMPONENT_INVENTORY.md) com a árvore real de `web/app/components/` e `web/layers/*/components/`.
- [ ] Atualizar OU substituir por um script gerador (`scripts/dev/gen-component-inventory.mjs`).

---

## Fase 3 — Remover código morto

### Tarefa 3.1 — Backend Go

Subtarefas:
- [ ] Remover [back/cmd/debuginvite/](../back/cmd/debuginvite/) (pasta vazia).
- [ ] Remover função `StreamCSV` em [back/internal/modules/erp/csv_parser.go:185](../back/internal/modules/erp/csv_parser.go); migrar 0 callers (já usam `StreamCSVWithLimit`).
- [ ] Remover funções `NewMemoryUserStore` e `SeedDemoUsers` em [back/internal/modules/auth/store_memory.go](../back/internal/modules/auth/store_memory.go). Avaliar se o arquivo inteiro pode ir embora (provavelmente sim).
- [ ] Rodar `go vet ./...` e `go test ./...` em `back/`.
- [ ] Atualizar [back/internal/modules/auth/AGENT.md](../back/internal/modules/auth/AGENT.md) com a remoção.

### Tarefa 3.2 — Frontend Vue

Subtarefas:
- [ ] Remover [web/app/features/operation/components/OperationCampaignBrief.vue](../web/app/features/operation/components/OperationCampaignBrief.vue) (0 referências).
- [ ] Remover [web/layers/tasks/components/admin/AdminPageHeader.vue](../web/layers/tasks/components/admin/AdminPageHeader.vue) (todas as páginas importam a versão core).
- [ ] Tornar `useDashboardState` em [web/app/composables/useDashboardShell.ts](../web/app/composables/useDashboardShell.ts) função interna privada (ou remover o `export`).
- [ ] Remover [web/dist/](../web/dist/) local (re-gera no próximo build).
- [ ] Rodar `npm --prefix web run build`.
- [ ] Atualizar [web/layers/tasks/AGENT.md](../web/layers/tasks/AGENT.md) com a remoção.

### Tarefa 3.3 — Auditoria complementar (opcional)

Subtarefas:
- [ ] Configurar `eslint-plugin-unused-imports` (ou `oxlint`) em [web/](../web/) para detectar imports não usados automaticamente.
- [ ] Rodar `vue-tsc --noEmit` para validar tipos.
- [ ] Decidir se vale rodar `knip` / `ts-prune` para uma segunda passada de mortos.

---

## Fase 4 — Renomeação do projeto para `Omni`

> **Bloqueia em**: Tarefa 0.1 (✅ concluída — nome decidido).
>
> **Valores canônicos:**
> - Display: `Omni`
> - Slug: `omni`
> - DB local (dev): `omni`
> - DB prod: `omni` (após migration de rename — ver Tarefa 4.6)
> - APP_NAME: `omni-api`
>
> **Não tocar** (são identificadores externos/de outro projeto):
> - `omnichannel-mvp_default` (rede Docker compartilhada da VPS)
> - `omnichannel-mvp-caddy-1` (container Caddy da VPS)
> - `/opt/omnichannel/Caddyfile` (path no servidor)
> - `acesso.omni.local` em `AUTH_CONSULTANT_EMAIL_DOMAIN` — já está alinhado ao slug, manter.

### Tarefa 4.1 — Renomear `package.json` (raiz e web)

Subtarefas:
- [ ] [package.json](../package.json) — `name: "lista-da-vez"` → `name: "omni"`.
- [ ] [web/package.json](../web/package.json) — `name: "lista-da-vez-web"` → `name: "omni-web"`.
- [ ] Rodar `npm install` (e `npm --prefix web install`) para regenerar locks.

### Tarefa 4.2 — Renomear no Docker Compose

Subtarefas:
- [ ] [docker-compose.yml:1](../docker-compose.yml) — `name: lista-da-vez` → `name: omni`.
- [ ] [docker-compose.yml:30](../docker-compose.yml) — `APP_NAME: lista-da-vez-api` → `APP_NAME: omni-api`.
- [ ] [docker-compose.prod.yml:1](../docker-compose.prod.yml) — `name: ${COMPOSE_PROJECT_NAME:-listaatendimento}` → `name: ${COMPOSE_PROJECT_NAME:-omni}`.
- [ ] [docker-compose.prod.yml:147](../docker-compose.prod.yml) — **manter** `name: ${PROXY_NETWORK_NAME:-omnichannel-mvp_default}` (rede externa).
- [ ] Validar `docker compose config` e `docker compose -f docker-compose.prod.yml config`.

### Tarefa 4.3 — Renomear nos templates `.env`

Subtarefas:
- [ ] [.env.docker.example](../.env.docker.example):
  - [ ] `POSTGRES_DB=lista_da_vez` → `POSTGRES_DB=omni`.
  - [ ] `POSTGRES_USER=lista_da_vez` → `POSTGRES_USER=omni`.
  - [ ] `POSTGRES_PASSWORD=lista_da_vez_dev` → `POSTGRES_PASSWORD=omni_dev`.
- [ ] [.env.production.example](../.env.production.example):
  - [ ] `COMPOSE_PROJECT_NAME=listaatendimento` → `COMPOSE_PROJECT_NAME=omni`.
  - [ ] `APP_NAME=lista-da-vez-api` → `APP_NAME=omni-api`.
  - [ ] `POSTGRES_DB=listaatendimento` → `POSTGRES_DB=omni`.
  - [ ] `POSTGRES_USER=listaatendimento` → `POSTGRES_USER=omni`.
  - [ ] `SMTP_FROM_NAME=Lista da Vez` → `SMTP_FROM_NAME=Omni`.
  - [ ] **Manter** `PROXY_NETWORK_NAME=omnichannel-mvp_default` (rede externa).
- [ ] [back/.env.example](../back/.env.example) — mesmas trocas onde aparecer `lista*`.
- [ ] Atualizar `.env`, `.env.docker` locais do usuário (não versionados) para refletir os mesmos valores.
- [ ] Documentar em [README.md](../README.md) o procedimento de troca local.

### Tarefa 4.4 — Renomear no código Go

Subtarefas:
- [ ] [back/internal/platform/config/config.go:68](../back/internal/platform/config/config.go) — `"lista-da-vez-api"` → `"omni-api"`.
- [ ] [back/internal/modules/auth/password_reset_delivery.go:136](../back/internal/modules/auth/password_reset_delivery.go) — `"Lista da Vez"` → `"Omni"`.
- [ ] `grep -i "lista[-_ ]da[-_ ]vez\|lista[ -]atendimento\|fila de atendimento"` em `back/` — corrigir resíduos.
- [ ] Rodar `go vet ./...` e `go test ./...` em `back/`.

### Tarefa 4.5 — Renomear no Frontend Nuxt

Subtarefas:
- [ ] [web/nuxt.config.ts:105](../web/nuxt.config.ts) — `title: "Fila de Atendimento MVP"` → `title: "Omni"`.
- [ ] `grep -i "lista da vez\|fila de atendimento"` em `web/` — corrigir resíduos em componentes, README internos, comentários.
- [ ] Verificar [web/layers/core/components/CoreAccountSwitcher.vue](../web/layers/core/components/CoreAccountSwitcher.vue) e telas de header.
- [ ] Rodar `npm --prefix web run build`.

### Tarefa 4.6 — Renomear o banco em produção (com janela)

> Decisão Fase 0.1: **renomear o banco prod** para `omni`.

Subtarefas:
- [ ] Criar `docs/deploy/db-rename.md` com o procedimento detalhado.
- [ ] **Local**: validar com `npm run dev:down:volumes && npm run dev` (banco novo nasce com `POSTGRES_DB=omni`).
- [ ] **Prod (janela curta)**: abordagem recomendada — derrubar serviços que conectam, executar `ALTER DATABASE listaatendimento RENAME TO omni;`, atualizar `.env` e subir.
- [ ] **Fallback se houver bloqueio de rename**: `pg_dump` → criar banco `omni` → `pg_restore` → trocar `.env`.
- [ ] Atualizar `POSTGRES_USER` (que hoje vale `listaatendimento`) — pode renomear o role separadamente com `ALTER ROLE listaatendimento RENAME TO omni;`.
- [ ] Pós-rename: rodar `docker compose exec api ./migrate up` para confirmar idempotência das migrations no nome novo.

### Tarefa 4.7 — Atualizar READMEs e AGENT.md

Subtarefas:
- [ ] [README.md](../README.md) — título "# Omni", descrição, exemplos de URL/e-mail.
- [ ] [AGENT.md](../AGENT.md) — nome do produto.
- [ ] [back/AGENT.md](../back/AGENT.md), [web/AGENTS.md](../web/AGENTS.md) — referências.
- [ ] AGENT.md por módulo que mencione o produto pelo nome antigo.
- [ ] [back/README.md](../back/README.md), [back/START_LOCAL.md](../back/START_LOCAL.md), [back/PLAN.md](../back/PLAN.md) — referências.

### Tarefa 4.8 — (Opcional) Renomear o diretório local do repo

> Mudar o nome da pasta `c:\Users\Mike\Documents\Projects\fila-atendimento` para `c:\Users\Mike\Documents\Projects\omni`.

Subtarefas:
- [ ] Confirmar com o usuário (operação fecha VSCode e re-abre no caminho novo).
- [ ] Atualizar bookmarks/atalhos pessoais do usuário.
- [ ] Atualizar settings do agente Claude se referenciam o path antigo.

---

## Fase 5 — Estabilizar a infra local

### Tarefa 5.1 — Renomear `Controlle10 - ftp/`

Subtarefas:
- [ ] Renomear para `erp-source-local/` (sem espaço, em inglês).
- [ ] Atualizar [docker-compose.yml:84](../docker-compose.yml) e [.gitignore](../.gitignore).
- [ ] Atualizar [docs/ERP_FTP_INGESTION.md](ERP_FTP_INGESTION.md) e [docs/ERP_CONSOLIDATED_PIPELINE.md](ERP_CONSOLIDATED_PIPELINE.md) se referenciam o nome antigo.
- [ ] Rodar `npm run dev:down && npm run dev` para validar.

### Tarefa 5.2 — Confirmar cobertura no `.gitignore`

Subtarefas:
- [ ] Adicionar `.playwright-mcp/` (se ainda não estiver).
- [ ] Adicionar `tmp/*.log` explicitamente, se ainda não estiver.
- [ ] Garantir `*.bak`, `*.old` cobertos.

### Tarefa 5.3 — Padronizar nomes de logs locais

Subtarefas:
- [ ] [web/.codex-devserver.*.log](../web/) — confirmar que estão no `.gitignore` global (já estão).
- [ ] [back/tmp_api_8081.log](../back/tmp_api_8081.log) — remover (ver Tarefa 1.4).

---

## Fase 6 — Qualidade & Padronização (linters, formatters, hooks)

> **Foco**: parar de aceitar código sem rede de proteção. Toda regressão de padrão deve ser bloqueada por máquina, não por reviewer cansado.

### Tarefa 6.1 — ESLint + Prettier no `web/`

Subtarefas:
- [ ] Adicionar `eslint`, `@nuxt/eslint`, `eslint-plugin-vue`, `eslint-plugin-unused-imports` em [web/package.json](../web/package.json).
- [ ] Criar `web/eslint.config.mjs` com flat config (Nuxt 4 padrão).
- [ ] Adicionar `prettier` + `eslint-config-prettier` para não conflitar.
- [ ] Criar `web/.prettierrc.json` com regras mínimas (2 spaces, single quotes ou double, trailing comma).
- [ ] Adicionar scripts `lint`, `lint:fix`, `format` em [web/package.json](../web/package.json).
- [ ] Rodar `npm --prefix web run lint:fix` 1 vez e fixar a baseline.
- [ ] Documentar em [web/AGENTS.md](../web/AGENTS.md).

### Tarefa 6.2 — `golangci-lint` no `back/`

Subtarefas:
- [ ] Criar [back/.golangci.yml](../back/) habilitando: `govet`, `staticcheck`, `errcheck`, `ineffassign`, `unused`, `gosimple`, `gocritic`, `gosec` (segurança).
- [ ] Rodar localmente, fixar baseline (anotar warnings legados com `//nolint:` apenas se justificado).
- [ ] Documentar em [back/AGENT.md](../back/AGENT.md).

### Tarefa 6.3 — Pre-commit hook

Subtarefas:
- [ ] Adicionar `husky` + `lint-staged` em [package.json](../package.json) raiz.
- [ ] Hook `pre-commit`: roda `eslint --fix` + `prettier --write` nos arquivos staged do web e `gofmt`/`goimports` nos do back.
- [ ] Hook opcional `commit-msg`: validar conventional commits (escopo: `chore:`, `feat:`, `fix:`, `refactor:`).
- [ ] Documentar em [AGENT.md](../AGENT.md).

### Tarefa 6.4 — `tsconfig.json` próprio no `web/`

Subtarefas:
- [ ] Criar [web/tsconfig.json](../web/) que estenda `./.nuxt/tsconfig.json`.
- [ ] Habilitar `strict: true`, `noUncheckedIndexedAccess: true`, `noImplicitOverride: true`.
- [ ] Rodar `vue-tsc --noEmit` e tratar os erros gerados (ou registrar como dívida em [BACKLOG.md](BACKLOG.md)).

### Tarefa 6.5 — Padronizar `AGENT.md` × `AGENTS.md`

Subtarefas:
- [ ] Escolher uma grafia única (recomenda-se `AGENT.md` por já ser maioria).
- [ ] Renomear os discrepantes (sobretudo no `web/`).
- [ ] Atualizar referências cruzadas.

### Tarefa 6.6 — Convenção de migration numbering

Subtarefas:
- [ ] Adotar prefixo único de 4 dígitos + slug obrigatório.
- [ ] Resolver as 4 colisões atuais (`0015a`+`0015`, `0019_*` duplos, `0031_*` duplos, `0039_*` duplos) — se a ordem real diverge da intenção, renomear.
- [ ] Documentar em [back/database/AGENT.md](../back/database/AGENT.md).

### Tarefa 6.7 — Decidir entre `features/` e `components/`

Subtarefas:
- [ ] Discutir convenção: feature-first (`features/<dominio>/`) ou domain-first (`components/<dominio>/`).
- [ ] Migrar os arquivos no padrão escolhido.
- [ ] Atualizar [web/AGENTS.md](../web/AGENTS.md).

---

## Fase 7 — Fatiamento de arquivos gigantes

> **Foco**: nenhum arquivo Vue/TS acima de **500 linhas**, nenhum arquivo Go acima de **800 linhas**. Esses limites são arbitrários mas defensáveis: acima disso, o blast radius de qualquer alteração cresce demais.
>
> Métricas baseline (2026-05-18):
> - **Vue**: 7 arquivos > 1.000 linhas (crítico) · 12 entre 700-1.000 (alto)
> - **TS front**: 2 arquivos > 1.400 linhas (crítico)
> - **Go**: 8 arquivos > 1.000 linhas (crítico)

### Tarefa 7.1 — Top 3 Vue críticos

Subtarefas:
- [ ] Fatiar [UsersAccessManager.vue](../web/app/components/users/UsersAccessManager.vue) (2.187 linhas):
  - [ ] Extrair os 20+ helpers do `<script>` para `web/app/domain/utils/user-access.ts`.
  - [ ] Extrair drafts (`createRowDraft`, `createDetailDraft`, etc.) para composable `useUserAccessDrafts.ts`.
  - [ ] Dividir o template grande em 3-4 sub-componentes (`UsersAccessTable`, `UsersAccessDetailDrawer`, `UsersAccessCreateModal`).
- [ ] Fatiar [OperationFinishModal.vue](../web/app/features/operation/components/OperationFinishModal.vue) (2.143 linhas):
  - [ ] Extrair cada passo do wizard em sub-componente (`FinishStepClient`, `FinishStepProduct`, `FinishStepOutcome`).
  - [ ] Extrair regra de validação para `web/app/domain/utils/finish-modal.ts`.
- [ ] Fatiar [layers/tasks/pages/tasks.vue](../web/layers/tasks/pages/tasks.vue) (1.340 linhas):
  - [ ] Mover lógica de filtros/board para composables/sub-components dedicados.

### Tarefa 7.2 — Workspaces inflados

Subtarefas:
- [ ] [FeedbackWorkspace.vue](../web/app/components/feedback/FeedbackWorkspace.vue) (1.297) → dividir em `FeedbackList`, `FeedbackFilters`, `FeedbackDetailPanel`.
- [ ] [SettingsWorkspace.vue](../web/app/components/settings/SettingsWorkspace.vue) (1.282) → mover seções para sub-components em `web/app/components/settings/sections/`.
- [ ] [ErpWorkspace.vue](../web/app/components/erp/ErpWorkspace.vue) (1.230) → dividir por aba (`ErpStatusTab`, `ErpProductsTab`, `ErpRunsTab`).
- [ ] [ThemeColorInput.vue](../web/layers/core/components/theme/ThemeColorInput.vue) (1.007) → extrair color picker para `ThemeColorPicker.vue`.

### Tarefa 7.3 — Composables e stores monolíticos

Subtarefas:
- [ ] [useTasksPageContext.ts](../web/layers/tasks/composables/useTasksPageContext.ts) (**1.737 linhas, 1 export**) — fatiar em composables menores (`useTasksFilters`, `useTasksColumns`, `useTasksDrafts`, `useTasksRealtimePresence`).
- [ ] [layers/tasks/stores/tasks.ts](../web/layers/tasks/stores/tasks.ts) (1.486) — separar normalizers em `stores/tasks/normalizers.ts`, actions em `stores/tasks/actions/`.
- [ ] [stores/erp.ts](../web/app/stores/erp.ts) (762), [stores/auth.ts](../web/app/stores/auth.ts) (629), [stores/alerts.ts](../web/app/stores/alerts.ts) (628), [stores/operations.ts](../web/app/stores/operations.ts) (619), [stores/settings.ts](../web/app/stores/settings.ts) (615) — aplicar o mesmo padrão usado em `stores/dashboard/runtime/` (state + actions/* + shared).

### Tarefa 7.4 — Backend Go acima de 1000 linhas

Subtarefas:
- [ ] [erp/repository_postgres.go](../back/internal/modules/erp/repository_postgres.go) (**2.942 linhas, 58 funções**) — separar helpers CRM em `repository_crm.go`, agregadores em `repository_aggregates.go`.
- [ ] [operations/service.go](../back/internal/modules/operations/service.go) (1.975) — separar fluxos (`service_queue.go`, `service_pause.go`, `service_finish.go`).
- [ ] [alerts/store_postgres.go](../back/internal/modules/alerts/store_postgres.go) (1.466), [tasks/repository_postgres.go](../back/internal/modules/tasks/repository_postgres.go) (1.402), [settings/service.go](../back/internal/modules/settings/service.go) (1.320), [erp/service.go](../back/internal/modules/erp/service.go) (1.259), [reports/service.go](../back/internal/modules/reports/service.go) (1.215), [analytics/service.go](../back/internal/modules/analytics/service.go) (1.003) — mesma abordagem.
- [ ] Rodar `go test ./...` e `golangci-lint run` após cada fatiamento.

### Tarefa 7.5 — Definir limite e validar via lint

Subtarefas:
- [ ] Adicionar regra ESLint `max-lines` com `{ max: 500, skipBlankLines: true, skipComments: true }` no `web/`.
- [ ] Adicionar `gocyclo` no `.golangci.yml` (complexidade ciclomática) com threshold 15.
- [ ] Documentar exceções legítimas (se houver) com comentário explicando.

---

## Fase 8 — Performance, segurança e documentação inline

### Tarefa 8.1 — Lazy loading no front

Subtarefas:
- [ ] Substituir import síncrono de modais pesados por `defineAsyncComponent`:
  - [ ] [OperationFinishModal.vue](../web/app/features/operation/components/OperationFinishModal.vue) (2.143 linhas) — só carrega quando o usuário clica em "Finalizar atendimento".
  - [ ] [UsersAccessManager.vue](../web/app/components/users/UsersAccessManager.vue) (2.187) e demais workspaces de admin.
- [ ] Adicionar `<Suspense>` com fallback `CoreLoadingOverlay` nos lazy components.
- [ ] Medir bundle antes/depois com `vite-bundle-visualizer` (subtarefa 8.4).

### Tarefa 8.2 — Análise de bundle do front

Subtarefas:
- [ ] Instalar `rollup-plugin-visualizer` ou `vite-bundle-visualizer`.
- [ ] Rodar `npm --prefix web run build` e gerar relatório.
- [ ] Identificar top 10 dependências por tamanho (TipTap costuma ser top).
- [ ] Documentar baseline em [docs/historico/bundle-baseline-2026-05.md](historico/bundle-baseline-2026-05.md) (nome com data).

### Tarefa 8.3 — N+1 e cache HTTP no back

Subtarefas:
- [ ] Identificar com `EXPLAIN ANALYZE` 3 queries mais quentes (provavelmente em `erp` e `operations`).
- [ ] Refatorar queries N+1 evidentes para batch (ex.: `WHERE id IN (...)`).
- [ ] Avaliar `ETag` + `If-None-Match` em endpoints de leitura grandes (relatórios).
- [ ] Avaliar índices faltantes (auditar [back/internal/platform/database/migrations/0107_perf_hotpath_stats.sql](../back/internal/platform/database/migrations/) como referência).

### Tarefa 8.4 — HTTP security headers em prod

Subtarefas:
- [ ] Definir matriz: `Strict-Transport-Security`, `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`, `Permissions-Policy`, `Content-Security-Policy`.
- [ ] Aplicar no proxy Caddy da VPS (preferível) — atualizar [docs/DEPLOY_VPS.md](DEPLOY_VPS.md) com o snippet.
- [ ] Validar com [securityheaders.com](https://securityheaders.com/) após deploy.

### Tarefa 8.5 — Auditoria de dependências em CI

Subtarefas:
- [ ] Adicionar step `npm audit --audit-level=high` em [.github/workflows/deploy-vps.yml](../.github/workflows/deploy-vps.yml).
- [ ] Adicionar step `go list -m -u all` + `govulncheck ./...` no mesmo workflow.
- [ ] Decidir política para vulnerabilidades found (block vs warn).

### Tarefa 8.6 — Endurecer defaults de dev

Subtarefas:
- [ ] [docker-compose.yml:41](../docker-compose.yml) — `AUTH_TOKEN_SECRET` default `dev-secret-change-me`: mudar para gerar erro se não vier do `.env` em produção (já tem `.env.production.example` correto, mas reforçar no `config.go`).
- [ ] Adicionar guard em [back/internal/platform/config/config.go](../back/internal/platform/config/config.go): se `APP_ENV=production` e secret de auth ainda for dev, abortar boot.

### Tarefa 8.7 — Documentação inline

Subtarefas:
- [ ] Adicionar godoc comments em funções exportadas dos módulos `core`, `auth`, `operations`, `settings` (os mais sensíveis).
- [ ] Adicionar TSDoc nos composables de qualquer layer (`useNav`, `usePermission`, `useOmniTheme`, `useTasksPageContext`).
- [ ] Criar `docs/adr/` com primeiro ADR: ADR-0001 — decisão do nome `Omni` e renomeação do DB.

### Tarefa 8.8 — Atualizar `COMPONENT_INVENTORY.md`

Subtarefas:
- [ ] Criar `scripts/dev/gen-component-inventory.mjs` que varre `web/` e gera o inventário.
- [ ] Rodar e substituir [docs/COMPONENT_INVENTORY.md](COMPONENT_INVENTORY.md).
- [ ] Adicionar ao CI (opcional): falhar se houver drift entre o inventário e o repo.

---

## Fase 9 — Testes do frontend

### Tarefa 9.1 — Setup mínimo

Subtarefas:
- [ ] Confirmar `vitest` rodando: `npm --prefix web run test`.
- [ ] 1 teste por store crítica: `auth`, `operations`, `settings`, `tasks`.
- [ ] 1 teste por composable de realtime: `useOperationsRealtime`, `useContextRealtime`, `useTasksRealtime`.
- [ ] 1 teste por util de domínio: `permissions`, `campaigns`, `reports`.

### Tarefa 9.2 — CI

Subtarefas:
- [ ] Adicionar step `vitest` em [.github/workflows/deploy-vps.yml](../.github/workflows/deploy-vps.yml) (ou criar workflow dedicado `test.yml` que dispare em PR).
- [ ] Adicionar step `go test ./...` no mesmo workflow.
- [ ] Adicionar coverage report (opcional).

---

## Fora de escopo deste plano (vai para o ROADMAP)

Estas decisões pertencem a [ROADMAP.md](ROADMAP.md) e seguem cadências próprias:

- Migração do layer `queue` (Fase 4 do roadmap).
- Cisão de `queue` × `crm` (Fase 8 do roadmap).
- Importação de `finance`, `tasks`, `omni` como módulos satélites (Fase 6 do roadmap).
- Module Registry frontend completo (Fase 5 do roadmap, em progresso conforme commits recentes).

Este plano (PLANO_REFATORACAO.md) é só para **higienização e endurecimento do que já existe**.

---

## Validação ao final

Antes de marcar este plano como concluído:

**Build & infra**
- [ ] `npm --prefix web run build` passa sem erro.
- [ ] `npm --prefix web run lint` passa (após Fase 6).
- [ ] `go test ./...` em `back/` passa.
- [ ] `golangci-lint run` em `back/` passa (após Fase 6).
- [ ] `docker compose config` e `docker compose -f docker-compose.prod.yml config` retornam OK.
- [ ] `npm run dev` sobe e responde nas 3 portas (`5432`, `8080`, `3003`).

**Renomeação para Omni**
- [ ] `http://localhost:3003` mostra `<title>Omni</title>` no `<head>`.
- [ ] `docker compose exec api env | grep APP_NAME` retorna `APP_NAME=omni-api`.
- [ ] `docker compose exec postgres psql -U omni -d omni -c '\l'` lista o banco `omni`.
- [ ] `grep -ri "lista da vez\|lista-da-vez\|fila de atendimento" .` (excluindo `docs/historico/`, `node_modules/`, `.git/`, `web-reference/`, `Controlle10*`) retorna **0 ocorrências**.

**Organização**
- [ ] Lista de arquivos da raiz tem **≤ 12 itens**.
- [ ] Nenhum arquivo `.vue`/`.ts` no `web/` acima de **500 linhas** (lint `max-lines`).
- [ ] Nenhum arquivo `.go` acima de **800 linhas**.
- [ ] [ESTADO_ATUAL.md](ESTADO_ATUAL.md) atualizado com o novo retrato pós-refatoração.

**Qualidade & segurança**
- [ ] ESLint, Prettier, `golangci-lint` configurados e rodando.
- [ ] Pre-commit hook ativo (Husky + lint-staged).
- [ ] `npm audit --audit-level=high` e `govulncheck ./...` rodando no CI.
- [ ] HTTP security headers aplicados no Caddy de produção.
- [ ] Boot do back aborta se `APP_ENV=production` com secret default.
- [ ] Pelo menos 4 testes de store e 3 de composable no front.
