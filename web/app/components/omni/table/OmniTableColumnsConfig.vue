<script setup lang="ts">
import type { OmniTableColumn } from '~/types/omni/collection'

const props = withDefaults(
  defineProps<{
    columns: OmniTableColumn[]
    modelValue: string[]
    excludeKeys?: string[]
    label?: string
    // C16: locked column keys (admin marca como sempre visivel).
    lockedKeys?: string[]
    // C16: ordem persistida das colunas (admin reordena por drag).
    columnOrder?: string[]
    // C16: so admin ve cadeado + drag handle; demais usuarios so checkbox.
    viewerUserType?: 'admin' | 'client'
  }>(),
  {
    excludeKeys: () => [],
    label: 'Colunas',
    lockedKeys: () => [],
    columnOrder: () => [],
    viewerUserType: 'client',
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: string[]]
  'update:lockedKeys': [value: string[]]
  'update:columnOrder': [value: string[]]
  reset: []
}>()

const selectedSet = computed(() => new Set(props.modelValue))
const lockedSet = computed(() => new Set(props.lockedKeys))
const isAdmin = computed(() => props.viewerUserType === 'admin')

const configurableColumns = computed(() => {
  const excluded = new Set(props.excludeKeys)
  const ordered = orderColumns(props.columns, props.columnOrder)
  return ordered.filter((column) => !excluded.has(column.key))
})

function orderColumns(columns: OmniTableColumn[], order: string[]): OmniTableColumn[] {
  if (order.length === 0) {
    return [...columns].sort((a, b) => {
      const orderA = a.defaultOrder ?? Number.MAX_SAFE_INTEGER
      const orderB = b.defaultOrder ?? Number.MAX_SAFE_INTEGER
      return orderA - orderB
    })
  }
  const orderIndex = new Map<string, number>()
  order.forEach((key, idx) => orderIndex.set(key, idx))
  return [...columns].sort((a, b) => {
    const idxA = orderIndex.has(a.key) ? orderIndex.get(a.key)! : Number.MAX_SAFE_INTEGER
    const idxB = orderIndex.has(b.key) ? orderIndex.get(b.key)! : Number.MAX_SAFE_INTEGER
    return idxA - idxB
  })
}

function isChecked(key: string) {
  return selectedSet.value.has(key) || lockedSet.value.has(key)
}

function isLocked(key: string) {
  return lockedSet.value.has(key)
}

function toggleColumn(key: string, value: boolean | 'indeterminate') {
  // Locked columns nao podem ser destravadas via checkbox; so via cadeado.
  if (lockedSet.value.has(key)) return

  const next = new Set(selectedSet.value)
  const checked = value === true
  if (checked) {
    next.add(key)
    emit('update:modelValue', [...next])
    return
  }
  const visibleCount = [...next].filter((columnKey) =>
    configurableColumns.value.some((column) => column.key === columnKey),
  ).length
  if (visibleCount <= 1 && next.has(key)) return
  next.delete(key)
  emit('update:modelValue', [...next])
}

function toggleLock(key: string) {
  if (!isAdmin.value) return
  const next = new Set(lockedSet.value)
  if (next.has(key)) {
    next.delete(key)
  } else {
    next.add(key)
    // Locked implica visivel — adiciona a visibleKeys se ja nao estava.
    if (!selectedSet.value.has(key)) {
      emit('update:modelValue', [...selectedSet.value, key])
    }
  }
  emit('update:lockedKeys', [...next])
}

// Drag-n-drop nativo HTML5 (admin only).
const dragKey = ref<string | null>(null)

function onDragStart(event: DragEvent, key: string) {
  if (!isAdmin.value) return
  dragKey.value = key
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', key)
  }
}

function onDragOver(event: DragEvent) {
  if (!isAdmin.value || !dragKey.value) return
  event.preventDefault()
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'move'
}

