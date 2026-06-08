# Arquitetura - Estado atual em 2026-06-07

> Escopo: banco PostgreSQL, backend Go, comunicacao com Nuxt, WebSockets, multi-tenant, seguranca e pontos de integracao.
> Fonte: codigo no workspace, container Docker ativo (`postgres`, `api`, `web`) e introspeccao do banco `omni`.
> Observacao: o worktree esta bem ativo, com muitas alteracoes nao commitadas. Este documento e uma fotografia do estado atual em disco e no banco local, nao uma promessa do que esta em producao.

## Resumo executivo

- O sistema ja esta em uma arquitetura modular: Go em `back/`, Nuxt 4 em `web/`, PostgreSQL 16 no Docker e schemas separados por dominio (`core`, `queue`, `tasks`, `site`, `notifications`, `roadmap`).
- O banco tem 87 tabelas base no ambiente local: `core` 16, `queue` 26, `tasks` 17, `public` 18, `notifications` 4, `site` 4 e `roadmap` 2.
- O modelo multi-tenant novo esta centrado em `core.accounts`, `core.users`, `core.account_users`, `core.account_modules`, RBAC em `core.roles`/`core.permissions`, e dominios operacionais usando `tenant_id` ou `account_id`.
- Nao ha Row-Level Security ativo em nenhuma tabela. O isolamento hoje depende de middleware, membership, guards, claims do token, filtros de repository/service e FKs.
- O backend tem suporte de codigo para sessoes em `core.user_sessions` e cache de principal, mas no bootstrap atual eu nao vi `SetSessionRepository` nem `SetPrincipalCache` sendo chamados. Na pratica, a revogacao de logout parece nao estar ligada ao fluxo principal.
- O frontend usa Bearer token em cookie legivel por JS, envia `Authorization` nas APIs e injeta `X-Account-Id`. Ha um bug/risco de semantica: o plugin global usa `auth.activeTenantId`, enquanto o Core V2 tem store proprio de `activeAccountId`.
- O realtime usa WebSocket com hub em memoria. Funciona para uma instancia da API, mas ainda nao esta pronto para escala horizontal sem broker externo.
- Existem modulos/tabelas para `site` e `roadmap`, e o catalogo do banco possui linhas para ambos, mas o bootstrap Go atual registra no Core V2 apenas `core`, `notifications`, `tasks`, `queue` e `crm`. O modulo `operationgoals` tambem existe, mas nao foi visto registrado em `app.go`.

## Stack e runtime

| Area | Estado atual |
|---|---|
| Backend | Go, `net/http`, `pgx/v5`, `gorilla/websocket`, arquitetura modular por `internal/modules` |
| Frontend | Nuxt 4, Vue 3, Pinia, Nuxt UI, Tailwind 4, layers `core`, `queue`, `tasks` |
| Banco | PostgreSQL 16 no Docker, migrations SQL embedadas no Go |
| Dev local | `docker compose up`, portas atuais: API `9091 -> 8080`, web `3003`, Postgres `5432` |
| Realtime | WebSockets por `gorilla/websocket`, hub local em memoria |
| Automacao local | perfil opcional com Redis, WAHA e n8n no `docker-compose.yml` |

### Drift de versao/configuracao

- `back/go.mod` declara `go 1.25.0`.
- `back/Dockerfile` usa `golang:1.26.3-bookworm`.
- `AGENT.md` e `back/AGENT.md` ainda documentam contratos de versao que precisam ser reconciliados com o Dockerfile e `go.mod`.
- `docker-compose.yml` usa `CORE_V2_ENABLED=true` e `AUTH_ROLES_SOURCE=core`.
- `docker-compose.prod.yml` usa `CORE_V2_ENABLED=false` e `AUTH_ROLES_SOURCE=core_with_fallback`. Como a migration `0135` removeu tabelas legadas de role, esse default de producao merece revisao antes de deploy.

## Banco de dados

### Inventario por schema

| Schema | Tabelas base | Responsabilidade |
|---|---:|---|
| `core` | 16 | Accounts, usuarios, organizacoes, catalogo de modulos, RBAC, sessoes |
| `queue` | 26 | Fila de atendimento, lojas, consultores, operacao, alertas, feedback, ERP operacional |
| `tasks` | 17 | Boards, tasks, campos customizados, comentarios, relacoes, tempo, auditoria |
| `site` | 4 | Leads, produtos, tracking e fontes de webhook |
| `notifications` | 4 | Notificacoes, canais, mutes e log de entrega |
| `roadmap` | 2 | Modulos/regras de roadmap por account |
| `public` | 18 | Tabelas residuais e compatibilidade: reset/convite, access legado, ERP raw, settings legados e views para schemas novos |

