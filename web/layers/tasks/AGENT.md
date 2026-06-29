# AGENT — web/layers/tasks

## Escopo

Layer Nuxt `web/layers/tasks` — frontend do orquestrador de tarefas. Hoje em localStorage;
migrar para backend Go nas Fases T1–T5.

## Estado atual (localStorage)

- `web/layers/tasks/pages/tasks.vue` — protótipo monolítico (foi ~2955 linhas, agora dividido em sub-componentes)
- `web/layers/tasks/composables/useTasksWorkspace.ts` — CRUD localStorage, chave `omni.admin.tasks.workspace.v1`
- `web/layers/tasks/composables/useTimeTracking.ts` — timer localStorage, chave `tasks-tracking-v1`
- `web/layers/tasks/stores/tasks-client.ts` — store de contexto do cliente (userType/clientId derivados do `useAuthStore().role`). Substituiu o antigo store de simulação de sessão (shim de re-export removido em 2026-06-04, critério 5 da multitenant-completion). `clientId`/`clientOptions` ainda são placeholders até a API de contatos do CRM.
- `web/layers/tasks/composables/useTasksPageContext.ts` — contexto compartilhado via provide/inject (Fase T0.5)

## Backend disponível após Fase T1

- `back/internal/platform/database/migrations/0108_tasks_schema_foundation.sql` cria o schema `tasks.*` com 17 tabelas.
- `back/internal/modules/tasks/` já existe com `model`, `repository_postgres`, `service`, `service_tracking`, `service_dto`, `http`, `http_tracking`, `module`, `permissions` e `publisher`.
- O módulo `tasks` é registrado em `back/internal/platform/app/app.go` quando `CORE_V2_ENABLED=true`; nesse boot, `SyncCatalog` popula `core.modules`, `core.permissions` e `core.role_templates`, e o realtime é injetado como publisher.
- A T2 entregou `GET /v1/realtime/tasks`, `/presence` e `/notifications`, com topics `tasks:account`, `tasks:board`, `tasks:task`, `presence:board`, `presence:task` e `notifications:user`.
- O frontend ainda não deve consumir essa API diretamente antes da T5; a troca de localStorage para Pinia/API real é a próxima grande virada.
- Validação executada na T2: `go test ./...` em `back/`.

Status 2026-05-15: o frontend ja consome API/WS real para Tasks; nao voltar para localStorage como fonte principal.

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
    tasks-client.ts                      contexto do cliente (userType/clientId) derivado do auth real
    tasks.ts                             (Fase T5) Pinia store real
  components/
    TasksFilterBar.vue                   toolbar: project selector, filtros, view toggle
    TasksBoardView.vue                   kanban: colunas + cards + drag/drop
    TasksTableView.vue                   wrapper OmniDataTable
    TasksProjectSettings.vue             USlideover: configuração da página
    TasksTaskModal.vue                   detalhe/edição da task — usa OmniEntityDrawer (template-core)
    inputs/OmniSelectMenuInput.vue
    omni/table/OmniDataTable.vue
    AppDatePicker.vue
  app/components/omni/
    OmniEditor.vue                       editor compartilhado com slash menu, bubble tools e drag handle
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
    : 'agency',
)
```

### Componentes condicionais — NUNCA dados condicionais

```html
<!-- CORRETO: servidor retorna apenas boards com share para client_viewer -->
<TasksFilterBar v-if="!isClientView" />

