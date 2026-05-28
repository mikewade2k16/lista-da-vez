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

- O frontend ja abre os WS de tasks e presence via `useTasksRealtime` e `useTaskPresence`; notifications continuam para a fase dedicada.
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

### T5 - Registro de regressao e correcoes aplicadas em 2026-05-14

Este registro existe para evitar repeticao do mesmo erro na virada `localStorage -> Pinia/API`.

**Sintomas vistos no front:**

- drag/drop do board nao movia a task de forma confiavel;
- task criada parecia existir na tela, mas sumia ou voltava errada apos reload;
- modal abria incompleto, sem `Prazo/Data` em alguns boards;
- criacao pela tabela podia focar uma task sem `id`, porque a chamada async nao era aguardada;
- board era reconstruido com colunas fake quando o list de boards vinha sem detalhes;
- responsavel podia aparecer como email em vez de nome de exibicao;
- falhas de sync ficavam silenciosas, sem alerta visual para o usuario;
- tentativa de expor `GET /v1/tasks/boards/{boardId}` quebrava o boot do Go `ServeMux`, por conflito com rotas como `GET /v1/tasks/{taskId}/comments`.
- card do board mostrava placeholders de campos vazios (`Sem data`, prioridade default, tipo vazio) mesmo quando o usuario nao tinha cadastrado o dado;
- selects de `responsavel` e `envolvidos` ainda permitiam criar pessoas manuais, apesar de esses valores virem do diretorio de usuarios;
- responsavel podia ser duplicado em `envolvidos`;
- cores de opcoes editadas no select apareciam no modal, mas nao no board, e cliente/usuario usavam badge preenchido em vez de avatar + nome com borda opcional;
- prazo com inicio/fim e hora era exibido no modal, mas o board nao recebia `endDate`; a hora ficava em uma linha separada do date picker;
- tracking do front ainda usava `localStorage`, mesmo com endpoints server-side ja disponiveis para a T6.
- dois usuarios viam cliente, responsavel, envolvidos, tipo e data final diferentes, porque esses campos ainda vinham de metadata local por navegador.

**Causa raiz:**

O smoke inicial validou um caminho estreito, mas a T5 ainda misturava contrato incompleto do backend com suposicoes do front:

- `GET /v1/tasks/boards` retorna resumo do board, nao colunas/views/fields completos;
- a rota canonica de detalhe do board conflita com rotas parametrizadas de task no `ServeMux`;
- chaves de campos do backend (`due_date`) e chaves camelCase do front (`dueDate`, `clientId`) eram normalizadas como valores diferentes;
- campos front-only do prototipo (`responsible`, `involved`, `clientName`, `type`, `createdBy`, `dueEndDate`) nao tinham persistencia server-side e ficavam divergentes entre navegadores;
- `TasksBoardView.vue` usava `onDropCard` no template sem expor a funcao no destructuring local.

**Correcoes aplicadas:**

| Area | Correcao |
|------|----------|
| Backend routes | Detalhe de board usa `GET /v1/task-boards/{boardId}` para evitar conflito no `ServeMux`; nao reintroduzir `GET /v1/tasks/boards/{boardId}` sem mudar o desenho das rotas. |
| Board hydration | `web/layers/tasks/stores/tasks.ts` carrega detalhe do board antes de montar projeto, colunas, views e fields. |
| Field keys | `normalizeFieldKey` preserva aliases canonicos como `dueDate`, `clientId`, `createdAt` e `createdBy`. |
| Drag/drop | `TasksBoardView.vue` passa a receber `onDropCard` do contexto; smoke real deve validar `POST /v1/tasks/{taskId}/move`. |
| Criacao tabela | `createTableTask` agora aguarda `tasksWorkspace.createTask()` antes de focar a celula. |
| Nome do usuario | `currentUserName` usa `displayName` antes de `email`. |
| Sync visivel | `/tasks` exibe alerta quando `tasksWorkspace.errorMessage` estiver preenchido. |
| Metadata da task | `tasks.tasks.ui_metadata` persiste `responsible`, `involved`, `clientId`, `clientName`, `type`, `dueEndDate`, `prioritySet` e `createdBy`; o DTO sempre devolve `uiMetadata`, mesmo `{}`, para impedir fallback em cache local velho. |
| Card sem placeholder | `isCardFieldVisible` mantem campos vazios fora do card; `Sem data` so aparece dentro do editor/draft quando o campo foi aberto. |
| Pessoas reais | Selects de `responsavel` e `envolvidos` nao usam `creatable`; opcoes vem de `/v1/users`, projeto e tasks existentes. |
| Involved limpo | `sanitizeInvolved` remove automaticamente o responsavel da lista de envolvidos. |
| Visual unificado | Pessoas/clientes usam avatar + nome e `badge-style="entity"`; cores manuais viram somente borda. Status/tipo/prioridade continuam como badges preenchidos. |
| Prazo completo | `AppDatePicker` emite `modelValue` e `endDate`; data e hora ficam na mesma linha para inicio e fim; `dueEndDate` agora persiste em `ui_metadata` e aparece igual no modal e no board. |
| T6 tracking front | `useTimeTracking` deixou de usar `localStorage` e passou a hidratar/acionar `/v1/tasks/tracking/active` e `/v1/tasks/{taskId}/tracking/{start,pause,resume,stop}` com `X-Account-Id`. |

