<script setup lang="ts">
import OmniSwitchInput from '../../../../layers/tasks/components/omni/inputs/OmniSwitchInput.vue'
import OmniTableColumnsConfig from '~/components/omni/table/OmniTableColumnsConfig.vue'
import type {
  OmniFilterDefinition,
  OmniSelectOption,
  OmniTableColumn,
} from '~/types/omni/collection'

const props = withDefaults(
  defineProps<{
    modelValue: Record<string, unknown>
    filters: OmniFilterDefinition[]
    viewerUserType?: 'admin' | 'client'
    tableColumns?: OmniTableColumn[]
    visibleColumns?: string[]
    columnExcludeKeys?: string[]
    columnFilterLabel?: string
    showColumnFilter?: boolean
    loading?: boolean
    showReset?: boolean
    // C16: estado de lock + ordem das colunas (admin configura).
    lockedColumns?: string[]
    columnOrder?: string[]
  }>(),
  {
    viewerUserType: 'admin',
    tableColumns: () => [],
    visibleColumns: () => [],
    columnExcludeKeys: () => [],
    columnFilterLabel: 'Colunas',
    showColumnFilter: true,
    loading: false,
    showReset: true,
    lockedColumns: () => [],
    columnOrder: () => [],
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: Record<string, unknown>]
  'update:visibleColumns': [value: string[]]
  'update:lockedColumns': [value: string[]]
  'update:columnOrder': [value: string[]]
  'reset-columns': []
  reset: []
}>()

const visibleFilters = computed(() => {
  const isAdmin = props.viewerUserType === 'admin'
  if (isAdmin) {
    return props.filters
  }

  return props.filters.filter((filter) => !filter.adminOnly)
})

const visibleTableColumns = computed(() => {
  const isAdmin = props.viewerUserType === 'admin'
  if (isAdmin) {
    return props.tableColumns
  }

  return props.tableColumns.filter((column) => !column.adminOnly)
})

function nextModel(patch: Record<string, unknown>) {
  emit('update:modelValue', {
    ...props.modelValue,
    ...patch,
  })
}

function normalizeSelectItems(options: OmniSelectOption[] | undefined) {
  if (!Array.isArray(options)) {
    return []
  }

  return options
    .filter((option) => !(typeof option.value === 'string' && option.value.trim() === ''))
    .map((option) => ({ label: option.label, value: option.value }))
}

function selectModelValue(key: string): string | number | undefined {
  const raw = props.modelValue[key]
  if (raw === null || raw === undefined || raw === '') {
    return undefined
  }
  if (typeof raw === 'number') {
    return raw
  }
  return String(raw)
}

function onFilterChange(key: string, value: unknown) {
  nextModel({ [key]: value })
}

function switchOnValue(filter: OmniFilterDefinition) {
  if (Object.prototype.hasOwnProperty.call(filter, 'switchOnValue')) {
    return filter.switchOnValue
  }

  return true
}

function switchOffValue(filter: OmniFilterDefinition) {
  if (Object.prototype.hasOwnProperty.call(filter, 'switchOffValue')) {
    return filter.switchOffValue
  }

  return false
}

function isSwitchOn(filter: OmniFilterDefinition, currentValue: unknown) {
  const onValue = switchOnValue(filter)

  if (typeof currentValue === 'boolean' && typeof onValue === 'boolean') {
    return currentValue === onValue
  }

  return String(currentValue ?? '').toLowerCase() === String(onValue ?? '').toLowerCase()
}

function onSwitchChange(filter: OmniFilterDefinition, value: boolean | 'indeterminate') {
  onFilterChange(filter.key, value === true ? switchOnValue(filter) : switchOffValue(filter))
}

function hasFilterValue(key: string) {
  const value = props.modelValue[key]
  if (value === null || value === undefined) {
    return false
  }
  if (Array.isArray(value)) {
    return value.length > 0
  }
  if (typeof value === 'boolean') {
    return value
  }
  if (typeof value === 'string') {
    return value.trim() !== ''
  }

  return true
}

function clearFilter(key: string) {
  onFilterChange(key, '')
}

const resolvedVisibleColumns = computed(() => {
  const excluded = new Set(props.columnExcludeKeys)
  const available = visibleTableColumns.value
    .filter((column) => !excluded.has(column.key))
    .map((column) => column.key)

  if (available.length === 0) {
    return []
  }

  const selected = props.visibleColumns.filter((key) => available.includes(key))
  return selected.length > 0 ? selected : available
})