<!-- ERRADO: receber todos os boards e filtrar no front -->
```

### Travas de regressao T5 (2026-05-14)

Antes de marcar T5 como concluida, validar no navegador real:

- criar task no board;
- abrir modal;
- definir prazo/data;
- salvar;
- arrastar para outra coluna;
- recarregar a pagina;
- confirmar titulo, status, prazo e nome do responsavel preservados.

Cuidados obrigatorios:

- nao montar colunas fallback (`column-todo`, `column-doing`, etc.) quando a API real tem board criado; buscar detalhe do board primeiro;
- usar `GET /v1/task-boards/{boardId}` para detalhe de board. Nao reintroduzir `GET /v1/tasks/boards/{boardId}` no Go `ServeMux` atual, pois conflita com rotas `GET /v1/tasks/{taskId}/...`;
- preservar aliases de field keys: `due_date -> dueDate`, `client_id -> clientId`, `created_at -> createdAt`, `created_by -> createdBy`;
- garantir que `TasksBoardView.vue` receba `onDropCard` do contexto quando o template usar `@drop`;
- chamadas que criam task e dependem do `id` precisam de `await`;
- nome de usuario deve priorizar `displayName` antes de `email`;
- card nao mostra campos vazios; `Sem data`/`Empty` so pode aparecer dentro do editor/draft;
- `responsavel` e `envolvidos` nao podem usar `creatable`; pessoas vem do modulo de usuarios e o responsavel deve ser removido de envolvidos;
- usuario/cliente renderizam como avatar + nome com borda opcional; status/tipo/prioridade continuam como badge preenchido;
- prazo precisa manter inicio e fim (`dueDate` + `dueEndDate`) e hora ao lado da data;
- campos visuais do card (`responsible`, `involved`, `clientId`, `clientName`, `type`, `dueEndDate`, `prioritySet`, `createdBy`) sao server-backed em `tasks.tasks.ui_metadata`; nao voltar a usar `omni.tasks.api.workspace.ui.v1` como fonte autoritativa desses dados.
- enquanto o backend nao persistir views/filtros equivalentes, manter a ponte local `omni.tasks.api.workspace.ui.v1` apenas para configuracao de workspace/view.

## Composables novos (Fases T2–T7)

### useRealtimeSocket — base de WS compartilhada (refactor 2026-06-29)

`composables/useRealtimeSocket.ts` concentra a maquina de ciclo de vida do WebSocket que antes
estava DUPLICADA em `useTaskPresence.ts` e `useTasksRealtime.ts`: `desiredConnection` (resolucao de
conta via `resolveRealtimeAccountId` — switcher v2 primeiro, fallback `auth.activeTenantId`/`principal.tenantId`/
`tenantContext[0].id` — + montagem da key `${scope}:${accountId}:${boardId}:${taskId}:${accessToken}`),
`scheduleReconnect` (backoff exponencial 1–10s), `silencedSockets` (isolamento por conta: fecha o
socket antigo sem disparar reconexao), `ensureConnection` (handshake via ticket + open) e
`updateStatus`. Tambem exporta os helpers `sourceValue` e `resolveRealtimeAccountId`.

Cada consumidor so passa `path`/`scope`/`isValid` + hooks (`onOpen`, `onMessage`, `onSocketClosed`,
`onError`, `onDisconnect`, `onBeforeConnect`, `preserveOnReconnect`, `logClose`, `logTicketError`):

- **useTasksRealtime**: stateless — `onMessage` so encaminha o evento parseado (`applyEvent`); sem
  heartbeat/draft/snapshot.
- **useTaskPresence**: heartbeat 15s (`startHeartbeat` no `onOpen`), `draftTimers`, snapshot de
  participantes; `onDisconnect` limpa heartbeat/drafts e (quando `clearState`) zera participantes;
  `preserveOnReconnect` mantem `activeFieldKey`/draft local quando reconecta na mesma key;
  `onBeforeConnect` zera participantes antes de reabrir; o `watch`/disconnect-no-unmount agora vivem
  no base, e o composable so registra os listeners proprios (`visibilitychange`/`pagehide`).

Comportamento de reconexao/isolamento por conta e a API publica de retorno dos dois composables
ficaram IDENTICOS (refactor puro). O `normalizeText` dos dois composables agora importa de
`utils/text.ts` (eram copias locais byte-a-byte); o re-export em `useTasksPageContext.ts` segue.

### useTasksRealtime (Fase T2)

Clone de `web/app/composables/useOperationsRealtime.ts`:

- Tópicos: `tasks:account:{accountId}` e `tasks:board:{boardId}`
- Reconexão exponencial 1–10s com jitter
- Handler `applyRealtimeEvent(evt)` no Pinia store
- Backend já disponível em `GET /v1/realtime/tasks?scope=account|board|task&accountId=&boardId=&taskId=&access_token=...`

Status 2026-05-16: `useTasksRealtime.ts` esta ligado em `useTasksPageContext.ts` com dois canais: `account` para mudancas gerais/lista de boards e `board` para o board ativo. Manter o canal de board e obrigatorio para usuarios em contas/escopos diferentes verem a mesma task compartilhada. Nao confundir com `useTaskPresence`: presence continua sem persistir/hidratar task, mas agora tambem carrega draft efemero (`presence.field_draft`) para preview ao vivo de campos travados.

**T7.2 — refresh full debounced:** a tentativa de patch local com `tasksWorkspace.hydrateTask(taskId)` foi revertida em 2026-05-15. O handler de realtime segue o padrao de operations: qualquer evento `task.*`, `board.*` ou `field.*` agenda `tasksWorkspace.refresh()` com debounce de 200ms. Nao reintroduzir hydrate por evento sem prova forte, porque ele quebrou sincronizacao quando a task remota nao existia no store local.

### useTaskPresence (Fase T7)

- Abre canal `presence:task:{taskId}` quando o modal abre e `presence:board:{boardId}` enquanto o board esta ativo
- Envia heartbeat a cada 15s
- Escuta `presence.snapshot`, `presence.user_joined`, `presence.user_left`, `presence.field_locked`, `presence.field_draft` e `presence.field_unlocked`
- Exibe avatar e badge "Fulano editando X" e bloqueia o mesmo campo quando outro usuario esta nele
- Para campos textuais (`title` e editor rico em `description`), mostra o draft remoto ao vivo com debounce curto no mesmo input/editor travado; persistencia continua via autosave/REST normal.
- Status 2026-05-15: `useTaskPresence.ts` implementado para modal e board com heartbeat 15s, reconexao simples, filtro do usuario atual, avatares de outros participantes, aviso visual de campo em edicao e lock de campo remoto. WebSocket nao deve ir direto para componentes.
- Backend já disponível em `GET /v1/realtime/presence?scope=board|task&accountId=&boardId=&taskId=&access_token=...`

**T7.2 — presence sem guard local:** `useTaskPresence.focusField()` sempre libera o campo ativo anterior e envia `field_focus`; o servidor (`presence.go:LockField`) e a UI `:disabled` sao as fontes de verdade para lock exclusivo. Reconnect no mesmo canal preserva `activeFieldKey` para reenviar o lock ao abrir. Nao voltar com guard client-side baseado em `usersForField()`: estado local defasado pode impedir uma aba de anunciar presença e criar assimetria entre usuarios.

**T7.1 — nick em mascaras:** `currentUserName` e `directoryUserLabels` priorizam `user.nick` (vazio = fallback `displayName`/`name`/`fullName`/`email`). O badge "Fulano editando..." e os selects de Responsavel/Envolvidos mostram nick quando preenchido. Backfill do nick e' manual via SQL ate haver UI dedicada.

### Inputs inline e digitacao com espaco (T7.2)

`useTasksPageContext.ts` expoe dois helpers de texto:

- `normalizeText(value, max)`: faz `.replace(/\s+/g,' ').trim().slice(0,max)`. Usar em flush/autosave e em situacoes onde o valor sera persistido.
- `clampText(value, max)`: apenas `String(value).slice(0,max)`, sem trim/colapso. Usar em `@update:model-value` de inputs controlados — `<UInput :model-value :update:model-value="...clampText(...)">`. Sem ele, o cursor "salta" porque o `model-value` re-renderiza com o valor trimado a cada keystroke (impossivel digitar espaco no final).

O store `tasks.ts` mantem a mesma distincao: optimistic update do titulo usa `clampText`; o `requestBody.title` enviado ao backend usa `normalizeText`; quando o response volta com a versao trimada e o local diverge apenas em whitespace, o store preserva o local pra nao truncar o que o user esta digitando.

### useTaskRelations (Fase T5 — fechamento)

- Carrega vinculos cross-module (crm/erp/operations) via `GET /v1/tasks/{taskId}/relations:expand` (backend T4 ja entrega com cache 60s no `task_relations`)
- Cache local por `taskId` em mapa reativo; primeira abertura do modal dispara o fetch, reaberturas reusam.
- Listener de `tasksRealtime.lastEvent` invalida o cache quando o canal `tasks` publica `task.relation_added` ou `task.relation_removed` para a task ativa (refetch automatico).
- Modal renderiza secao "Vinculos" entre Comments e o editor rico, com icone por modulo, label, tipo do recurso, badge de status (`active`/`unknown`/...) e botao de abrir URL externa quando `metadataCache.url` existe.
- Backend ainda NAO publica eventos `task.relation_added/removed` no realtime — quando publicar, o composable ja escuta. Por enquanto, invalidacao manual (`refresh()`) ou reabertura do modal.

### Paginacao cursor-based de tasks (Fase T5 — fechamento)

- Backend `GET /v1/tasks/boards/{boardId}/tasks?limit=&cursor=&archived=` agora responde `{ tasks, nextCursor }`. Cursor opaco (base64url de `{s:sort_order, c:created_at, i:id}`); keyset stable mesmo com ties.
- Store `listBoardTasks` itera `nextCursor` em loop interno (100/page, hard cap 100 paginas) ate esgotar — board kanban precisa de tudo.
- Tabela view futura (T5.1) pode lazy-load via `fetchMoreTasks(boardId, cursor)` separado para infinite scroll. Por enquanto, mesma logica do board.
- Nao confundir com `task_doc_snapshots` (T7 future-yjs) — cursor de tasks e' independente do cursor de blocos no editor.

### Lazy-load por board + skeleton (performance §6 — 2026-06-10)

Boot da pagina de Tasks nao pode bloquear ate carregar TODOS os boards e TODAS as tasks. Regra:

- **Skeleton imediato**: `pages/tasks.vue` ja renderiza skeleton enquanto `pageBootstrapping` (inicia `true` em `useTasksPageContext`); a primeira pintura nao espera a API.
- **Skeleton tambem na TROCA de board** (fim do flash de "Sem tasks" — 2026-06-26): `activeBoardLoading` (computed em `useTasksPageContext` = `pageBootstrapping || !loadedBoardIds.has(activeBoardId)`) cobre tanto o boot quanto o lazy-load ao trocar de board. `TasksBoardView` mostra `CoreSkeleton` nas colunas e `TasksTableView` passa `:loading="activeBoardLoading"` em vez do estado vazio enquanto o board ativo carrega.
- **Above-the-fold primeiro**: `store.refresh()` carrega a lista de boards + detalhes de todos, mas busca as **tasks somente do board ativo**. Os demais boards ficam lazy.
- **Lazy por board**: `setActiveProject(id)` dispara `ensureBoardTasksLoaded(id)` (fire-and-forget) para buscar as tasks do board que virou ativo. `loadedBoardIds`/`archivedBoardIds` (refs `Set<string>`) registram o que ja foi carregado e evitam refetch. Trocar de board faz `force` para garantir dados frescos; aborta o fetch do board anterior via `AbortController` (`boardLoadControllers`).
- **Arquivadas sob demanda**: `listBoardTasks` agora usa `archived=false` por padrao. Quando o usuario desativa "ocultar arquivadas" (`filters.hideArchived = false`), um watcher em `useTasksPageContext` chama `ensureArchivedTasksLoaded(activeBoardId)` (que faz `force` com `includeArchived`). Nao recarrega os outros boards.
- **Refresh do realtime**: o `refresh()` debounced preserva verbatim as tasks de boards nao-ativos ja carregados (sem refetch nem remap, pra nao perder labels resolvidos do backend); so o board ativo e' re-buscado.
- Novos metodos publicos do store: `ensureBoardTasksLoaded(boardId, { includeArchived, force })`, `ensureArchivedTasksLoaded(boardId)`; novos refs: `loadedBoardIds`, `archivedBoardIds` (expostos tambem em `useTasksWorkspace`).
- Cuidado: `normalizeLegacyDefaultProject` agora chama `ensureBoardTasksLoaded` antes de decidir reescrever colunas legadas (board nao-ativo pode ter tasks ainda nao carregadas).

### Modal-template unico (2026-06-26)

O `TasksTaskModal.vue` deixou de ser um `USlideover` proprio e passou a montar sobre o
TEMPLATE-CORE de modal `web/app/components/ui/OmniEntityDrawer.vue` (header fechar/expandir-toggle/
popover de modo, resize no modo lado, modos lado/centro/fullscreen, Esc, overlay). O corpo da task,
as acoes de header especificas (presenca/Compartilhar/link/estrela/mais) e o rodape (Excluir +
autosave + Fechar) vivem nos slots `default`/`#header-extra`/`#footer`. Modo e largura sao do shell,
espelhados no contexto via `v-model` (`taskEditorMode`/`taskEditorWidth`); o board encolhe pela var
`--tasks-editor-width` (escrita por `useTasksPageContext` a partir de `taskEditorWidth`). O
`setTaskEditorMode` segue exposto (o `RoadmapModulesBoard` abre a task em modo `center`). Ajustes do
comportamento do modal sao feitos no `OmniEntityDrawer` (um lugar so). Ver `docs/frontend/MODAL_TEMPLATE.md`.

