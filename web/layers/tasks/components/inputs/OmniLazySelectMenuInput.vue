<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import OmniSelectMenuInput from './OmniSelectMenuInput.vue'
import type { OmniSelectOption } from '../../types/omni/collection'
import { optionColorConfig, normalizeOptionText, type BadgeStyle } from './option-colors'

// `inheritAttrs: false`: as props extras (searchable, fullContentWidth, variant,
// creatable, optionEditMode, etc.) entram via `$attrs` e devem ir para o editor
// real, nunca cair no `<button>` placeholder.
defineOptions({ inheritAttrs: false })

// Wrapper de montagem tardia para os selects do card/modal de Tasks.
//
// Problema: o board renderiza ate ~250 cards, cada um com 5-6 `OmniSelectMenuInput`
// (cada um e' um `USelectMenu`/combobox pesado da reka-ui). Montar todos de uma vez
// na primeira pintura trava a rota /tasks (cold T3 15s+).
//
// Solucao: antes de interagir, renderizamos so um badge estatico leve (o valor
// formatado, com a mesma cor/estilo que o editor usaria). No primeiro clique/foco
// (ou Enter/Espaco via teclado), montamos o `OmniSelectMenuInput` real e ja o
// abrimos (`:open`), entao um unico clique vira o editor com o dropdown aberto.
//
// O componente repassa TODAS as props e eventos 1:1 para o editor real — nao muda
// regra de negocio, so adia o custo de montagem. Visualmente o badge do placeholder
// espelha o badge `badgeMode` do `OmniSelectMenuInput`.

type SelectPrimitive = string | number
type SelectModelValue = SelectPrimitive | SelectPrimitive[] | null

interface LazyItem {
  label?: string
  value: SelectPrimitive
  avatar?: Record<string, unknown>
  color?: string
}

const props = withDefaults(
  defineProps<{
    modelValue?: SelectModelValue
    items?: Array<LazyItem | SelectPrimitive>
    placeholder?: string
    multiple?: boolean
    showAvatar?: boolean
    badgeStyle?: BadgeStyle
    disabled?: boolean
    // Repassadas via $attrs ao editor real; declaradas aqui apenas as que o
    // placeholder precisa para renderizar identico ao editor.
  }>(),
  {
    modelValue: null,
    items: () => [],
    placeholder: 'Selecione',
    multiple: false,
    showAvatar: false,
    badgeStyle: 'filled',
    disabled: false,
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: SelectModelValue]
  create: [option: OmniSelectOption]
  'update:open': [open: boolean]
}>()

// Estado de meta (rename/recolor) compartilhado com o editor real, para que o
// placeholder mostre o mesmo label/cor de uma option renomeada ou recolorida.
interface OptionMeta {
  label?: string
  color?: string
  deleted?: boolean
}
const optionMetaState = useState<Record<string, OptionMeta>>(
  '__omni_select_menu_option_meta__',
  () => ({}),
)

const activated = ref(false)
const innerOpen = ref<boolean | undefined>(undefined)

function optionKey(value: unknown) {
  return normalizeOptionText(value).toLowerCase()
}

const normalizedValues = computed<SelectPrimitive[]>(() => {
  const raw = props.modelValue
  if (Array.isArray(raw)) {
    return raw.filter((v) => normalizeOptionText(v).length > 0)
  }
  if (raw == null || normalizeOptionText(raw).length === 0) return []
  return [raw]
})

const itemMap = computed(() => {
  const map = new Map<string, LazyItem>()
  ;(props.items || []).forEach((source) => {
    const item: LazyItem =
      typeof source === 'object' && source !== null && 'value' in source
        ? (source as LazyItem)
        : { label: normalizeOptionText(source), value: source as SelectPrimitive }
    map.set(optionKey(item.value), item)
  })
  return map
})

interface DisplayChip {
  key: string
  label: string
  avatar?: Record<string, unknown>
  badgeClass: string
}

const displayChips = computed<DisplayChip[]>(() =>
  normalizedValues.value.map((value) => {
    const key = optionKey(value)
    const item = itemMap.value.get(key)
    const meta = optionMetaState.value[key]
    const label = normalizeOptionText(meta?.label || item?.label || value)
    const config = optionColorConfig(meta?.color || item?.color)
    const badgeClass = props.badgeStyle === 'entity' ? config.entityClass : config.badgeClass
    return {
      key: String(value),
      label,
      avatar: props.showAvatar ? item?.avatar : undefined,
      badgeClass,
    }
  }),
)

const hasValue = computed(() => displayChips.value.length > 0)

// Ao ativar, monta o editor real e ja o abre (um clique = editar com dropdown).
async function activate() {
  if (props.disabled || activated.value) return
  activated.value = true
  await nextTick()
  innerOpen.value = true
}

function onActivateKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter' || event.key === ' ' || event.key === 'Spacebar') {
    event.preventDefault()
    void activate()
  }
}

// Quando o editor real fecha o dropdown, mantemos ele montado (custo ja pago) e
// soltamos o controle de `open` para o proprio editor. Repassamos o evento.
function onInnerOpenUpdate(open: boolean) {
  innerOpen.value = open
  emit('update:open', open)
}
</script>

<template>
  <OmniSelectMenuInput
    v-if="activated"
    v-bind="$attrs"
    :model-value="props.modelValue"
    :items="props.items"
    :placeholder="props.placeholder"
    :multiple="props.multiple"
    :show-avatar="props.showAvatar"
    :badge-style="props.badgeStyle"
    :disabled="props.disabled"
    :open="innerOpen"
    @update:model-value="emit('update:modelValue', $event)"
    @create="emit('create', $event)"
    @update:open="onInnerOpenUpdate"
  />
  <button
    v-else
    type="button"
    class="omni-lazy-select"
    :class="{ 'omni-lazy-select--disabled': props.disabled }"
    :disabled="props.disabled"
    :title="props.placeholder"
    @click.stop="activate"
    @focus="activate"
    @keydown="onActivateKeydown"
  >
    <span v-if="hasValue" class="omni-lazy-select__chips flex flex-wrap items-center gap-1">
      <span
        v-for="chip in displayChips"
        :key="chip.key"
        class="omni-lazy-select__chip ring-1 ring-inset"
        :class="[
          chip.badgeClass,
          { 'omni-lazy-select__chip--entity': props.badgeStyle === 'entity' },
        ]"
      >
        <UAvatar
          v-if="chip.avatar"
          v-bind="chip.avatar"
          size="3xs"
          class="omni-lazy-select__chip-avatar"
        />
        <span class="omni-lazy-select__chip-label truncate">{{ chip.label }}</span>
      </span>
    </span>
    <span v-else class="omni-lazy-select__placeholder">{{ props.placeholder }}</span>
  </button>
</template>

<style>
.omni-lazy-select {
  display: inline-flex;
  align-items: center;
  max-width: 100%;
  padding: 0.1rem 0.3rem;
  border: none;
  background: transparent;
  text-align: left;
  cursor: pointer;
  border-radius: 6px;
}

.omni-lazy-select--disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.omni-lazy-select__chip {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  max-width: 220px;
  padding: 0.1rem 0.3rem;
  border-radius: 0.375rem;
  font-size: 0.75rem;
  font-weight: 500;
  line-height: 1.1;
  color: rgb(var(--text));
}

.omni-lazy-select__chip-label {
  min-width: 0;
}

.omni-lazy-select__placeholder {
  color: rgb(var(--muted));
  font-size: 0.75rem;
}
</style>
