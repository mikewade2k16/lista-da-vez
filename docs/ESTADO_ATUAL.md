# Estado Atual do Projeto

> Data da análise: 2026-05-16 · atualizado 2026-05-18
> Branch analisada: `refactor/multi-tenant-core`
> Diretório analisado: `c:\Users\Mike\Documents\Projects\fila-atendimento`
> Escopo: leitura completa de raiz, `back/`, `web/`, `docs/`, `docs_depoy/`, `scripts/`, infra Docker, env templates. Análise de código morto front + back. Mapeamento de duplicações e inconsistências.

**Versão visual com gráficos:** [estado-atual.html](estado-atual.html)

Este documento é o retrato fiel do projeto em 16/05/2026 antes do trabalho de reorganização. O plano operacional de quem-faz-o-quê está em [PLANO_REFATORACAO.md](PLANO_REFATORACAO.md) (visual: [plano-refatoracao.html](plano-refatoracao.html)).

## 0. Decisões fechadas (16/05/2026)

| Decisão | Valor |
|---|---|
| **Nome oficial do produto** | `Omni` (display) / `omni` (slug técnico). Substitui `lista-da-vez`, `listaatendimento` e `Fila de Atendimento`. |
| **Renomear DB em prod** | Sim, padronizar com `omni`. Exige janela de manutenção (`ALTER DATABASE` ou dump/restore). |
| **Layer `web/layers/queue/`** | Deixar como placeholder e documentar via `AGENT.md` próprio. Migração concreta entra na Fase 4 do [ROADMAP.md](ROADMAP.md). |

**Notas sobre `omni`:**
- O nome já está difuso pelo código: [docs/BACKLOG.md](BACKLOG.md) o usa, `omni-design-system.css`, `omni-tokens.css`, `OmniEditor.vue`, `OmniDataTable.vue`, `useOmniTheme`, `acesso.omni.local`. A renomeação só formaliza o que o time já vinha usando.
- **NÃO confundir** com:
  - `omnichannel` — nome do módulo/página de chat (`/omnichannel`). Continua existindo como módulo distinto.
  - `omnichannel-mvp` / `omnichannel-mvp_default` — nome da stack VPS de OUTRO projeto que compartilha rede Docker em produção. Não muda.

---

## 1. Sumário executivo

| Aspecto | Avaliação | Observação |
|---|---|---|
| Arquitetura backend | Saudável | 19 módulos Go bem isolados, AGENT.md por módulo, `go vet` limpo. |
| Arquitetura frontend | Em transição | Nuxt 4 com 3 layers (`core`, `queue`, `tasks`) — `queue` ainda é esqueleto. |
| Cobertura de testes | Parcial | Back tem testes em pontos críticos. Front praticamente sem testes (1 utilitário). |
| Documentação | Densa e fragmentada | 19 docs em `docs/` + 3 em `docs_depoy/` (typo) + 9 .md soltos na raiz. |
| Organização da raiz | **Bagunçada** | 35 arquivos soltos na raiz: tokens, screenshots, TODOs antigos, planos concluídos, HTML de teste. |
| Nomenclatura | **Inconsistente** | Convivem 3 nomes: `fila-atendimento` (pasta), `lista-da-vez` (compose/package) e "Fila de Atendimento" (UI/README). |
| Código morto | Baixo, identificável | 3 funções Go mortas + 2 componentes Vue órfãos + 1 layer Nuxt esqueleto. Plus pasta `back/cmd/debuginvite/` vazia. |
| Infra Docker | Funcional | `docker-compose.yml` (dev) + `docker-compose.prod.yml` (prod). Templates `.env.docker.example` e `.env.production.example`. |

**Veredicto**: a base de código é sólida; o problema é organizacional. A maior parte da dor está na raiz do repositório (arquivos de trabalho que viraram entulho) e em divergências históricas de nomenclatura. Refatoração é principalmente de **limpeza, mover-arquivar-renomear**, não de reescrita.

---

## 2. Inventário da raiz do repositório

> **Atualização 2026-05-16**: a Fase 1 do plano foi executada. As tabelas abaixo refletem o **estado original** (antes da limpeza) e servem como histórico do "antes". Para o estado pós-limpeza, ver `git status` ou rodar `ls` na raiz.

A raiz **antes da limpeza** tinha 35 arquivos + 17 pastas. Boa parte dos arquivos solto era entulho de sessões antigas.

### 2.1 Arquivos que pertencem à raiz (manter)

| Arquivo | Função |
|---|---|
| [README.md](../README.md) | Documento de entrada. |
| [AGENT.md](../AGENT.md) | Regras gerais para o agente. |
| [package.json](../package.json) | Scripts agregadores (npm run dev → docker compose). |
| [docker-compose.yml](../docker-compose.yml) | Stack dev. |
| [docker-compose.prod.yml](../docker-compose.prod.yml) | Stack prod. |
| [.env.docker.example](../.env.docker.example) | Template de env para o compose dev. |
| [.env.production.example](../.env.production.example) | Template de env para o compose prod. |
| [.gitignore](../.gitignore) | OK, já cobre tokens e logos locais. |

### 2.2 Arquivos `.md` soltos que deveriam ser arquivados, removidos ou movidos

| Arquivo | Tamanho | Situação real | Recomendação |
|---|---|---|---|
| [diagnostico.md](../diagnostico.md) | 7 KB | Transcrição da reunião de 23/03/2026. Histórico. | Mover para `docs/historico/diagnostico-2026-03-23.md`. |
| [dev-compose-perfis.md](../dev-compose-perfis.md) | 2 KB | **Desatualizado**. Menciona `docker-compose.dev.yml` (não existe), serviços `painel-web`, `plataforma-api`, `redis` (não existem no nosso compose). Veio de outro projeto. | Remover. |
| [ERP-auto.md](../ERP-auto.md) | 14 KB | Snapshot de estado da automação ERP em 05/05/2026. | Mover para `docs/historico/erp-auto-2026-05-05.md`. |
| [PLANO_PROGRESSO.md](../PLANO_PROGRESSO.md) | 13 KB | Plano de alertas marcado como ✅ COMPLETO em 02/05/2026. | Mover para `docs/historico/plano-alertas-concluido.md`. |
| [TESTE_ALERTAS.md](../TESTE_ALERTAS.md) | 4 KB | Guia de teste do plano acima. | Mover para `docs/historico/` junto. |
| [todo.md](../todo.md) | 42 KB | TODO antigo da refatoração de Settings (com itens já concluídos). | Avaliar item-a-item e mover o vivo para `docs/BACKLOG.md`. O restante vai para `docs/historico/`. |
| [todo-alertas.md](../todo-alertas.md) | 39 KB | Continuação do plano de alertas (concluído). | Mover para `docs/historico/`. |
| [todo-reuniao.md](../todo-reuniao.md) | 12 KB | Checklist 23/03/2026 da reunião acima. | Mover para `docs/historico/`. |
| ~~`roadmap.md`~~ → [docs/ROADMAP.md](ROADMAP.md) | 63 KB | Plano vivo da plataforma multi-tenant. **Documento mestre da reestruturação atual**. Movido para `docs/` em 2026-05-16. |

