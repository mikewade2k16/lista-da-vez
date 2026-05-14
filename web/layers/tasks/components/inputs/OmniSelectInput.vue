<script setup lang="ts">
import type { OmniSelectOption } from '../../types/omni/collection'

type OmniCreateOptionConfig =
  | boolean
  | 'always'
  | {
    position?: 'top' | 'bottom'
    when?: 'empty' | 'always'
  }

type OmniSelectColorKey =
  | 'default'
  | 'gray'
  | 'brown'
  | 'orange'
  | 'yellow'
  | 'green'
  | 'blue'
  | 'purple'
  | 'pink'
  | 'red'

interface OmniSelectViewOption extends OmniSelectOption {
  color?: OmniSelectColorKey
}

interface OmniSelectOptionMeta {
  label?: string
  color?: OmniSelectColorKey
  deleted?: boolean
}

const DEFAULT_COLOR: {
  key: OmniSelectColorKey
  label: string
  swatchClass: string
  chipClass: string
} = {
  key: 'default',
  label: 'Padrao',
  swatchClass: 'bg-zinc-500',
  chipClass: 'bg-zinc-700/40 text-zinc-100 ring-zinc-500/50'
}

const COLOR_PALETTE: Array<{
  key: OmniSelectColorKey
  label: string
  swatchClass: string
  chipClass: string
}> = [
  DEFAULT_COLOR,
  {
    key: 'gray',
    label: 'Cinza',
    swatchClass: 'bg-slate-500',
    chipClass: 'bg-slate-600/50 text-slate-100 ring-slate-400/50'
  },
  {
    key: 'brown',
    label: 'Marrom',
    swatchClass: 'bg-amber-700',
    chipClass: 'bg-amber-700/50 text-amber-50 ring-amber-500/50'
  },
  {
    key: 'orange',
    label: 'Laranja',
    swatchClass: 'bg-orange-500',
    chipClass: 'bg-orange-600/50 text-orange-50 ring-orange-400/50'
  },
  {
    key: 'yellow',
    label: 'Amarelo',
    swatchClass: 'bg-yellow-500',
    chipClass: 'bg-yellow-600/50 text-yellow-50 ring-yellow-400/50'
  },
  {
    key: 'green',
    label: 'Verde',
    swatchClass: 'bg-emerald-500',
    chipClass: 'bg-emerald-700/55 text-emerald-50 ring-emerald-400/50'
  },
  {
    key: 'blue',
    label: 'Azul',
    swatchClass: 'bg-blue-500',
    chipClass: 'bg-blue-700/55 text-blue-50 ring-blue-400/50'
  },
  {
    key: 'purple',
    label: 'Roxo',
    swatchClass: 'bg-violet-500',
    chipClass: 'bg-violet-700/55 text-violet-50 ring-violet-400/50'
  },
  {
    key: 'pink',
    label: 'Rosa',
    swatchClass: 'bg-pink-500',
    chipClass: 'bg-pink-700/55 text-pink-50 ring-pink-400/50'
  },
  {
    key: 'red',
    label: 'Vermelho',
    swatchClass: 'bg-rose-500',
    chipClass: 'bg-rose-700/55 text-rose-50 ring-rose-400/50'
  }
]

const props = withDefaults(defineProps<{
  modelValue?: string | number | Array<string | number> | null
  items?: OmniSelectOption[]
  placeholder?: string
  multiple?: boolean
  disabled?: boolean
  creatable?: OmniCreateOptionConfig
  searchInput?: boolean
  clear?: boolean
  manageOptions?: boolean
  overlayOnOpen?: boolean
}>(), {
  modelValue: null,
  items: () => [],
  placeholder: 'Selecione',
  multiple: false,
  disabled: false,
  creatable: false,
  searchInput: true,
  clear: false,
  manageOptions: false,
  overlayOnOpen: false
})

const emit = defineEmits<{
  'update:modelValue': [value: string | number | Array<string | number> | null]
  create: [option: OmniSelectOption]
}>()