### Montagem tardia dos selects do card (Track C perf — 2026-06-15)

O board renderiza ate ~250 cards (Crow tem 247). Cada card montava 5-6 `OmniSelectMenuInput`
(status, responsavel, envolvidos, cliente, tipo, prioridade) de uma vez no render. Cada
`OmniSelectMenuInput` e' um `USelectMenu`/combobox pesado (reka-ui: popover + listbox + busca +
cadeias de `computed`). Montar tudo na primeira pintura travava a rota /tasks (cold T3 15s+, nunca
"settlava"). Solucao de RENDER (nao muda regra de negocio):

- **`OmniLazySelectMenuInput.vue`** (novo wrapper): antes de interagir, renderiza so um badge
  estatico leve com o valor formatado (mesma cor/estilo que o editor usaria). No primeiro
  clique/foco (ou Enter/Espaco no teclado) monta o `OmniSelectMenuInput` real e ja o abre
  (`:open`), entao um unico clique vira o editor com o dropdown aberto. Repassa TODAS as props e
  eventos 1:1 via `v-bind="$attrs"` + `defineOptions({ inheritAttrs: false })` —
  `update:modelValue`, `create`, `update:open` (presence focus/blur) seguem identicos. Depois de
  montado, o editor permanece montado (custo ja pago).
