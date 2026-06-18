<script setup>
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { Plus, RotateCcw, Trash2, Check, X } from 'lucide-vue-next'

import AppEntityGrid from '~/components/ui/AppEntityGrid.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import { useMultiStoreStore } from '~/stores/multistore'
import { useUiStore } from '~/stores/ui'

const CREATE_ROW_ID = '__create-store-row__'

const props = defineProps({
  managedStores: {
    type: Array,
    default: () => [],
  },
  operationTemplates: {
    type: Array,
    default: () => [],
  },
  canEdit: {
    type: Boolean,
    default: false,
  },
})

const multiStore = useMultiStoreStore()
const ui = useUiStore()

const search = ref('')
const statusFilter = ref('active')
const sortBy = ref('identity')
const sortDir = ref('asc')
const drafts = ref({})
const rowBusy = reactive({})
// touched[id] = o usuario editou essa linha e ainda nao salvou. So enquanto
// touched/rowBusy o draft e' preservado; fora disso ele e' SEMPRE re-hidratado
// do servidor (fonte = banco via /v1/stores). Sem isso, um draft semeado de uma
// fonte parcial (contexto sem storeType, que chega antes do /v1/stores) ficava
// preso e o select revertia para 'bairro' no reload mesmo com o banco 'shopping'.
const touched = reactive({})
const showCreate = ref(false)
const createBusy = ref(false)
const createDraft = reactive({
  name: '',
  code: '',
  city: '',
  storeType: 'bairro',
  defaultTemplateId: '',
})

const statusOptions = [
  { value: 'active', label: 'Status: ativas' },
  { value: 'archived', label: 'Status: arquivadas' },
  { value: 'all', label: 'Status: todas' },
]

// Tipo de loja: define a tabela de faixas do payout de gerente no back
// (Gerente Shopping vs Gerente Bairro). Default 'bairro'.
const storeTypeOptions = [
  { value: 'bairro', label: 'Bairro' },
  { value: 'shopping', label: 'Shopping' },
]

const templateOptions = computed(() => [
  { value: '', label: 'Template padrao' },
  ...(props.operationTemplates || []).map((t) => ({
    value: normalizeText(t.id),
    label: normalizeText(t.label),
  })),
])

const filteredRows = computed(() => {
  const term = normalizeSearch(search.value)
  return (props.managedStores || []).filter((store) => {
    if (statusFilter.value === 'active' && store.isActive === false) return false
    if (statusFilter.value === 'archived' && store.isActive !== false) return false
    if (!term) return true
    return normalizeSearch([store.name, store.code, store.city].filter(Boolean).join(' ')).includes(
      term,
    )
  })
})

const sortedRows = computed(() => {
  const rows = [...filteredRows.value]
  rows.sort((a, b) => compareRows(a, b, sortBy.value, sortDir.value))
  return rows
})

const tableRows = computed(() => {
  if (!props.canEdit || !showCreate.value) return sortedRows.value
  return [{ id: CREATE_ROW_ID }, ...sortedRows.value]
})

const gridColumns = computed(() => {
  const columns = [
    {
      id: 'identity',
      label: 'Loja',
      width: 'minmax(200px, 1.5fr)',
      sortable: true,
      locked: true,
    },
    { id: 'code', label: 'Codigo', width: '120px', sortable: true },
    { id: 'city', label: 'Cidade', width: 'minmax(120px, 1fr)', sortable: true },
    {
      id: 'storeType',
      label: 'Tipo de loja',
      width: 'minmax(130px, 1fr)',
    },
    {
      id: 'defaultTemplateId',
      label: 'Template',
      width: 'minmax(140px, 1fr)',
    },
    { id: 'status', label: 'Status', width: '140px', sortable: true },
  ]
  if (props.canEdit) {
    columns.push({ id: 'actions', label: '', width: '60px', align: 'end', locked: true })
  }
  return columns
})

