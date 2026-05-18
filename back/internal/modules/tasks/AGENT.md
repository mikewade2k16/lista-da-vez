# AGENT — módulo tasks

## Escopo

`back/internal/modules/tasks` — orquestrador de tarefas multi-tenant com RBAC dinâmico,
realtime e tracking server-side.

## Responsabilidade

- CRUD de boards, colunas, fields, views e tasks (escopo obrigatório por `account_id`)
- Tracking server-side: start/pause/resume/stop com persistência em `tasks.task_time_entries`
- Compartilhamento com cliente externo via `tasks.task_shares` (grant explícito)
- Cross-module: vincular tasks a recursos CRM/ERP/operations via `tasks.task_relations`
- Audit log server-side (não trigger DB — principal.UserID precisa estar disponível)
- Publicação de eventos WS via interface `Publisher` injetada pelo módulo realtime

## Estado atual (Fase T2 concluída em 2026-05-14)

- Migration `0108_tasks_schema_foundation.sql` criada com 17 tabelas em `tasks.*`.
- Módulo Go implementado em `back/internal/modules/tasks/` e registrado no Module Registry via `tasks.New(realtimeService)` quando `CORE_V2_ENABLED=true`.
- `SyncCatalog` passa a declarar 13 permissões e 3 role templates (`tasks.admin`, `tasks.member`, `tasks.client_viewer`).
- Endpoints REST base existem para boards, columns, fields, tasks, comments, shares, relations, audit e tracking inicial.
- Realtime está plugado via interface `Publisher`: mutations REST publicam eventos em `tasks:account`, `tasks:board` e `tasks:task`.
- Presence e notifications têm transporte WS no módulo `realtime`; o frontend ainda consome localStorage até a T5.
- Validação executada: `go test ./...` em `back/`.

## Não é responsabilidade deste módulo

- Autenticação e sessão (módulo auth)
- Transporte WebSocket (módulo realtime)
- Entrega de notificações (módulo notifications)
- Resolução de metadados de recursos externos (RelationResolver — plataforma modules/)

## Estrutura de arquivos esperada

```
back/internal/modules/tasks/
  AGENT.md
  model.go              Board, Column, Field, Task, TimeEntry, Comment, Relation, Share, Perspective
  errors.go             ErrBoardNotFound, ErrTaskNotFound, ErrVersionConflict, ErrShareRequired, ErrForbidden
  permissions.go        constantes das 13 permission keys
  module.go             implementa modules.Module (Permissions, RoleTemplates, Build)
  service.go            CRUD boards/tasks/comments + audit
  service_tracking.go   StartTracking, PauseTracking, ResumeTracking, StopTracking, ActiveEntries, AggregateMetrics
  service_dto.go        BuildTaskDTO(task, perspective) — shapes diferentes por perfil
  repository_postgres.go scopedQuery + todos os métodos CRUD
  http.go               registro de rotas REST (boards/tasks/comments/shares/relations)
  http_tracking.go      handlers de tracking isolados
  ports.go              AccessContext, Perspective, RelationResolver
  publisher.go          interface Publisher (injetada pelo realtime)
```

## Contrato HTTP

Todos os endpoints exigem `Authorization: Bearer` + `X-Account-Id`.

### Boards / Colunas / Fields

```
GET    /v1/tasks/boards
POST   /v1/tasks/boards
GET    /v1/task-boards/:boardId          detalhe completo do board
PATCH  /v1/tasks/boards/:boardId
POST   /v1/tasks/boards/:boardId/columns
PATCH  /v1/tasks/columns/:columnId
DELETE /v1/tasks/columns/:columnId        body: { remapToColumnId }
POST   /v1/tasks/boards/:boardId/fields
```

**Trava T5 (2026-05-14):** nao reintroduzir `GET /v1/tasks/boards/:boardId` no Go `ServeMux` atual. Essa rota conflita com rotas parametrizadas como `GET /v1/tasks/:taskId/comments`, pois ambas podem casar caminhos como `/v1/tasks/boards/comments`. O detalhe de board fica em `GET /v1/task-boards/:boardId` ate o desenho de rotas ser reorganizado.