const createdItems = ref<OmniSelectViewOption[]>([])
const optionMetaState = useState<Record<string, OmniSelectOptionMeta>>('__omni_select_option_meta__', () => ({}))
const optionDraftLabelMap = reactive<Record<string, string>>({})
const searchTerm = ref('')
const menuOpen = ref(false)
const inputMenuRef = ref<any>(null)
const selectMenuRef = ref<any>(null)

const multiUi = computed(() => ({
  root: [
    'omni-select__multi-root flex-nowrap items-center gap-1 overflow-hidden w-full min-w-0',
    props.overlayOnOpen && menuOpen.value
      ? 'omni-select__multi-root--overlay-open absolute left-0 top-1/2 z-[80] -translate-y-1/2 w-max min-w-[320px] max-w-[560px] flex-wrap items-start gap-1.5 overflow-visible rounded-md border border-[rgb(var(--border))] bg-[rgb(var(--surface))] p-1.5 py-2 shadow-[var(--shadow-lg)]'
      : ''
  ].join(' ').trim(),
  base: [
    'omni-select__multi-base border-0 bg-transparent py-0 shadow-none',
    props.overlayOnOpen && menuOpen.value
      ? 'min-h-9 h-auto px-1'
      : 'min-h-9 max-h-9 px-2'
  ].join(' '),
  tagsItem: 'omni-select__tag-item p-0 bg-transparent ring-0 max-w-[184px]',
  tagsItemText: 'omni-select__tag-item-text p-0',
  tagsItemDelete: 'omni-select__tag-item-delete hidden',
  tagsInput: menuOpen.value
    ? 'omni-select__search-input min-w-[128px] px-1 opacity-100'
    : 'omni-select__search-input w-0  max-w-0 px-0 opacity-0 pointer-events-none'
}))

const singleUi = {
  base: 'omni-select__single-base border-0 bg-transparent px-0 py-0 shadow-none',
  trailing: 'omni-select__single-trailing relative inset-auto ms-1'
}

const wrapperClasses = computed(() => [
  'omni-select relative w-full min-w-0 cursor-text',
  props.multiple && props.overlayOnOpen && menuOpen.value
    ? 'omni-select--menu-open z-[70] overflow-visible'
    : 'overflow-hidden'
].join(' '))

function normalizeText(value: unknown) {
  return String(value ?? '').replace(/\s+/g, ' ').trim()
}

function optionKey(value: unknown) {
  return normalizeText(value).toLowerCase()
}

function isCreatableEnabled() {
  return props.creatable !== false
}

function isManageEnabled() {
  return props.manageOptions || isCreatableEnabled()
}

function colorConfig(color: OmniSelectColorKey | undefined) {
  const key = color || 'default'
  return COLOR_PALETTE.find(item => item.key === key) || DEFAULT_COLOR
}

function setOptionMeta(optionValue: string | number, patch: Partial<OmniSelectOptionMeta>) {
  const key = optionKey(optionValue)
  const current = optionMetaState.value[key] || {}

  optionMetaState.value = {
    ...optionMetaState.value,
    [key]: {
      ...current,
      ...patch
    }
  }
}

function normalizeOption(option: OmniSelectOption): OmniSelectViewOption {
  const meta = optionMetaState.value[optionKey(option.value)]
  const fallbackLabel = normalizeText(option.label || option.value)
  const label = normalizeText(meta?.label || fallbackLabel)

  return {
    label: label || fallbackLabel || normalizeText(option.value),
    value: option.value,
    color: (meta?.color || option.color || 'default') as OmniSelectColorKey
  }
}

function dedupeOptions(options: OmniSelectOption[]) {
  const seen = new Set<string>()
  const output: OmniSelectViewOption[] = []

  for (const option of options) {
    const normalized = normalizeOption(option)
    const key = optionKey(normalized.value)
    if (!normalized.label || seen.has(key)) {
      continue
    }

    seen.add(key)
    output.push(normalized)
  }

  return output
}