### Dados locais relevantes

Contagem exata no banco local no momento da analise:

- `core.accounts`: 4 accounts.
- `core.users`: 66 usuarios.
- `queue.stores`: 6 lojas.
- `tasks.tasks`: 262 tasks.
- `site.leads`: 0 leads.

O volume local e pequeno. Portanto, conclusoes de performance por carga real ainda dependem de testes com massa e `EXPLAIN ANALYZE`.

### Migrations

- As migrations em disco vao de `0001_init.sql` ate `0136_drop_legacy_public_objects.sql`.
- A tabela `public.schema_migrations` mostra aplicacao ate `0136_drop_legacy_public_objects.sql`.
- Ha sinais de historico irregular no ledger, incluindo um registro `0126` alem de `0126_stores_billing_amount.sql`. Isso nao deve ser "corrigido" reescrevendo migration antiga; o caminho seguro e documentar e criar migration/rotina de saneamento se necessario.
- Existem versoes com sufixos ou prefixos duplicados (`0015a`, multiplos `0019`, `0031`, `0039`, `0110`). O migrator ordena por nome de arquivo, entao a convencao precisa ser congelada para evitar surpresa em ambientes novos.

### Catalogo de modulos no banco

`core.modules` no banco local contem:

| Modulo | Schema | Permissoes ativas | Observacao |
|---|---|---:|---|
| `core` | `core` | 8 | Core platform |
| `crm` | `crm` | 5 | Catalogo/ERP no codigo fica em `internal/modules/crm/*` |
| `notifications` | `notifications` | 2 | Rotas registradas via Core V2 |
| `queue` | `queue` | 8 | Catalogo Core V2, mas HTTP principal ainda em wiring legado |
| `tasks` | `tasks` | 13 | Rotas registradas via Core V2 |
| `roadmap` | `roadmap` | 0 | Tabelas e modulo existem, mas nao vi registro no bootstrap atual |
| `site` | `site` | 0 | Tabelas e modulo existem, front guarda rotas, mas nao vi registro no bootstrap atual |

`core.account_modules` tem modulos habilitados assim no banco local:

- `crm`: 3 accounts.
- `queue`: 3 accounts.
- `tasks`: 3 accounts.
- `site`: 1 account.
- `core`, `notifications`, `roadmap`: 0 accounts habilitadas.

### Tabelas e campos principais

#### `core`

- `core.accounts`: `id`, `organization_id`, `slug`, `name`, `is_active`, `plan_code`, `billing_mode`, `monthly_payment_amount`, `payment_due_day`, `webhook_enabled`, `webhook_key`, contatos, logo, flags de exigencia de usuario/loja.
- `core.users`: `id`, `email`, `display_name`, `password_hash`, `must_change_password`, `avatar_path`, `is_platform_admin`, `is_active`, `nick`, `employee_code`, `job_title`.
- `core.account_users`: vinculo `account_id` + `user_id`, `is_active`, `invited_by_user_id`, `joined_at`.
- `core.organizations` e `core.organization_users`: agrupamento organizacional acima de accounts.
- `core.modules`: catalogo de modulo com `id`, `schema_name`, `label`, `description`, `is_core`, dependencias e ordenacao.
- `core.account_modules`: habilitacao de modulo por account, com `enabled`, `enabled_at`, `config`.
- `core.permissions`: permissoes por modulo, `key`, `module_id`, `scope`, `deprecated_at`.
- `core.roles`: papel por account, `code`, `label`, `is_default`, `is_locked`, template de origem.
- `core.role_permissions`: relacao role-permission.
- `core.role_templates` e `core.role_template_permissions`: templates de papeis por modulo.
- `core.user_role_assignments`: usuario recebe role dentro de uma account.
- `core.user_permission_overrides`: allow/deny especifico por usuario/account/permissao.
- `core.user_module_settings`: configuracao de modulo por usuario.
- `core.user_sessions`: `id`, `user_id`, `revoked_at`, `last_seen_at`, `user_agent`, `ip`, `created_at`.

#### `queue`