**Checklist obrigatorio antes de marcar T5 como concluido:**

1. API sobe sem panic apos `docker compose up -d --build api`.
2. `curl http://localhost:8883/healthz` responde `status: ok`.
3. `go test ./...` passa em `back/`.
4. `npm --prefix web run build` passa.
5. Smoke real em `http://localhost:3003/tasks`:
   - criar task no board;
   - abrir modal;
   - definir prazo;
   - salvar;
   - arrastar para outra coluna;
   - recarregar a pagina;
   - confirmar titulo, status, prazo e nome do responsavel preservados.
6. Network do smoke deve conter `GET /v1/task-boards/{boardId}`, `POST /v1/tasks/boards/{boardId}/tasks`, `PATCH /v1/tasks/{taskId}` e `POST /v1/tasks/{taskId}/move` com `2xx`.
7. Console do navegador nao deve ter erro novo de Tasks, CORS, rota `405/500` ou promise sem tratamento.

**Nao repetir:**

- Nao montar colunas fallback (`column-todo`, `column-doing`, etc.) quando a API real tem board criado; buscar detalhe do board primeiro.
- Nao declarar T5 pronto apenas com smoke de criar/editar/arquivar; reload e drag/drop sao parte do aceite.
- Nao usar a rota `GET /v1/tasks/boards/{boardId}` no Go `ServeMux` atual.
- Nao usar `omni.tasks.api.workspace.ui.v1` como fonte autoritativa para campos do card (`cliente`, `responsavel`, `envolvidos`, `tipo`, `prazo fim`); esses valores ficam em `tasks.tasks.ui_metadata`. A ponte local e apenas fallback/configuracao de view enquanto nao houver tabela dedicada para views/filtros.
- Nao renderizar placeholder de campo vazio no card: se nao cadastrou valor, o card nao mostra o campo.
- Nao reabilitar criacao manual em selects de pessoa; novas pessoas devem vir do modulo de usuarios.

---

### T7 - Presence MVP front aplicado em 2026-05-15

**Entrega:** o front agora usa o canal `GET /v1/realtime/presence` no modal e no board, com avatares de participantes, nome de quem esta editando e bloqueio do mesmo campo enquanto outro usuario esta nele.

| Arquivo | Correcao |
|---------|----------|
| `web/layers/tasks/composables/useTaskPresence.ts` | Composable para `presence:task:{taskId}` e `presence:board:{boardId}` com auth por `access_token`, reconexao simples, heartbeat a cada 15s e handlers de `presence.snapshot`, `presence.user_joined`, `presence.user_left`, `presence.field_locked` e `presence.field_unlocked`. |
| `web/layers/tasks/composables/useTasksPageContext.ts` | Contexto expoe participantes e helpers de focus/blur para modal e board; modal tambem publica presence no escopo do board para o card refletir a edicao. |
| `web/layers/tasks/components/TasksTaskModal.vue` | Header mostra avatares de outros usuarios no mesmo modal; campos principais publicam focus/blur; labels exibem "Fulano editando" e bloqueiam o campo se outro usuario estiver nele. |
| `web/layers/tasks/components/TasksBoardView.vue` | Cards mostram avatar/nome de quem esta editando e bloqueiam inline title/status/responsavel/envolvidos/cliente/tipo/prioridade/prazo quando ha lock remoto. |
| `web/layers/tasks/pages/tasks.vue` | CSS compacto para pilha de avatares, badges de presenca e presence no card. |

**Nao repetir:**

- Nao deixar field lock apenas visual: se outro usuario esta no mesmo campo, o input/select correspondente deve ficar desabilitado ate o blur/TTL.
- Nao espalhar `WebSocket` direto nos componentes; a conexao fica em `useTaskPresence`.
- Nao reduzir o heartbeat para menos de 15s sem rever o rate limit de 30 eventos/s por conexao.
- Nao mostrar participante atual como colaborador remoto; o modal deve destacar apenas outras pessoas na mesma task.

**Correcao em 2026-05-15 apos teste com dois usuarios:**

| Arquivo | Correcao |
|---------|----------|
| `web/layers/tasks/composables/useTasksRealtime.ts` | Criado composable real para `GET /v1/realtime/tasks` em escopo `account`, com reconexao e handler de eventos. |
| `web/layers/tasks/composables/useTasksPageContext.ts` | Ligado `useTasksRealtime` ao workspace; eventos `task.*`, `board.*` e `field.*` agora agendam `tasksWorkspace.refresh()` com debounce para atualizar board/modal sem reiniciar a pagina. |

**Nao repetir:** presence nao substitui realtime de dados. `presence.field_locked` so informa/bloqueia edicao simultanea; sincronizacao de cards precisa do canal `tasks`, e campos visuais de card devem persistir em `ui_metadata`.

---

### T7.1 / T7.2 — Hardening de presence + nick + DX inline (em curso 2026-05-15)

