<script setup lang="ts">
import OmniSelectInput from '../../inputs/OmniSelectInput.vue'
import OmniMoneyInput from '../inputs/OmniMoneyInput.vue'
import OmniSwitchInput from '../inputs/OmniSwitchInput.vue'
import type {
  OmniFocusCell,
  OmniTableAction,
  OmniTableCellUpdate,
  OmniTableImageUpload,
  OmniTableColumn,
  OmniSelectOption
} from '../../../types/omni/collection'

const props = withDefaults(
  defineProps<{
    rows: Array<Record<string, any>>
    columns: OmniTableColumn[]
    viewerUserType?: 'admin' | 'client'
    rowKey?: string
    loading?: boolean
    emptyText?: string
    selectable?: boolean
    modelValue?: Array<string | number>
    focusCell?: OmniFocusCell | null
  }>(),
  {
    viewerUserType: 'admin',
    rowKey: 'id',
    loading: false,
    emptyText: 'Nenhum item encontrado.',
    selectable: true,
    modelValue: () => [],
    focusCell: null
  }
)

const resolvedColumns = computed(() => {
  const isAdmin = props.viewerUserType === 'admin'
  if (isAdmin) return props.columns
  return props.columns.filter(column => !column.adminOnly)
})

const emit = defineEmits<{
  'update:modelValue': [value: Array<string | number>]
  'update:cell': [payload: OmniTableCellUpdate]
  'row-action': [payload: { action: string, row: Record<string, any> }]
  'upload:image': [payload: OmniTableImageUpload]
}>()

const tableRootRef = ref<HTMLElement | null>(null)
const imagePreviewOpen = ref(false)
const imagePreviewUrl = ref('')

const selectedSet = computed(() => new Set(props.modelValue))

const rowIds = computed(() => props.rows
  .map(row => row[props.rowKey])
  .filter(id => id !== undefined && id !== null) as Array<string | number>)

const allSelected = computed(() => {
  if (!rowIds.value.length) return false
  return rowIds.value.every(id => selectedSet.value.has(id))
})

const partiallySelected = computed(() => {
  if (!rowIds.value.length) return false
  if (allSelected.value) return false
  return rowIds.value.some(id => selectedSet.value.has(id))
})

function columnAlignClass(align: OmniTableColumn['align']) {
  if (align === 'center') return 'text-center'
  if (align === 'right') return 'text-right'
  return 'text-left'
}

function columnStyle(column: OmniTableColumn) {
  const style: Record<string, string> = {}

  if (typeof column.minWidth === 'number' && Number.isFinite(column.minWidth) && column.minWidth > 0) {
    style.minWidth = `${column.minWidth}px`
  }

  if (typeof column.maxWidth === 'number' && Number.isFinite(column.maxWidth) && column.maxWidth > 0) {
    style.maxWidth = `${column.maxWidth}px`
  }

  return Object.keys(style).length ? style : undefined
}

function normalizeSelectItems(options: OmniSelectOption[] | undefined) {
  if (!Array.isArray(options)) return []
  return options.map(option => ({
    label: option.label,
    value: option.value
  }))
}

function rowId(row: Record<string, any>) {
  return row[props.rowKey] as string | number
}

function onSelectAll(value: boolean | 'indeterminate') {
  if (value === true) {
    emit('update:modelValue', [...rowIds.value])
    return
  }

  emit('update:modelValue', [])
}

function onSelectRow(id: string | number, value: boolean | 'indeterminate') {
  const next = new Set(selectedSet.value)

  if (value === true) {
    next.add(id)
  } else {
    next.delete(id)
  }

  emit('update:modelValue', [...next])
}

function emitCellUpdate(row: Record<string, any>, column: OmniTableColumn, value: unknown) {
  emit('update:cell', {
    rowId: rowId(row),
    key: column.key,
    value,
    immediate: column.immediate
  })
}

function isColumnEditable(column: OmniTableColumn, row: Record<string, any>) {
  if (!column.editable) return false
  if (typeof column.editableWhen !== 'function') return true
  return column.editableWhen(row)
}

function stringValue(value: unknown) {
  if (value === null || value === undefined) return ''
  return String(value)
}

function numberValue(value: unknown) {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : 0
}

function switchOnValue(column: OmniTableColumn) {
  return column.switchOnValue ?? true
}

function switchOffValue(column: OmniTableColumn) {
  return column.switchOffValue ?? false
}