### 2.3 Binários, screenshots e tokens soltos

| Arquivo | Status | Recomendação |
|---|---|---|
| [logo.png](../logo.png), [logo.webp](../logo.webp), [logo.avif](../logo.avif) | Já existem cópias em [web/public/](../web/public/). Duplicação na raiz é desnecessária. | Remover da raiz. Fonte de verdade em `web/public/`. |
| [editor-page-check.png](../editor-page-check.png) | Screenshot avulso de debug (13/05/2026). | Remover (ou mover para `tmp/` se ainda útil). |
| [tasks-3004.png](../tasks-3004.png) | Screenshot avulso (12/05/2026). | Remover. |
| [gif-indeva.gif](../gif-indeva.gif) | 293 KB. Sem referência. | Remover. |
| [test-perola-api.html](../test-perola-api.html) | HTML standalone de teste manual de API externa. | Mover para `scripts/manual-tests/` ou remover. |
| [token.txt](../token.txt), [token_gen.js](../token_gen.js), [token_gen_real.js](../token_gen_real.js), [gen_token.js](../gen_token.js), [full_token.txt](../full_token.txt), [payload.b64](../payload.b64), [secret.key](../secret.key), [verify.sh](../verify.sh) | Já estão no `.gitignore`. São ferramentas pessoais locais. | Mover para `tmp/` ou `scripts/dev/token-helpers/`. Não é bom poluir a raiz. |

### 2.4 Pastas a revisar

| Pasta | Tamanho local | Situação | Recomendação |
|---|---|---|---|
| [Controlle10 - ftp/](../Controlle10%20-%20ftp/) | **493 MB** | Dados locais do ERP para teste, montados no container (`docker-compose.yml:84`). Já está no `.gitignore`. | Renomear para `erp-source-local/` (sem espaço, sem ambiguidade). Atualizar o volume no compose. |
| [tmp/](../tmp/) | 0,5 MB | 22 arquivos de log antigos (29/03 a 30/04/2026) + 1 script Python + 1 .go de utilidade. | Limpar conteúdo. Garantir `tmp/*` no `.gitignore` (já está coberto indiretamente? confirmar). |
| [docs_depoy/](../docs_depoy/) | 10 KB | **Typo** ("depoy" → "deploy"). 3 .md, sendo um declarado como "arquivo arquivado, não se aplica a este repositório". | Migrar conteúdo vivo para `docs/deploy/`. Remover a pasta. |
| [.codex-logs/](../.codex-logs/) | < 1 KB | Logs do agente Codex. Já no `.gitignore`. | Manter, mas opcionalmente mover para `.tmp/codex/`. |
| [.playwright-mcp/](../.playwright-mcp/) | 2,1 MB | Snapshots de páginas da MCP do Playwright. | Adicionar ao `.gitignore` se ainda não está e considerar limpar. |
| [.claude/](../.claude/) | < 1 KB | Settings locais do agente. Já no `.gitignore`. | Manter. |
| [qa-bot/](../qa-bot/) | 116 MB (com `.venv`) | Bot Python (Playwright + cenários YAML) para smoke E2E. 20 arquivos próprios + `.venv`. | Manter. `qa-bot/.venv` já está no `.gitignore`. |
| [web-reference/](../web-reference/) | 414 MB | Frontend de outro projeto, trazido só para referência. Já no `.gitignore`. | Manter localmente enquanto for útil; documentar que **nada** desta pasta vai a produção. |

---

## 3. Backend Go (`back/`)

### 3.1 Layout

```
back/
├── cmd/
│   ├── api/main.go            # Entrypoint do servidor HTTP
│   ├── migrate/main.go        # CLI de migrations
│   └── debuginvite/           # ⚠ Pasta vazia (resíduo)
├── database/
│   ├── AGENT.md
│   └── ERD.md                 # Diagrama Mermaid
├── internal/
│   ├── modules/               # 19 módulos de domínio
│   └── platform/              # Infra compartilhada
├── scripts/
│   ├── api/                   # Scripts PowerShell de start/stop/status (fallback local)
│   └── postgres/              # Scripts PowerShell de PostgreSQL local
├── AGENT.md
├── CORE_MODULES_PORTABILITY.md
├── Dockerfile
├── go.mod, go.sum
├── PLAN.md
├── README.md
├── START_LOCAL.md
└── tmp_api_8081.log           # ⚠ Log antigo na raiz
```

### 3.2 Módulos de domínio em [back/internal/modules/](../back/internal/modules/)

19 módulos, todos com `AGENT.md` próprio:

| Módulo | Responsabilidade |
|---|---|
| `access` | RBAC + permissões + overrides por usuário. |
| `alerts` | Regras e instâncias de alertas operacionais. |
| `analytics` | Dashboards de ranking, dados e inteligência. |
| `auth` | Login, JWT, convites, reset de senha, store em memória + Postgres. |
| `catalog` | Catálogo manual de produtos e source registry. |
| `consultants` | Roster operacional e sincronização de perfis. |
| `core` | Schema novo `core` da reestruturação multi-tenant (RBAC v2). |
| `erp` | Ingestão de CSVs ERP (FTP/local), parser, resolver. |
| `feedback` | Mensagens e leitura de feedback de usuários. |
| `notifications` | Adapters de e-mail, in-app, push, WhatsApp. |
| `operations` | Atendimento, fila, pausa, finish modal, alertas. |
| `realtime` | Hub WebSocket, presença, eventos por tópico. |
| `reports` | Endpoints de relatórios (incl. comparativo multi-loja). |
| `settings` | Configurações de tenant (operação, modal, alertas). |
| `stores` | CRUD de lojas. |
| `tasks` | Schema novo `tasks` (board, time tracking, relations). |
| `tenants` | CRUD de tenants/contas. |
| `users` | Usuários globais + memberships. |

### 3.3 Infra compartilhada [back/internal/platform/](../back/internal/platform/)

