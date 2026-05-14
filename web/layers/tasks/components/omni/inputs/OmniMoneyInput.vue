<script setup lang="ts">
const props = withDefaults(
  defineProps<{
    modelValue?: number | null
    placeholder?: string
    disabled?: boolean
  }>(),
  {
    modelValue: 0,
    placeholder: 'R$ 0,00',
    disabled: false
  }
)

const emit = defineEmits<{
  'update:modelValue': [value: number]
}>()

const formatter = new Intl.NumberFormat('pt-BR', {
  style: 'currency',
  currency: 'BRL',
  minimumFractionDigits: 2,
  maximumFractionDigits: 2
})

function toNumeric(value: unknown) {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return Math.max(0, Number(value.toFixed(2)))
  }

  const raw = String(value ?? '').trim()
  if (!raw) return 0

  let normalized = raw
    .replace(/\s+/g, '')
    .replace(/^R\$/i, '')
    .replace(/[^\d,.-]/g, '')

  const hasComma = normalized.includes(',')
  const hasDot = normalized.includes('.')

  if (hasComma && hasDot) {
    if (normalized.lastIndexOf(',') > normalized.lastIndexOf('.')) {
      normalized = normalized.replace(/\./g, '').replace(',', '.')
    } else {
      normalized = normalized.replace(/,/g, '')
    }
  } else if (hasComma) {
    normalized = normalized.replace(/\./g, '').replace(',', '.')
  }

  const parsed = Number(normalized)
  if (!Number.isFinite(parsed)) return 0
  return Math.max(0, Number(parsed.toFixed(2)))
}

function toDigitsFromNumber(value: number) {
  const safe = Number.isFinite(value) ? Math.max(0, value) : 0
  return String(Math.round(safe * 100))
}

function toNumberFromDigits(value: string) {
  const digits = value.replace(/\D/g, '')
  if (!digits) return 0
  return Number(digits) / 100
}

const digits = ref(toDigitsFromNumber(toNumeric(props.modelValue)))

const displayValue = computed(() => {
  const numeric = toNumberFromDigits(digits.value)
  if (numeric <= 0) return ''
  return formatter.format(numeric)
})

watch(
  () => props.modelValue,
  (value) => {
    const next = toDigitsFromNumber(toNumeric(value))
    if (next !== digits.value) {
      digits.value = next
    }
  }
)

function updateFromDigits(nextDigits: string) {
  digits.value = nextDigits.replace(/\D/g, '')
  const numeric = Number(toNumberFromDigits(digits.value).toFixed(2))
  emit('update:modelValue', numeric)
}

function onKeydown(event: KeyboardEvent) {
  const key = event.key
  const ctrlOrMeta = event.ctrlKey || event.metaKey
  const shortcuts = ['a', 'c', 'v', 'x']

  if (ctrlOrMeta && shortcuts.includes(key.toLowerCase())) {
    return
  }

  if (['Tab', 'Enter', 'Escape', 'ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(key)) {
    return
  }

  if (key === 'Backspace' || key === 'Delete') {
    event.preventDefault()
    updateFromDigits(digits.value.slice(0, -1))
    return
  }

  if (/^\d$/.test(key)) {
    event.preventDefault()
    updateFromDigits(`${digits.value}${key}`)
    return
  }

  event.preventDefault()
}

function onPaste(event: ClipboardEvent) {
  event.preventDefault()
  const pasted = event.clipboardData?.getData('text') ?? ''
  const parsed = toNumeric(pasted)
  updateFromDigits(toDigitsFromNumber(parsed))
}

function onInput(value: string | number | undefined) {
  const parsed = toNumeric(value)
  updateFromDigits(toDigitsFromNumber(parsed))
}
</script>

<template>
  <UInput
    :model-value="displayValue"
    :placeholder="props.placeholder"
    inputmode="numeric"
    :disabled="props.disabled"
    @keydown="onKeydown"
    @paste="onPaste"
    @update:model-value="onInput"
  />
</template>