function onDrop(event: DragEvent, targetKey: string) {
  if (!isAdmin.value || !dragKey.value || dragKey.value === targetKey) return
  event.preventDefault()
  const allKeys = configurableColumns.value.map((c) => c.key)
  // Se columnOrder estiver vazio, materializa ordem atual visível primeiro.
  const baseOrder = props.columnOrder.length > 0 ? [...props.columnOrder] : allKeys
  // Garante que toda coluna existente está em baseOrder.
  for (const k of allKeys) {
    if (!baseOrder.includes(k)) baseOrder.push(k)
  }
  const next = [...baseOrder]
  const sourceIdx = next.indexOf(dragKey.value)
  const targetIdx = next.indexOf(targetKey)
  if (sourceIdx < 0 || targetIdx < 0) return
  next.splice(sourceIdx, 1)
  next.splice(targetIdx, 0, dragKey.value)
  emit('update:columnOrder', next)
  dragKey.value = null
}

function onDragEnd() {
  dragKey.value = null
}

function showAll() {
  emit(
    'update:modelValue',
    configurableColumns.value.map((column) => column.key),
  )
}

function onReset() {
  emit('reset')
}
</script>

<template>
  <UPopover :content="{ align: 'end', side: 'bottom' }">
    <UButton
      icon="i-lucide-columns-3"
      :label="props.label"
      color="neutral"
      variant="soft"
      class="omni-table-columns-config__trigger"
    />

    <template #content>
      <div
        class="omni-table-columns-config w-[320px] max-w-[90vw] space-y-3 rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface))] p-3 shadow-[var(--shadow-sm)]"
      >
        <div class="omni-table-columns-config__header flex items-center justify-between gap-2">
          <p class="omni-table-columns-config__title text-sm font-semibold text-[rgb(var(--text))]">
            Configurar colunas
          </p>
          <div class="flex items-center gap-1">
            <UButton
              v-if="isAdmin"
              icon="i-lucide-undo-2"
              color="neutral"
              variant="ghost"
              size="xs"
              class="omni-table-columns-config__reset"
              aria-label="Restaurar padrao"
              @click="onReset"
            />
            <UButton
              icon="i-lucide-eye"
              color="neutral"
              variant="ghost"
              size="xs"
              class="omni-table-columns-config__show-all"
              aria-label="Mostrar todas"
              @click="showAll"
            />
          </div>
        </div>

        <p
          v-if="isAdmin"
          class="omni-table-columns-config__hint text-[10px] text-[rgb(var(--muted))]"
        >
          Cadeado: trava a coluna como sempre visivel. Arraste para reordenar.
        </p>

        <div class="omni-table-columns-config__list max-h-72 space-y-2 overflow-y-auto pr-1">
          <div
            v-for="column in configurableColumns"
            :key="column.key"
            class="omni-table-columns-config__item flex items-center justify-between gap-2 rounded-[var(--radius-sm)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] px-2 py-1"
            :class="{
              'omni-table-columns-config__item--locked': isLocked(column.key),
              'omni-table-columns-config__item--dragging': dragKey === column.key,
            }"
            :draggable="isAdmin"
            @dragstart="onDragStart($event, column.key)"
            @dragover="onDragOver($event)"
            @drop="onDrop($event, column.key)"
            @dragend="onDragEnd()"
          >
            <div class="flex flex-1 items-center gap-2 min-w-0">
              <UIcon
                v-if="isAdmin"
                name="i-lucide-grip-vertical"
                class="omni-table-columns-config__drag-handle text-[rgb(var(--muted))] cursor-grab"
              />
              <span
                class="omni-table-columns-config__item-label truncate text-sm text-[rgb(var(--text))]"
              >
                {{ column.label }}
              </span>
            </div>

            <div class="flex items-center gap-2">
              <UButton
                v-if="isAdmin"
                :icon="isLocked(column.key) ? 'i-lucide-lock' : 'i-lucide-lock-open'"
                :color="isLocked(column.key) ? 'primary' : 'neutral'"
                variant="ghost"
                size="xs"
                :aria-label="isLocked(column.key) ? 'Destravar coluna' : 'Travar coluna'"
                @click="toggleLock(column.key)"
              />
              <UCheckbox
                :model-value="isChecked(column.key)"
                :disabled="isLocked(column.key)"
                @update:model-value="toggleColumn(column.key, $event)"
              />
            </div>
          </div>
        </div>
      </div>
    </template>
  </UPopover>
</template>

<style scoped>
.omni-table-columns-config__item--dragging {
  opacity: 0.5;
}

.omni-table-columns-config__item--locked {
  border-color: rgb(var(--primary) / 0.4);
}
</style>