```
platform/
├── app/             # Bootstrap (app.go), HTTP context, adapters
├── config/          # Leitura de env (AppName default: "lista-da-vez-api")
├── database/        # Pool, migrator, bootstrap de owner/erp_store
│   └── migrations/  # 75 arquivos .sql (0001 a 0111)
├── events/          # Event bus interno
├── httpapi/         # Middleware, account guard, rate limit
├── modules/         # Module Registry (catalog Postgres, registry, relations)
└── server/          # HTTP server
```

### 3.4 Migrations

75 migrations entre `0001_init.sql` e `0111_users_nick.sql`. Numeração com salto deliberado:

- `0001`–`0059`: schema legado (`public.*`).
- `0100`–`0111`: schema novo da reestruturação multi-tenant (`core.*`, `tasks.*`).

Há colisões históricas de prefixo (ex.: `0015a_…` + `0015_…`, `0019_user_password_reset…` + `0019_workspace_access_matrix…`, `0031_active_service_store…` + `0031_per_consultant_concurrent…`, `0039_erp_store_184_products…` + `0039_finish_modal_purchase…`). Funciona porque o migrator ordena por nome completo, mas é frágil — convencionar prefixo único nas próximas seria mais seguro.

### 3.5 Testes Go

Cobertura amostral:

- `access`: `permissions_test.go`, `service_realtime_test.go`
- `auth`: `roles_test.go`
- `alerts`: `service_test.go`
- `catalog`: `service_test.go`
- `erp`: `crm_test.go`, `csv_parser_test.go`, `service_test.go`, `source_ftp_test.go`, `source_local_test.go`
- `feedback`: `service_test.go`
- `operations`: `access_test.go`, `service_alerts_test.go`, `service_parallel_test.go`
- `realtime`: `presence_test.go`
- `settings`: `http_test.go`, `service_test.go`, `store_postgres_test.go`
- `stores`: `service_test.go`
- `tasks`: 6 arquivos `_test.go`
- `users`: `service_test.go`
- `platform/app`: `app_test.go`
- `platform/httpapi`: `rate_limit_test.go`

Módulos **sem** teste: `analytics`, `consultants`, `core`, `notifications`, `reports`, `tenants`.

### 3.6 Documentos internos do backend

- [back/PLAN.md](../back/PLAN.md) — plano original do backend.
- [back/CORE_MODULES_PORTABILITY.md](../back/CORE_MODULES_PORTABILITY.md) — guia de extração futura para microserviços.
- [back/START_LOCAL.md](../back/START_LOCAL.md) — fallback sem Docker.
- [back/database/ERD.md](../back/database/ERD.md) — diagrama Mermaid do banco.
- [back/internal/modules/settings/SETTINGS_REFACTOR_PLAN.md](../back/internal/modules/settings/SETTINGS_REFACTOR_PLAN.md) — refator interno do settings.
- [back/internal/modules/settings/PHASE1_SMOKE_GUIDE.md](../back/internal/modules/settings/PHASE1_SMOKE_GUIDE.md).
- [back/internal/modules/operations/CONCURRENT_SERVICES.md](../back/internal/modules/operations/CONCURRENT_SERVICES.md).

---

## 4. Frontend Nuxt (`web/`)

### 4.1 Layout

```
web/
├── app/                    # Shell principal (Nuxt 4)
│   ├── assets/styles/      # CSS local
│   ├── components/         # ~63 .vue por domínio
│   ├── composables/        # 4 composables de shell
│   ├── domain/             # Funções puras + dados/seed
│   │   ├── data/
│   │   └── utils/
│   ├── features/           # Domínios "feature-first" (só operation hoje)
│   ├── layouts/            # auth.vue, dashboard.vue
│   ├── middleware/         # auth.global.ts
│   ├── pages/              # Rotas (32 .vue + 1 .md errante)
│   ├── plugins/            # 2 plugins client
│   ├── stores/             # 19 stores Pinia (com runtime fatiado em dashboard/)
│   └── utils/              # Helpers de app
├── layers/
│   ├── core/               # ✅ Layer ativo (admin, theme, account, loading)
│   ├── queue/              # ⚠ Esqueleto: só nav.config.ts + nuxt.config.ts vazio
│   └── tasks/              # ✅ Layer ativo (todo o módulo Tasks)
├── public/                 # logo.* + erp-agent.md
├── scripts/                # ensure-node-modules.mjs
├── nuxt.config.ts          # Layers, head, CSS, runtimeConfig
├── package.json            # name: "lista-da-vez-web"
├── vitest.config.ts
├── dist/                   # ⚠ Build artifact local (.gitignore raiz cobre, mas ocupa espaço)
├── .codex-devserver.*.log  # ⚠ Logs locais (no .gitignore)
└── PANEL_EMBED_CONTRACT.md
```

### 4.2 Páginas Nuxt em [web/app/pages/](../web/app/pages/)

32 páginas + um arquivo errante. As rotas que o `nuxt.config.ts` declara como `ssr: false`:

```
/, /alertas, /banco, /campanhas, /clientes, /configuracoes, /consultor,
/crm, /dados, /erp, /feedback, /finance, /inteligencia, /meus-feedbacks,
/monitoramento, /multiloja, /omnichannel, /perfil, /ranking, /relatorios,
/roadmap, /tracking, /tasks, /usuarios,
/auth/{login, esqueceu-senha, convite/[token]},
/manage/[area], /operacao/index, /site/[area], /team/[area], /tools/[tool]
```

Detalhe importante: [web/app/pages/operacao/operations.md](../web/app/pages/operacao/operations.md) — arquivo `.md` **dentro de `pages/`** é confuso (Nuxt ignora, mas o local errado polui). Deveria mover para `docs/`.

### 4.3 Componentes em [web/app/components/](../web/app/components/)

~63 `.vue` agrupados por domínio: `alerts/`, `banco/`, `campaigns/`, `consultant/`, `crm/`, `dashboard/`, `data/`, `demo/`, `erp/`, `feedback/`, `intelligence/`, `layout/`, `multistore/`, `omni/`, `ranking/`, `reports/`, `roadmap/`, `settings/`, `tenants/`, `ui/`, `users/`.

### 4.4 Features em [web/app/features/operation/](../web/app/features/operation/)

15 componentes específicos da operação (`AlertDisplay*`, `Operation*`). É a única pasta `features/` — todo o resto vive em `components/`. **Inconsistência**: parte do produto adotou "feature-first", parte ficou em "domain-first". Decidir um padrão único e migrar.

### 4.5 Stores Pinia em [web/app/stores/](../web/app/stores/)