function isSwitchOn(value: unknown, column: OmniTableColumn) {
  const onValue = switchOnValue(column)
  if (typeof value === 'boolean' && typeof onValue === 'boolean') {
    return value === onValue
  }

  return String(value ?? '').toLowerCase() === String(onValue ?? '').toLowerCase()
}

function displayValue(row: Record<string, any>, column: OmniTableColumn) {
  const value = row[column.key]
  if (column.formatter) {
    return column.formatter(value, row)
  }

  if (Array.isArray(value)) {
    if (value.length === 0) return '-'
    return value.map(item => String(item ?? '')).filter(Boolean).join(', ')
  }

  if (value === null || value === undefined || value === '') {
    return '-'
  }

  return String(value)
}

function openExternal(url: string) {
  if (!import.meta.client) return
  const target = String(url ?? '').trim()
  if (!target) return
  window.open(target, '_blank', 'noopener,noreferrer')
}

function openImagePreview(url: unknown) {
  const target = String(url ?? '').trim()
  if (!target) return
  imagePreviewUrl.value = target
  imagePreviewOpen.value = true
}

function imageInputId(row: Record<string, any>, column: OmniTableColumn) {
  const rowValue = String(rowId(row)).replace(/[^a-zA-Z0-9_-]/g, '_')
  const columnValue = String(column.key || '').replace(/[^a-zA-Z0-9_-]/g, '_')
  return `table-image-upload-${rowValue}-${columnValue}`
}

function multiselectValues(value: unknown) {
  if (Array.isArray(value)) {
    return value as Array<string | number>
  }

  if (value === null || value === undefined) {
    return []
  }

  const text = String(value).trim()
  if (!text) return []

  return text
    .split(/[\n,;|]+/)
    .map(item => item.trim())
    .filter(Boolean) as Array<string | number>
}

function onImageFileChange(event: Event, row: Record<string, any>, column: OmniTableColumn) {
  const target = event.target as HTMLInputElement | null
  if (!target) return

  const file = target.files?.[0]
  if (!file) return

  emit('upload:image', {
    rowId: rowId(row),
    key: column.key,
    file
  })

  target.value = ''
}

function actionVisible(action: OmniTableAction, row: Record<string, any>) {
  return action.visible ? action.visible(row) : true
}

function actionDisabled(action: OmniTableAction, row: Record<string, any>) {
  return action.disabled ? action.disabled(row) : false
}

function fireAction(action: OmniTableAction, row: Record<string, any>) {
  emit('row-action', {
    action: action.id,
    row
  })
}

function focusCellInput(target: OmniFocusCell | null | undefined) {
  if (!target || !tableRootRef.value) return

  const selector = `[data-cell-input="${target.rowId}:${target.columnKey}"]`
  const wrapper = tableRootRef.value.querySelector(selector)
  if (!wrapper) return

  const focusable = wrapper.querySelector('input, textarea, [role="switch"], button') as HTMLElement | null
  if (!focusable) return

  focusable.focus()
  if (focusable instanceof HTMLInputElement) {
    focusable.select()
  }
}

watch(
  () => props.focusCell,
  async (target) => {
    await nextTick()
    focusCellInput(target)
  },
  { deep: true }
)

watch(imagePreviewOpen, (open) => {
  if (open) return
  imagePreviewUrl.value = ''
})
</script>

