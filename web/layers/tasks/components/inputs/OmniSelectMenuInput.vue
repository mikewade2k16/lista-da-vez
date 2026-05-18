<script setup lang="ts">
import type { OmniSelectOption } from '../../types/omni/collection'

type SelectPrimitive = string | number
type SelectModelValue = SelectPrimitive | SelectPrimitive[] | null
type SelectColor = 'primary' | 'secondary' | 'success' | 'info' | 'warning' | 'error' | 'neutral'
type SelectVariant = 'outline' | 'soft' | 'subtle' | 'ghost' | 'none'
type OptionEditMode = 'none' | 'color' | 'full'
type BadgeStyle = 'filled' | 'entity'
type OptionColorKey = 'default' | 'gray' | 'brown' | 'orange' | 'yellow' | 'green' | 'blue' | 'purple' | 'pink' | 'red'
type SelectCreateConfig =
  | boolean
  | 'always'
  | {
    position?: 'top' | 'bottom'
    when?: 'empty' | 'always'
  }

interface OmniSelectMenuInputItem {
  label?: string
  value: SelectPrimitive
  description?: string
  icon?: string
  avatar?: Record<string, unknown>
  color?: string
  disabled?: boolean
  class?: unknown
}

interface OptionMeta {
  label?: string
  color?: OptionColorKey
  deleted?: boolean
}

interface OptionColorConfig {
  key: OptionColorKey
  label: string
  swatchClass: string
  badgeClass: string
  entityClass: string
}

type SelectSourceItem = OmniSelectMenuInputItem | SelectPrimitive

const OPTION_COLOR_PALETTE: OptionColorConfig[] = [
  { key: 'default', label: 'Padrao', swatchClass: 'bg-zinc-500', badgeClass: 'bg-zinc-700/45 text-zinc-100 ring-zinc-500/50', entityClass: 'bg-transparent text-[rgb(var(--text))] ring-transparent' },
  { key: 'gray', label: 'Cinza', swatchClass: 'bg-slate-500', badgeClass: 'bg-slate-600/50 text-slate-100 ring-slate-400/50', entityClass: 'bg-transparent text-[rgb(var(--text))] ring-slate-400/70' },
  { key: 'brown', label: 'Marrom', swatchClass: 'bg-amber-700', badgeClass: 'bg-amber-700/50 text-amber-50 ring-amber-500/50', entityClass: 'bg-transparent text-[rgb(var(--text))] ring-amber-600/70' },
  { key: 'orange', label: 'Laranja', swatchClass: 'bg-orange-500', badgeClass: 'bg-orange-600/55 text-orange-50 ring-orange-400/50', entityClass: 'bg-transparent text-[rgb(var(--text))] ring-orange-400/80' },
  { key: 'yellow', label: 'Amarelo', swatchClass: 'bg-yellow-500', badgeClass: 'bg-yellow-500/55 text-zinc-950 ring-yellow-400/50', entityClass: 'bg-transparent text-[rgb(var(--text))] ring-yellow-400/80' },
  { key: 'green', label: 'Verde', swatchClass: 'bg-emerald-500', badgeClass: 'bg-emerald-700/55 text-emerald-50 ring-emerald-400/50', entityClass: 'bg-transparent text-[rgb(var(--text))] ring-emerald-400/80' },
  { key: 'blue', label: 'Azul', swatchClass: 'bg-blue-500', badgeClass: 'bg-blue-700/55 text-blue-50 ring-blue-400/50', entityClass: 'bg-transparent text-[rgb(var(--text))] ring-blue-400/80' },
  { key: 'purple', label: 'Roxo', swatchClass: 'bg-violet-500', badgeClass: 'bg-violet-700/55 text-violet-50 ring-violet-400/50', entityClass: 'bg-transparent text-[rgb(var(--text))] ring-violet-400/80' },
  { key: 'pink', label: 'Rosa', swatchClass: 'bg-fuchsia-500', badgeClass: 'bg-fuchsia-700/55 text-fuchsia-50 ring-fuchsia-400/50', entityClass: 'bg-transparent text-[rgb(var(--text))] ring-fuchsia-400/80' },
  { key: 'red', label: 'Vermelho', swatchClass: 'bg-rose-500', badgeClass: 'bg-rose-700/55 text-rose-50 ring-rose-400/50', entityClass: 'bg-transparent text-[rgb(var(--text))] ring-rose-400/80' }
]