- `queue.stores`: lojas por tenant/account, com `tenant_id`, `code`, `name`, `city`, metas comerciais, `billing_amount`, `is_active`.
- `queue.consultants`: consultores por `tenant_id`/`store_id`, dados de apresentacao, metas, comissao, `user_id`, `is_active`.
- `queue.operation_current_status`: status atual do consultor por loja.
- `queue.operation_queue_entries`: fila atual por loja/consultor.
- `queue.operation_active_services`: atendimentos abertos, tempo de fila, paralelismo, stop/cancel.
- `queue.operation_service_history`: historico de atendimento, fechamento, produtos, cliente, motivos, valores, campanhas e metadata operacional.
- `queue.operation_status_sessions`: sessoes de status e duracoes.
- `queue.operation_paused_consultants`: pausas por loja/consultor.
- `queue.operation_goal_targets`: metas por loja/consultor/mes.
- `queue.alert_instances` e `queue.alert_actions`: alertas operacionais e acoes/respostas.
- `queue.tenant_*`: configuracoes por tenant para operacao, modal de finalizacao, regras, temas, produtos e opcoes.
- `queue.feedback_messages`, `queue.feedback_read_states`, `queue.user_feedback`: feedback e mensagens.
- `queue.erp_sync_runs`, `queue.erp_item_raw`, `queue.erp_item_current`, `queue.consultant_erp_links`: ingestao/sincronia ERP e vinculos com consultores.

#### `tasks`

- `tasks.boards`: board por account, organization opcional, slug, nome, criador, archived.
- `tasks.columns`: colunas por board.
- `tasks.tasks`: task por account/board/column, titulo, conteudo HTML, status, prioridade, datas, responsavel, cliente/account, versao, metadata de UI e ponte com roadmap.
- `tasks.fields`, `tasks.field_options`, `tasks.field_values`: campos customizados e valores por task.
- `tasks.task_assignees`, `tasks.task_subscribers`: responsaveis e inscritos.
- `tasks.task_comments`, `tasks.task_mentions`: comentarios e mencoes.
- `tasks.task_relations`: relacoes com outros modulos/recursos.
- `tasks.task_shares`: compartilhamento com account cliente.
- `tasks.task_time_entries`: apontamento de tempo.
- `tasks.task_doc_snapshots`: snapshots binarios/versionados de documento.
- `tasks.audit_log`: log de auditoria do modulo.
- `tasks.views`, `tasks.view_widgets`: visoes e widgets.

#### `site`

- `site.webhook_sources`: fonte por account, `slug`, `entity_type`, `secret`, `payload_mapping`, `is_active`.
- `site.leads`: lead por account/fonte, nome, email, telefone, pagina, cupom, consentimento, tracking, payload bruto, status.
- `site.products`: produto por account/fonte, codigo, descricao, imagem, categorias/campanhas JSON, preco, estoque, status.
- `site.tracking_events`: eventos de tracking por account/fonte, visitante, sessao, pagina, elemento, produto, device, UTM, payload, IP/user agent e timestamps.

#### `notifications`

- `notifications.user_notifications`: notificacoes por account/usuario, modulo/evento de origem, titulo, corpo, link, payload e leitura.
- `notifications.notification_channels`: preferencia por canal/evento.
- `notifications.mutes`: mute por recurso.
- `notifications.delivery_log`: tentativas de entrega por canal.

#### `roadmap`

- `roadmap.modules`: itens/modulos do roadmap por account, rota, status, prioridade, categoria, escopo JSON e dependencias.
- `roadmap.rules`: regras de roadmap por account, categoria, titulo, corpo, motivo e aplicabilidade.

#### `public`

Ainda ha objetos em `public` por legado/compatibilidade:

- Tabelas base: `access_permissions`, `access_role_permissions`, `alert_rule_definitions`, `erp_*_raw`, `erp_export_outbox`, `erp_sync_files`, `store_*`, `user_*`, `users_backup`, `schema_migrations`.
- Views de compatibilidade apontando para schemas novos: `public.operation_* -> queue.operation_*`, `public.site_* -> site.*`, `public.tenant_* -> queue.tenant_*`, `public.erp_item_* -> queue.erp_item_*`, entre outras.
- O plano `docs/LEGACY_PUBLIC_REMOVAL_PLAN.md` ja registra remocao de alguns legados antigos, mas o `public` ainda e usado como ponte para varios objetos.