<template>
  <div ref="tableRootRef" class="omni-data-table overflow-x-auto rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface))] shadow-[var(--shadow-sm)]">
    <table class="omni-data-table__table w-full min-w-[1100px] text-sm">
      <thead class="omni-data-table__head sticky top-0 z-10 bg-[rgb(var(--surface-2))]">
        <tr class="omni-data-table__head-row border-b border-[rgb(var(--border))] text-[rgb(var(--text))]">
          <th v-if="props.selectable" class="omni-data-table__head-cell omni-data-table__head-cell--select w-12 px-2 py-3 text-center">
            <div class="flex justify-center">
              <UCheckbox
                :model-value="allSelected"
                :indeterminate="partiallySelected"
                aria-label="Selecionar todos"
                @update:model-value="onSelectAll"
              />
            </div>
          </th>

          <th
            v-for="column in resolvedColumns"
            :key="column.key"
            class="omni-data-table__head-cell px-3 py-3 font-semibold"
            :class="columnAlignClass(column.align)"
            :style="columnStyle(column)"
          >
            {{ column.label }}
          </th>
        </tr>
      </thead>

      <tbody>
        <tr v-if="props.loading && props.rows.length === 0">
          <td :colspan="resolvedColumns.length + (props.selectable ? 1 : 0)" class="omni-data-table__state-cell omni-data-table__state-cell--loading px-3 py-8 text-center text-sm text-[rgb(var(--muted))]">
            Carregando...
          </td>
        </tr>

        <tr
          v-for="(row, index) in props.rows"
          :key="String(rowId(row))"
          class="omni-data-table__row border-b border-[rgb(var(--border))] transition-colors"
          :class="index % 2 === 0 ? 'bg-[rgb(var(--surface))]' : 'bg-[rgb(var(--surface-2))]'"
        >
          <td v-if="props.selectable" class="omni-data-table__cell omni-data-table__cell--select w-12 px-2 py-3 text-center align-middle">
            <div class="flex justify-center">
              <UCheckbox
                :model-value="selectedSet.has(rowId(row))"
                aria-label="Selecionar item"
                @update:model-value="onSelectRow(rowId(row), $event)"
              />
            </div>
          </td>

          <td
            v-for="column in resolvedColumns"
            :key="column.key"
            class="omni-data-table__cell px-3 py-3 align-middle"
            :class="columnAlignClass(column.align)"
            :style="columnStyle(column)"
          >
            <template v-if="column.type === 'custom'">
              <slot :name="`cell-${column.key}`" :row="row" :column="column" />
            </template>

            <template v-else-if="isColumnEditable(column, row) && column.type === 'switch'">
              <div :data-cell-input="`${rowId(row)}:${column.key}`">
                <OmniSwitchInput
                  :model-value="isSwitchOn(row[column.key], column)"
                  :disabled="props.loading"
                  :aria-label="`Alternar ${column.label}`"
                  @update:model-value="emitCellUpdate(row, column, $event === true ? switchOnValue(column) : switchOffValue(column))"
                />
              </div>
            </template>

            <template v-else-if="isColumnEditable(column, row) && column.type === 'select'">
              <div :data-cell-input="`${rowId(row)}:${column.key}`">
                <OmniSelectInput
                  :model-value="row[column.key]"
                  :items="normalizeSelectItems(column.options)"
                  :placeholder="column.placeholder || 'Selecione'"
                  :creatable="column.creatable ?? false"
                  :disabled="props.loading"
                  @update:model-value="emitCellUpdate(row, column, $event)"
                />
              </div>
            </template>

            <template v-else-if="isColumnEditable(column, row) && column.type === 'multiselect'">
              <div :data-cell-input="`${rowId(row)}:${column.key}`">
                <OmniSelectInput
                  :model-value="multiselectValues(row[column.key])"
                  :items="normalizeSelectItems(column.options)"
                  :placeholder="column.placeholder || 'Selecione uma ou mais opcoes'"
                  :creatable="column.creatable ?? false"
                  :disabled="props.loading"
                  overlay-on-open
                  multiple
                  @update:model-value="emitCellUpdate(row, column, $event)"
                />
              </div>
            </template>

            <template v-else-if="isColumnEditable(column, row) && column.type === 'money'">
              <div :data-cell-input="`${rowId(row)}:${column.key}`">
                <OmniMoneyInput
                  :model-value="numberValue(row[column.key])"
                  @update:model-value="emitCellUpdate(row, column, $event)"
                />
              </div>
            </template>

            <template v-else-if="isColumnEditable(column, row) && column.type === 'image'">
              <div :data-cell-input="`${rowId(row)}:${column.key}`" class="flex min-w-[88px] justify-center">
                <div class="group w-50 relative">
                  <button
                    v-if="stringValue(row[column.key])"
                    type="button"
                    class="h-14 w-50 overflow-hidden rounded-[var(--radius-sm)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))]"
                    :title="stringValue(row[column.key])"
                    @click="openImagePreview(stringValue(row[column.key]))"
                  >
                    <img
                      :src="stringValue(row[column.key])"
                      alt="Imagem"
                      class="h-full w-full object-cover"
                      loading="lazy"
                    >
                  </button>

                  <label
                    v-else
                    :for="imageInputId(row, column)"
                    class="flex h-14 w-14 cursor-pointer items-center justify-center rounded-[var(--radius-sm)] border border-dashed border-[rgb(var(--border))] text-[10px] text-[rgb(var(--muted))]"
                  >
                    sem img
                  </label>

                  <label
                    :for="imageInputId(row, column)"
                    class="absolute -bottom-1 -right-1 inline-flex cursor-pointer items-center justify-center rounded-full border border-[rgb(var(--border))] bg-[rgb(var(--surface))] p-1 opacity-100 shadow-[var(--shadow-xs)] transition-opacity md:opacity-0 md:group-hover:opacity-100"
                    :title="stringValue(row[column.key]) ? 'Substituir imagem' : 'Adicionar imagem'"
                  >
                    <UIcon name="i-lucide-upload" class="text-[11px]" />
                  </label>

                  <input
                    :id="imageInputId(row, column)"
                    type="file"
                    accept="image/*"
                    class="hidden"
                    @change="onImageFileChange($event, row, column)"
                  >
                </div>
              </div>
            </template>

            <template v-else-if="isColumnEditable(column, row) && column.type === 'number'">
              <div :data-cell-input="`${rowId(row)}:${column.key}`">
                <UInput
                  :model-value="stringValue(row[column.key])"
                  type="number"
                  min="0"
                  :placeholder="column.placeholder"
                  @update:model-value="emitCellUpdate(row, column, $event)"
                />
              </div>
            </template>

            <template v-else-if="isColumnEditable(column, row)">
              <div :data-cell-input="`${rowId(row)}:${column.key}`">
                <UInput
                  :model-value="stringValue(row[column.key])"
                  :placeholder="column.placeholder"
                  @update:model-value="emitCellUpdate(row, column, $event)"
                />
              </div>
            </template>

            <template v-else-if="column.editable">
              <div class="flex items-center gap-1">
                <span class="line-clamp-1" :title="displayValue(row, column)">{{ displayValue(row, column) }}</span>
                <UIcon name="i-lucide-lock" class="text-xs text-[rgb(var(--muted))]" />
              </div>
            </template>

            <template v-else-if="column.actions?.length">
              <div class="flex items-center justify-end gap-1">
                <template v-for="action in column.actions" :key="action.id">
                  <UButton
                    v-if="actionVisible(action, row)"
                    :icon="action.icon"
                    :aria-label="action.label"
                    :title="action.label"
                    size="sm"
                    :color="action.color ?? 'neutral'"
                    :variant="action.variant ?? 'ghost'"
                    :disabled="actionDisabled(action, row)"
                    @click="fireAction(action, row)"
                  />
                </template>
              </div>
            </template>

            <template v-else-if="column.type === 'image'">
              <div class="flex justify-center">
                <button
                  v-if="stringValue(row[column.key])"
                  type="button"
                  class="h-12 w-12 overflow-hidden rounded-[var(--radius-sm)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))]"
                  :title="stringValue(row[column.key])"
                  @click="openImagePreview(stringValue(row[column.key]))"
                >
                  <img
                    :src="stringValue(row[column.key])"
                    alt="Imagem"
                    class="h-full w-full object-cover"
                    loading="lazy"
                  >
                </button>
                <span v-else class="text-xs text-[rgb(var(--muted))]">-</span>
              </div>
            </template>

            <template v-else>
              <span class="line-clamp-1" :title="displayValue(row, column)">{{ displayValue(row, column) }}</span>
            </template>
          </td>
        </tr>

        <tr v-if="!props.loading && props.rows.length === 0">
          <td :colspan="resolvedColumns.length + (props.selectable ? 1 : 0)" class="omni-data-table__state-cell omni-data-table__state-cell--empty px-3 py-8 text-center text-sm text-[rgb(var(--muted))]">
            {{ props.emptyText }}
          </td>
        </tr>
      </tbody>
    </table>
  </div>

  <UModal
    v-model:open="imagePreviewOpen"
    title="Visualizar imagem"
    :description="imagePreviewUrl || undefined"
    :ui="{ content: 'max-w-5xl' }"
  >
    <template #body>
      <div class="omni-data-table__image-preview-body flex min-h-[240px] w-full items-center justify-center rounded-[var(--radius-md)] bg-[rgb(var(--surface-2))] p-2">
        <img
          v-if="imagePreviewUrl"
          :src="imagePreviewUrl"
          alt="Imagem"
          class="max-h-[80vh] max-w-full rounded-[var(--radius-sm)] object-contain"
        >
      </div>
    </template>

    <template #footer>
      <div class="flex w-full items-center justify-end gap-2">
        <UButton
          label="Abrir em nova aba"
          icon="i-lucide-external-link"
          color="neutral"
          variant="soft"
          :disabled="!imagePreviewUrl"
          @click="openExternal(imagePreviewUrl)"
        />
        <UButton label="Fechar" color="neutral" variant="ghost" @click="imagePreviewOpen = false" />
      </div>
    </template>
  </UModal>
</template>