### Tasks

```
GET    /v1/tasks/boards/:boardId/tasks    cursor-based, filtros forçados por perspective
POST   /v1/tasks/boards/:boardId/tasks
GET    /v1/tasks/:taskId
PATCH  /v1/tasks/:taskId                  header If-Match: <version>
POST   /v1/tasks/:taskId/move             { columnId, sortOrder }
DELETE /v1/tasks/:taskId                  soft delete (archived=true)
POST   /v1/tasks/:taskId/comments
POST   /v1/tasks/:taskId/shares           { clientAccountId, permission }
```

**Metadata visual da task (2026-05-15):** `tasks.tasks.ui_metadata` persiste os campos que ainda nao viraram field values dedicados no backend: `responsible`, `involved`, `clientId`, `clientName`, `type`, `dueEndDate`, `prioritySet` e `createdBy`. O DTO de task deve sempre devolver `uiMetadata`, mesmo `{}`, para impedir que o front use cache local antigo como fonte autoritativa entre usuarios.

**Paginacao cursor-based em ListTasks (T5 — fechamento 2026-05-15):** o repository usa keyset pagination
sobre a tupla `(sort_order, created_at, id)` — ordem total estavel mesmo quando varias tasks
compartilham `sort_order`/`created_at`. O cursor exposto na API e' opaco (`base64url(JSON{s,c,i})`).
A query pede `limit+1` rows para detectar `hasMore` sem segunda chamada; se vier o extra, a
ultima e' descartada e o `nextCursor` aponta para a anterior. Response: `{ tasks, nextCursor }`
— `nextCursor` vazio = fim da paginacao. `ListTasksResult` em `service.go` carrega ambos.
Nao expor `sort_order`/`created_at` direto na URL — manter cursor opaco para permitir mudar a
estrategia (ex: adicionar filtro) sem quebrar URLs salvas.

**Hardening T8 (2026-05-15):**

- **Rate limit REST**: `httpapi.RateLimit(httpapi.DefaultRESTRateLimit)` aplicado globalmente em
  `app.go` antes do middleware de Logging — 60 req/min por (user_id, fallback IP). Quando
  excedido, devolve `429` com header `Retry-After: <segundos>`. Token bucket in-memory com
  cleanup a cada 5 min; aceitavel ate broker externo (multi-replica) em T9.
- **slog em mutations**: `service.audit(...)` ja era o ponto unico chamado por todas as 13
  mutations (CreateBoard, UpdateBoard, CreateColumn, UpdateColumn, DeleteColumn, CreateField,
  CreateTask, UpdateTask, MoveTask, DeleteTask, AddComment, AddShare, AddRelation). Agora ele
  tambem dispara `service.logMutation` -> `slog.LogAttrs(LevelInfo, "tasks.mutation", ...)` com
  `action`, `account_id`, `user_id`, `resource_type`, `resource_id`. Persistencia segue em
  `tasks.tasks_audit` via `repository.InsertAuditEntry`.
- **404 vs 403**: `ResolveAccessContext` retorna `ErrAccountNotFound` (mapeado para 404) sempre
  que o user nao e' membro da account ou ela nao existe. `ErrForbidden` (403) fica reservado
  para "user na account certa mas falta a permissao especifica". `scopedQuery(accountID, ...)`
  no repository garante que recursos de outras accounts nunca aparecem (vira `pgx.ErrNoRows` ->
  `ErrTaskNotFound`/`ErrBoardNotFound`). Nao misturar os dois.
- **accountId nunca vem do body**: `withPermission` em `http.go` resolve a partir de `X-Account-Id`
  (header), `accountId` (query) e `principal.TenantID` (fallback) — *nessa ordem*. Inputs com
  campo `AccountID` JSON sao ignorados pelo service. Para alterar a account de uma mutation, o
  cliente DEVE trocar o header.