### Relacionamentos principais

#### Core identity e RBAC

- `core.accounts.organization_id -> core.organizations.id`.
- `core.account_users(account_id, user_id)` liga accounts e users.
- `core.organization_users(organization_id, user_id)` liga organizations e users.
- `core.account_modules(account_id, module_id)` habilita modulos por account.
- `core.permissions.module_id -> core.modules.id`.
- `core.roles.account_id -> core.accounts.id`.
- `core.role_permissions(role_id, permission_key)` liga roles e permissions.
- `core.user_role_assignments(account_id, user_id, role_id)` aplica papeis por account.
- `core.user_permission_overrides(account_id, user_id, permission_key)` permite overrides por usuario.
- `core.user_sessions.user_id -> core.users.id`.

#### Queue/operacao

- `queue.stores.tenant_id -> core.accounts.id`.
- `queue.consultants.tenant_id -> core.accounts.id`.
- `queue.consultants.store_id -> queue.stores.id`.
- Tabelas de estado operacional (`operation_current_status`, `operation_queue_entries`, `operation_active_services`, `operation_paused_consultants`, `operation_status_sessions`) referenciam `queue.stores` e `queue.consultants`.
- `queue.operation_service_history.store_id -> queue.stores.id` e `person_id -> queue.consultants.id`.
- `queue.alert_instances` e `queue.alert_actions` referenciam tenant/account, loja, alerta e usuario ator.
- `queue.erp_*` referencia account, loja, arquivos de sync e runs de sync.

#### Tasks

- `tasks.boards.account_id -> core.accounts.id`.
- `tasks.columns.board_id -> tasks.boards.id`.
- `tasks.tasks.account_id -> core.accounts.id`.
- `tasks.tasks(board_id, account_id)` reforca board dentro da mesma account.
- `tasks.tasks.column_id -> tasks.columns.id`.
- `tasks.tasks.client_account_id -> core.accounts.id`.
- `tasks.tasks.roadmap_module_id -> roadmap.modules.id`.
- Assignees, subscribers, comments, mentions, shares, relations e time entries referenciam `tasks.tasks` e/ou `core.users`.

#### Site e notificacoes

- `site.*.account_id -> core.accounts.id`.
- `site.leads/products/tracking_events.source_id -> site.webhook_sources.id`.
- `notifications.*.account_id -> core.accounts.id`.
- `notifications.*.user_id -> core.users.id`.

### Chaves primarias e indices

- O padrao geral e UUID como PK (`id`) em entidades principais.
- Tabelas de join usam PK composta: `core.account_modules(account_id,module_id)`, `core.account_users(account_id,user_id)`, `tasks.task_assignees(task_id,user_id)`, `queue.operation_current_status(store_id,consultant_id)`.
- Tabelas de hot path possuem indices multiplos: por exemplo `queue.operation_service_history` tem 9 indices, `queue.alert_instances` 7, `queue.user_feedback` 8, `tasks.tasks` 8, `site.tracking_events` 7.
- `pg_stat_user_tables` local nao tinha `last_analyze` recente preenchido para muitas tabelas. Em producao, monitorar autovacuum/analyze e planos reais sera importante.

### Isolamento multi-tenant no banco

- Nenhuma tabela esta com `relrowsecurity=true`; ou seja, RLS esta desabilitado em todos os schemas.
- O isolamento depende de:
  - `Authorization` assinado no backend.
  - membership em `core.account_users`.
  - `X-Account-Id` para rotas Core V2/multi-account.
  - `RequireModuleByPath` no backend.
  - filtros por `tenant_id`/`account_id` nos services e repositories.
  - FKs para impedir referencias soltas.
- Esse modelo funciona se todos os handlers respeitam o contrato. Para porte enterprise, falta a rede final no banco.

## Backend Go

### Estrutura

- `back/cmd/api/main.go`: carrega config, abre `pgxpool`, monta handler e sobe `http.Server`.
- `back/internal/platform/*`: app bootstrap, config, database, HTTP middleware, events, modules registry.
- `back/internal/modules/*`: dominios de negocio:
  - `auth`, `access`, `tenants`, `stores`, `users`.
  - `queue/*`: operations, alerts, analytics, consultants, feedback, reports, settings.
  - `crm/*`: catalog, erp.
  - `core`, `tasks`, `notifications`, `site`, `roadmap`, `realtime`, `bi`, `operationgoals`.

