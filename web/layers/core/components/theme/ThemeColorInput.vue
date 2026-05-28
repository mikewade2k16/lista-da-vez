<script setup lang="ts">
import ThemeColorPicker from './ThemeColorPicker.vue'

const props = withDefaults(
  defineProps<{
    modelValue?: string
    allowGradient?: boolean
  }>(),
  {
    modelValue: '',
    allowGradient: false,
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const pickerOpen = ref(false)
const rawValue = ref('')
const internalUpdateInFlight = ref(false)

watch(
  () => props.modelValue,
  (value) => {
    const nextValue = toStringValue(value)
    if (nextValue === rawValue.value) {
      return
    }

    if (internalUpdateInFlight.value) {
      rawValue.value = nextValue
      return
    }

    rawValue.value = nextValue
  },
  { immediate: true },
)

function toStringValue(value: unknown) {
  if (typeof value === 'string') return value
  if (typeof value === 'number') return String(value)
  return ''
}

function emitValue(nextValue: string) {
  if (nextValue === rawValue.value) {
    return
  }

  internalUpdateInFlight.value = true
  rawValue.value = nextValue
  emit('update:modelValue', nextValue)

  queueMicrotask(() => {
    internalUpdateInFlight.value = false
  })
}

function onRawValueInput(value: string | number | undefined) {
  emitValue(toStringValue(value))
}

function onPickerApply(value: string) {
  emitValue(value)
}
</script>

<template>
  <div class="space-y-1">
    <div class="flex items-center gap-2">
      <ThemeColorPicker
        v-model:open="pickerOpen"
        :value="rawValue"
        :allow-gradient="allowGradient"
        @apply="onPickerApply"
      />

      <UInput
        :model-value="rawValue"
        placeholder="Valor CSS da cor"
        class="w-full"
        @update:model-value="onRawValueInput"
      />
    </div>
  </div>
</template>
