# AGENT - `back/internal/modules/operationgoals`

## Escopo

Metas de operacao por loja/consultor/mes (`queue.operation_goal_targets`). Expoe `/v1/operations/goals` (GET/POST/PUT/DELETE) para configurar e ler as metas usadas na operacao da Fila.

## Semana (migration 0198, 2026-07-10)

A tabela ganhou a coluna `week smallint` (0 = meta MENSAL/mes inteiro; 1..4 = semana do mes — S1=1-7, S2=8-14, S3=15-21, S4=22-fim). Os indices unicos incluem `week` (um registro por escopo por mes por semana). `GoalTarget.Week`/`GoalTargetView.week`/`CreateInput.Week` carregam a semana; `List` devolve TODAS as semanas do mes (o front particiona por `week`); `Create` grava a semana; `Update` NAO muda semana/mes/escopo. A leitura semanal + rateio da mensal (quando nao ha meta da semana) vive no CRM payout (`crm/erp/repository_crm_payout.go`: `crmPayoutTargetWeek` + `loadCRMGoalTargets(week)` + `weekProrationRatio`/`mergeWeekGoals`) — recebimento passa a ser apurado por semana. Valores editados so na tela Multi-loja/Metas (por semana via seletor de periodo).

## Estado real (P0.5, 2026-06-07)

Rotas REGISTRADAS no boot via `operationgoals.RegisterRoutes(mux, service, authMiddleware)` no `app.go`. Antes o modulo existia no codigo mas NAO era montado, e o front (`web/app/stores/operation-goals.ts` / `useContextRealtime`) recebia 404 ao chamar `/v1/operations/goals`.

Como o prefixo `/v1/operations` e gateado pelo modulo `queue` (`RequireModuleByPath`), a account precisa ter `queue` habilitado — o que e o default (seed 0124).

## Dependencias (construcao no app.go)

- `Repository` -> `operationgoals.NewPostgresRepository(pool)`.
- `StoreFinder` -> `storeService` (modulo `stores`), satisfaz `FindAccessible`/`ListAccessible`.
- `ContextPublisher` -> `realtimeService`, publica eventos de contexto pos-escrita.

## Seguranca

Escopo validado no service contra o Principal (tenant/store), seguindo o mesmo padrao dos demais modulos de queue. `account_id`/`tenant_id` nunca confiados do body sem checagem.

## Estrutura

- `http.go` — `RegisterRoutes`: rotas `/v1/operations/goals`.
- `service.go` — `NewService(repository, storeFinder, notifier)`, regras de escopo e validacao.
- `store_postgres.go` — `PostgresRepository` (List/FindByID/Create/Update/Delete + FindConsultantByID).
- `model.go` — `GoalTarget`, `Repository`, `StoreFinder`, `ContextPublisher`.