Sintomas reportados apos novo round de teste com dois browsers logados ao mesmo tempo:

- ao renomear titulo de card pelo board, nao era possivel digitar espaco no final da palavra (cursor pulava);
- antes da correcao 2026-05-15, apenas um usuario via o outro editando; depois, ambos passaram a ver o OUTRO editando o mesmo campo simultaneamente (visualmente confunde quando os display_names sao iguais ou parecidos);
- tentativa posterior de otimizar WebSocket com patch local quebrou sincronizacao entre abas quando a task remota nao existia no store local;
- selects e badges de presenca usam o `display_name` completo do usuario, sem identidade curta para reduzir ambiguidade.

**Causa raiz por item:**

| Sintoma | Causa | Onde |
|---------|-------|------|
| Espaco no final do input do card some | `normalizeText(value)` faz `.replace(/\s+/g,' ').trim()` e roda no `@update:model-value` — input controlado reflete o valor truncado a cada keystroke | `useTasksPageContext.ts` (`normalizeText`) usado em `TasksBoardView.vue` no input do titulo |
| "Dois editando o mesmo campo" | `PresenceStore.LockField` aceita lock de qualquer user mesmo quando outro user ja esta no `fieldKey`; servidor publica `presence.field_locked` para ambos | `back/internal/modules/realtime/presence.go` |
| Sync entre abas falhava apos patch local | `hydrateTask(taskId)` dependia da task ja existir no store local; se nao existia, o evento remoto era ignorado | `web/layers/tasks/composables/useTasksPageContext.ts` (handler `scheduleTasksRealtimeRefresh`) |
| Nomes longos/iguais nas mascaras | `core.users` (e `public.users`) so tem `display_name`; presence/selects/avatar usam ele direto | esquema do banco + `auth/model.go` + `realtime/service_tasks.go` + `useTasksPageContext.ts` |

**Plano (T7.1 nick infra + T7.2 fixes):**

1. Documentar T7 com sub-itens `lock-exclusivo`, `nick-infra`, `input-espaco`, `patch-local-realtime` em `roadmap-data.ts` ANTES de codar (regra do projeto).
2. Backend:
   - migration `0111_users_nick.sql`: `alter table users add column if not exists nick text not null default ''` (e mesmo em `core.users`).
   - `auth/model.go`: `Nick string` em `User`, `Principal`, `UserView` (json `nick,omitempty`).
   - `auth/store_postgres.go`: `LoadUserForAuth` e `findRecord` carregam `u.nick`.
   - `auth/tokens.go`: claims com `Nick`.
   - `auth/service.go`: `principal.Nick = user.Nick`.
   - `auth/store_memory.go`: seed com `Nick: ""` (fallback para display_name no front).
   - `users/model.go` + `store_postgres.go`: `Nick` no DTO e na projecao `baseProjectedUsersQuery`.
   - `realtime/service_tasks.go`: `HandlePresenceSocket` usa `principal.Nick` se preenchido; cai pra `principal.DisplayName` se vazio.
   - `realtime/presence.go`: `LockField` checa se ja existe outro user com o mesmo `fieldKey` (apenas mesmo `LockID`/`UserID` pode atualizar); se ocupado, nao mexe no estado de quem tentou tomar o lock e republica o `field_locked` do usuario atual para recuperar clients defasados.
3. Frontend:
   - `useTasksPageContext.ts`: criar `clampText(value, max)` (apenas `String(value).slice(0, max)`, sem trim/colapso). Trocar `normalizeText` por `clampText` em `updateTaskInline.title` e demais `@update:model-value` inline; `normalizeText` so no flush/autosave.
   - `currentUserName` e `directoryUserLabels` preferem `nick` (fallback `displayName` -> `name` -> `fullName` -> `email`).
   - `useTaskPresence.ts`: `focusField` sempre libera o campo ativo anterior e envia `field_focus`; nao usar guard local baseado em `usersForField()`, porque estado defasado pode esconder presenca em uma das abas. Backend e `:disabled` continuam sendo as fontes de verdade do lock.
   - `useTasksPageContext.ts`/`useTasksRealtime.ts`: handler de evento sempre agenda `tasksWorkspace.refresh()` com debounce de 200ms para qualquer `task.*`, `board.*` ou `field.*`, seguindo o padrao de operations.

**Validacao obrigatoria:**

- `cd back && go test ./...` passa.
- `npm --prefix web run build` passa.
- Smoke com 2 sessoes logadas no mesmo modulo:
  1. `nick` editado por SQL em uma conta -> badge de presenca exibe nick em vez de display_name.
  2. Focar mesmo campo em ambas as abas: a segunda nao consegue digitar (input disabled) e a primeira NAO recebe `field_locked` para a segunda sessao.
  3. Renomear card no board digitando `Smoke fase 12 ` (espaco no final, antes da proxima palavra) -> espaco fica preservado.
  4. Mudar status/responsavel em uma aba -> outra aba reflete via `[tasks-ws] executando refresh full do workspace`; full refresh debounced e esperado.

**Nao repetir:**