const normalizedModelArray = computed<Array<string | number>>(() => {
  if (Array.isArray(props.modelValue)) {
    return props.modelValue.filter(value => value !== null && value !== undefined && normalizeText(value) !== '') as Array<string | number>
  }

  if (props.modelValue === null || props.modelValue === undefined || normalizeText(props.modelValue) === '') {
    return []
  }

  return [props.modelValue as string | number]
})

const normalizedSingleValue = computed<string | number | undefined>(() => {
  if (Array.isArray(props.modelValue)) {
    return props.modelValue[0]
  }

  if (props.modelValue === null || props.modelValue === undefined || normalizeText(props.modelValue) === '') {
    return undefined
  }

  return props.modelValue as string | number
})

const selectedValueKeys = computed(() => new Set(normalizedModelArray.value.map(value => optionKey(value))))

const optionItems = computed(() => dedupeOptions(props.items || []))

const modelDerivedItems = computed<OmniSelectViewOption[]>(() => {
  const known = new Set([
    ...optionItems.value.map(item => optionKey(item.value)),
    ...createdItems.value.map(item => optionKey(item.value))
  ])

  return normalizedModelArray.value
    .filter(value => !known.has(optionKey(value)))
    .map(value => normalizeOption({
      label: normalizeText(value),
      value
    }))
})

const mergedItems = computed<OmniSelectViewOption[]>(() => {
  return dedupeOptions([
    ...optionItems.value,
    ...createdItems.value,
    ...modelDerivedItems.value
  ]).filter((option) => {
    const meta = optionMetaState.value[optionKey(option.value)]
    if (meta?.deleted && !selectedValueKeys.value.has(optionKey(option.value))) {
      return false
    }

    return true
  })
})

const optionMapByKey = computed(() => {
  const map = new Map<string, OmniSelectViewOption>()
  for (const item of mergedItems.value) {
    map.set(optionKey(item.value), item)
  }
  return map
})

const createItemConfig = computed<OmniCreateOptionConfig>(() => {
  if (props.creatable === false) return false
  if (props.creatable === true) {
    return {
      when: 'always',
      position: 'bottom'
    }
  }

  return props.creatable
})

function uniqueValues(values: Array<string | number>) {
  const seen = new Set<string>()
  const out: Array<string | number> = []

  for (const value of values) {
    const key = optionKey(value)
    if (seen.has(key)) continue
    seen.add(key)
    out.push(value)
  }

  return out
}

function valueFromItem(item: unknown) {
  if (typeof item === 'object' && item !== null && 'value' in item) {
    return (item as { value: string | number }).value
  }

  return item as string | number
}

function optionForValue(value: unknown) {
  const key = optionKey(value)
  return optionMapByKey.value.get(key)
}

function optionOrFallback(value: unknown): OmniSelectViewOption {
  const existing = optionForValue(value)
  if (existing) return existing

  const normalizedValue = valueFromItem(value)
  return {
    label: optionLabelForValue(normalizedValue) || normalizeText(normalizedValue),
    value: normalizedValue,
    color: optionColorForValue(normalizedValue) as OmniSelectColorKey
  }
}

function optionLabelForValue(value: unknown) {
  return optionForValue(value)?.label || normalizeText(value)
}

function optionColorForValue(value: unknown) {
  return optionForValue(value)?.color || 'default'
}

function optionChipClass(value: unknown) {
  return colorConfig(optionColorForValue(value)).chipClass
}

function optionSwatchClass(color: OmniSelectColorKey | undefined) {
  return colorConfig(color).swatchClass
}

function onSingleUpdate(value: unknown) {
  if (value === undefined) {
    emit('update:modelValue', null)
    return
  }

  emit('update:modelValue', value as string | number)
}

function resolveFocusableTarget(value: unknown): HTMLElement | null {
  if (!value) return null

  if (value instanceof HTMLElement) {
    return value
  }

  if (typeof value === 'object' && value !== null && 'value' in value) {
    return resolveFocusableTarget((value as { value: unknown }).value)
  }

  if (typeof value === 'object' && value !== null && '$el' in value) {
    return resolveFocusableTarget((value as { $el: unknown }).$el)
  }

  return null
}