### Bootstrap e wiring

Em `back/internal/platform/app/app.go`:

- Rotas legadas/atuais registradas diretamente: auth, tenants, stores, consultants, settings, catalog, operations, alerts, realtime, reports, analytics, access, feedback, erp, bi, users.
- Quando `CORE_V2_ENABLED=true`, o registry registra:
  - `core.New()`
  - `notifications.New(...)`
  - `tasks.New(...)`
  - `queue.New()`
  - `crm.New()`
- `queue` e `crm` entram no catalogo Core V2 para permissoes/templates, mas seus handles nao registram rotas HTTP novas; as rotas ainda ficam no wiring legado.
- `site.New()` e `roadmap.New()` existem como modulos, mas nao foram encontrados no registro do `app.go` atual.
- `operationgoals.RegisterRoutes` existe, mas o modulo nao foi visto registrado no bootstrap atual.

### Middlewares e protecoes HTTP

Camadas observadas:

- CORS com allowlist (`httpapi.CORS`).
- Request ID.
- Rate limit.
- Logging estruturado.
- Recover.
- Module guard por path quando Core V2 esta ativo.
- `RequireAuth`, `RequireRoles` e `RequireAuthWithAccount` nas rotas.
- `statusRecorder` preserva interfaces como `http.Flusher` e `http.Hijacker`, necessario para WebSocket.

### Auth

- Token proprio assinado por HMAC SHA-256, prefixo `ldv1`, payload JSON base64url.
- Claims incluem usuario, email, role, tenant/store IDs, `iat`, `exp` e opcionalmente `sid`.
- Senha usa bcrypt.
- O middleware autentica principalmente por header `Authorization: Bearer ...`.
- Ha codigo para sessoes em `core.user_sessions` (`Create`, `IsRevoked`, `Revoke`, `Touch`) e cache de principal, mas no `app.go` atual nao foi visto wiring com `authService.SetSessionRepository(...)` ou `authService.SetPrincipalCache(...)`.
- Consequencia: `core.user_sessions` existe, mas a revogacao imediata de logout e o cache de principal parecem nao estar efetivamente ativos no bootstrap atual.

### Realtime

- Usa `gorilla/websocket`.
- Endpoints principais:
  - `/v1/realtime/operations?storeId=...&access_token=...`
  - `/v1/realtime/context?tenantId=...&access_token=...`
  - `/v1/realtime/tasks`
  - `/v1/realtime/presence`
  - `/v1/realtime/notifications`
- O token pode vir por query string `access_token` ou por header `Authorization`.
- O frontend atual usa query string para operations/context.
- `CheckOrigin` valida origem contra a allowlist de CORS.
- O hub e em memoria. Em multiplas replicas, eventos publicados em uma instancia nao chegam automaticamente nas outras.

### Jobs/background

No bootstrap existem goroutines para:

- Monitoramento de alertas de operacao em loop.
- Cleanup de anexos de feedback.
- Scheduler opcional de auto-sync ERP.

Esses loops usam `context.Background()` e nao estao claramente ligados a graceful shutdown do `http.Server`.

### Rotas principais

- Auth: login/logout, `/v1/me`, profile, avatar, password, invitations, password reset.
- Contexto legado: `/v1/me/context`.
- Core V2/admin: `/v2/me/accounts`, `/v2/me/context`, `/v1/admin/accounts`, modules, memberships, users, organizations, roles.
- Queue: operations, alerts, reports, analytics, feedback, consultants, settings, stores.
- CRM/ERP: catalog search, ERP status/overview/crm/runs/products/stats/customers/employees/orders/backfill/sync.
- Tasks: boards, task-boards, tasks CRUD, comments, shares, relations, audit, tracking e videos.
- Notifications: listagem, marcar lida/todas, preferencias, mute.
- BI: endpoints Perola.
- Site/Roadmap: handlers existem nos modulos, mas nao vi registro no bootstrap principal.

## Frontend Nuxt

### Estrutura

- `web/nuxt.config.ts` estende layers:
  - `./layers/core`
  - `./layers/queue`
  - `./layers/tasks`
- Usa Pinia, Nuxt UI, Tailwind e componentes/composables por dominio.
- Variaveis:
  - `apiInternalBase`
  - `public.apiBase`
  - `public.apiWsBase`
- Varias rotas autenticadas tem SSR desabilitado, entao o painel roda principalmente no client.