const props = withDefaults(defineProps<{
  modelValue?: SelectModelValue
  items?: SelectSourceItem[]
  placeholder?: string
  multiple?: boolean
  creatable?: SelectCreateConfig
  searchable?: boolean
  fullContentWidth?: boolean
  loading?: boolean
  loadingIcon?: string
  disabled?: boolean
  clear?: boolean
  size?: 'xs' | 'sm' | 'md' | 'lg' | 'xl'
  color?: SelectColor
  variant?: SelectVariant
  highlight?: boolean
  trailingIcon?: string
  selectedIcon?: string
  ignoreFilter?: boolean
  badgeMode?: boolean
  badgeStyle?: BadgeStyle
  showAvatar?: boolean
  itemDisplayMode?: 'rich' | 'text'
  optionEditMode?: OptionEditMode
  open?: boolean
}>(), {
  modelValue: null,
  items: () => [],
  placeholder: 'Selecione',
  multiple: false,
  creatable: false,
  searchable: true,
  fullContentWidth: false,
  loading: false,
  loadingIcon: 'i-lucide-loader-circle',
  disabled: false,
  clear: false,
  size: 'sm',
  color: 'neutral',
  variant: 'none',
  highlight: false,
  trailingIcon: 'i-lucide-chevron-down',
  selectedIcon: 'i-lucide-check',
  ignoreFilter: false,
  badgeMode: false,
  badgeStyle: 'filled',
  showAvatar: false,
  itemDisplayMode: 'rich',
  optionEditMode: 'none',
  open: undefined
})

const emit = defineEmits<{
  'update:modelValue': [value: SelectModelValue]
  create: [option: OmniSelectOption]
  'update:open': [open: boolean]
}>()

const createdItems = ref<OmniSelectMenuInputItem[]>([])
const optionMetaState = useState<Record<string, OptionMeta>>('__omni_select_menu_option_meta__', () => ({}))
const optionDraftLabelMap = reactive<Record<string, string>>({})

function normalizeText(value: unknown, max = 180) {
  return String(value ?? '').replace(/\s+/g, ' ').trim().slice(0, max)
}

function optionKey(value: unknown) {
  return normalizeText(value).toLowerCase()
}

function normalizeColorKey(value: unknown): OptionColorKey {
  const key = normalizeText(value, 30).toLowerCase() as OptionColorKey
  const known = OPTION_COLOR_PALETTE.find(item => item.key === key)
  return known?.key || 'default'
}

function colorConfig(color: unknown): OptionColorConfig {
  const key = normalizeColorKey(color)
  return OPTION_COLOR_PALETTE.find(item => item.key === key) || OPTION_COLOR_PALETTE[0]!
}

function setOptionMeta(value: unknown, patch: Partial<OptionMeta>) {
  const key = optionKey(value)
  if (!key) return
  optionMetaState.value = {
    ...optionMetaState.value,
    [key]: {
      ...(optionMetaState.value[key] || {}),
      ...patch
    }
  }
}

function normalizeOption(source: SelectSourceItem): OmniSelectMenuInputItem {
  const base: OmniSelectMenuInputItem = (typeof source === 'object' && source !== null && 'value' in source)
    ? {
      ...source,
      value: source.value as SelectPrimitive
    }
    : {
      label: normalizeText(source),
      value: source as SelectPrimitive
    }

  const meta = optionMetaState.value[optionKey(base.value)]
  const label = normalizeText(meta?.label || base.label || base.value)
  return {
    ...base,
    label,
    color: normalizeColorKey(meta?.color || base.color)
  }
}

function isValidPrimitive(value: unknown): value is SelectPrimitive {
  if (typeof value === 'number') return Number.isFinite(value)
  if (typeof value === 'string') return normalizeText(value).length > 0
  return false
}

function dedupeOptions(options: OmniSelectMenuInputItem[]) {
  const seen = new Set<string>()
  const output: OmniSelectMenuInputItem[] = []
  options.forEach((option) => {
    const key = optionKey(option.value)
    if (!key || seen.has(key)) return
    seen.add(key)
    output.push(option)
  })
  return output
}

const normalizedModelArray = computed<SelectPrimitive[]>(() => {
  if (Array.isArray(props.modelValue)) {
    return props.modelValue.filter(isValidPrimitive)
  }
  if (!isValidPrimitive(props.modelValue)) return []
  return [props.modelValue]
})

const selectedValueKeys = computed(() => new Set(normalizedModelArray.value.map(value => optionKey(value))))

const sourceItems = computed(() => {
  return (props.items || [])
    .map(normalizeOption)
    .filter(item => isValidPrimitive(item.value))
})