19 arquivos no nível raiz + runtime fatiado em [web/app/stores/dashboard/runtime/](../web/app/stores/dashboard/runtime/) (`shared.ts`, `state.ts`, `status.ts`, `create-dashboard-runtime.ts`, `actions/{consultant,operation,settings,workspace}-actions.ts`).

Stores: `access-control`, `alerts`, `analytics`, `app-runtime`, `auth`, `campaigns`, `consultants`, `crm`, `dashboard` (facade), `erp`, `feedback`, `multistore`, `nav`, `operations`, `reports`, `settings`, `tenants`, `ui`, `users`, `workspace`.

### 4.6 Layers

**`core`** — layer base ativo:
- Components: `CoreAccountSwitcher`, `CoreEmptyState`, `CoreErrorState`, `CoreLoadingOverlay`, `CorePermissionGate`, `CoreSkeleton`, `admin/AdminPageHeader`, `theme/*`.
- Composables: `useAdminPageHeaderVisibility`, `useCoreLoading`, `useNav`, `useOmniTheme`, `usePermission`, `useThemeStudio`.
- Pages: `themes.vue`.
- Plugins: `omni-theme.client.ts`.
- Stores: `account.ts`, `loading.ts`.

**`queue`** — **layer esqueleto**:
- Apenas `nav.config.ts` (define todo o menu do produto) e `nuxt.config.ts` vazio.
- Nenhum componente, nenhuma página, nenhuma store. **Intenção é mover o produto fila para cá** (Fase 4 do roadmap), mas a migração não começou de fato.

**`tasks`** — layer ativo:
- Pages: `tasks.vue`, `editor.vue`, `tracking.vue`.
- Components: `TasksBoardView`, `TasksFilterBar`, `TasksProjectSettings`, `TasksTableView`, `TasksTaskModal`, `AppDatePicker`, `admin/AdminPageHeader` (**duplicado** com core), `editor/TasksRichEditor`, `inputs/OmniSelectInput`, `inputs/OmniSelectMenuInput`, `omni/inputs/OmniMoneyInput`, `omni/inputs/OmniSwitchInput`, `omni/table/OmniDataTable`.
- Composables: `useCan`, `useDateFormat`, `useTaskPresence`, `useTaskRelations`, `useTasksPageContext`, `useTasksRealtime`, `useTasksWorkspace`, `useTimeTracking`.
- Stores: `tasks.ts`, `session-simulation.ts`.
- Types: `tasks.ts`, `omni/collection.ts`.
- Utils: `text.ts` (+ test).

### 4.7 Testes frontend

Praticamente inexistentes: **1 arquivo** [web/layers/tasks/utils/text.test.ts](../web/layers/tasks/utils/text.test.ts). `vitest` configurado mas subutilizado.

---

## 5. Documentação (`docs/` e `docs_depoy/`)

### 5.1 [docs/](../docs/)

19 documentos:

| Documento | Função |
|---|---|
| [BACKLOG.md](BACKLOG.md) | Backlog de produto. |
| [CAMPANHAS_CORRIDINHAS_RULES.md](CAMPANHAS_CORRIDINHAS_RULES.md) | Regras de campanhas. |
| [COMPONENT_INVENTORY.md](COMPONENT_INVENTORY.md) | Inventário (provavelmente desatualizado). |
| [CONTRACT_FREEZE.md](CONTRACT_FREEZE.md) | Contratos invariantes. |
| [DEPLOY_VPS.md](DEPLOY_VPS.md) | Documento oficial de deploy. |
| [ERP_CONSOLIDATED_PIPELINE.md](ERP_CONSOLIDATED_PIPELINE.md) | Pipeline ERP. |
| [ERP_CRM_STORE_ATTRIBUTION.md](ERP_CRM_STORE_ATTRIBUTION.md) | Atribuição loja CRM. |
| [ERP_FTP_INGESTION.md](ERP_FTP_INGESTION.md) | Ingestão FTP. |
| [GUIA_TREINAMENTO_PAPEIS.md](GUIA_TREINAMENTO_PAPEIS.md) | Treinamento por papel. |
| [NUXT_4_STORE_ARCHITECTURE.md](NUXT_4_STORE_ARCHITECTURE.md) | Arquitetura de stores. |
| [NUXT_FULL_REFERENCE.md](NUXT_FULL_REFERENCE.md) | Referência Nuxt. |
| [NUXT_MIGRATION_BLUEPRINT.md](NUXT_MIGRATION_BLUEPRINT.md) | Plano da migração ao Nuxt. |
| [OPERATION_DOCKER_BUG_LOG.md](OPERATION_DOCKER_BUG_LOG.md) | Log de bug específico. |
| [OPERATIONS_ALERTS_TIMER_FLOW.md](OPERATIONS_ALERTS_TIMER_FLOW.md) | Fluxo de timers de alerta. |
| [plan-feedback-5.2.1.md](plan-feedback-5.2.1.md) | Plano de feedback (versionado). |
| [review-feedback.md](review-feedback.md) | Notas de review. |
| [SCHEMA_TARGET.md](SCHEMA_TARGET.md) | Schema-alvo. |
| [tasks-orquestrador-plano.html](tasks-orquestrador-plano.html) | 100 KB de HTML estático (visual). |
| [TASKS_ORCHESTRATOR_PHASE12.md](TASKS_ORCHESTRATOR_PHASE12.md) | Plano de Tasks fase 12. |

### 5.2 [docs_depoy/](../docs_depoy/)

3 documentos com **typo na pasta** (`depoy` → `deploy`):

| Documento | Status |
|---|---|
| [docs_depoy/deploy-vps.md](../docs_depoy/deploy-vps.md) | Auto-declarado "arquivo arquivado, não se aplica a este repo". Remete a `docs/DEPLOY_VPS.md`. |
| [docs_depoy/deploy-main-vps-auto.md](../docs_depoy/deploy-main-vps-auto.md) | Verificar. |
| [docs_depoy/deploy-producao-checklist.md](../docs_depoy/deploy-producao-checklist.md) | Provável duplicação com [DEPLOY_VPS.md](DEPLOY_VPS.md). |

---

## 6. Infraestrutura, Docker e env

### 6.1 Docker Compose

- [docker-compose.yml](../docker-compose.yml) — dev. Nome do projeto: `lista-da-vez`. Serviços: `postgres`, `api`, `web`. Bind mount de `./web` no container web, montagem read-only de `./Controlle10 - ftp` em `/app/data/erp/source` no container `api`.
- [docker-compose.prod.yml](../docker-compose.prod.yml) — prod. Nome do projeto: `${COMPOSE_PROJECT_NAME:-listaatendimento}`.

