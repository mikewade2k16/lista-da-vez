# Fase 12 - Tasks Orchestrator / Notion-like

## Estado atual

A rota `/tasks` ja existe como port inicial do `web-reference`.

Ja foi feito:

- `web/layers/tasks/` criado e incluido no `extends` do Nuxt.
- pagina `/tasks` portada com board e tabela.
- `useTasksWorkspace.ts`, `types/tasks.ts` e store local de simulacao trazidos.
- `OmniDataTable` e `OmniSelectMenuInput` portados localmente no layer tasks.
- menu/rota liberados para acesso dev/admin inicial.
- visual conectado ao Theme Studio da Fase 11.
- primeira migracao notion-like aplicada: paginas, columns, fields e views existem no estado front-first.
- colunas do board sao objetos com `id`, `label`, `color` e `order`.
- board permite criar task diretamente na coluna.
- coluna pode ser renomeada e recolorida por popover inline no menu da coluna.
- renomear coluna remapeia os itens daquele status.
- excluir coluna remapeia itens para a primeira coluna disponivel.
- colunas podem ser reordenadas por drag.
- drag de coluna usa handle dedicado; drag de card move itens entre colunas e reordena dentro da coluna.
- criar task no board agora usa draft inline com foco automatico no titulo.
- draft inline mostra campos configurados do card, aplica responsavel/cliente padrao e salva como `Nova task` se sair sem titulo.
- se o usuario sai do draft sem digitar, a task vira `Nova task` e nao mostra campos vazios no card.
- cards editam inline titulo, status, responsavel, cliente, tipo, prioridade e data sem abrir o modal.
- cards escondem campos vazios quando nao estao em foco.
- menu de coluna tem editar grupo, ocultar/mostrar contagem, ocultar grupo e excluir cards do grupo.
- board pode agrupar por status, responsavel, cliente, tipo ou prioridade.
- configuracao de pagina controla agrupamento do board, campos visiveis no board, na tabela e no modal.
- tabela cria nova linha com foco no titulo e respeita colunas visiveis da view.
- modal respeita campos visiveis configurados.
- modal ganhou modos lado a lado, centralizado e pagina inteira, com resize no modo lateral.
- modal usa editor rico TipTap/Nuxt UI para conteudo longo, links, imagens, HTML, emojis e mencoes.

Ainda nao foi feito:

- backend Go, migrations e API real.
- modelo final de fields custom com criacao/remocao de campos arbitrarios.
- ordenacao avancada por multiplos campos.
- drag/reordenacao visual de campos dentro do card/tabela/modal.
- editor rico ainda precisa virar componente completo de produto, com UX tipo documento.
- mencoes e imagens precisam de API/asset storage real depois.
- mover `/tasks` e novos modulos sem relacao direta com fila-atendimento para um layout externo/full-width igual ao front de referencia.

## Conceito de produto

O nome inicial da rota continua `Tasks`, mas a feature nao deve ser uma tela rigida de tarefas. Ela deve funcionar como um orquestrador de paginas configuraveis, parecido com Notion databases.

Uma pagina pode representar:

- tarefas;
- aprovacoes;
- producao de conteudo;
- pipeline interno;
- demandas de cliente;
- qualquer outro fluxo que use itens, campos, views e agrupamentos.

## Entidades front-first

### Workspace

Container da experiencia em `/tasks`.

Responsavel por:

- listar paginas disponiveis;
- guardar pagina ativa;
- persistir configuracoes locais enquanto o backend nao existe;
- aplicar tema e componentes compartilhados.

### Page

Uma base configuravel.

Exemplos:

- `Tasks`;
- `Aprovacoes`;
- `Conteudo`;
- `Demandas de Cliente`.

Campos esperados:

- `id`;
- `name`;
- `description`;
- `icon`;
- `views`;
- `fields`;
- `items`;
- `defaultViewId`.

### View

Representa como os itens aparecem.

Tipos iniciais:

- `board`;
- `table`.

Configuracoes esperadas:

- `name`;
- `type`;
- `groupByFieldId`;
- `sorts`;
- `filters`;
- `visibleFieldIds`;
- `cardLayout`;
- `tableLayout`;
- `modalLayout`;
- `density`.

### Field

Define propriedades dos itens.

Tipos iniciais:

- `title`;
- `text`;
- `select`;
- `multiSelect`;
- `status`;
- `person`;
- `client`;
- `date`;
- `priority`;
- `number`;
- `checkbox`.

Cada campo pode ter:

- `id`;
- `key`;
- `label`;
- `type`;
- `required`;
- `hidden`;
- `options`;
- `defaultValue`;
- `color`.

### Item

Registro editavel dentro da pagina.

Nao deve ser acoplado para sempre a `TaskItem`. A primeira pagina pode chamar item de tarefa, mas internamente o dado deve migrar para valores por campo.

Formato alvo:

```ts
interface OrchestratorItem {
  id: string
  pageId: string
  values: Record<string, unknown>
  orderByGroup: Record<string, number>
  archived: boolean
  createdAt: string
  updatedAt: string
}
```

## Board

O board deve ser gerado pela view ativa.

Comportamentos esperados:

- agrupar por `status` inicialmente;
- depois permitir agrupar por pessoa, cliente, tipo ou outro campo select-like;
- renomear colunas;
- colorir colunas/status;
- reordenar colunas por drag;
- adicionar coluna;
- excluir coluna com remapeamento ou arquivamento dos itens;
- botao de criar novo item direto na coluna;
- menu de coluna para editar nome, cor, ordem e regras;
- mover cards entre colunas;
- mover cards dentro da mesma coluna.

## Card

O card deve ser editavel sem abrir modal quando o usuario interage com uma peca interna.

Regras:

- clique em select/input/botao edita inline;
- clique neutro no card abre o modal;
- `OmniSelectMenuInput` deve ser usado para selects inline sempre que couber;
- titulo deve poder ser editado inline;
- status, pessoa, cliente, tipo, prioridade e data devem poder ser editados no card quando visiveis;
- campos exibidos, ordem e formato devem vir de `cardLayout`.

Configuracoes do card:

- campos visiveis;
- ordem dos campos;
- mostrar/esconder labels;
- formato badge/texto;
- cor por campo/opcao;
- densidade compacta/confortavel.

## Tabela

A tabela deve continuar usando `OmniDataTable` como base.

Comportamentos esperados:

- edicao inline de celulas;
- escolher colunas visiveis;
- reordenar colunas;
- ordenar por qualquer campo suportado;
- filtros da view ativa;
- mesma fonte de dados e schema do board;
- troca board/tabela sem perder configuracao da pagina.

## Modal

O modal vem depois de board/tabela.

Ele deve ser configuravel por pagina/view:

- quais campos aparecem;
- ordem dos campos;
- secoes;
- campos obrigatorios;
- campos somente leitura;
- layout compacto ou detalhado.

O modal nao deve ser a unica forma de editar. Ele serve para edicao detalhada, notas, descricao longa e campos menos usados.

## Layout dos novos modulos

As novas paginas e modulos que nao pertencem diretamente ao fluxo de fila-atendimento nao devem usar o layout operacional antigo da fila.

Regra:

- criar/usar um layout externo full-width, inspirado no shell do `web-reference`;
- manter topbar horizontal, logo, navegacao por modulos e acoes globais;
- remover a sidebar operacional de fila dessas paginas;
- aplicar em `/tasks` primeiro e depois nos proximos modulos migrados;
- manter acesso dev/admin enquanto os modulos estiverem em construcao;
- preservar Nuxt UI e tokens do Theme Studio nesse layout.

## Editor completo

O editor deve ser um componente nosso, reutilizavel no modal de tasks e em uma pagina dedicada de documentos/editor.

Comportamentos obrigatorios:

- area do editor com rolagem interna;
- header/toolbar do editor sempre visivel durante a rolagem;
- cada bloco/linha com icone de drag para reordenacao;
- hover em bloco mostra handle de drag e botao `+`;
- botao `+` abre menu de insercao com blocos e acoes;
- digitar `/` abre command menu com opcoes de estilo e insercao;
- digitar `@` abre menu de pessoas;
- avaliar UX para `@@` abrir clientes, tasks ou entidades relacionadas;
- selecionar texto abre bubble menu com opcoes de IA/Improve, estilo do bloco, negrito, italico, underline, strike, codigo, link e imagem;
- suportar headings H1/H2/H3, paragraph, bullet list, numbered list, blockquote, code block e imagens;
- suportar links, HTML/code, emojis, mencoes e upload/URL de imagem;
- estruturar o componente para poder evoluir com comandos de IA depois;
- deve funcionar dentro do modal e tambem como pagina de documento dedicada.

## Persistencia antes do backend

Nesta fase, a prioridade e fechar o front e a UX. A persistencia deve ficar em localStorage estruturado.

Depois que o comportamento estiver correto, criar backend Go para:

- pages/templates;
- views;
- fields;
- field options;
- items;
- item values;
- ordering;
- permissions/account_modules.

## Roadmap de implementacao

1. Ajustar types/composable para `page/view/field/item`. Concluido como fundacao compativel.
2. Migrar dados seedados atuais para a primeira pagina `Tasks`. Concluido.
3. Criar seletor/criador de paginas. Concluido.
4. Evoluir colunas do board para objetos com `id`, `label`, `color`, `order`. Concluido.
5. Implementar drag de colunas. Concluido.
6. Adicionar botao de criar item por coluna. Concluido.
7. Implementar menu de coluna. Concluido.
8. Implementar edicao inline no card para campos principais. Concluido.
9. Criar configuracao de view board/tabela. Concluido.
10. Implementar configuracao de card. Concluido.
11. Evoluir tabela para schema dinamico base. Concluido para campos padrao.
12. Implementar configuracao de tabela. Concluido.
13. Implementar modal configuravel. Concluido para campos visiveis, modos e editor rico.
14. Criar layout externo/full-width para `/tasks` e novos modulos fora do fluxo de fila-atendimento.
15. Evoluir editor para componente completo com block drag, slash menu, mention menu, bubble menu e pagina dedicada.
16. Validar tudo em `light`, `dark`, `apple` e `custom`. Parcial: `/tasks` validado em Docker 3003 no tema atual.
17. So depois criar API Go e migrations.

## Criterios de aceite front-first

- `/tasks` permite criar pelo menos duas paginas diferentes.
- cada pagina tem views independentes.
- board pode agrupar por status inicialmente e manter caminho para outros campos.
- colunas podem ser criadas, renomeadas, coloridas e reordenadas.
- cards podem ser movidos entre colunas e editados inline.
- tabela edita os mesmos itens e respeita a view ativa.
- card, tabela e modal usam configuracao de campos.
- clique neutro no card abre modal; clique em componentes internos nao abre.
- editor tem toolbar fixa, rolagem interna, blocos arrastaveis, slash menu, mention menu e bubble menu de selecao.
- `/tasks` usa layout externo/full-width, sem ficar presa ao layout operacional de fila-atendimento.
- estado persiste ao recarregar.

---

## Apêndice — Fases T0 e T0.5 concluídas + planejamento do backend

Branch ativa: `refactor/multi-tenant-core`. Data de conclusão das fases abaixo: 2026-05-14.

O plano detalhado de backend está em `C:\Users\Mike\.claude\plans\analisa-nossa-p-gina-de-agile-pumpkin.md`.

---

### Fase T0 — Documentação prévia (concluída)

Objetivo: criar toda a documentação de arquitetura **antes** de qualquer código de backend, para que qualquer dev ou agente possa pegar o trabalho de onde parou sem contexto de sessão.

**Arquivos criados/modificados:**