function focusInteractiveTarget() {
  if (props.disabled) return

  nextTick(() => {
    const target = props.multiple
      ? resolveFocusableTarget(inputMenuRef.value?.inputRef)
      : resolveFocusableTarget(selectMenuRef.value?.triggerRef)

    if (target && typeof target.focus === 'function') {
      target.focus()
    }
  })
}

function onMultiUpdate(value: unknown) {
  const next = Array.isArray(value) ? uniqueValues(value as Array<string | number>) : []
  emit('update:modelValue', next)

  if (menuOpen.value) {
    focusInteractiveTarget()
  }
}

function onCreateItem(rawValue: string) {
  const normalized = normalizeText(rawValue)
  if (!normalized) return

  const existing = mergedItems.value.find(item => optionKey(item.value) === optionKey(normalized))
  const option = existing || normalizeOption({ label: normalized, value: normalized })

  if (!existing) {
    createdItems.value = dedupeOptions([...createdItems.value, option])
  }

  setOptionMeta(option.value, { deleted: false })
  emit('create', option)
  searchTerm.value = ''

  if (props.multiple) {
    const next = uniqueValues([...normalizedModelArray.value, option.value])
    emit('update:modelValue', next)
    focusInteractiveTarget()
    return
  }

  emit('update:modelValue', option.value)
}

function onMenuOpenChange(value: boolean) {
  menuOpen.value = value
  if (value && props.multiple) {
    focusInteractiveTarget()
    return
  }

  if (!value) {
    searchTerm.value = ''
  }
}

function onContainerMouseDown() {
  if (props.disabled || !props.multiple) return

  menuOpen.value = true
  focusInteractiveTarget()
}

function onContainerClick() {
  if (props.disabled || !props.multiple || !menuOpen.value) return
  focusInteractiveTarget()
}

function removeTagValue(value: unknown) {
  if (!props.multiple) return
  const key = optionKey(value)
  const next = normalizedModelArray.value.filter(current => optionKey(current) !== key)
  emit('update:modelValue', next)
  focusInteractiveTarget()
}

function ensureDraftLabel(option: OmniSelectViewOption) {
  const key = optionKey(option.value)
  if (!optionDraftLabelMap[key]) {
    optionDraftLabelMap[key] = option.label
  }
}

function draftLabelForValue(value: unknown) {
  const option = optionOrFallback(value)
  ensureDraftLabel(option)
  return optionDraftLabelMap[optionKey(option.value)] || option.label
}

function renameOption(option: OmniSelectViewOption) {
  const key = optionKey(option.value)
  const draft = normalizeText(optionDraftLabelMap[key])
  if (!draft) return

  setOptionMeta(option.value, {
    label: draft,
    deleted: false
  })
}

function updateOptionColor(option: OmniSelectViewOption, color: OmniSelectColorKey) {
  setOptionMeta(option.value, {
    color,
    deleted: false
  })
}

function removeOption(option: OmniSelectViewOption) {
  const key = optionKey(option.value)

  setOptionMeta(option.value, { deleted: true })
  createdItems.value = createdItems.value.filter(item => optionKey(item.value) !== key)

  if (props.multiple) {
    const next = normalizedModelArray.value.filter(value => optionKey(value) !== key)
    emit('update:modelValue', next)
    return
  }

  if (normalizedSingleValue.value !== undefined && optionKey(normalizedSingleValue.value) === key) {
    emit('update:modelValue', null)
  }
}
</script>