### 6.2 Templates de env

- [.env.docker.example](../.env.docker.example) — dev. `POSTGRES_DB=lista_da_vez`, `POSTGRES_USER=lista_da_vez`. `AUTH_CONSULTANT_EMAIL_DOMAIN=acesso.omni.local`.
- [.env.production.example](../.env.production.example) — prod. `COMPOSE_PROJECT_NAME=listaatendimento`, `POSTGRES_DB=listaatendimento`. `SMTP_FROM_NAME=Lista da Vez`. Domínio público `lista.whenthelightsdie.com`.
- [back/.env.example](../back/.env.example) — env do backend isolado.
- [web/.env.example](../web/.env.example) — env do frontend isolado.

### 6.3 Scripts auxiliares

- [scripts/deploy/deploy-vps-fast.ps1](../scripts/deploy/deploy-vps-fast.ps1) — deploy.
- [scripts/dev/](../scripts/dev/) — 7 scripts shell para fallback sem Docker (Git Bash).
- [back/scripts/api/](../back/scripts/api/) e [back/scripts/postgres/](../back/scripts/postgres/) — 8 PowerShell para start/stop local do back e do Postgres local.
- [web/scripts/ensure-node-modules.mjs](../web/scripts/ensure-node-modules.mjs) — hook do container web.
- [.github/workflows/deploy-vps.yml](../.github/workflows/deploy-vps.yml) — CI/CD para VPS.

---

## 7. Código morto, duplicações e arquivos órfãos

Achados consolidados por análise estática (grep + leitura cruzada).

### 7.1 Backend Go

| Item | Localização | Evidência | Recomendação |
|---|---|---|---|
| Função `StreamCSV` | [back/internal/modules/erp/csv_parser.go:185](../back/internal/modules/erp/csv_parser.go) | Wrapper que só delega para `StreamCSVWithLimit`. Sem chamadores. | Remover; quem precisar chama `StreamCSVWithLimit` direto. |
| Função `NewMemoryUserStore` | [back/internal/modules/auth/store_memory.go:14](../back/internal/modules/auth/store_memory.go) | 1 referência (apenas a definição). Aplicação usa `PostgresUserStore`. | Mover para `_test.go` se servir a testes; senão remover. |
| Função `SeedDemoUsers` | [back/internal/modules/auth/store_memory.go:59](../back/internal/modules/auth/store_memory.go) | 1 referência. Dependente de `MemoryUserStore` (também morto). | Remover junto. |
| Pasta `back/cmd/debuginvite/` | [back/cmd/debuginvite/](../back/cmd/debuginvite/) | Vazia (sem `.go`). | Remover. |
| Arquivo `back/tmp_api_8081.log` | [back/tmp_api_8081.log](../back/tmp_api_8081.log) | Log antigo (30/03/2026) na raiz do back. | Remover. |

### 7.2 Frontend Vue

| Item | Localização | Evidência | Recomendação |
|---|---|---|---|
| Componente `OperationCampaignBrief.vue` | [web/app/features/operation/components/OperationCampaignBrief.vue](../web/app/features/operation/components/OperationCampaignBrief.vue) | 0 ocorrências em busca global por `\bOperationCampaignBrief\b` fora da própria definição. | Remover. Git preserva caso volte. |
| Componente `AdminPageHeader.vue` (versão tasks) | [web/layers/tasks/components/admin/AdminPageHeader.vue](../web/layers/tasks/components/admin/AdminPageHeader.vue) | Todas as páginas importam a versão core. A versão tasks é simplificada e não referenciada. | Remover; padronizar a versão `layers/core/components/admin/AdminPageHeader.vue`. |
| Função `useDashboardState` | [web/app/composables/useDashboardShell.ts:7-14](../web/app/composables/useDashboardShell.ts) | 0 importações externas. `useDashboardShell()` é o ponto público. | Tornar privada (`function _useDashboardState(...)`) ou remover a exportação. |
| Arquivo `web/app/pages/operacao/operations.md` | [web/app/pages/operacao/operations.md](../web/app/pages/operacao/operations.md) | `.md` dentro de `pages/` (Nuxt ignora, mas não é o local). | Mover para `docs/operacao/`. |
| Pasta `web/dist/` | [web/dist/](../web/dist/) | Build artifact local. Coberto por `dist/` no `.gitignore`. | Remover (build re-gera quando necessário). |

### 7.3 Componentes não utilizados — suspeita média

A análise inicial reporta que **fora os 3 itens acima, todos os outros componentes têm alguma referência**. Vale, antes de remover qualquer outro, fazer uma 2ª passada com `vue-tsc` ou `eslint-plugin-unused-imports` para evitar falso negativo.

### 7.4 Stores Pinia

A varredura amostral em [app-runtime.ts](../web/app/stores/app-runtime.ts) e [ui.ts](../web/app/stores/ui.ts) mostra todo state com leitor e mutador. Não há suspeita de state morto. Recomendação: rodar lint dedicado se quiser certeza completa, mas o custo provavelmente não compensa.

### 7.5 Layer `queue` esqueleto

[web/layers/queue/](../web/layers/queue/) tem apenas `nav.config.ts` (config) e `nuxt.config.ts` vazio. Não é código morto — é o ponto de chegada da migração de Fase 4 do roadmap. Estado atual: **placeholder**. Decisão a tomar:

- (a) Manter como está, com nota explícita no `AGENT.md`.
- (b) Avançar a migração agora (mover `operations`, `consultants`, `alerts`, `analytics`, `reports` para este layer).

---

## 8. Inconsistências de nomenclatura

> **Resolução (Seção 0)**: nome oficial é `Omni` / slug `omni`. Os 11 pontos abaixo são os locais que precisam mudar; ver [PLANO_REFATORACAO.md](PLANO_REFATORACAO.md) Fase 4 para os valores concretos.

Convivem três nomes para o mesmo produto:

| Nome | Onde aparece |
|---|---|
| `fila-atendimento` | Nome do diretório do repo. |
| `Fila de Atendimento` / `Fila de Atendimento MVP` | [README.md](../README.md), `web/nuxt.config.ts:105` (title). |
| `lista-da-vez` / `lista_da_vez` / `Lista da Vez` | [package.json](../package.json), [web/package.json](../web/package.json), [docker-compose.yml:1](../docker-compose.yml), `POSTGRES_DB`, `POSTGRES_USER`, `APP_NAME`, `SMTP_FROM_NAME`. |
| `listaatendimento` | `COMPOSE_PROJECT_NAME` em prod, `POSTGRES_DB` em prod. |