/* ===== Drafts: preserva entre reloads, salva no blur ===== */

watch(
  () => props.managedStores,
  (stores) => {
    const next = {}
    for (const store of stores || []) {
      const keepDraft = touched[store.id] || rowBusy[store.id]
      next[store.id] = keepDraft
        ? (drafts.value[store.id] ?? createDraftFromStore(store))
        : createDraftFromStore(store)
    }
    drafts.value = next
  },
  { immediate: true, deep: true },
)

onBeforeUnmount(() => {
  for (const store of props.managedStores || []) {
    if (rowBusy[store.id]) continue
    if (!isRowDirty(store)) continue
    void saveRow(store)
  }
})

/* ===== Ações ===== */

async function saveRow(row) {
  if (rowBusy[row.id]) return { ok: true, noChange: true }
  if (!isRowDirty(row)) return { ok: true, noChange: true }

  rowBusy[row.id] = true
  try {
    const draft = getDraft(row.id)
    const result = await multiStore.updateStore(row.id, {
      name: draft.name,
      code: draft.code,
      city: draft.city,
      storeType: draft.storeType,
      defaultTemplateId: draft.defaultTemplateId,
    })
    if (result?.ok === false) {
      ui.error(result.message || 'Nao foi possivel atualizar a loja.')
      return result
    }
    // Salvou: o servidor (banco) volta a ser a fonte; libera a re-hidratacao.
    touched[row.id] = false
    return result
  } finally {
    rowBusy[row.id] = false
  }
}

async function toggleStatus(row) {
  if (rowBusy[row.id]) return
  const isArchived = row.isActive === false

  if (!isArchived) {
    const { confirmed } = await ui.confirm({
      title: 'Arquivar loja',
      message: `A loja ${row.name} sera removida da operacao ativa. Voce pode restaurar depois.`,
      confirmLabel: 'Arquivar',
    })
    if (!confirmed) return
  }

  rowBusy[row.id] = true
  try {
    const result = isArchived
      ? await multiStore.restoreStore(row.id)
      : await multiStore.archiveStore(row.id)
    if (result?.ok === false) {
      ui.error(result.message || 'Nao foi possivel alterar o status.')
      return
    }
    ui.success(isArchived ? 'Loja restaurada.' : 'Loja arquivada.')
  } finally {
    rowBusy[row.id] = false
  }
}

async function deleteRow(row) {
  if (rowBusy[row.id]) return
  const { confirmed } = await ui.confirm({
    title: 'Excluir loja',
    message: `${row.name} sera excluida permanentemente. Consultores cadastrados ficarao sem loja e podem ser realocados depois. Atendimentos em andamento podem ser encerrados; nao e possivel iniciar novos. Continuar?`,
    confirmLabel: 'Excluir',
  })
  if (!confirmed) return

  rowBusy[row.id] = true
  try {
    const result = await multiStore.deleteStore(row.id)
    if (result?.ok === false) {
      ui.error(result.message || 'Nao foi possivel excluir.')
      return
    }
    ui.success('Loja removida.')
  } finally {
    rowBusy[row.id] = false
  }
}

async function createInline() {
  const name = normalizeText(createDraft.name)
  const code = normalizeText(createDraft.code).toUpperCase()
  if (!name) {
    ui.error('Informe o nome da loja.')
    return
  }
  if (!code) {
    ui.error('Informe o codigo da loja.')
    return
  }

  createBusy.value = true
  try {
    const result = await multiStore.createStore({
      name,
      code,
      city: normalizeText(createDraft.city),
      storeType: createDraft.storeType,
      defaultTemplateId: normalizeText(createDraft.defaultTemplateId),
    })
    if (result?.ok === false) {
      ui.error(result.message || 'Nao foi possivel criar a loja.')
      return
    }
    createDraft.name = ''
    createDraft.code = ''
    createDraft.city = ''
    createDraft.storeType = 'bairro'
    createDraft.defaultTemplateId = ''
    showCreate.value = false
    ui.success('Loja criada.')
  } finally {
    createBusy.value = false
  }
}

