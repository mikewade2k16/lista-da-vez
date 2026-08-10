# AGENT — `web/app/components/omni/`

## Escopo

Componentes genéricos de tabela admin reutilizáveis em qualquer workspace que precise listar coleções com filtros, configuração de colunas e CRUD inline. **Não confundir com `omnichannel` (módulo de mensageria) — `omni` aqui é só "tabela genérica".**

## Componentes

### `table/OmniDataTable.vue`

A tabela admin canônica. Consome `columns: OmniTableColumn[]` já ordenadas e filtradas (feito pelo `useOmniVisibleColumns`) e renderiza linha por linha. Edição inline emite `update:cell` com `{rowId, key, value, immediate?}`. Suporta seleção em massa via `v-model` (Array de ids), focus programático via `focusCell`, e slots por coluna via `cell-<columnKey>`.

### `table/OmniTableColumnsConfig.vue` (C16, 2026-05-29)

Popover de configuração de colunas. Por usuário comum: só checkboxes para mostrar/esconder. **Para admin** (`viewerUserType === 'admin'`):

- **Cadeado por coluna** (ícone `i-lucide-lock`/`i-lucide-lock-open`): admin trava uma coluna como sempre visível; usuários não conseguem escondê-la. Quando travada, o checkbox fica `disabled` e mostra estilo destacado.
- **Drag handle por coluna** (ícone `i-lucide-grip-vertical`): admin reordena via drag-n-drop HTML5 nativo (sem lib externa).
- **Reset** (ícone `i-lucide-undo-2`): emite `reset`; workspace conecta ao `resetToDefaults` do composable.

Estado emitido via `update:modelValue` (visibleKeys), `update:lockedKeys`, `update:columnOrder`.

### `filters/OmniCollectionFilters.vue`

Barra superior da tabela. Renderiza filtros declarados em `OmniFilterDefinition[]` + slot `actions` + integra o `OmniTableColumnsConfig`. Propaga `viewerUserType`, `lockedColumns`, `columnOrder` para o config.

### `overlay/OmniMinimalPopover.vue`

Popover simples reutilizável. Emite `opened` quando abre (use isso para fetch lazy de detalhes em vez de `@click` no botão trigger — o popover engole o click).

### `OmniEditor.vue`

Editor rico reutilizável. A prop `compact` reduz toolbar, espaçamento e área de escrita para
modais densos. Menus de comando, menção e emoji são portaled no `document.body` com
posicionamento fixo para não serem recortados por painéis e modais com rolagem.

## Composable de suporte

### `useOmniVisibleColumns({ preferenceKey, allColumns, columnExcludeKeys })`

Persiste 3 estados em `useAdminPreferences` (localStorage, chaves `ui.columns`, `ui.columns_locked`, `ui.columns_order` indexadas por `preferenceKey`). Retorna `visibleColumnKeys`, `lockedColumnKeys`, `columnOrder`, `tableColumns` (já ordenadas + filtradas), `resetToDefaults`.

Ordem aplicada: `columnOrder` (custom do user) → `defaultOrder` declarado na coluna → ordem original. Stable.

Filtro aplicado em `tableColumns`: coluna passa se está em `excludeKeys` (ex.: 'actions'), em `lockedKeys` (admin travou) ou em `visibleKeys` (usuário escolheu).

`alwaysVisibleColumnKeys` é input deprecated — quando passado, vira `declaredLockedKeys`. Compat retroativa para workspaces antigos (`Site*AdminWorkspace`).

## Padrões para criar workspace novo

1. Defina `allTableColumns` com `defaultOrder` numérico (10, 20, 30... — deixa espaço para inserções) e `locked: true` na coluna de identidade (nome, email).
2. `actions` no `columnExcludeKeys` para o seletor não permitir esconder.
3. Use `<OmniCollectionFilters v-model:visible-columns v-model:locked-columns v-model:column-order :viewer-user-type :column-exclude-keys @reset-columns="resetToDefaults">`.
4. Use `<OmniDataTable :columns="tableColumns">` (vem do composable, já ordenada/filtrada).
5. Scroll: envolva o `OmniDataTable` em `<div class="flex-1 min-h-0 overflow-y-auto">` dentro de uma `<section class="flex h-full min-h-0 flex-col gap-4 overflow-hidden">`.

## Drift cross-camada

A regra de auto-gerar nick (`buildNickname`) vive em DUAS camadas:

- Frontend: `web/app/domain/utils/person-display.ts > buildNickname`
- Backend: `back/internal/modules/core/nick.go > BuildNickname`

Mudou uma → mude a outra na mesma PR. Drift gera nicks diferentes dependendo de onde o user foi criado.