- Nunca normalizar (trim/colapso) o valor durante `@update:model-value` em inputs controlados — input vira saltitante. Normalizar so no flush/save.
- `PresenceStore.LockField` precisa ser exclusivo por `fieldKey`. Sem isso, dois clientes veem o outro editando o mesmo campo simultaneamente.
- Nao reintroduzir patch local/hydrate por evento em tasks. O padrao valido e `tasksWorkspace.refresh()` full com debounce de 200ms, igual operations; otimizar antes de medir quebrou sync entre abas.
- Nick e identidade curta: fallback obrigatorio para `display_name` quando vazio; nao expor email em mascaras.

---

### T5 — Fechamento (relations + paginacao cursor-based), iniciado 2026-05-15

Sintomas/pendencias que sobraram da T5 inicial:

- `useTaskRelations.ts` nao existe — modal nao mostra vinculos com `crm`/`erp`/`operations` apesar do backend T4 (`GET /v1/tasks/:id/relations:expand`) estar pronto;
- `listBoardTasks` no Pinia store pede `limit=200` direto e nao usa o `cursor`/`nextCursor`; backend repository ignora `input.Cursor`. Boards com >200 tasks perdem itens silenciosamente.

**Causa raiz:**

| Sintoma | Causa | Onde |
|---------|-------|------|
| Sem vinculos no modal | composable `useTaskRelations` nao existe; modal nunca chama o endpoint | `web/layers/tasks/` |
| `limit=200` hard-cap | front passa fixo; backend repository tambem nao implementou cursor | `back/internal/modules/tasks/repository_postgres.go` (`ListTasks`) + `store/tasks.ts` (`listBoardTasks`) |

**Plano:**

1. **Backend** — `tasks/model.go`: tornar `ListTasksInput.Limit` opcional (service ja defaulta 50, max 200). Adicionar `ListTasksResult { Tasks []TaskDTO; NextCursor string }`.
2. **Backend** — `repository_postgres.go:ListTasks`: usar cursor opaco. Encoding `base64url(JSON{ "s": sort_order, "c": created_at, "i": id })`. WHERE tuplado: `(t.sort_order, t.created_at, t.id) > ($cs, $cc, $ci::uuid)`. Pedir `limit+1` para detectar `hasMore`; cortar a lista, devolver o cursor do ultimo item retornado.
3. **Backend** — `service.go:ListTasks`: monta `nextCursor` baseado no ultimo task.
4. **Backend** — `http.go:listTasks`: response `{ tasks, nextCursor }`.
5. **Frontend** — `stores/tasks.ts:listBoardTasks`: loop interno (50/page) at e' `nextCursor` vazio. Para board kanban precisamos de tudo; para tabela view futura, expor `fetchMoreTasks(boardId, cursor)` separado.
6. **Frontend novo** — `composables/useTaskRelations.ts`:
   - lazy load via `GET /v1/tasks/{taskId}/relations:expand` na primeira abertura do modal;
   - cache por `taskId` em mapa local;
   - listener de eventos `task.relation_added`/`task.relation_removed` no canal `tasks:task` invalida o cache;
   - retorna `{ relations, status, refresh }`.
7. **Frontend** — `TasksTaskModal.vue`: nova secao "Vinculos" entre Comments e o editor rico. Cada relation mostra `labelCache`, `resourceType`, e badge de `metadataCache.status`.

**Eventos WS de relations:**

A T1 ja publica eventos `task.relation_added` e `task.relation_removed` quando `AddRelation`/`RemoveRelation` rodam? Verificar; se nao, adicionar publish na service para alimentar o realtime refresh do composable. (Se nao publica, o composable so atualiza no `refresh()` manual ou ao reabrir o modal — degradacao aceitavel ate T8.)

**Validacao obrigatoria:**

- `cd back && go test ./...` passa.
- `npm --prefix web run build` passa.
- Smoke: criar board com >50 tasks via API, abrir `/tasks`, board renderiza todas (loop de pages funciona); abrir modal de task com relations cadastradas no banco -> secao Vinculos aparece com labels.

**Nao repetir:**

- Paginar so no backend nao serve: front precisa fazer loop ate esgotar para o board kanban (UX espera ver tudo). Tabela futura pode lazy-load.
- Cursor opaco (base64) e' obrigatorio — nao expor sort_order/created_at em URL. Trocar a estrategia internamente sem quebrar URLs salvas/historico.
- Relations nao podem disparar `tasksWorkspace.refresh()` (full reload) — sao independentes da task em si. Cache local + invalidacao por evento dedicado.

---

### T8 — Hardening (audit, rate limit REST, slog), iniciado 2026-05-15

Cobertura do que ja existia antes da T8:

- `GET /v1/tasks/:taskId/audit` com `tasks.boards.manage` — implementado na T1 (service.ListAudit + repository.ListAudit) com `tasks.tasks_audit` populada por `service.audit(...)` em CreateTask, UpdateTask, MoveTask, DeleteTask, AddRelation, AddShare, AddComment.
- Rate limit WS — 30 events/seg por conexao com close code 1008 — implementado na T2 (`realtime/service_tasks.go`).
- Cross-account = 404 — `service.ResolveAccessContext` retorna `ErrAccountNotFound` (mapeado para 404) quando o user nao e' membro da account ou ela nao existe. `scopedQuery(accountID, ...)` no repository garante que recursos de outras accounts viram `pgx.ErrNoRows` → `ErrTaskNotFound`/`ErrBoardNotFound` → 404. **403 fica reservado para "user esta na account certa mas falta permissao".** Nao misturar os dois.
- `X-Account-Id` no body **nunca** e' usado. Vem do header (`r.Header.Get("X-Account-Id")`), fallback para query (`accountId`), fallback para `principal.TenantID`. Inputs com campo `AccountID` JSON (ex: `CreateBoardInput.AccountID`) sao ignorados pelo service — usa-se `access.AccountID` derivado do `Principal`.

Gaps identificados:

| Item | Status | Causa |
|------|--------|-------|
| Rate limit REST 60 req/min | ❌ inexistente | `httpapi/middleware.go` so tem RequestID/Logging/Recover/CORS |
| slog estruturado em mutations | ❌ inexistente | `tasks/service.go` nao tem `*slog.Logger`; logs so existem no middleware genérico do httpapi |
| `task.relation_removed` no WS | ⚠️ N/A | `service.RemoveRelation` ainda nao existe (sem rota DELETE); quando for implementado, lembrar de publicar o evento |
| 404 cross-account audit/integration test | ⚠️ aplica-se quando T9 chegar | Sem testes integration ainda — T9 cobre |

Plano:

1. **httpapi.RateLimit middleware** — token bucket in-memory por (userID|IP, minuto). Default 60 req/min; `429 Too Many Requests` com `Retry-After`. Identidade preferida = `principal.UserID` (via `auth.PrincipalFromContext` se presente), fallback para IP (`r.RemoteAddr` ou `X-Forwarded-For`). Cleanup periodico de buckets velhos (a cada 5 min). Plugar no `Chain` do app.go *antes* do `Logging` para o 429 ser logado.
2. **slog em mutations de tasks** — adicionar `logger *slog.Logger` em `tasks.Service`. Helper privado `service.logMutation(action, access, attrs...)` com `slog.Info` carregando `accountId`, `userId`, `action`, e `resourceType:resourceId` quando aplicavel. Aplicado a CreateTask, UpdateTask, MoveTask, DeleteTask, CreateBoard, UpdateBoard, DeleteBoard, AddRelation, AddShare, AddComment, StartTracking, StopTracking. Erros sem expor IDs de outras accounts (na pratica, ja garantido porque `scopedQuery` rejeita antes de qualquer mensagem).
3. **NÃO mexer em RemoveRelation** agora — endpoint ainda nao existe. Anotar no AGENT.md que quando for criado, deve publicar `task.relation_removed` para o composable `useTaskRelations` invalidar cache.

Validacao:

- `cd back && go test ./...` passa.
- Smoke manual: 70 requisicoes em 60s contra `/v1/tasks/boards` -> a 61a retorna 429 com `Retry-After: <s>`. (User faz o smoke; nao roda CI agora.)

Nao repetir:

- Rate limit REST tem que considerar `principal.UserID` antes do IP — usuario atras de proxy compartilhado nao pode ser limitado em grupo. IP e' fallback so para nao-autenticados.
- slog **nunca** inclui IDs de outras accounts em mensagens de erro. Mantemos sempre o `accountId` do principal/access — qualquer cross-account ja virou 404 antes do log.
- Nao logar payloads inteiros — so chaves estruturais (`taskId`, `boardId`). Comentarios/titulos podem ter PII; evitar.

---

### T9 — Testes Go (sem DB), iniciado 2026-05-15

Escopo desta rodada — testes unitarios em Go que rodam sem Postgres real:

| Arquivo | O que cobre |
|---------|-------------|
| `tasks/dto_test.go` | `BuildTaskDTO` em perspective `agency` (mantem `clientAccountId`) vs `client_viewer` (omite `clientAccountId`). Tambem cobre `UIMetadata` sempre nao-nil e formatos ISO. |
| `tasks/cursor_test.go` | `encodeListTasksCursor` / `decodeListTasksCursor` — round-trip preserva tupla, base64url, decodifica vazio = `false`, cursor invalido = `false` (nao panica). |
| `realtime/presence_test.go` | `PresenceStore`: Join publica `user_joined` apenas na primeira conexao; LockField recusa quando outro user ja tem o `fieldKey` (T7.2); UnlockField libera; Leave decrementa connections; cleanup expira por TTL. |
| `httpapi/rate_limit_test.go` | `RateLimit`: dentro do limite passa; excede vira 429 com `Retry-After`; reset depois da janela; resolver custom precede IP; fallback para X-Forwarded-For e RemoteAddr. |

Pendencias deixadas para o usuario (ou T9.1):