- **Logger injetado via `deps.Logger`**: `module.Build(...)` chama `service.SetLogger(deps.Logger)`.
  Quando o Module Registry nao for usado (tests, scripts), o servico cai para `slog.Default()`.

**Quando implementar `RemoveRelation` (sem rota DELETE ainda):** lembrar de publicar
`task.relation_removed` no `Publisher.PublishTaskEvent`. O composable front `useTaskRelations`
ja escuta esse evento para invalidar cache.

## Testes (T9 — fechada 2026-05-15)

Cobertura completa adicionada na T9 (todos os testes Go rodam sem Postgres real, via mocks):

### Backend Go (50 testes novos)

- `dto_test.go` (4) — `BuildTaskDTO` agency vs client_viewer; uiMetadata sempre nao-nil; ISO dates.
- `cursor_test.go` (4) — round-trip do `listTasksCursor`; URL-safe; decode invalido nao panica.
- `service_test.go` (10) — CreateTask happy/no-perm/validation; GetTask perspective + 404; ListTasks default-limit/clamp/no-perm/perspective/nextCursor.
- `scope_test.go` (8) — accountID vazio, account 404, **cross-account 404 (nunca 403)**, platform_admin bypass, client_viewer perspective, manage override, **fuzz 100 IDs cross-account = 100% 404**, `scopedQuery` panica sem accountID.
- `tracking_test.go` (8) — no-perm/task-not-found, happy-path publica WS + audita, **ErrVersionConflict propaga (409)**, expectedVersion passa intacto, StopTracking 404 nao publica nem audita.
- `realtime/presence_test.go` (6) — user_joined unico, **LockField exclusivo (T7.2)**, owner reclaim, UnlockField publica, Leave decrementa, TTL expira.
- `httpapi/rate_limit_test.go` (6) — dentro do limite passa, 429+Retry-After, resolver custom precede IP, fallback X-Forwarded-For, janela reseta.

### Mock leve

`repository_mock_test.go` — satisfaz `Repository` (30+ metodos) com hooks `onXxx` opcionais; captura passiva de `auditEntries`. Cada teste sobrescreve so o que precisa.

### Frontend Vitest (9 testes)

- `web/layers/tasks/utils/text.ts` extraido com `clampText`/`normalizeText` (re-export no `useTasksPageContext`).
- `web/layers/tasks/utils/text.test.ts` — 9 testes cobrindo trim/colapso/clamp, **`clampText('palavra ')` preserva o espaco (T7.2)**.
- Vitest 2.1 instalado em `web/package.json` (dev); scripts `npm test` / `npm run test:watch`.
- `capturingPublisher` em `tracking_test.go` — Publisher mockado que captura `events`/`boards`/`presences` para asserts sem WebSocket real.

### Pendencias futuras (T9.1 / nao bloqueantes)

- **Integration tests com Postgres real** — alternativa pesada aos mocks. Smoke E2E manual cobre na pratica.
- **`@nuxt/test-utils` + `happy-dom`** — para testar composables Vue completos (`useTaskRelations` com fetch mockado, `useTaskPresence` com WebSocket fake, `useTasksRealtime` reconnect+jitter). Por enquanto, util puros (`clampText`/`normalizeText`) cobrem o caso critico T7.2.
- **Roteiro smoke E2E 12 passos** — documentado em `docs/TASKS_ORCHESTRATOR_PHASE12.md` para o usuario rodar manualmente em staging.

### Tracking

```
GET    /v1/tasks/tracking/active
POST   /v1/tasks/:taskId/tracking/start
POST   /v1/tasks/:taskId/tracking/pause   If-Match
POST   /v1/tasks/:taskId/tracking/resume  If-Match
POST   /v1/tasks/:taskId/tracking/stop    If-Match
GET    /v1/tasks/tracking/metrics         accountId, clientAccountId, userId, from, to
```

### Relations