const modelDerivedItems = computed<OmniSelectMenuInputItem[]>(() => {
  const known = new Set(sourceItems.value.map(item => optionKey(item.value)))
  createdItems.value.forEach(item => known.add(optionKey(item.value)))
  return normalizedModelArray.value
    .filter(value => !known.has(optionKey(value)))
    .map(value => ({
      label: normalizeText(value),
      value,
      color: 'default'
    }))
})

const mergedItems = computed(() => {
  return dedupeOptions([
    ...sourceItems.value,
    ...createdItems.value,
    ...modelDerivedItems.value
  ]).filter((item) => {
    const meta = optionMetaState.value[optionKey(item.value)]
    if (meta?.deleted && !selectedValueKeys.value.has(optionKey(item.value))) {
      return false
    }
    return true
  })
})

const itemMapByValueKey = computed(() => {
  const map = new Map<string, OmniSelectMenuInputItem>()
  mergedItems.value.forEach(item => map.set(optionKey(item.value), item))
  return map
})

function itemForValue(value: unknown): OmniSelectMenuInputItem {
  const known = itemMapByValueKey.value.get(optionKey(value))
  if (known) return known
  const normalized = normalizeText(value)
  return {
    label: normalized,
    value: normalized,
    color: 'default'
  }
}

const selectedItems = computed(() => normalizedModelArray.value.map(value => itemForValue(value)))

const resolvedModelValue = computed<SelectPrimitive | SelectPrimitive[] | undefined>(() => {
  if (props.multiple) return normalizedModelArray.value
  return normalizedModelArray.value[0]
})

const canManageOptions = computed(() => props.badgeMode && props.optionEditMode !== 'none')
const canRenameOptions = computed(() => props.optionEditMode === 'full')
const canDeleteOptions = computed(() => props.optionEditMode === 'full')

const selectUi = computed(() => ({
  base: 'omni-select-menu-input__base',
  content: props.fullContentWidth
    ? 'omni-select-menu-input__content omni-select-menu-input__content--full min-w-fit max-w-[min(38rem,calc(100vw-2rem))]'
    : 'omni-select-menu-input__content',
  item: 'omni-select-menu-input__item',
  itemLabel: 'omni-select-menu-input__item-label',
  itemDescription: 'omni-select-menu-input__item-description',
  itemTrailingIcon: canManageOptions.value
    ? 'omni-select-menu-input__item-trailing-icon-default hidden'
    : 'omni-select-menu-input__item-trailing-icon-default',
  value: 'omni-select-menu-input__value',
  placeholder: 'omni-select-menu-input__placeholder',
  input: 'omni-select-menu-input__search',
  trailing: 'omni-select-menu-input__trailing',
  trailingIcon: 'omni-select-menu-input__trailing-icon'
}))

function optionBadgeClassForValue(value: unknown) {
  const option = itemForValue(value)
  const config = colorConfig(option.color)
  return props.badgeStyle === 'entity' ? config.entityClass : config.badgeClass
}

function emitModelValue(value: SelectModelValue) {
  emit('update:modelValue', value)
}

function onUpdateModelValue(nextValue: unknown) {
  if (props.multiple) {
    const rawArray = Array.isArray(nextValue) ? nextValue : []
    const next = dedupeOptions(rawArray
      .filter(isValidPrimitive)
      .map(value => ({ label: normalizeText(value), value })))
      .map(option => option.value)
    emitModelValue(next)
    return
  }

  if (!isValidPrimitive(nextValue)) {
    emitModelValue(null)
    return
  }

  emitModelValue(nextValue)
}

function onCreateItem(rawItem: string) {
  const label = normalizeText(rawItem, 120)
  if (!label) return

  const existing = mergedItems.value.find(item => optionKey(item.value) === optionKey(label))
  const option = existing || {
    label,
    value: label,
    color: 'default'
  }

  if (!existing) {
    createdItems.value = dedupeOptions([...createdItems.value, option])
  } else {
    setOptionMeta(option.value, { deleted: false })
  }

  emit('create', {
    label: option.label || label,
    value: option.value
  })

  if (props.multiple) {
    const nextValues = dedupeOptions([
      ...selectedItems.value,
      option
    ]).map(item => item.value)
    emitModelValue(nextValues)
    return
  }

  emitModelValue(option.value)
}

function removeSingleSelection() {
  if (props.disabled || !props.clear) return
  emitModelValue(null)
}

function removeMultiSelection(value: SelectPrimitive) {
  if (props.disabled || !props.clear) return
  const next = normalizedModelArray.value.filter(item => optionKey(item) !== optionKey(value))
  emitModelValue(next)
}