| Arquivo | O que documenta |
|---------|----------------|
| `web/app/components/roadmap/roadmap-data.ts` | Nova lane "Tasks Orquestrador — Backend" com 11 cards (`tasks-t0` a `tasks-t9` + `tasks-t05`). Adicionado campo `group?: string` a `RoadmapPhase`, interface `RoadmapGroup` e const `ROADMAP_GROUPS`. |
| `web/app/components/roadmap/RoadmapTimeline.vue` | Atualizado para renderizar fases agrupadas por `group`; computed `groupedPhases` agrupa por `group` (default `"multi-tenant"`); template exibe `<section>` por grupo com cabeçalho. |
| `back/internal/modules/tasks/AGENT.md` | Spec completo: escopo, 30+ endpoints REST, regras de escopo (3 camadas: middleware→service→repository), `scopedQuery` com panic, `BuildTaskDTO` por `Perspective`, 13 permission keys, 3 role templates, eventos WS. |
| `back/internal/modules/notifications/AGENT.md` | Adapter pattern: `InAppAdapter` funcional no MVP; stubs `EmailAdapter`, `WhatsAppAdapter`, `PushAdapter` retornam `ErrNotConfigured`. Migration 0109, schema `notifications.*`, integração com módulo tasks. |
| `back/internal/modules/realtime/AGENT.md` | Adicionada seção "Canais Tasks / Presence / Notifications (Fase T2)": 6 novos tópicos WS, autorização dos canais, 25+ eventos, `PresenceStore` com TTL 30s, rate limit 30 events/s, buffer 16, interface `Publisher`. |
| `web/layers/tasks/AGENT.md` | Estado atual (localStorage), estrutura de componentes após T0.5, padrão de wipe do storage legado, derivação de `perspective` por permissões reais, spec de composables T2–T7 com exemplos de código. |
| `docs/TASKS_ORCHESTRATOR_PHASE12.md` | Este arquivo — apêndice adicionado. |

---

### Fase T0.5 — Quebrar tasks.vue em sub-componentes (concluída)

**Contexto:** `tasks.vue` tinha 2955 linhas (script + template + CSS). Antes de plugar o backend, o arquivo precisava ser dividido para que cada sub-componente pudesse ser migrado/testado independentemente.

**Padrão adotado: `provide/inject` com composable tipado**

Todo o estado e lógica foram movidos para `useTasksPageContext.ts`. O `tasks.vue` chama o composable, faz `provide`, e os sub-componentes fazem `inject`:

```typescript
// tasks.vue
const context = useTasksPageContext()
provide(TASKS_PAGE_CONTEXT_KEY, context)

// qualquer sub-componente
const ctx = inject(TASKS_PAGE_CONTEXT_KEY)!
const { activeProject, viewMode, filters, ... } = ctx
```

`TasksPageContext = ReturnType<typeof useTasksPageContext>` — o tipo é derivado automaticamente, sem interface manual.

**Arquivos criados:**

| Arquivo | Responsabilidade |
|---------|-----------------|
| `web/layers/tasks/composables/useTasksPageContext.ts` | Todo o estado (`ref`/`reactive`), `computed`, funções (drag/drop, CRUD, filtros, settings, tracking, resize do modal). Exporta `TASKS_PAGE_CONTEXT_KEY` e `TasksPageContext`. Ciclo de vida (`onMounted`, `watch`) funciona pois o composable é chamado dentro do `<script setup>` do componente pai. |
| `web/layers/tasks/components/TasksFilterBar.vue` | Toolbar completa: seletor de projeto, chips de filtro ativo, busca, filtros (responsável/cliente/tipo/arquivadas), stats (total/filtradas/arquivadas), toggle board↔tabela, botão "Nova task", menu "Mais ações". |
| `web/layers/tasks/components/TasksBoardView.vue` | Kanban: colunas ordenáveis por drag, cards com drag/drop entre colunas e reordenação interna, edição inline de todos os campos, draft card com campos configuráveis, menu de coluna (editar/ocultar/excluir cards). |
| `web/layers/tasks/components/TasksTableView.vue` | Wrapper de `OmniDataTable` + botão "Nova linha". |
| `web/layers/tasks/components/TasksProjectSettings.vue` | `USlideover` com todas as configurações da página: nome, descrição, colunas, agrupamento do board, padrões de criação, responsáveis, tipos, filtros ativos, campos visíveis no card/board/tabela/modal. |
| `web/layers/tasks/components/TasksTaskModal.vue` | `USlideover` de detalhe/edição: título, todas as properties, tracking timer, editor rico (`OmniEditor`), comentários, switch de arquivado. Suporta modos `side`/`center`/`fullscreen` com resize no modo lateral. |

