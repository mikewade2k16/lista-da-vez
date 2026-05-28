<script setup lang="ts">
defineProps<{
  title: string
  hex: string
  hexInput: string
  alphaEnabled: boolean
  alpha: number
  placeholder: string
  screenPickerSupported: boolean
}>()

const emit = defineEmits<{
  'picker-change': [value: string | undefined]
  'hex-input': [value: string | number | undefined]
  'alpha-toggle': [value: boolean | 'indeterminate']
  'alpha-slider': [value: number | number[] | undefined]
  'alpha-input': [value: string | number | undefined]
  'pick-screen': []
}>()
</script>

<template>
  <div
    class="space-y-2 rounded-[var(--radius-sm)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] p-1.5"
  >
    <p class="text-xs font-medium text-[rgb(var(--muted))]">{{ title }}</p>
    <div class="flex justify-start">
      <UColorPicker
        :model-value="hex"
        size="xs"
        @update:model-value="emit('picker-change', $event)"
      />
    </div>
    <div class="flex items-center gap-2">
      <UInput
        :model-value="hexInput"
        :placeholder="placeholder"
        class="w-full min-w-0"
        @update:model-value="emit('hex-input', $event)"
      />
      <UButton
        v-if="screenPickerSupported"
        icon="i-lucide-pipette"
        color="neutral"
        variant="outline"
        size="sm"
        @click="emit('pick-screen')"
      />
    </div>

    <div
      class="flex items-center gap-2 rounded-[var(--radius-sm)] border border-[rgb(var(--border))] px-2 py-1"
    >
      <USwitch :model-value="alphaEnabled" @update:model-value="emit('alpha-toggle', $event)" />
      <span class="text-xs text-[rgb(var(--muted))]">Transparencia</span>
    </div>

    <div v-if="alphaEnabled" class="flex items-center gap-2">
      <USlider
        :model-value="alpha"
        :min="0"
        :max="100"
        :step="1"
        class="w-full"
        @update:model-value="emit('alpha-slider', $event)"
      />
      <UInput
        :model-value="String(alpha)"
        type="number"
        min="0"
        max="100"
        class="w-10"
        @update:model-value="emit('alpha-input', $event)"
      />
    </div>
  </div>
</template>