function ensureDraftLabel(option: OmniSelectMenuInputItem) {
  const key = optionKey(option.value)
  if (!key) return
  if (!optionDraftLabelMap[key]) {
    optionDraftLabelMap[key] = String(option.label || option.value)
  }
}

function draftLabelForOption(option: OmniSelectMenuInputItem) {
  ensureDraftLabel(option)
  return optionDraftLabelMap[optionKey(option.value)] || String(option.label || option.value)
}

function renameOption(option: OmniSelectMenuInputItem) {
  if (!canRenameOptions.value) return
  const key = optionKey(option.value)
  const draft = normalizeText(optionDraftLabelMap[key], 120)
  if (!draft) return
  setOptionMeta(option.value, { label: draft, deleted: false })
}

function updateOptionColor(option: OmniSelectMenuInputItem, color: OptionColorKey) {
  setOptionMeta(option.value, { color, deleted: false })
}

function isOptionSelected(value: unknown) {
  return selectedValueKeys.value.has(optionKey(value))
}

function removeOption(option: OmniSelectMenuInputItem) {
  if (!canDeleteOptions.value) return
  const key = optionKey(option.value)
  setOptionMeta(option.value, { deleted: true })
  createdItems.value = createdItems.value.filter(item => optionKey(item.value) !== key)

  if (props.multiple) {
    const next = normalizedModelArray.value.filter(item => optionKey(item) !== key)
    emitModelValue(next)
    return
  }

  if (isOptionSelected(option.value)) {
    emitModelValue(null)
  }
}
</script>