### Comunicacao HTTP

`web/app/utils/api-client.ts`:

- Resolve base URL por ambiente (`apiInternalBase` no server, `public.apiBase` no client).
- Injeta `Authorization: Bearer <token>`.
- Injeta `X-Account-Id` a partir de provider global.
- Converte body de objetos em JSON para metodos mutaveis.
- Deduplica GETs em voo por `baseURL + path + headers`.

Ponto de atencao: a chave de dedupe nao inclui explicitamente `query`, `params` ou outras opcoes do `$fetch`. Dois GETs para o mesmo path com queries diferentes podem ser colapsados indevidamente se a query for passada em `options.query`.

### Account ID e tenant ID

- `web/app/plugins/account-id-bridge.client.ts` liga o provider global de `X-Account-Id` a `auth.activeTenantId`.
- `web/layers/core/stores/account.ts` tem `activeAccountId`, cookie `ldv_active_account_id`, `/v2/me/accounts` e `/v2/me/context`.
- Isso cria drift semantico: o header se chama `X-Account-Id`, o Core V2 espera account, mas o provider global ainda le `activeTenantId` do store legado.
- Se `tenantId` e `accountId` forem sempre iguais no modelo atual, funciona por coincidencia de modelagem. Para evolucao enterprise, o nome e a fonte precisam ser alinhados.

### Auth no front

`web/app/stores/auth.ts`:

- Token fica no cookie `ldv_access_token` com `sameSite: 'lax'` e `maxAge` de 12h.
- Como o JS le o token para mandar `Authorization`, o cookie nao e `httpOnly`.
- `fetchContext()` usa `/v1/me/context`.
- `login()` chama `/v1/auth/login`, salva token e hidrata contexto.
- Ha persistencia de "lembrar login" em `localStorage` com email e senha. Isso e um risco alto de seguranca.

### Guards

- `auth.global.ts` protege rotas autenticadas, chama `ensureSession`, bloqueia `mustChangePassword` e carrega accounts.
- `module-enabled.global.ts` guarda rotas por modulo no front:
  - `tasks`: `/tasks`
  - `crm`: `/crm`, `/erp`
  - `site`: `/site/*`, `/manage/leads-web`, `/manage/produtos-web`
  - `queue`: operacao, consultor, ranking, dados, inteligencia, relatorios, multiloja, configuracoes, alertas, feedback
- O backend tambem tem guard por path, mas o mapa nao e identico. `site` aparece no front, mas nao foi visto registrado no backend atual.

### Realtime no front

- `useOperationsRealtime.ts` abre sockets por loja ativa ou por todas as lojas acessiveis em modo multiloja.
- `useContextRealtime.ts` abre socket de contexto por tenant.
- Eventos recebidos disparam refresh de stores como snapshots, overview, settings, alerts, CRM e goals.
- Reconnect com backoff ate 10s.
- O token e colocado em query string no URL do WebSocket.

## Infraestrutura local

- `docker-compose.yml` sobe Postgres, API e Web.
- API aplica migrations embedadas no boot.
- `CORE_V2_ENABLED=true` no compose dev.
- `AUTH_ROLES_SOURCE=core` no compose dev.
- `docker-compose.prod.yml` ainda tem aliases/nomeacoes antigas e defaults divergentes (`CORE_V2_ENABLED=false`, `AUTH_ROLES_SOURCE=core_with_fallback`).
- Sem evidencia, nesta analise, de observabilidade completa como Prometheus/OpenTelemetry, tracing distribuido, dashboards ou alertas de SLO.

## Riscos atuais mais importantes

1. Senha salva em `localStorage` no frontend.
2. Sessao/revogacao em `core.user_sessions` aparentemente nao ligada no boot.
3. `X-Account-Id` alimentado por `auth.activeTenantId` em vez do `activeAccountId` do Core Account Store.
4. Modulos `site`, `roadmap` e `operationgoals` existem, mas nao foram vistos registrados no bootstrap principal.
5. Front e back tem mapas diferentes de gating por modulo.
6. WebSocket usa token em query string.
7. Realtime e event bus em memoria nao escalam horizontalmente.
8. Sem RLS no Postgres.
9. Dedupe de GET no front nao considera query/opcoes.
10. Drift de env/prod: Core V2 off e fallback legado em producao, apesar de migrations removerem legado de roles.