function cancelCreate() {
  createDraft.name = ''
  createDraft.code = ''
  createDraft.city = ''
  createDraft.storeType = 'bairro'
  createDraft.defaultTemplateId = ''
  showCreate.value = false
}

function clearFilters() {
  search.value = ''
  statusFilter.value = 'active'
  sortBy.value = 'identity'
  sortDir.value = 'asc'
}

function toggleSort(columnId) {
  if (sortBy.value === columnId) {
    sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
    return
  }
  sortBy.value = columnId
  sortDir.value = 'asc'
}

function updateDraftField(row, field, event) {
  const value = String(event?.target?.value ?? '')
  if (isCreateRow(row)) {
    createDraft[field] = field === 'code' ? value.toUpperCase() : value
    return
  }
  touched[row.id] = true
  const draft = getDraft(row.id)
  draft[field] = field === 'code' ? value.toUpperCase() : value
}

function updateTemplate(row, value) {
  if (isCreateRow(row)) {
    createDraft.defaultTemplateId = String(value || '')
    return
  }
  touched[row.id] = true
  getDraft(row.id).defaultTemplateId = String(value || '')
  void saveRow(row)
}

function updateStoreType(row, value) {
  const next = normalizeStoreType(value)
  if (isCreateRow(row)) {
    createDraft.storeType = next
    return
  }
  touched[row.id] = true
  getDraft(row.id).storeType = next
  void saveRow(row)
}

function handleInlineBlur(row) {
  if (isCreateRow(row)) return
  void saveRow(row)
}

function handleEnter(event) {
  event?.target?.blur?.()
}

/* ===== Helpers ===== */

function getDraft(id) {
  if (!drafts.value[id]) drafts.value[id] = createDraftFromStore({})
  return drafts.value[id]
}

function createDraftFromStore(store = {}) {
  return {
    name: String(store.name || ''),
    code: String(store.code || ''),
    city: String(store.city || ''),
    storeType: normalizeStoreType(store.storeType),
    defaultTemplateId: String(store.defaultTemplateId || ''),
  }
}

function isRowDirty(row) {
  const draft = getDraft(row.id)
  return (
    normalizeText(draft.name) !== normalizeText(row.name) ||
    normalizeText(draft.code).toUpperCase() !== normalizeText(row.code).toUpperCase() ||
    normalizeText(draft.city) !== normalizeText(row.city) ||
    normalizeStoreType(draft.storeType) !== normalizeStoreType(row.storeType) ||
    normalizeText(draft.defaultTemplateId) !== normalizeText(row.defaultTemplateId)
  )
}

function normalizeStoreType(value) {
  return normalizeText(value).toLowerCase() === 'shopping' ? 'shopping' : 'bairro'
}

function resolveStoreTypeLabel(value) {
  return normalizeStoreType(value) === 'shopping' ? 'Shopping' : 'Bairro'
}

function compareRows(a, b, field, direction) {
  const mult = direction === 'asc' ? 1 : -1
  const av = sortValue(a, field)
  const bv = sortValue(b, field)
  if (av < bv) return -1 * mult
  if (av > bv) return 1 * mult
  return 0
}

function sortValue(row, field) {
  switch (field) {
    case 'identity':
      return normalizeSearch(row.name)
    case 'code':
      return normalizeSearch(row.code)
    case 'city':
      return normalizeSearch(row.city)
    case 'status':
      return row.isActive === false ? 1 : 0
    default:
      return ''
  }
}

function normalizeText(value) {
  return String(value || '').trim()
}

function normalizeSearch(value) {
  return String(value || '')
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .trim()
    .toLowerCase()
}

function resolveTemplateLabel(id) {
  const normalized = normalizeText(id)
  if (!normalized) return 'Padrao'
  const found = (props.operationTemplates || []).find((t) => normalizeText(t.id) === normalized)
  return found ? normalizeText(found.label) : 'Padrao'
}