<template>
  <div class="omni-select-menu-input w-full">
    <USelectMenu
      class="omni-select-menu-input__control w-full"
      :model-value="resolvedModelValue"
      :items="mergedItems"
      value-key="value"
      label-key="label"
      :multiple="props.multiple"
      :placeholder="props.placeholder"
      :size="props.size"
      :color="props.color"
      :variant="props.variant"
      :highlight="props.highlight"
      :loading="props.loading"
      :loading-icon="props.loadingIcon"
      :disabled="props.disabled"
      :clear="props.clear && !props.badgeMode"
      :create-item="props.creatable"
      :search-input="props.searchable ? { placeholder: props.placeholder } : false"
      :ignore-filter="props.ignoreFilter"
      :trailing-icon="props.trailingIcon"
      :selected-icon="props.selectedIcon"
      :ui="selectUi"
      :open="props.open"
      @update:model-value="onUpdateModelValue"
      @update:open="emit('update:open', $event)"
      @create="onCreateItem"
    >
      <template v-if="props.badgeMode" #default>
        <div class="omni-select-menu-input__badge-wrap flex flex-wrap items-center gap-1">
          <template v-if="selectedItems.length > 0">
            <template v-if="props.multiple">
              <UBadge
                v-for="item in selectedItems"
                :key="String(item.value)"
                class="omni-select-menu-input__selected-badge ring-1 ring-inset"
                :class="[optionBadgeClassForValue(item.value), { 'omni-select-menu-input__selected-badge--entity': props.badgeStyle === 'entity' }]"
                color="neutral"
                variant="solid"
              >
                <UAvatar
                  v-if="props.showAvatar && item.avatar"
                  v-bind="item.avatar"
                  size="3xs"
                  class="omni-select-menu-input__selected-badge-avatar"
                />
                <span class="omni-select-menu-input__selected-badge-label truncate">
                  {{ item.label }}
                </span>
                <button
                  v-if="props.clear"
                  type="button"
                  class="omni-select-menu-input__selected-badge-clear"
                  aria-label="Limpar item selecionado"
                  @mousedown.prevent
                  @click.stop.prevent="removeMultiSelection(item.value)"
                >
                  <UIcon name="i-lucide-x" class="size-3" />
                </button>
              </UBadge>
            </template>

            <template v-else>
              <UBadge
                class="omni-select-menu-input__selected-badge ring-1 ring-inset"
                :class="[optionBadgeClassForValue(selectedItems[0]?.value), { 'omni-select-menu-input__selected-badge--entity': props.badgeStyle === 'entity' }]"
                color="neutral"
                variant="solid"
              >
                <UAvatar
                  v-if="props.showAvatar && selectedItems[0]?.avatar"
                  v-bind="selectedItems[0].avatar"
                  size="3xs"
                  class="omni-select-menu-input__selected-badge-avatar"
                />
                <span class="omni-select-menu-input__selected-badge-label truncate">
                  {{ selectedItems[0]?.label }}
                </span>
                <button
                  v-if="props.clear"
                  type="button"
                  class="omni-select-menu-input__selected-badge-clear"
                  aria-label="Limpar item selecionado"
                  @mousedown.prevent
                  @click.stop.prevent="removeSingleSelection"
                >
                  <UIcon name="i-lucide-x" class="size-3" />
                </button>
              </UBadge>
            </template>
          </template>

          <UBadge
            v-else
            class="omni-select-menu-input__placeholder-badge"
            color="neutral"
            variant="soft"
          >
            {{ props.placeholder }}
          </UBadge>
        </div>
      </template>

      <template v-if="props.itemDisplayMode === 'rich'" #item-leading="{ item, ui }">
        <UAvatar
          v-if="props.showAvatar && item.avatar"
          v-bind="item.avatar"
          :size="ui.itemLeadingAvatarSize() as any"
          class="omni-select-menu-input__item-avatar"
        />
        <UIcon
          v-else-if="item.icon"
          :name="String(item.icon)"
          class="omni-select-menu-input__item-icon"
        />
      </template>

      <template #item-label="{ item }">
        <span
          v-if="props.badgeMode"
          class="omni-select-menu-input__item-badge inline-flex max-w-[220px] items-center rounded-md px-2 py-1 text-xs font-medium leading-none ring-1 ring-inset"
          :class="[optionBadgeClassForValue(item.value), { 'omni-select-menu-input__item-badge--entity': props.badgeStyle === 'entity' }]"
        >
          <span class="truncate">{{ item.label }}</span>
        </span>
        <span v-else class="omni-select-menu-input__item-text truncate">
          {{ item.label }}
        </span>
      </template>

      <template v-if="props.itemDisplayMode === 'rich'" #item-description="{ item }">
        <span v-if="item.description" class="omni-select-menu-input__item-description truncate">
          {{ item.description }}
        </span>
      </template>

      <template v-if="canManageOptions" #item-trailing="{ item }">
        <div class="omni-select-menu-input__item-actions flex items-center gap-1">
          <UIcon
            v-if="isOptionSelected(item.value)"
            :name="props.selectedIcon"
            class="omni-select-menu-input__item-selected-icon text-[rgb(var(--text))]"
          />

          <UPopover :content="{ side: 'right', align: 'start' }">
            <UButton
              icon="i-lucide-ellipsis"
              color="neutral"
              variant="ghost"
              size="xs"
              aria-label="Gerenciar opcao"
              @click.stop.prevent
            />

            <template #content>
              <div class="omni-select-menu-input__option-popover w-[250px] space-y-3 p-3" @click.stop>
                <div v-if="canRenameOptions" class="omni-select-menu-input__option-group space-y-1">
                  <p class="omni-select-menu-input__option-title text-[11px] font-semibold uppercase tracking-wide text-[rgb(var(--muted))]">
                    Nome
                  </p>
                  <UInput
                    :model-value="draftLabelForOption(item)"
                    size="sm"
                    @update:model-value="optionDraftLabelMap[optionKey(item.value)] = String($event ?? '')"
                    @keydown.enter.prevent="renameOption(item)"
                  />
                </div>

                <div class="omni-select-menu-input__option-group space-y-1">
                  <p class="omni-select-menu-input__option-title text-[11px] font-semibold uppercase tracking-wide text-[rgb(var(--muted))]">
                    Cores
                  </p>
                  <button
                    v-for="color in OPTION_COLOR_PALETTE"
                    :key="color.key"
                    type="button"
                    class="omni-select-menu-input__color-row flex w-full items-center justify-between rounded px-2 py-1.5 text-left text-xs transition hover:bg-[rgb(var(--surface-2))]"
                    @click="updateOptionColor(item, color.key)"
                  >
                    <span class="flex items-center gap-2">
                      <span class="omni-select-menu-input__color-swatch inline-flex h-4 w-4 rounded" :class="color.swatchClass" />
                      {{ color.label }}
                    </span>
                    <UIcon
                      v-if="colorConfig(item.color).key === color.key"
                      name="i-lucide-check"
                      class="omni-select-menu-input__color-check text-[rgb(var(--text))]"
                    />
                  </button>
                </div>

                <div v-if="canDeleteOptions" class="omni-select-menu-input__option-delete border-t border-[rgb(var(--border))] pt-2">
                  <UButton
                    icon="i-lucide-trash-2"
                    label="Excluir"
                    color="error"
                    variant="ghost"
                    size="xs"
                    @click="removeOption(item)"
                  />
                </div>
              </div>
            </template>
          </UPopover>
        </div>
      </template>

      <template #create-item-label="{ item }">
        Criar "{{ item }}"
      </template>
    </USelectMenu>
  </div>
</template>