- **`option-colors.ts`** (novo util): paleta de cores/labels das options extraida do
  `OmniSelectMenuInput` para ser FONTE UNICA. O placeholder do wrapper e o editor real resolvem
  cor/label da mesma forma (inclusive options renomeadas/recoloridas via `useState`
  `__omni_select_menu_option_meta__`), entao o badge nao "pisca" ao trocar placeholder pelo editor.
- **Espelhamento modal/board**: `TasksBoardView.vue` (6 selects de display do card) e
  `TasksTaskModal.vue` (6 selects de propriedade) usam o wrapper. O draft-card (criacao inline,
  so 1 por vez) continua com `OmniSelectMenuInput` direto, pois ja e' um contexto de edicao ativo
  com wiring de `:open` controlado.
- **NAO quebra**: drag-and-drop (cards continuam montados no DOM, so os selects sao adiados),
  edicao inline, realtime, presence/lock, tracking — tudo intacto. O `AppDatePicker` NAO virou
  lazy: o conteudo pesado (calendario) ja fica no `<template #content>` do `UPopover` (so monta ao
  abrir) e adia-lo arriscaria a formatacao de data/range; o ganho dominante vem dos 6 selects.
- **Windowing por viewport**: avaliado e NAO aplicado — render real por viewport removeria cards do
  DOM e quebraria os alvos de drag-and-drop e a presence/realtime (que precisam dos cards montados).
  Mantido o render progressivo ja existente em `TasksBoardView.vue` (`INITIAL_RENDER` above-the-fold
  - lotes via `requestIdleCallback`), que agora pinta cards muito mais baratos.