function isCreateRow(row) {
  return row?.id === CREATE_ROW_ID
}
</script>

<template>
  <section class="multistore-lojas" data-testid="multistore-lojas-section">
    <AppEntityGrid
      :columns="gridColumns"
      :rows="tableRows"
      :row-key="(row) => row.id"
      :loading="false"
      :search-value="search"
      :sort-by="sortBy"
      :sort-dir="sortDir"
      storage-key="multistore-lojas-columns-v1"
      empty-title="Nenhuma loja"
      empty-text="Clique no botao + para cadastrar a primeira loja."
      testid="multistore-lojas-grid"
      class="multistore-lojas__grid"
      @update:search-value="search = $event"
      @sort="toggleSort"
    >
      <template #toolbar-filters>
        <AppSelectField
          class="multistore-lojas__toolbar-select"
          :model-value="statusFilter"
          :options="statusOptions"
          :show-leading-icon="false"
          compact
          @update:model-value="statusFilter = $event"
        />
      </template>

      <template #toolbar-actions>
        <button
          v-if="canEdit"
          class="multistore-lojas__icon-btn multistore-lojas__icon-btn--primary"
          type="button"
          :disabled="showCreate"
          title="Nova loja"
          aria-label="Nova loja"
          @click="showCreate = true"
        >
          <Plus :size="14" :stroke-width="2.2" />
        </button>

        <button
          class="multistore-lojas__icon-btn multistore-lojas__icon-btn--ghost"
          type="button"
          title="Limpar filtros"
          aria-label="Limpar filtros"
          @click="clearFilters"
        >
          <RotateCcw :size="13" :stroke-width="2.1" />
        </button>
      </template>

      <template #cell-identity="{ row }">
        <input
          v-if="(canEdit && row.isActive !== false) || isCreateRow(row)"
          :value="isCreateRow(row) ? createDraft.name : getDraft(row.id).name"
          class="multistore-lojas__input multistore-lojas__input--strong"
          type="text"
          :placeholder="isCreateRow(row) ? 'Nome da loja' : 'Nome'"
          :disabled="!isCreateRow(row) && rowBusy[row.id]"
          @input="updateDraftField(row, 'name', $event)"
          @blur="handleInlineBlur(row)"
          @keydown.enter.prevent="handleEnter($event)"
        />
        <span v-else class="multistore-lojas__readonly">{{ row.name }}</span>
      </template>

      <template #cell-code="{ row }">
        <input
          v-if="(canEdit && row.isActive !== false) || isCreateRow(row)"
          :value="isCreateRow(row) ? createDraft.code : getDraft(row.id).code"
          class="multistore-lojas__input multistore-lojas__input--mono"
          type="text"
          placeholder="ABC"
          maxlength="12"
          :disabled="!isCreateRow(row) && rowBusy[row.id]"
          @input="updateDraftField(row, 'code', $event)"
          @blur="handleInlineBlur(row)"
          @keydown.enter.prevent="handleEnter($event)"
        />
        <span v-else class="multistore-lojas__readonly multistore-lojas__readonly--mono">
          {{ row.code || '—' }}
        </span>
      </template>

      <template #cell-city="{ row }">
        <input
          v-if="(canEdit && row.isActive !== false) || isCreateRow(row)"
          :value="isCreateRow(row) ? createDraft.city : getDraft(row.id).city"
          class="multistore-lojas__input"
          type="text"
          placeholder="Cidade"
          :disabled="!isCreateRow(row) && rowBusy[row.id]"
          @input="updateDraftField(row, 'city', $event)"
          @blur="handleInlineBlur(row)"
          @keydown.enter.prevent="handleEnter($event)"
        />
        <span v-else class="multistore-lojas__readonly">{{ row.city || '—' }}</span>
      </template>

      <template #cell-storeType="{ row }">
        <AppSelectField
          v-if="(canEdit && row.isActive !== false) || isCreateRow(row)"
          class="multistore-lojas__inline-select"
          :model-value="isCreateRow(row) ? createDraft.storeType : getDraft(row.id).storeType"
          :options="storeTypeOptions"
          :show-leading-icon="false"
          compact
          :disabled="!isCreateRow(row) && rowBusy[row.id]"
          @update:model-value="updateStoreType(row, $event)"
        />
        <span v-else class="multistore-lojas__readonly">
          {{ resolveStoreTypeLabel(row.storeType) }}
        </span>
      </template>

      <template #cell-defaultTemplateId="{ row }">
        <AppSelectField
          v-if="(canEdit && row.isActive !== false) || isCreateRow(row)"
          class="multistore-lojas__inline-select"
          :model-value="
            isCreateRow(row) ? createDraft.defaultTemplateId : getDraft(row.id).defaultTemplateId
          "
          :options="templateOptions"
          :show-leading-icon="false"
          compact
          :disabled="!isCreateRow(row) && rowBusy[row.id]"
          @update:model-value="updateTemplate(row, $event)"
        />
        <span v-else class="multistore-lojas__readonly">
          {{ resolveTemplateLabel(row.defaultTemplateId) }}
        </span>
      </template>

      <template #cell-status="{ row }">
        <span v-if="isCreateRow(row)" class="multistore-lojas__status-chip is-pending">
          Rascunho
        </span>
        <label
          v-else
          class="multistore-lojas__toggle"
          :class="{
            'is-on': row.isActive !== false,
            'is-disabled': !canEdit || rowBusy[row.id],
          }"
          :title="
            !canEdit
              ? row.isActive === false
                ? 'Arquivada'
                : 'Ativa'
              : row.isActive === false
                ? 'Clique para ativar a loja'
                : 'Clique para arquivar (bloqueia novos atendimentos)'
          "
        >
          <input
            type="checkbox"
            :checked="row.isActive !== false"
            :disabled="!canEdit || rowBusy[row.id]"
            @change="toggleStatus(row)"
          />
          <span class="multistore-lojas__toggle-track">
            <span class="multistore-lojas__toggle-thumb"></span>
          </span>
          <span class="multistore-lojas__toggle-label">
            {{ row.isActive === false ? 'Arquivada' : 'Ativa' }}
          </span>
        </label>
      </template>

      <template v-if="canEdit" #cell-actions="{ row }">
        <div class="multistore-lojas__row-actions">
          <template v-if="isCreateRow(row)">
            <button
              class="multistore-lojas__icon-btn multistore-lojas__icon-btn--primary"
              type="button"
              :disabled="createBusy"
              title="Salvar nova loja"
              aria-label="Salvar"
              @click="createInline"
            >
              <Check :size="13" :stroke-width="2.2" />
            </button>
            <button
              class="multistore-lojas__icon-btn multistore-lojas__icon-btn--ghost"
              type="button"
              :disabled="createBusy"
              title="Cancelar"
              aria-label="Cancelar"
              @click="cancelCreate"
            >
              <X :size="13" :stroke-width="2.2" />
            </button>
          </template>
          <template v-else>
            <span v-if="rowBusy[row.id]" class="multistore-lojas__pulse" aria-label="Salvando">
              <span></span>
              <span></span>
              <span></span>
            </span>
            <button
              v-else
              class="multistore-lojas__icon-btn multistore-lojas__icon-btn--ghost"
              type="button"
              title="Excluir loja"
              aria-label="Excluir"
              @click="deleteRow(row)"
            >
              <Trash2 :size="13" :stroke-width="2.1" />
            </button>
          </template>
        </div>
      </template>
    </AppEntityGrid>
  </section>