- **Integration tests com DB real** (fuzz 100 IDs cross-account, scope_test.go, tracking_test.go com version conflict) — exigem `pg_test` / docker-compose; mantemos fora da CI agora.
- **Vitest no front** — `web/package.json` nao tem Vitest configurado. Adicionar `vitest`/`@vue/test-utils`/`happy-dom` e plugar no Nuxt e' uma instalacao nao trivial; deixei como pendencia explicita.
- **Smoke E2E 12 passos** — depende do user rodar `docker compose up` + Nuxt em 3003 + login real. Roteiro detalhado fica no roadmap.

Validacao desta rodada:

- `cd back && go test ./...` precisa rodar verde — os novos arquivos `_test.go` devem ser executados (modulos tasks/realtime/httpapi nao tinham testes antes).
- `go test ./internal/modules/tasks -run TestBuildTaskDTO -v` deve mostrar pelo menos 1 caso para cada perspective.

Nao repetir:

- Nao usar pgx pool em unit test — repository mock satisfazendo `Repository` interface, ou testar so funcoes puras (cursor, DTO). Integration test fica em ambiente separado.
- Tests de presence devem usar `time.Now()` injetavel quando possivel — TTL fixo de 30s em produccao, mas no test usar TTL curto (`100ms`) para nao depender de `time.Sleep` longo.
- Rate limit test nao pode depender de `time.Sleep(window)` — exposer `now` injetavel ou usar `Window` curto (100ms).

---

### T9 — Fechamento (mock Repository + Vitest + smoke roteiro), 2026-05-15

Cobertura adicional desta rodada:

| Arquivo | O que cobre |
|---------|-------------|
| `tasks/repository_mock_test.go` | Mock leve de `Repository` (30+ metodos com hooks `onXxx` opcionais e captura passiva de audit). Usado pelos service/scope/tracking tests. |
| `tasks/service_test.go` | 10 testes: CreateTask happy path (audit gerado), no-perm = 403, validation, GetTask perspective controla `clientAccountId`, GetTask 404 passthrough, ListTasks default limit / clamp >200 / no-perm 403 / perspective propaga / nextCursor propaga. |
| `tasks/scope_test.go` | 8 testes: accountID vazio -> ErrAccountRequired; account inexistente -> 404; **cross-account = 404 (nunca 403)**; platform_admin bypassa membership; perspective client_viewer quando so tem `client_view`; `boards.manage` override; **fuzz 100 IDs cross-account -> 100% 404**; `scopedQuery` panica sem accountID. |
| `tasks/tracking_test.go` | 8 testes: no-perm 403; task not found 404; happy path publica WS + audita; **PauseTracking propaga `ErrVersionConflict` -> 409**; ResumeTracking passa `expectedVersion`; StopTracking 404 nao publica nem audita; ListActiveTimeEntries aceita `view_all` e rejeita sem perm. |
| `web/layers/tasks/utils/text.ts` + `text.test.ts` | Extracao de `clampText`/`normalizeText` para util compartilhado; 9 testes Vitest cobrindo trim/colapso/clamp e o caso critico T7.2 (`clampText('palavra ') === 'palavra '`). |
| `web/vitest.config.ts` + `package.json` | Vitest 2.1 instalado como devDependency; scripts `test`/`test:watch`; `npm test` roda no node environment. |

Estatisticas:

- **Go**: 50 testes novos passando (4 cursor + 4 DTO + 6 presence + 6 rate-limit + 10 service + 8 scope + 8 tracking + 4 antigos cursor/DTO).
- **Vitest**: 9 testes passando.
- **Cobertura por funcao critica**: `BuildTaskDTO`, `ResolveAccessContext`, `ListTasks`, `CreateTask`, `GetTask`, `StartTracking`, `PauseTracking`, `ResumeTracking`, `StopTracking`, `ListActiveTimeEntries`, `LockField` (T7.2), `RateLimit` (T8), `listTasksCursor`, `clampText`, `normalizeText`.

Pendencias deixadas:

- **Integration tests com Postgres real** ainda nao implementados. O smoke E2E manual abaixo cobre na pratica.
- **`@nuxt/test-utils` + `happy-dom`** para testar composables Vue completos: fica para rodada futura. Por enquanto, util puros sao testaveis.

#### Smoke E2E 12 passos — roteiro manual para staging

Pre-requisitos:
- `docker compose up -d postgres api`
- `cd back && go run ./cmd/migrate`
- `npm --prefix web run dev` em port 3003

Passos:

1. **Migrate fresh** — `go run ./cmd/migrate` aplica ate `0111_users_nick.sql`.
2. **Seed nick** — `psql -c "update core.users set nick='alice' where email='owner@demo.local'; update users set nick='alice' where email='owner@demo.local';"`. Repetir para um segundo user de teste.
3. **Login agencia** — abrir `http://localhost:3003/login`, logar como `owner@demo.local` / `dev123456`. Verificar no DevTools que `auth/me` retorna `nick` no JSON.
4. **Criar task** — em `/tasks`, criar uma task num board. Network deve mostrar `POST /v1/tasks/boards/:id/tasks` 201 com `X-Account-Id` no header.
5. **WS task event** — abrir DevTools > WS > filtrar `/v1/realtime/tasks` — deve receber `task.created` com `accountId` correto.
6. **Presence dual-tab** — abrir uma segunda aba/browser com outro user logado, abrir o mesmo modal. Avatar do outro user aparece com o nick. Focar mesmo campo nas duas abas: a segunda input fica `:disabled` e NAO consegue digitar (T7.2 lock exclusivo).
7. **Espaco no card** — renomear card inline pelo board digitando `"smoke fase 12 "` (espaco no final, depois digite outra palavra). Espaco DEVE ficar preservado durante a digitacao.
8. **Tracking** — start/pause/resume/stop no modal. Network mostra `If-Match: <version>`. Forcar conflito: editar a task em outra aba antes do pause -> 409.
9. **Share** — adicionar share com um clientAccountId. Outro browser logado como cliente desse account ve a task com `clientAccountId === undefined` no payload (perspective `client_viewer`).
10. **Cross-account = 404** — `curl -H "Authorization: Bearer ..." -H "X-Account-Id: ACCOUNT_DIFERENTE" http://localhost:8883/v1/tasks/TASK_DA_MINHA_ACCOUNT` -> deve voltar `404` (nunca 403).
11. **Rate limit REST** — script `for i in {1..70}; do curl -H "Authorization: Bearer ..." http://localhost:8883/v1/tasks/boards; done`. A 61a vira `429` com header `Retry-After`.
12. **Audit + slog** — `psql -c "select action, resource_type, resource_id, account_id from tasks.tasks_audit order by created_at desc limit 10;"` mostra as mutations dos passos 4-9. `tail -f` no log do API mostra `tasks.mutation` com `account_id`/`user_id`/`resource_type` em cada CRUD.

---

### Reversao 2026-05-15 — patch-local-realtime trocado por refresh full debounced

**Sintoma reportado:** com duas abas/usuarios abertos no mesmo board, edicoes em uma aba NAO apareciam na outra. O modulo de `operations` funciona perfeitamente nesse cenario; o de `tasks` ficou silencioso.

**Causa raiz:** o `scheduleTasksRealtimeRefresh` em `useTasksPageContext.ts` tentava otimizar evitando `tasksWorkspace.refresh()` (full reload) e chamando `tasksWorkspace.hydrateTask(taskId)` (single GET) para eventos `task.updated`/`task.moved`. O guard era:

```ts
if (taskId && TASK_REALTIME_HYDRATE_EVENTS.has(type) && tasksWorkspace.tasks.value.some(t => t.id === taskId)) {
    tasksWorkspace.hydrateTask(taskId)
    return
}
```

Quando user B editava uma task que user A acabou de carregar via `refresh()`, funcionava. Mas em N cenarios comuns nao:

- User B cria task -> evento `task.created` chega em A; mas se o tipo nao estiver em `TASK_REALTIME_HYDRATE_EVENTS` (criar nao estava), caia no refresh full — OK. Porem se chegasse antes do refresh full inicial, `hydrateTask` nao saberia da task e retornaria null. Race.
- User B edita uma task que user A nao tinha carregado ainda (filtro/paginacao) -> guard `some(t => t.id === taskId)` retorna false -> NADA acontece. Bug.
- Eventos sem `taskId` populado caiam no refresh full, OK; mas evento `task.tracking_changed` nao publicado pelo backend -> tracking nao atualizava em outras abas.

**Resultado:** silencio total em muitos casos. UX degradou ao ponto de parecer "WS quebrado", que e' o que o usuario reportou.

**Decisao:** voltar ao padrao do `useOperationsRealtime` — SEMPRE `tasksWorkspace.refresh()` debounced (200ms). Sem otimizacao, sem branching, sem hidratacao individual. Operations roda assim em producao ha meses sem queixas; tasks nao precisa ser "mais esperto".

**O que foi removido:**

- `TASK_REALTIME_HYDRATE_EVENTS` (set de tipos de evento) — apagado de `useTasksPageContext.ts`.
- `tasksWorkspace.hydrateTask(taskId)` no path de eventos realtime — nao chamado mais.
- Endpoint `GET /v1/tasks/{taskId}` continua funcional (usado por outras coisas como `useTaskRelations` quando precisar) mas a funcao `hydrateTask` foi REMOVIDA do store, pois nao era usada por mais nada.
- Funcao auxiliar `shouldApplyTasksRealtimeEvent` inlineada dentro do `scheduleTasksRealtimeRefresh` (era usada so la).

**O que foi adicionado:**

- Logs `[tasks-ws]` no `console.info`/`debug`/`warn`/`error` em pontos chave:
  - `[tasks-ws] socket OPEN` quando conecta
  - `[tasks-ws] evento recebido — refresh agendado: { type, taskId, boardId, version }` quando chega um evento aplicavel
  - `[tasks-ws] ignorando evento nao-tasks: realtime.connected` (e similares) — filtrados
  - `[tasks-ws] evento de outra account, ignorado: { eventAccountId, currentAccountId }` — defesa em camada
  - `[tasks-ws] executando refresh full do workspace` quando o debounce dispara
  - `[tasks-ws] refresh concluido — tasks: N` quando termina
  - `[tasks-ws] socket CLOSED — agendando reconexao: { code, reason, wasClean }`
  - `[tasks-ws] socket ERROR`