Pontos de troca obrigatórios para `omni`:

1. `package.json` (raiz) — campo `name`.
2. [web/package.json](../web/package.json) — campo `name`.
3. [docker-compose.yml](../docker-compose.yml) — campo `name:` (linha 1) e `APP_NAME`.
4. [docker-compose.prod.yml](../docker-compose.prod.yml) — `name:` e default de `COMPOSE_PROJECT_NAME`.
5. [.env.docker.example](../.env.docker.example) — `POSTGRES_DB`, `POSTGRES_USER`, `POSTGRES_PASSWORD`.
6. [.env.production.example](../.env.production.example) — `POSTGRES_DB`, `POSTGRES_USER`, `APP_NAME`, `SMTP_FROM_NAME`.
7. [back/internal/platform/config/config.go:68](../back/internal/platform/config/config.go) — default de `APP_NAME` (hoje "lista-da-vez-api").
8. [back/internal/modules/auth/password_reset_delivery.go:136](../back/internal/modules/auth/password_reset_delivery.go) — default de `AppName` no e-mail de reset.
9. [web/nuxt.config.ts:105](../web/nuxt.config.ts) — `head.title` ("Fila de Atendimento MVP").
10. [README.md](../README.md) — título e referências.
11. [AGENT.md](../AGENT.md) — referências.

**Cuidado especial com 5 e 6**: trocar `POSTGRES_DB` quebra ambientes locais existentes — a renomeação do banco exige dump/restore ou um migration de rename. Em produção, é mudança operacional.

---

## 9. Dependências e versões (auditadas)

### Backend Go ([back/go.mod](../back/go.mod) — não inspecionado em detalhe nesta passagem)

- Go 1.24.0 / toolchain 1.24.3
- Imagem base: `golang:1.24.0-bookworm`

### Frontend Nuxt ([web/package.json](../web/package.json))

- Nuxt `4.4.2` / Vue `3.5.30` / Pinia `3.0.4` / Tailwind `4.3.0`.
- Stack TipTap completa (`@tiptap/extension-*`, `@tiptap/starter-kit`, `@tiptap/vue-3`) — para o editor rich-text.
- `@nuxt/ui` `4.7.1`, `@nuxt/icon` `2.2.2`, ícones `@iconify-json/lucide`.
- Vitest `2.1.9` no dev (subutilizado).
- Node `24.11.1` em container.

---

## 10. Pontos de atenção para a próxima sessão

1. **A pasta `back/cmd/debuginvite/` está vazia** — provavelmente sobrou de uma remoção. Verificar git log e apagar.
2. **`web/app/pages/operacao/operations.md`** existe e o Nuxt ignora — risco zero, mas é poluição.
3. **`dev-compose-perfis.md`** descreve serviços que não existem (`painel-web`, `plataforma-api`). É lixo de outro projeto.
4. **A pasta `Controlle10 - ftp/`** (493 MB local) tem espaço no nome — funciona via `"./Controlle10 - ftp:/app/data/erp/source:ro"` no compose, mas qualquer script que faça `cd` precisa lidar com as aspas. Renomear ajuda.
5. **75 migrations** com 4 colisões de prefixo numérico. Nenhuma quebra, mas é uma armadilha futura.
6. **Layer `queue` esqueleto** — o roadmap detalha como preencher (Fase 4 de [ROADMAP.md](ROADMAP.md)). Hoje é a maior dívida arquitetural latente do frontend.
7. **Cobertura de testes do frontend** praticamente zero. Vitest configurado mas com 1 teste só. Antes de mexer no layer `queue`, vale fundar 4-5 testes de stores críticos (`auth`, `operations`, `settings`).

---

## 13. Qualidade do código (segurança, performance, organização, padronização, documentação)

> Auditoria adicionada em 2026-05-18 a pedido. Aqui está o "nível" real do código sob 5 ângulos.

### 13.1 Padronização — alinhamento com a comunidade

| Aspecto | Status | Evidência |
|---|---|---|
| Vue 3 Composition API com `<script setup>` | ✅ excelente | 100% dos componentes — padrão atual recomendado pela comunidade. |
| TypeScript no front | ⚠ parcial | Maioria dos arquivos é `.ts`/`.vue` tipado, mas **não há `web/tsconfig.json` próprio** — depende do gerado em `.nuxt/`. Sem `strict: true` customizado. |
| Uso de `any` | ✅ disciplinado | Apenas **17 ocorrências em 11 arquivos** (de 225 arquivos `.ts`/`.vue` somados). |
| `console.log` deixados pra trás | ✅ limpo | Apenas **10 ocorrências em 4 arquivos** — quase todo o ruído de debug foi removido. |
| TODO/FIXME/HACK abandonados | ✅ raríssimo | **1 ocorrência** no projeto inteiro. |
| ESLint configurado no front | ❌ ausente | Sem `.eslintrc*`, sem `eslint.config.*`. |
| Prettier configurado no front | ❌ ausente | Sem `.prettierrc*`. |
| `golangci-lint` no back | ❌ ausente | Sem `.golangci.yml`. Há `go vet`/`go test` mas sem suite de lint configurada. |
| Husky / lint-staged / pre-commit hooks | ❌ ausente | Nenhum hook configurado. |
| Layout Go (handler / service / repository) | ✅ aderente | Cada módulo segue o padrão idiomático Go com `http.go`, `service.go`, `store_postgres.go`, `model.go`. |
| `AGENT.md` por módulo Go | ✅ excelente | 100% dos módulos têm seu `AGENT.md`. |
| `AGENT.md` × `AGENTS.md` no front | ⚠ inconsistente | Convive `AGENT.md` (raiz/back) com `AGENTS.md` (web e algumas pastas). Decidir um. |

### 13.2 Organização — tamanho de arquivos e separação de responsabilidades

**Distribuição dos componentes Vue (141 arquivos):**

| Faixa | Quantidade | % | Diagnóstico |
|---|---|---|---|
| ≤ 200 linhas | 81 | 57% | ✅ saudável |
| 201 – 400 | 21 | 15% | ✅ ok |
| 401 – 700 | 20 | 14% | ⚠ atenção |
| 701 – 1000 | 12 | 9% | ⚠ alto |
| > 1000 linhas | **7** | **5%** | 🔴 **crítico — refatorar** |

**Os 7 componentes Vue acima de 1000 linhas:**