</template>

<style scoped>
.multistore-lojas {
  display: grid;
  gap: 0.5rem;
}

/* Toolbar em linha única (espelha padrão de Metas) */
.multistore-lojas__grid :deep(.app-entity-grid) {
  padding: 0.6rem;
  gap: 0.5rem;
}

.multistore-lojas__grid :deep(.app-entity-grid__toolbar) {
  align-items: center;
  flex-wrap: nowrap;
  overflow-x: auto;
  gap: 0.4rem;
  padding-bottom: 0.1rem;
}

.multistore-lojas__grid :deep(.app-entity-grid__toolbar-main) {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  flex: 1 1 auto;
  min-width: 0;
}

.multistore-lojas__grid :deep(.app-entity-grid__search) {
  flex: 0 1 13rem;
  min-width: 10rem;
  min-height: 2rem;
}

.multistore-lojas__grid :deep(.app-entity-grid__search-input) {
  font-size: 0.74rem;
}

.multistore-lojas__grid :deep(.app-entity-grid__filters),
.multistore-lojas__grid :deep(.app-entity-grid__toolbar-actions) {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  flex-wrap: nowrap;
  flex: 0 0 auto;
}

.multistore-lojas__grid :deep(.app-entity-grid__toolbar-btn) {
  min-height: 2rem;
  padding: 0 0.6rem;
  font-size: 0.7rem;
}