**`tasks.vue` após a refatoração**: o arquivo saiu de ~2955 para **832 linhas totais** no estado atual. O template ficou fino e a regra pesada foi extraída para `useTasksPageContext.ts` e sub-componentes.

```vue
<script setup lang="ts">
const context = useTasksPageContext()
provide(TASKS_PAGE_CONTEXT_KEY, context)
const { pageBootstrapping, activeProject, viewMode, taskEditorCssVars } = context
</script>

<template>
  <section class="tasks-page space-y-4" :style="taskEditorCssVars">
    <AdminPageHeader ... />
    <!-- skeleton de carregamento -->
    <template v-else>
      <TasksFilterBar />
      <UAlert v-if="!activeProject" ... />
      <TasksBoardView v-else-if="viewMode === 'board'" />
      <TasksTableView v-else />
    </template>
    <TasksProjectSettings />
    <TasksTaskModal />
  </section>
</template>
```

**CSS:** movido de `<style scoped>` para `<style>` (global) no `tasks.vue`. Como todas as classes são prefixadas com `tasks-page__` ou `tasks-toolbar__`, não há risco de colisão. Os `:deep()` e `:global()` foram removidos (desnecessários em escopo global).

---

### Fase T1 — Schema multi-tenant + módulo Go tasks (concluída)

**Data:** 2026-05-14.

**Entrega:** fundação backend do orquestrador, ainda sem substituir o localStorage do frontend.

**Arquivos criados/modificados:**

| Arquivo | O que entrega |
|---------|---------------|
| `back/internal/platform/database/migrations/0108_tasks_schema_foundation.sql` | Cria schema `tasks` com 17 tabelas: boards, columns, fields/options, views/widgets, tasks/field_values, assignees/subscribers, comments/mentions, time entries, relations, shares, audit e snapshots futuros. |
| `back/internal/modules/tasks/model.go` | Modelos, inputs, `Perspective`, `AccessContext` e interface `Repository`. |
| `back/internal/modules/tasks/permissions.go` | 13 permission keys e matrizes dos role templates `tasks.admin`, `tasks.member` e `tasks.client_viewer`. |
| `back/internal/modules/tasks/repository_postgres.go` | Repository Postgres com `scopedQuery(accountID, ...)`, CRUD base, audit, relations, shares e tracking inicial. |
| `back/internal/modules/tasks/service.go` | Resolve `X-Account-Id`, valida membership/permissões, CRUD de boards/tasks/comments/shares/relations e audit helper. |
| `back/internal/modules/tasks/service_tracking.go` | Endpoints de tracking server-side inicial: active/start/pause/resume/stop/metrics. |
| `back/internal/modules/tasks/service_dto.go` | `BuildTaskDTO` com payload menor para `client_viewer` (omite campos de agência como `clientAccountId`). |
| `back/internal/modules/tasks/http.go` e `http_tracking.go` | Rotas REST v1 com `RequireAuth`, `withPermission` e erros 404 para recurso fora do escopo. |
| `back/internal/modules/tasks/module.go` | Registra o módulo `tasks` no Module Registry com schema, permissões e role templates. |
| `back/internal/platform/app/app.go` | Registra `tasks.New(realtimeService)` quando `CORE_V2_ENABLED=true`, para `SyncCatalog` popular `core.modules`, `core.permissions` e `core.role_templates` e injetar o publisher WS real. |

**Validação executada:**

```bash
cd back
go test ./...
```

Resultado: passou.

**Observações para o próximo dev/agente:**

- A migration foi adicionada e compilada no embed do migrator, mas ainda precisa ser aplicada em um banco fresh/staging para validar SQL em ambiente real.
- O frontend `/tasks` continua usando localStorage até a T5.
- Existing accounts não recebem automaticamente roles `tasks.*`; o `SyncCatalog` cria templates/permissões, e a atribuição/clonagem deve seguir o fluxo RBAC.