| Arquivo | Linhas | Problema típico |
|---|---|---|
| [web/app/components/users/UsersAccessManager.vue](../web/app/components/users/UsersAccessManager.vue) | **2.187** | 548 linhas de template + 1.076 de script com 20+ helpers que deveriam ser composable/util. |
| [web/app/features/operation/components/OperationFinishModal.vue](../web/app/features/operation/components/OperationFinishModal.vue) | **2.143** | Modal multi-passo com toda a lógica do fluxo de finalização inline. |
| [web/layers/tasks/pages/tasks.vue](../web/layers/tasks/pages/tasks.vue) | 1.340 | Página com lógica de board + filtros + modal + drag inline. |
| [web/app/components/feedback/FeedbackWorkspace.vue](../web/app/components/feedback/FeedbackWorkspace.vue) | 1.297 | Workspace concentra listagem + form + filtros + estado. |
| [web/app/components/settings/SettingsWorkspace.vue](../web/app/components/settings/SettingsWorkspace.vue) | 1.282 | Workspace com várias seções inline em vez de delegar para sub-componentes. |
| [web/app/components/erp/ErpWorkspace.vue](../web/app/components/erp/ErpWorkspace.vue) | 1.230 | Idem. |
| [web/layers/core/components/theme/ThemeColorInput.vue](../web/layers/core/components/theme/ThemeColorInput.vue) | 1.007 | Input de cor com picker custom inline. |

**Arquivos TS/composables/stores grandes:**

| Arquivo | Linhas | Problema |
|---|---|---|
| [web/layers/tasks/composables/useTasksPageContext.ts](../web/layers/tasks/composables/useTasksPageContext.ts) | **1.737** | **1 único export**, tudo encapsulado numa única função monolítica. |
| [web/layers/tasks/stores/tasks.ts](../web/layers/tasks/stores/tasks.ts) | **1.486** | Store gigante com normalizers, validações e actions misturadas. |
| [web/app/stores/erp.ts](../web/app/stores/erp.ts) | 762 | Store ERP com várias responsabilidades. |
| [web/app/stores/dashboard/runtime/state.ts](../web/app/stores/dashboard/runtime/state.ts) | 721 | OK — runtime já está fatiado em pasta dedicada (bom exemplo). |
| [web/app/stores/auth.ts](../web/app/stores/auth.ts) | 629 | Sessão + login + reset + remember + perfil. |
| [web/app/stores/alerts.ts](../web/app/stores/alerts.ts) | 628 | |
| [web/app/stores/operations.ts](../web/app/stores/operations.ts) | 619 | |
| [web/app/stores/settings.ts](../web/app/stores/settings.ts) | 615 | |

**Backend Go (158 arquivos sem testes — média 290 linhas, mediana 167):**

| Arquivo | Linhas | Problema |
|---|---|---|
| [back/internal/modules/erp/repository_postgres.go](../back/internal/modules/erp/repository_postgres.go) | **2.942** | 58 funções num arquivo só. Mistura repository + helpers CRM + agregadores + normalizers. |
| [back/internal/modules/operations/service.go](../back/internal/modules/operations/service.go) | **1.975** | Service monolítico do fluxo principal de atendimento. |
| [back/internal/modules/alerts/store_postgres.go](../back/internal/modules/alerts/store_postgres.go) | 1.466 | |
| [back/internal/modules/tasks/repository_postgres.go](../back/internal/modules/tasks/repository_postgres.go) | 1.402 | |
| [back/internal/modules/settings/service.go](../back/internal/modules/settings/service.go) | 1.320 | |
| [back/internal/modules/erp/service.go](../back/internal/modules/erp/service.go) | 1.259 | |
| [back/internal/modules/reports/service.go](../back/internal/modules/reports/service.go) | 1.215 | |
| [back/internal/modules/analytics/service.go](../back/internal/modules/analytics/service.go) | 1.003 | |

**Diagnóstico geral**: o backend tem ~8 arquivos críticos. O frontend tem 7 componentes + 2 arquivos TS críticos. **Não é catástrofe**, mas explica a fricção típica de "mudei aqui e quebrou ali" — quando um arquivo passa de 1000 linhas, qualquer alteração tem blast radius alto.

### 13.3 Segurança

| Risco | Status | Detalhe |
|---|---|---|
| SQL Injection (Go) | ✅ baixo | Uso de prepared statements via `pgx`. 0 `fmt.Sprintf` em queries SQL reais (matches no grep eram só rotas HTTP). |
| Senhas no código | ✅ baixo | bcrypt em [back/internal/modules/auth/passwords.go](../back/internal/modules/auth/passwords.go). 1 hardcoded em [auth/store_memory.go:114](../back/internal/modules/auth/store_memory.go) (`dev123456`) — **já marcado como morto na Fase 3**. |
| XSS no front | ✅ baixo | **0 `v-html`**, **0 `innerHTML`** em todo o web. Vue escapa por padrão. |
| `localStorage` com dados sensíveis | ⚠ atenção | Usado para tema, layout do sidebar, "remember me" (e-mail + flag). **Não armazena token JWT** — token vai em cookie de app. Bom. |
| CORS | ✅ configurado | [middleware.go:130](../back/internal/platform/httpapi/middleware.go) lê lista de origens via env `CORS_ALLOWED_ORIGINS`. |
| Rate limit | ✅ existe | [rate_limit.go](../back/internal/platform/httpapi/rate_limit.go) + teste. Verificar se está aplicado a todas as rotas sensíveis. |
| HTTP security headers | ⚠ verificar | Não vi explicitamente `Strict-Transport-Security`, `X-Content-Type-Options`, `Content-Security-Policy`. Em prod, costuma ser papel do Caddy. |
| Segredos no `.env.example` | ✅ ok | Templates trazem placeholders (`troque-por-um-segredo-longo-e-aleatorio`). |
| Tokens crus na raiz do repo | ✅ corrigido | Movidos para `tmp/` (gitignored) na Fase 1. |
| `JWT_SECRET` default fraco | ⚠ atenção | Default `dev-secret-change-me` em [docker-compose.yml:41](../docker-compose.yml) — ok pra dev, mas se algum ambiente subir sem override é vulnerável. |
| Dependências desatualizadas | ⚠ verificar | Sem `npm audit`/`go mod download -x` rodando em CI. |
| Auditoria de RBAC | ✅ robusto | Módulo `access` + `core/rbac_*` com testes. |

### 13.4 Performance