```
GET    /v1/tasks/:taskId/relations
POST   /v1/tasks/:taskId/relations
GET    /v1/tasks/:taskId/relations:expand
```

### Audit

```
GET    /v1/tasks/:taskId/audit            perm tasks.boards.manage
```

## Regras de escopo (defense in depth — 3 camadas)

1. **Middleware HTTP**: `requirePermission("tasks.tasks.view")` antes de qualquer handler
2. **Service**: `principal.AccountID == task.AccountID || share exists`
3. **Repository**: `WHERE account_id = $1` injetado por `scopedQuery`

```go
func (r *PostgresRepository) scopedQuery(accountID string, baseSQL string, args ...any) (string, []any) {
    if strings.TrimSpace(accountID) == "" {
        panic("tasks: scopedQuery called without accountID")
    }
    return baseSQL, append([]any{accountID}, args...)
}
```

**Regra**: cross-account retorna **404**, nunca 403 (não vazar existência de recurso).

## Perspectives e DTO mínimo

```go
type Perspective string
const (
    PerspectiveAgency       Perspective = "agency"
    PerspectiveClientViewer Perspective = "client_viewer"
)
```

`BuildTaskDTO(task, perspective)` decide o shape:

| Campo                    | agency | client_viewer |
|--------------------------|--------|---------------|
| id, title, status, etc.  | ✅     | ✅ se share permite |
| client_account_id        | ✅     | ❌ omitido |
| tracking total           | ✅     | ❌ (a menos que tasks.tracking.view_all) |
| audit_log                | ✅     | ❌ |
| assignees                | ✅     | apenas os marcados como "compartilhar" |
| tasks de outros clientes | listadas | **nunca aparecem** (filtro SQL) |

## Permissões (13 keys)

| Key                       | Notas |
|---------------------------|-------|
| `tasks.boards.view`       | Lista boards |
| `tasks.boards.manage`     | CRUD board/columns/fields |
| `tasks.tasks.view`        | Default para membros |
| `tasks.tasks.create`      | |
| `tasks.tasks.edit`        | Inclui mover |
| `tasks.tasks.delete`      | Soft delete |
| `tasks.tasks.assign`      | |
| `tasks.tasks.comment`     | |
| `tasks.tracking.use`      | Tracking próprio |
| `tasks.tracking.view_all` | Tracking de outros + métricas |
| `tasks.relations.manage`  | Vincular cross-module |
| `tasks.shares.manage`     | Compartilhar com cliente externo |
| `tasks.client_view`       | Visão limitada do cliente externo |

## Role templates

- `tasks.admin` — todas exceto `tasks.client_view`
- `tasks.member` — view/create/edit/comment/assign/tracking.use/relations.manage
- `tasks.client_viewer` — apenas `tasks.client_view` + `tasks.tasks.comment` (limitado pela share)

## Eventos WS publicados

Via `Publisher.PublishTaskEvent` / `PublishBoardEvent`:

```
task.created, task.updated, task.moved, task.deleted, task.assigned
task.comment_added, task.relation_added, task.relation_removed
task.share_added, task.share_revoked
task.time_started, task.time_paused, task.time_resumed, task.time_stopped
board.column_added, board.column_updated, board.column_deleted
```

Payload: leve, orientado a invalidação (front busca snapshot via REST).

## Padrão Go

Seguir `back/internal/modules/operations/` e `back/internal/modules/core/`:
- UUID como `string`; scan nullable com `*string`
- Sem `github.com/google/uuid` no service
- Permissões catalogadas no DB via `core.permissions` (SyncCatalog no boot)
- `onMounted`: lifecycle hooks apenas nas camadas HTTP/module, não no service/repository

## Evolução esperada

1. Fase T3: NotificationDispatcher injetado pelo notifications (noop se módulo ausente)
2. Fase T4: RelationResolver registry para expand cross-module
3. Fase T5: Front substitui localStorage por este backend e abre os canais WS
4. Fase T10: Sistema de views (11 tipos) com endpoints /views/:id/data por layout
