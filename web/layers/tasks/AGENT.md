# AGENT — web/layers/tasks

## Escopo

Layer Nuxt `web/layers/tasks` — frontend do orquestrador de tarefas. Hoje em localStorage;
migrar para backend Go nas Fases T1–T5.

## Estado atual (localStorage)

- `web/layers/tasks/pages/tasks.vue` — protótipo monolítico (foi ~2955 linhas, agora dividido em sub-componentes)
- `web/layers/tasks/composables/useTasksWorkspace.ts` — CRUD localStorage, chave `omni.admin.tasks.workspace.v1`
- `web/layers/tasks/composables/useTimeTracking.ts` — timer localStorage, chave `tasks-tracking-v1`
- `web/layers/tasks/stores/session-simulation.ts` — simulação de RBAC (será deletado na Fase T5)
- `web/layers/tasks/composables/useTasksPageContext.ts` — contexto compartilhado via provide/inject (Fase T0.5)

## Backend disponível após Fase T1

- `back/internal/platform/database/migrations/0108_tasks_schema_foundation.sql` cria o schema `tasks.*` com 17 tabelas.
- `back/internal/modules/tasks/` já existe com `model`, `repository_postgres`, `service`, `service_tracking`, `service_dto`, `http`, `http_tracking`, `module`, `permissions` e `publisher`.
- O módulo `tasks` é registrado em `back/internal/platform/app/app.go` quando `CORE_V2_ENABLED=true`; nesse boot, `SyncCatalog` popula `core.modules`, `core.permissions` e `core.role_templates`, e o realtime é injetado como publisher.
- A T2 entregou `GET /v1/realtime/tasks`, `/presence` e `/notifications`, com topics `tasks:account`, `tasks:board`, `tasks:task`, `presence:board`, `presence:task` e `notifications:user`.
- O frontend ainda não deve consumir essa API diretamente antes da T5; a troca de localStorage para Pinia/API real é a próxima grande virada.
- Validação executada na T2: `go test ./...` em `back/`.

## Estrutura de componentes (após Fase T0.5)

```
web/layers/tasks/
  pages/tasks.vue                       thin wrapper: provide(TASKS_PAGE_CONTEXT_KEY) + sub-componentes
  composables/
    useTasksPageContext.ts               todo estado/lógica — provide/inject key
    useTasksWorkspace.ts                 CRUD localStorage (temporário, substituído na Fase T5)
    useTimeTracking.ts                   timer localStorage (substituído na Fase T6)
    useTasksRealtime.ts                  (Fase T2) WS para topics tasks:account + tasks:board
    useTaskPresence.ts                   (Fase T7) WS para presence:task:{id}, heartbeat 15s
    useTaskTracking.ts                   (Fase T6) server-backed, clockOffset
    useTaskRelations.ts                  (Fase T4) lazy load + cache cross-module
    useCan.ts                            (Fase T5) computed contra useMeContext().permissions
  stores/
    session-simulation.ts               simulação RBAC (deletar na Fase T5)
    tasks.ts                             (Fase T5) Pinia store real
  components/
    TasksFilterBar.vue                   toolbar: project selector, filtros, view toggle
    TasksBoardView.vue                   kanban: colunas + cards + drag/drop
    TasksTableView.vue                   wrapper OmniDataTable
    TasksProjectSettings.vue             USlideover: configuração da página
    TasksTaskModal.vue                   USlideover: detalhe/edição da task
    inputs/OmniSelectMenuInput.vue
    omni/table/OmniDataTable.vue
    editor/TasksRichEditor.vue
    AppDatePicker.vue
  types/
    tasks.ts                             TaskItem, TaskProjectItem, TaskBoardColumn, OrchestratorField
```

## Migração de localStorage → backend (Fase T5)

### Wipe do storage legado

```typescript
// boot da página detecta versão antiga e descarta com aviso single-shot
const LEGACY_KEY = 'omni.admin.tasks.workspace.v1'
if (import.meta.client && localStorage.getItem(LEGACY_KEY)) {
  localStorage.removeItem(LEGACY_KEY)
  // mostrar toast "Dados locais migrados para o servidor"
}
```

### Perspective derivada de permissões reais (não simulação)

```typescript
const perspective = computed(() =>
  meContext.permissions.includes('tasks.client_view') &&
  !meContext.permissions.includes('tasks.boards.manage')
    ? 'client_viewer'
    : 'agency'
)
```

### Componentes condicionais — NUNCA dados condicionais

```html
<!-- CORRETO: servidor retorna apenas boards com share para client_viewer -->
<TasksFilterBar v-if="!isClientView" />

<!-- ERRADO: receber todos os boards e filtrar no front -->
```

## Composables novos (Fases T2–T7)

### useTasksRealtime (Fase T2)

Clone de `web/app/composables/useOperationsRealtime.ts`:
- Tópicos: `tasks:account:{accountId}` e `tasks:board:{boardId}`
- Reconexão exponencial 1–10s com jitter
- Handler `applyRealtimeEvent(evt)` no Pinia store
- Backend já disponível em `GET /v1/realtime/tasks?scope=account|board|task&accountId=&boardId=&taskId=&access_token=...`

### useTaskPresence (Fase T7)

- Abre canal `presence:task:{taskId}` quando o modal abre
- Envia heartbeat a cada 15s
- Escuta `presence.snapshot`, `presence.user_joined`, `presence.user_left`, `presence.field_locked`
- Exibe avatar e badge "Fulano editando X" (sem bloquear input)
- Backend já disponível em `GET /v1/realtime/presence?scope=board|task&accountId=&boardId=&taskId=&access_token=...`

### useTaskTracking (Fase T6)

Server-backed com clock offset:
```typescript
const serverOffset = serverNow - clientNow  // calculado no evento task.time_started
const displayMs = computed(() =>
  durationMsFromServer + (Date.now() - localStartAt + serverOffset)
)
```

### useTaskRelations (Fase T4)

- Lazy load de metadados cross-module via `GET /v1/tasks/:id/relations:expand`
- Cache por `taskId` (TTL 60s)
- Re-fetch quando evento `task.relation_added` chegar no WS

### useCan (Fase T5)

```typescript
export function useCan(permissionKey: string) {
  const me = useMeContext()
  return computed(() => me.permissions.includes(permissionKey))
}
```

## Paginação (Fase T5)

- Tasks: cursor-based, `limit=50`, infinite scroll na table view
- Board view: até 200 tasks; paginação por coluna acima de 100
- `Cache-Control: private, max-age=15` em GETs de lista

## Regras de arquitetura

- Front nunca é fonte de autoridade para permissões — usar `useCan` contra dados do backend
- Modal e card sempre espelham os mesmos campos (memória feedback_modal_board_mirror)
- Optimistic updates: aplica local → dispatcha REST → reconcilia via WS (server-version vence)
- Não aceitar `account_id` ou `client_account_id` do localStorage — derivar do MeContext