| Aspecto | Status | Detalhe |
|---|---|---|
| Code splitting por página | ✅ automático | Nuxt 4 faz por padrão (route-level). |
| Componentes lazy | ❌ não usado | **0 `defineAsyncComponent`**, **0 `<Suspense>`**, **0 `import()` dinâmicos**. Componentes pesados (`OperationFinishModal` 2.143 linhas, `UsersAccessManager` 2.187) carregam síncronos. |
| SSR controlado | ✅ desligado nas rotas certas | Todas as 24 rotas autenticadas têm `ssr: false` em [nuxt.config.ts:14-40](../web/nuxt.config.ts) — evita custo de SSR em painel admin. |
| TipTap optimizeDeps | ✅ configurado | Lista de extensões TipTap em `vite.optimizeDeps.include` para evitar re-bundle. |
| Bundle audit | ⚠ não medido | Não há análise de bundle (`vite-bundle-visualizer`/`rollup-plugin-analyzer`) configurada. |
| Imagens otimizadas | ✅ webp/avif | Logos em 3 formatos com `<picture>` + `srcset`. |
| Realtime WebSocket | ✅ usado | Hub próprio em módulo `realtime`. Atualizações via tópico em vez de polling. |
| N+1 nas queries | ⚠ verificar | Repository do ERP tem padrões "list X then list Y per X" — vale auditar com `EXPLAIN` em queries quentes. |
| Índices Postgres | ✅ parcialmente | Migration `0107_perf_hotpath_stats.sql` indica preocupação com performance, mas não foi feita varredura full. |
| Cache HTTP | ⚠ não medido | Sem `ETag`/`Cache-Control` evidente em endpoints públicos do back. |

### 13.5 Documentação

| Aspecto | Status | Detalhe |
|---|---|---|
| `AGENT.md` por módulo (back) | ✅ 100% | Todos os 19 módulos. |
| `AGENT.md`/`AGENTS.md` no front | ✅ parcial | Raiz do web tem, alguns subdirs também (operations features, users components). |
| `README.md` no projeto e no back | ✅ presente | Cobrem fluxo Docker e fallback local. |
| Doc inline (godoc) | ⚠ parcial | Funções exportadas nem sempre têm comentário no padrão godoc. |
| JSDoc / TSDoc nos composables | ❌ ausente | Composables grandes (`useTasksPageContext` 1.737 linhas) não têm bloco JSDoc explicando contrato. |
| Arquitetura geral | ✅ excelente | [ROADMAP.md](ROADMAP.md), [NUXT_4_STORE_ARCHITECTURE.md](NUXT_4_STORE_ARCHITECTURE.md), [SCHEMA_TARGET.md](SCHEMA_TARGET.md), [CONTRACT_FREEZE.md](CONTRACT_FREEZE.md). |
| ERD do banco | ✅ presente | [back/database/ERD.md](../back/database/ERD.md) (Mermaid). |
| `COMPONENT_INVENTORY.md` | ⚠ desatualizado | Não reflete a árvore atual de componentes — vai virar tarefa da Fase 2. |
| Decisões arquiteturais (ADR) | ❌ ausente | Sem pasta `docs/adr/` ou similar. Decisões ficam pulverizadas em planos. |

### 13.6 Veredicto da qualidade

| Pilar | Nota | Comentário |
|---|---|---|
| **Segurança** | 8/10 | Fundamentos sólidos (bcrypt, prepared statements, CORS, rate limit). Falta: HTTP security headers explícitos, `npm audit`/`go mod` em CI, revisão de defaults de dev. |
| **Performance** | 6/10 | Boa base (SSR controlado, realtime via WS, índices). Falta: lazy loading no front, análise de bundle, cache HTTP, audit de N+1. |
| **Organização** | 6/10 | Backend modular exemplar; frontend bem dividido por domínio. Penaliza: 7 componentes Vue >1000 linhas, 8 arquivos Go >1000 linhas, composables monolíticos. |
| **Padronização** | 7/10 | 100% Composition API moderna; quase 0 `any`; quase 0 `console.log`. Penaliza: sem ESLint, sem Prettier, sem `golangci-lint`, sem pre-commit. |
| **Documentação** | 7/10 | Inventário institucional impressionante. Penaliza: doc inline esparsa, sem ADRs, COMPONENT_INVENTORY desatualizado. |
| **Total** | **34/50** | Base sólida; ganhos rápidos disponíveis. |

### 13.7 Pontos críticos a tratar (top 10 por ROI)

1. 🔴 **Configurar ESLint + Prettier no `web/`** — captura 90% dos problemas de padronização automaticamente.
2. 🔴 **Configurar `golangci-lint` no `back/`** — captura mortos, shadowing, leaks de contexto, etc.
3. 🔴 **Fatiar os 7 componentes Vue acima de 1000 linhas** — começa por `UsersAccessManager` e `OperationFinishModal`.
4. 🔴 **Fatiar `useTasksPageContext.ts` (1.737 linhas, 1 export)** — split em composables menores por responsabilidade.
5. 🟡 **Fatiar `erp/repository_postgres.go` (2.942 linhas, 58 funções)** — separar CRM helpers em arquivo próprio.
6. 🟡 **Setar `strict: true` no tsconfig do web** — endurece a tipagem.
7. 🟡 **Adicionar pre-commit hook** (`lint-staged` + `husky`) — impede regressão.
8. 🟡 **Adicionar lazy load para componentes pesados** — modais, workspaces inteiras.
9. 🟢 **Adicionar HTTP security headers** em prod (preferencialmente no Caddy do servidor).
10. 🟢 **Criar pasta `docs/adr/`** para Architecture Decision Records.

---

## 11. O que NÃO foi analisado nesta passagem

Para manter o documento honesto:

- `go.sum` e árvore real de dependências Go (só li `go.mod` em alto nível).
- Conteúdo detalhado das 75 migrations.
- Lógica interna de cada um dos 19 módulos.
- Conteúdo interno de cada um dos ~63 componentes Vue.
- Análise de bundle size do frontend.
- Análise de query performance no Postgres.
- Cobertura real de testes (% medida).
- Acessibilidade e i18n do frontend.
- Logs de runtime e telemetria.

Esses são caminhos para passagens futuras quando o foco for qualidade/performance específica.

---

## 12. Referência cruzada

- Plano operacional → [PLANO_REFATORACAO.md](PLANO_REFATORACAO.md)
- Roadmap macro multi-tenant → [ROADMAP.md](ROADMAP.md)
- Backlog vivo → [BACKLOG.md](BACKLOG.md)
- Schema-alvo → [SCHEMA_TARGET.md](SCHEMA_TARGET.md)
- Contratos invariantes → [CONTRACT_FREEZE.md](CONTRACT_FREEZE.md)
- Deploy oficial → [DEPLOY_VPS.md](DEPLOY_VPS.md)