- **Re-medicao**: validar com `perf_audit.py --only /tasks --base-url http://localhost:3055` apos
  subir o build de prod. Alvo: cold T3 < 3s ou placeholder/skeleton imediato e interativo rapido.
- Validacao local desta entrega: `vue-tsc` sem novos erros nos arquivos tocados (o unico erro em
  `TasksBoardView.vue:793` e' do `creatable` do draft, pre-existente); `vitest run layers/tasks`
  15/15; `eslint` limpo nos arquivos novos; `npm run dev` compila `/tasks` (HTTP 200, sem warning
  de componente nao resolvido).

### Conta ativa via switcher v2 (CoreAccountSwitcher) — 2026-06-15

O Tasks resolve o `X-Account-Id` a partir do `useCoreAccountStore().activeAccountId` (switcher v2),
o mesmo store que o `web/app/plugins/account-id-bridge.client.ts` injeta no api-client global de
queue/crm. Antes o Tasks lia `auth.activeTenantId` (legado, setado no login e que NAO muda ao trocar
de conta no `CoreAccountSwitcher`), entao trocar de conta nao recarregava o board.

- Fonte do account id (em `stores/tasks.ts` e `composables/useTimeTracking.ts`):
  `accountStore.activeAccountId || auth.activeTenantId || auth.tenantContext?.[0]?.id`. O fallback
  legado cobre o periodo do boot antes de `activeAccountId` estar populado.
- `accountId` continua sendo um `computed` reativo. O store registra um `watch(accountId, ...)` que,
  ao mudar para um valor novo e nao-vazio (ignorando o primeiro setup e transicoes para vazio como
  logout), chama `reloadForAccountSwitch()`: aborta os fetches em voo (padrao existente de
  `boardLoadControllers`/`AbortController`), limpa `loadedBoardIds`/`archivedBoardIds`/`tasks`/
  `projects`/`activeProjectId`, zera `initialized` e refaz `initialize()`. Assim o board recarrega
  com os dados da nova conta sem respostas obsoletas sobrescrevendo o estado.
- NAO alterar o `account-id-bridge.client.ts` (queue/crm ja seguem o switcher por ele).
- **Realtime/presence DEVE usar a MESMA fonte (2026-06-25):** os WS de `useTasksRealtime` e
  `useTaskPresence` recebem o `accountId` via `useTasksPageContext`. Esse prop tem que espelhar o
  REST — `accountStore.activeAccountId || auth.activeTenantId || auth.tenantContext?.[0]?.id` — num
  unico `computed realtimeAccountId` reutilizado pelos quatro canais (task/board presence + account/
  board tasks). Bug corrigido: o page context passava `auth.activeTenantId` direto, que para
  platform_admin cai no seed `aaaaaaaa-...` e DIVERGE da conta real do switcher. O REST carregava o
  board na conta certa, mas o WS de escopo `board` abria na conta errada e o `authorizeTasksBoard`
  do back retornava 404 (board nao pertence aquela conta) -> handshake nunca virava 101 -> close
  1006 em loop de reconexao (presence e tasks). O canal `account` "abria" como falso-positivo
  (authorizeTasksAccount so valida que a conta-seed existe + platform_admin curto-circuita), mas
  assinava o topico da conta errada. Nao reintroduzir `auth.activeTenantId` como fonte unica do
  accountId de realtime/presence. Defesa em profundidade: o `resolveAccountId` interno de
  `useTasksRealtime.ts` e `useTaskPresence.ts` tambem recebe o `accountStore` e inclui
  `accountStore.activeAccountId` no fallback (entre o prop explicito e `auth.activeTenantId`), entao
  mesmo um chamador que esqueca de passar o prop resolve a conta certa. O teste
  `useTasksRealtime.test.ts` mocka `../../core/stores/account` com `activeAccountId: ''` para manter
  o fallback legado (`tenant-1`) verificado.

### useTaskTracking (Fase T6)

Server-backed com clock offset. O front nao deve voltar para `localStorage`; usar:

- `GET /v1/tasks/tracking/active`
- `POST /v1/tasks/{taskId}/tracking/start`
- `POST /v1/tasks/{taskId}/tracking/pause`
- `POST /v1/tasks/{taskId}/tracking/resume`
- `POST /v1/tasks/{taskId}/tracking/stop`

```typescript
const serverOffset = serverNow - clientNow // calculado no evento task.time_started
const displayMs = computed(() => durationMsFromServer + (Date.now() - localStartAt + serverOffset))
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