.multistore-lojas__grid :deep(.app-entity-grid__head-cell) {
  font-size: 0.6rem;
  letter-spacing: 0.05em;
  padding: 0 0.25rem;
}

.multistore-lojas__grid :deep(.app-entity-grid__row) {
  padding: 0.28rem 0;
  gap: 0.4rem;
  align-items: center;
}

.multistore-lojas__grid :deep(.app-entity-grid__cell) {
  padding: 0 0.25rem;
  font-size: 0.76rem;
}

.multistore-lojas__toolbar-select {
  min-width: 9rem;
}

/* Botões de ícone (mesmo padrão das outras seções) */
.multistore-lojas__icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2rem;
  height: 2rem;
  border-radius: 0.55rem;
  border: 1px solid rgb(var(--ring) / 0.18);
  background: rgb(var(--surface-2) / 0.88);
  color: var(--text-main);
  cursor: pointer;
  flex-shrink: 0;
  transition:
    border-color 0.12s ease,
    background 0.12s ease,
    color 0.12s ease;
}

.multistore-lojas__icon-btn:hover:not(:disabled) {
  border-color: rgb(var(--ring) / 0.4);
  color: rgb(var(--text));
}

.multistore-lojas__icon-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.multistore-lojas__icon-btn--primary {
  border-color: rgb(var(--primary) / 0.4);
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary));
}

.multistore-lojas__icon-btn--primary:hover:not(:disabled) {
  background: rgb(var(--primary) / 0.24);
}

.multistore-lojas__icon-btn--ghost {
  background: transparent;
  border-color: rgb(var(--ring) / 0.12);
  color: var(--text-muted);
}

/* Inputs inline */
.multistore-lojas__input {
  width: 100%;
  min-width: 0;
  min-height: 1.9rem;
  padding: 0 0.5rem;
  border-radius: 0.5rem;
  border: 1px solid transparent;
  background: transparent;
  color: var(--text-main);
  font-size: 0.78rem;
  font-weight: 500;
  outline: none;
  transition:
    border-color 0.12s ease,
    background 0.12s ease;
}

.multistore-lojas__input:hover:not(:disabled) {
  border-color: rgb(var(--ring) / 0.12);
}

.multistore-lojas__input:focus {
  border-color: rgb(var(--ring) / 0.4);
  background: rgb(var(--surface-2) / 0.5);
}

.multistore-lojas__input--strong {
  font-weight: 700;
  font-size: 0.82rem;
}

