<script setup lang="ts">
import type { OmniTableColumn } from '~/types/omni/collection'

const props = withDefaults(
  defineProps<{
    columns: OmniTableColumn[]
    modelValue: string[]
    excludeKeys?: string[]
    label?: string
  }>(),
  {
    excludeKeys: () => [],
    label: 'Colunas',
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: string[]]
}>()

const selectedSet = computed(() => new Set(props.modelValue))

const configurableColumns = computed(() => {
  const excluded = new Set(props.excludeKeys)
  return props.columns.filter((column) => !excluded.has(column.key))
})

function isChecked(key: string) {
  return selectedSet.value.has(key)
}

function toggleColumn(key: string, value: boolean | 'indeterminate') {
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
  if (visibleCount <= 1 && next.has(key)) {
    return
  }

  next.delete(key)
  emit('update:modelValue', [...next])
}

function showAll() {
  emit(
    'update:modelValue',
    configurableColumns.value.map((column) => column.key),
  )
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
        class="omni-table-columns-config w-[280px] max-w-[90vw] space-y-3 rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface))] p-3 shadow-[var(--shadow-sm)]"
      >
        <div class="omni-table-columns-config__header flex items-center justify-between gap-2">
          <p class="omni-table-columns-config__title text-sm font-semibold text-[rgb(var(--text))]">
            Configurar colunas
          </p>
          <UButton
            icon="i-lucide-refresh-cw"
            color="neutral"
            variant="ghost"
            size="xs"
            class="omni-table-columns-config__reset"
            @click="showAll"
          />
        </div>

        <div class="omni-table-columns-config__list max-h-72 space-y-2 overflow-y-auto pr-1">
          <label
            v-for="column in configurableColumns"
            :key="column.key"
            class="omni-table-columns-config__item flex items-center justify-between gap-2 rounded-[var(--radius-sm)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] px-2 py-1"
          >
            <span class="omni-table-columns-config__item-label text-sm text-[rgb(var(--text))]">
              {{ column.label }}
            </span>
            <UCheckbox
              :model-value="isChecked(column.key)"
              @update:model-value="toggleColumn(column.key, $event)"
            />
          </label>
        </div>
      </div>
    </template>
  </UPopover>
</template>