function onVisibleColumnsUpdate(value: string[]) {
  emit('update:visibleColumns', value)
}

function onReset() {
  const cleared = Object.fromEntries(
    visibleFilters.value.map((filter) => {
      if (filter.type === 'switch') {
        return [filter.key, switchOffValue(filter)]
      }

      return [filter.key, '']
    }),
  )

  emit('update:modelValue', {
    ...props.modelValue,
    ...cleared,
  })
  emit('reset')
}
</script>

<template>
  <div
    class="omni-collection-filters rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] p-3"
  >
    <div class="omni-collection-filters__row flex flex-wrap items-center gap-2">
      <template v-for="filter in visibleFilters" :key="filter.key">
        <div
          v-if="filter.type === 'text'"
          class="omni-collection-filters__field omni-collection-filters__field--text relative min-w-[240px] flex-1"
        >
          <UInput
            :model-value="String(props.modelValue[filter.key] ?? '')"
            :placeholder="filter.placeholder || filter.label"
            class="omni-collection-filters__input omni-collection-filters__input--text min-w-[240px] w-full"
            :ui="{ base: hasFilterValue(filter.key) ? 'pr-8' : '' }"
            @update:model-value="onFilterChange(filter.key, String($event ?? ''))"
          />

          <UButton
            v-if="hasFilterValue(filter.key)"
            icon="i-lucide-x"
            color="neutral"
            variant="ghost"
            size="xs"
            class="omni-collection-filters__clear absolute right-1 top-1/2 z-10 -translate-y-1/2"
            :disabled="props.loading"
            aria-label="Limpar filtro"
            @click="clearFilter(filter.key)"
          />
        </div>

        <div
          v-else-if="filter.type === 'select'"
          class="omni-collection-filters__field omni-collection-filters__field--select relative w-[220px]"
        >
          <USelect
            :model-value="selectModelValue(filter.key)"
            :items="normalizeSelectItems(filter.options)"
            :placeholder="filter.placeholder || filter.label"
            class="omni-collection-filters__input omni-collection-filters__input--select w-[220px]"
            :ui="{ base: hasFilterValue(filter.key) ? 'pr-8' : '' }"
            @update:model-value="onFilterChange(filter.key, $event)"
          />

          <UButton
            v-if="hasFilterValue(filter.key)"
            icon="i-lucide-x"
            color="neutral"
            variant="ghost"
            size="xs"
            class="omni-collection-filters__clear absolute right-6 top-1/2 z-10 -translate-y-1/2"
            :disabled="props.loading"
            aria-label="Limpar filtro"
            @click="clearFilter(filter.key)"
          />
        </div>

        <div
          v-else-if="filter.type === 'switch'"
          class="omni-collection-filters__field omni-collection-filters__field--switch flex h-9 items-center gap-2 rounded-[var(--radius-sm)] border border-[rgb(var(--border))] px-3"
        >
          <OmniSwitchInput
            :model-value="isSwitchOn(filter, props.modelValue[filter.key])"
            :disabled="props.loading"
            :aria-label="`Alternar filtro ${filter.label}`"
            @update:model-value="onSwitchChange(filter, $event)"
          />
          <span class="text-xs font-medium text-[rgb(var(--text))]">{{ filter.label }}</span>
        </div>
      </template>

      <OmniTableColumnsConfig
        v-if="props.showColumnFilter && visibleTableColumns.length > 0"
        :model-value="resolvedVisibleColumns"
        :columns="visibleTableColumns"
        :exclude-keys="props.columnExcludeKeys"
        :label="props.columnFilterLabel"
        :locked-keys="props.lockedColumns"
        :column-order="props.columnOrder"
        :viewer-user-type="props.viewerUserType"
        @update:model-value="onVisibleColumnsUpdate"
        @update:locked-keys="emit('update:lockedColumns', $event)"
        @update:column-order="emit('update:columnOrder', $event)"
        @reset="emit('reset-columns')"
      />

      <slot name="actions"></slot>

      <UButton
        v-if="props.showReset"
        icon="i-lucide-rotate-ccw"
        label="Limpar"
        color="neutral"
        variant="ghost"
        class="omni-collection-filters__reset"
        :disabled="props.loading"
        @click="onReset"
      />
    </div>

    <slot name="below"></slot>
  </div>
</template>