---

### Fase T2 — Realtime para tasks (concluída)

**Data:** 2026-05-14.

**Entrega:** canais WebSocket autenticados para tasks, presence e notifications, com publisher real plugado no módulo `tasks`.

**Arquivos criados/modificados:**

| Arquivo | O que entrega |
|---------|---------------|
| `back/internal/modules/realtime/service_tasks.go` | `HandleTasksSocket`, `HandlePresenceSocket`, `HandleNotificationsSocket`, autorização antes do upgrade, rate limit de entrada 30 eventos/s e implementação de `tasks.Publisher`. |
| `back/internal/modules/realtime/presence.go` | `PresenceStore` em memória com TTL 30s, snapshot, join/leave, heartbeat, field lock/unlock e limpeza periódica. |
| `back/internal/modules/realtime/model.go` | Tipos de eventos e campos novos: account/board/task/user, participants, payload, notificationId, version. |
| `back/internal/modules/realtime/http.go` | Rotas `GET /v1/realtime/tasks`, `/presence` e `/notifications`. |
| `back/internal/modules/realtime/service.go` | `Service` agora recebe `pool` opcional e inicializa `PresenceStore`. |
| `back/internal/modules/tasks/publisher.go` | Interface `Publisher` expandida com `PublishPresenceEvent`; `noopPublisher` mantido para testes/uso isolado. |
| `back/internal/modules/tasks/module.go` | `tasks.New(publisher, notifier)` permite injeção do realtime e do dispatcher de notifications sem acoplamento ao transporte. |
| `back/internal/platform/app/app.go` | `realtime.NewService(..., pool)`, `notifications.NewService(...)` e `tasks.New(realtimeService, notificationService)` compartilhando o mesmo publisher WS. |

**Canais entregues:**

```text
tasks:account:{accountId}
tasks:board:{boardId}
tasks:task:{taskId}
presence:board:{boardId}
presence:task:{taskId}
notifications:user:{userId}
```

**Eventos cobertos:** tasks/board/tracking já publicados pelo service de tasks; presence publica snapshot, joined, left, field_locked e field_unlocked; notifications expõe canal e helper de publish para a T3.

**Validação executada:**

```bash
cd back
go test ./...
```

Resultado: passou.

**Observações para o próximo dev/agente:**

- O frontend ainda não abre esses WS; isso entra na T5/T7 via `useTasksRealtime` e `useTaskPresence`.
- `notifications:user:{userId}` agora tem persistência/adapters na T3; o frontend ainda precisa consumir REST/WS na T5.
- Autorização WS usa `tasks.tasks.view` ou `tasks.client_view` e valida account/board/task antes do upgrade. Board compartilhado é permitido quando existe task share ativa naquele board.
- Presence é in-memory por processo; broker externo continua evolução futura quando houver múltiplas réplicas.

---

### Fase T3 — Módulo notifications (concluída)

**Data:** 2026-05-14.

**Entrega:** módulo `notifications` com persistência `notifications.*`, adapter in-app funcional, stubs externos e triggers em `tasks` para assign, comment mention/subscriber, status change e move.

**Arquivos criados/modificados:**

| Arquivo | O que entrega |
|---------|---------------|
| `back/internal/platform/database/migrations/0109_notifications_module.sql` | Schema `notifications` com `user_notifications`, `notification_channels`, `delivery_log` e `mutes`. |
| `back/internal/modules/notifications/*.go` | Service, repository, HTTP, module registry, `InAppAdapter`, stubs email/WhatsApp/push e publisher `notification.created`/`notification.read`. |
| `back/internal/modules/tasks/service.go` e `notifications.go` | Disparo best-effort para responsável, mentions, subscribers, change de status e move sem bloquear a mutation principal. |
| `back/internal/modules/tasks/repository_postgres.go` | Persistência de `task_mentions` e `task_subscribers` para suportar os triggers do módulo notifications. |
| `back/internal/platform/app/app.go` | `notifications.NewService(...)` compartilhado entre o módulo `notifications` e `tasks.New(realtimeService, notificationService)`. |