.multistore-lojas__input--mono {
  font-family: ui-monospace, 'SF Mono', Menlo, Consolas, monospace;
  font-weight: 700;
  font-size: 0.76rem;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.multistore-lojas__input:disabled {
  opacity: 0.55;
}

.multistore-lojas__readonly {
  font-size: 0.78rem;
  color: var(--text-muted);
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.multistore-lojas__readonly--mono {
  font-family: ui-monospace, 'SF Mono', Menlo, Consolas, monospace;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.multistore-lojas__inline-select {
  min-width: 9rem;
}

/* Status chip */
.multistore-lojas__status-chip {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 1.6rem;
  padding: 0 0.65rem;
  border-radius: 999px;
  font-size: 0.68rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  border: 1px solid rgb(var(--ring) / 0.16);
  background: rgb(var(--surface-2) / 0.7);
  color: var(--text-muted);
  cursor: pointer;
  transition:
    background 0.12s ease,
    border-color 0.12s ease;
}

button.multistore-lojas__status-chip:hover:not(:disabled) {
  border-color: rgb(var(--ring) / 0.42);
}

.multistore-lojas__status-chip.is-active {
  border-color: rgb(34 197 94 / 0.4);
  background: rgb(34 197 94 / 0.16);
  color: rgb(34 197 94);
}

.multistore-lojas__status-chip.is-archived {
  border-color: rgb(var(--muted) / 0.3);
  background: rgb(var(--surface-2) / 0.45);
  color: var(--text-muted);
}

.multistore-lojas__status-chip.is-pending {
  border-color: rgb(var(--primary) / 0.35);
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
}

/* Toggle switch */
.multistore-lojas__toggle {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  cursor: pointer;
  user-select: none;
}

.multistore-lojas__toggle.is-disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.multistore-lojas__toggle input {
  position: absolute;
  opacity: 0;
  width: 0;
  height: 0;
  pointer-events: none;
}

.multistore-lojas__toggle-track {
  position: relative;
  display: inline-block;
  width: 1.85rem;
  height: 1.05rem;
  border-radius: 999px;
  background: rgb(var(--muted) / 0.3);
  border: 1px solid rgb(var(--ring) / 0.18);
  transition:
    background 0.18s ease,
    border-color 0.18s ease;
  flex-shrink: 0;
}

.multistore-lojas__toggle-thumb {
  position: absolute;
  top: 50%;
  left: 0.12rem;
  width: 0.75rem;
  height: 0.75rem;
  border-radius: 50%;
  background: rgb(255 255 255);
  box-shadow: 0 1px 2px rgb(0 0 0 / 0.25);
  transform: translateY(-50%);
  transition: left 0.18s ease;
}

.multistore-lojas__toggle.is-on .multistore-lojas__toggle-track {
  background: rgb(34 197 94 / 0.85);
  border-color: rgb(34 197 94 / 0.7);
}

.multistore-lojas__toggle.is-on .multistore-lojas__toggle-thumb {
  left: calc(100% - 0.87rem);
}

.multistore-lojas__toggle-label {
  font-size: 0.7rem;
  font-weight: 600;
  color: var(--text-muted);
  letter-spacing: 0.02em;
}

.multistore-lojas__toggle.is-on .multistore-lojas__toggle-label {
  color: rgb(34 197 94);
}

.multistore-lojas__row-actions {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 0.25rem;
}

.multistore-lojas__pulse {
  display: inline-flex;
  gap: 3px;
  padding: 0 0.3rem;
}

.multistore-lojas__pulse span {
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: rgb(var(--primary) / 0.6);
  animation: multistore-lojas-pulse 1s infinite ease-in-out;
}

.multistore-lojas__pulse span:nth-child(2) {
  animation-delay: 0.15s;
}

.multistore-lojas__pulse span:nth-child(3) {
  animation-delay: 0.3s;
}

@keyframes multistore-lojas-pulse {
  0%,
  80%,
  100% {
    transform: scale(0.7);
    opacity: 0.4;
  }
  40% {
    transform: scale(1);
    opacity: 1;
  }
}
</style>