<template>
  <div :class="wrapperClasses" @mousedown="onContainerMouseDown" @click="onContainerClick">
    <UInputMenu
      v-if="props.multiple"
      ref="inputMenuRef"
      v-model:search-term="searchTerm"
      :open="menuOpen"
      :model-value="normalizedModelArray"
      :items="mergedItems"
      value-key="value"
      label-key="label"
      multiple
      open-on-click
      open-on-focus
      variant="ghost"
      :disabled="props.disabled"
      placeholder=""
      :create-item="createItemConfig"
      :clear="props.clear"
      :ui="multiUi"
      class="omni-select__control omni-select__control--multi w-full"
      @update:model-value="onMultiUpdate"
      @update:open="onMenuOpenChange"
      @create="onCreateItem"
    >
      <template #content-top>
        <div v-if="menuOpen && props.placeholder" class="omni-select__menu-hint border-b border-[rgb(var(--border))] px-2 py-1 text-[14px] text-[rgb(var(--muted))]">
          {{ props.placeholder }}
        </div>
      </template>

      <template #create-item-label="{ item }">
        Criar "{{ item }}"
      </template>

      <template #item-label="{ item }">
        <span class="omni-select__option-chip inline-flex max-w-[180px] items-center rounded-md px-2 py-1 text-[14px] font-medium leading-none ring-1 ring-inset" :class="optionChipClass(valueFromItem(item))">
          <span class="truncate">{{ optionLabelForValue(valueFromItem(item)) }}</span>
        </span>
      </template>

      <template #tags-item-text="{ item }">
        <span class="omni-select__tag-chip relative inline-flex max-w-[148px] items-center rounded-md px-2 py-1 text-[14px] font-medium leading-none ring-1 ring-inset transition-colors" :class="optionChipClass(item)">
          <span class="truncate" :class="menuOpen ? 'pe-4' : ''">{{ optionLabelForValue(item) }}</span>
          <button
            type="button"
            class="omni-select__tag-remove absolute end-0.5 top-1/2 inline-flex h-3.5 w-3.5 -translate-y-1/2 items-center justify-center rounded-sm bg-black/70 text-white ring-1 ring-white/20 transition hover:bg-black/90"
            :class="menuOpen ? 'opacity-100' : 'pointer-events-none opacity-0'"
            aria-label="Remover opcao"
            @mousedown.prevent
            @click.stop.prevent="removeTagValue(item)"
          >
            <UIcon name="i-lucide-x" class="size-2" />
          </button>
        </span>
      </template>

      <template #item-trailing="{ item }">
        <UPopover v-if="isManageEnabled()" :content="{ side: 'right', align: 'start' }">
          <UButton
            icon="i-lucide-ellipsis"
            color="neutral"
            variant="ghost"
            size="xs"
            aria-label="Editar opcao"
            @click.stop.prevent
          />

          <template #content>
            <div class="w-[250px] space-y-3 p-3" @click.stop>
              <div class="space-y-1">
                <p class="text-[14px] font-semibold uppercase tracking-wide text-[rgb(var(--muted))]">Nome</p>
                <UInput
                  :model-value="draftLabelForValue(valueFromItem(item))"
                  size="sm"
                  @update:model-value="optionDraftLabelMap[optionKey(valueFromItem(item))] = String($event ?? '')"
                  @keydown.enter.prevent="renameOption(optionOrFallback(valueFromItem(item)))"
                />
              </div>

              <div class="space-y-1.5">
                <p class="text-[14px] font-semibold uppercase tracking-wide text-[rgb(var(--muted))]">Cores</p>
                <button
                  v-for="color in COLOR_PALETTE"
                  :key="color.key"
                  type="button"
                  class="flex w-full items-center justify-between rounded px-2 py-1.5 text-left text-xs transition hover:bg-[rgb(var(--surface-2))]"
                  @click="updateOptionColor(optionOrFallback(valueFromItem(item)), color.key)"
                >
                  <span class="flex items-center gap-2">
                    <span class="inline-flex h-4 w-4 rounded" :class="color.swatchClass" />
                    {{ color.label }}
                  </span>
                  <UIcon
                    v-if="optionColorForValue(valueFromItem(item)) === color.key"
                    name="i-lucide-check"
                    class="text-[rgb(var(--text))]"
                  />
                </button>
              </div>

              <div class="border-t border-[rgb(var(--border))] pt-2">
                <UButton
                  icon="i-lucide-trash-2"
                  label="Excluir"
                  color="error"
                  variant="ghost"
                  size="xs"
                  @click="removeOption(optionOrFallback(valueFromItem(item)))"
                />
              </div>
            </div>
          </template>
        </UPopover>
      </template>
    </UInputMenu>

    <USelectMenu
      v-else
      ref="selectMenuRef"
      v-model:search-term="searchTerm"
      :model-value="normalizedSingleValue"
      :items="mergedItems"
      value-key="value"
      label-key="label"
      variant="ghost"
      :ui="singleUi"
      :disabled="props.disabled"
      :placeholder="menuOpen ? props.placeholder : ''"
      :create-item="createItemConfig"
      :search-input="props.searchInput"
      :clear="props.clear"
      class="omni-select__control omni-select__control--single w-full"
      @update:model-value="onSingleUpdate"
      @update:open="onMenuOpenChange"
      @create="onCreateItem"
    >
      <template #content-top>
        <div v-if="menuOpen && props.placeholder" class="omni-select__menu-hint border-b border-[rgb(var(--border))] px-2 py-1 text-[14px] text-[rgb(var(--muted))]">
          {{ props.placeholder }}
        </div>
      </template>

      <template #default="{ modelValue }">
        <span v-if="modelValue !== undefined" class="omni-select__selected-chip inline-flex max-w-[180px] items-center rounded-md px-2 py-1 text-[14px] font-medium leading-none ring-1 ring-inset" :class="optionChipClass(modelValue)">
          <span class="truncate">{{ optionLabelForValue(modelValue) }}</span>
        </span>
        <span v-else class="inline-block h-4 min-w-2" />
      </template>

      <template #item-label="{ item }">
        <span class="omni-select__option-chip inline-flex max-w-[180px] items-center rounded-md px-2 py-1 text-[14px] font-medium leading-none ring-1 ring-inset" :class="optionChipClass(valueFromItem(item))">
          <span class="truncate">{{ optionLabelForValue(valueFromItem(item)) }}</span>
        </span>
      </template>

      <template #item-trailing="{ item }">
        <UPopover v-if="isManageEnabled()" :content="{ side: 'right', align: 'start' }">
          <UButton
            icon="i-lucide-ellipsis"
            color="neutral"
            variant="ghost"
            size="xs"
            aria-label="Editar opcao"
            @click.stop.prevent
          />

          <template #content>
            <div class="w-[250px] space-y-3 p-3" @click.stop>
              <div class="space-y-1">
                <p class="text-[14px] font-semibold uppercase tracking-wide text-[rgb(var(--muted))]">Nome</p>
                <UInput
                  :model-value="draftLabelForValue(valueFromItem(item))"
                  size="sm"
                  @update:model-value="optionDraftLabelMap[optionKey(valueFromItem(item))] = String($event ?? '')"
                  @keydown.enter.prevent="renameOption(optionOrFallback(valueFromItem(item)))"
                />
              </div>

              <div class="space-y-1.5">
                <p class="text-[14px] font-semibold uppercase tracking-wide text-[rgb(var(--muted))]">Cores</p>
                <button
                  v-for="color in COLOR_PALETTE"
                  :key="color.key"
                  type="button"
                  class="flex w-full items-center justify-between rounded px-2 py-1.5 text-left text-xs transition hover:bg-[rgb(var(--surface-2))]"
                  @click="updateOptionColor(optionOrFallback(valueFromItem(item)), color.key)"
                >
                  <span class="flex items-center gap-2">
                    <span class="inline-flex h-4 w-4 rounded" :class="color.swatchClass" />
                    {{ color.label }}
                  </span>
                  <UIcon
                    v-if="optionColorForValue(valueFromItem(item)) === color.key"
                    name="i-lucide-check"
                    class="text-[rgb(var(--text))]"
                  />
                </button>
              </div>

              <div class="border-t border-[rgb(var(--border))] pt-2">
                <UButton
                  icon="i-lucide-trash-2"
                  label="Excluir"
                  color="error"
                  variant="ghost"
                  size="xs"
                  @click="removeOption(optionOrFallback(valueFromItem(item)))"
                />
              </div>
            </div>
          </template>
        </UPopover>
      </template>

      <template #create-item-label="{ item }">
        Criar "{{ item }}"
      </template>
    </USelectMenu>
  </div>
</template>