Filtre por `[tasks-ws]` no console do browser para diagnostico rapido sem mim.

**Heuristica de "preservar titulo local" ajustada:** continua existindo em `store.updateTask`, mas agora so dispara quando `localTask.title !== localTask.title.trim()` (ou seja, ha trailing whitespace, sinal de digitacao em curso). Antes disparava sempre que `normalizeText(local) === mapped.title`, o que podia mascarar updates remotos legitimos. Em refresh realtime (`tasksWorkspace.refresh()`), o caminho e' outro — passa por `replaceTask` direto sem essa logica.

**Nao repetir:**

- "Patching local" so faz sentido quando ha protocolo de sync com versao explicita (CRDT, Y.js). Para um refresh debounced, full reload e' o caminho — operations comprova.
- Logs de WS no console em pontos chave nao sao opcionais quando o feature e' WS — sem eles, diagnostico depende de Network > WS frames, que e' pesado.
- "Otimizar" antes de medir e' a fonte do bug. A T7 originalmente nao tinha patch local; introduzimos na pressa de "evitar flicker" sem checar se de fato havia flicker problematico. Nao havia.

### Correcao 2026-05-15 — presence assimetrico por guard local

**Sintoma reportado:** em duas sessoes com usuarios diferentes, uma pessoa via que a outra estava editando, mas o inverso nao acontecia de forma confiavel. Em alguns casos o bloqueio de campo sumia ou ficava preso.

**Causa raiz:** `useTaskPresence.focusField()` abortava antes de enviar `presence.field_focus` quando `usersForField(key).length > 0`. Esse guard era local, baseado em snapshot possivelmente defasado. Pior: ele rodava antes de liberar `activeFieldKey` antigo, entao clicar em um campo que o client achava ocupado podia deixar o lock anterior preso.

**Decisao:** servidor e UI sao as fontes de verdade. O client sempre:

- libera o campo ativo anterior antes de trocar;
- envia `field_focus` para o servidor decidir se aceita;
- preserva `activeFieldKey` quando o socket reconecta no mesmo canal, para reenviar o lock ao abrir;
- libera o lock ativo em `visibilitychange:hidden`, `pagehide`, fechamento de modal e unmount;
- registra logs `[tasks-presence]` para socket open/close/error, snapshot, field_focus, field_blur, field_locked e field_unlocked.

**Nao repetir:** nao reintroduzir guard client-side de presence baseado apenas em `usersForField()`. Se precisar de UX mais forte, adicionar ACK/denied direcionado no protocolo de presence; nao bloquear o envio antes do servidor.

### Correcao 2026-05-16 — board realtime + recuperacao de lock defasado

**Sintoma reportado:** o WS parecia "funcionando", mas dois usuarios ainda podiam ver dados diferentes no mesmo board e a presence continuava assimetrica em alguns casos.

**Causas adicionais:**

- `useTasksRealtime` estava conectado somente em `scope=account`. Isso falha para usuarios em outra conta/escopo vendo o mesmo board compartilhado, porque o backend tambem publica `tasks:board:{boardId}` e o front nao estava escutando esse canal.
- `PresenceStore.LockField` recusava lock concorrente em silencio. Se um client perdesse snapshot/evento anterior, tentar focar um campo ocupado nao recuperava o estado correto.
- `window.blur` liberava o lock ativo, atrapalhando teste manual com dois browsers lado a lado e qualquer troca rapida de janela.

**Correcoes:**

- `useTasksPageContext.ts` agora abre dois WS de tasks: `scope=account` e `scope=board` do board ativo. Ambos alimentam o mesmo debounce de refresh full.
- `useTasksRealtime.ts` loga `scope`, `accountId`, `boardId` e `taskId` no `[tasks-ws] socket OPEN`.
- `PresenceStore.LockField` continua exclusivo, mas quando nega uma tomada de lock republica o `presence.field_locked` do usuario que ja possui o campo.
- `useTaskPresence.ts` removeu release em `window.blur`; release continua em focusout dos campos, `visibilitychange:hidden`, `pagehide`, fechamento de modal e unmount.

**Nao repetir:** canal `account` nao substitui canal `board` em fluxo colaborativo. Para qualquer tela de board aberto, manter assinatura `tasks:board:{boardId}`.

---

### Próximas fases (planejadas)

| Fase | O que entrega | Depende de |
|------|---------------|------------|
| T5 | Front substitui localStorage pelo backend (Pinia store real, wipe legacy, useCan) | T1–T4 |
| T6 | Tracking server-side autoritativo (start/pause/resume/stop, clockOffset) | T1, T2, T5 |
| T7 | Presence MVP (front aplicado: avatares + field locking leve, heartbeat 15s) | T2 |
| T8 | Segurança, audit log, rate limit, hardening | T1–T7 |
| T9 | Testes E2E + observabilidade | T8 |
| T10 | Sistema de Views (11 tipos, /views/:id/data) | T5 |

**Caminho mínimo para MVP usável:** T5, porque T0, T0.5, T1, T2, T3 e T4 já foram concluídas em 2026-05-14.