**Endpoints entregues:**

```text
GET  /v1/notifications
POST /v1/notifications/{notificationId}/read
POST /v1/notifications/mark-all-read
GET  /v1/notifications/preferences
PUT  /v1/notifications/preferences
POST /v1/notifications/mute
```

**Eventos/tasks cobertos:**

- `task.assigned` → novo responsável.
- `task.comment_mentioned` → usuários mencionados no comentário.
- `task.comment_added` → subscribers quando não há mention explícita.
- `task.moved` e `task.status_changed` → subscribers.

**Validação executada:**

```bash
cd back
go test ./...
```

Resultado esperado no runtime: `user_notifications` persiste o histórico, `InAppAdapter` publica `notification.created` e `notification.read` no canal `notifications:user:{userId}`, `mute` silencia por `resourceType/resourceId` com TTL, e os adapters externos permanecem como stubs (`ErrNotConfigured`).

---

### Fase T4 — Registry de resolvers cross-module (concluída)

**Data:** 2026-05-14.

**Entrega:** registry `RelationResolver` em `platform/modules`, resolvers bulk para `crm`/`erp`/`operations` e endpoint real `GET /v1/tasks/{taskId}/relations:expand` com reaproveitamento de cache por 60s.

**Arquivos criados/modificados:**

| Arquivo | O que entrega |
|---------|---------------|
| `back/internal/platform/modules/relations.go` | Contratos `RelationRef`, `RelationResult`, `RelationResolver` e `RelationRegistry`. |
| `back/internal/modules/erp/relations_resolver.go` | Resolver bulk para `erp` e alias `crm`, cobrindo `customer/contact`, `employee`, `order/lead`, `record` e devolvendo `label`, `url`, `status`. |
| `back/internal/modules/operations/relations_resolver.go` | Resolver para `service_history`, com fallback para serviço ativo quando o histórico ainda não existe. |
| `back/internal/modules/tasks/service_relations.go` e `http_relations.go` | Fluxo de expansão, TTL 60s, persistência de `label_cache`/`metadata_cache`/`refreshed_at` e handler dedicado de `/relations:expand`. |
| `back/internal/modules/tasks/service.go` e `module.go` | Injeção do registry no service de tasks. |
| `back/internal/platform/app/app.go` | Bootstrap do registry com `erp.NewRelationResolver`, `erp.NewCRMRelationResolver` e `operations.NewRelationResolver`. |

**Comportamento entregue:**

- `GET /v1/tasks/{taskId}/relations:expand` resolve em bulk por módulo, evitando N+1 por relation individual.
- Relações com cache fresco (`refreshed_at < 60s`) retornam o cache atual sem nova resolução.
- Relações stale ou vazias atualizam `label_cache`, `metadata_cache` e `refreshed_at` reutilizando o `upsert` de `task_relations`.
- `metadata_cache` passa a carregar `status`, `url`, `resolvedModule` e campos específicos do recurso.
- Recurso inexistente ou fora da account retorna `status = "unknown"` sem quebrar a listagem.

**Validação executada:**

```bash
cd back
go test ./internal/platform/modules ./internal/modules/erp ./internal/modules/operations ./internal/modules/tasks ./internal/platform/app
go test ./...
```

Resultado: passou.

---

### Próximas fases (planejadas)

| Fase | O que entrega | Depende de |
|------|---------------|------------|
| T5 | Front substitui localStorage pelo backend (Pinia store real, wipe legacy, useCan) | T1–T4 |
| T6 | Tracking server-side autoritativo (start/pause/resume/stop, clockOffset) | T1, T2, T5 |
| T7 | Presence MVP (avatares + field locking leve, heartbeat 15s) | T2 |
| T8 | Segurança, audit log, rate limit, hardening | T1–T7 |
| T9 | Testes E2E + observabilidade | T8 |
| T10 | Sistema de Views (11 tipos, /views/:id/data) | T5 |

**Caminho mínimo para MVP usável:** T5, porque T0, T0.5, T1, T2, T3 e T4 já foram concluídas em 2026-05-14.
