<script setup lang="ts">
import { computed } from 'vue'
import { useThemeColorPicker } from '~/composables/useThemeColorPicker'
import ThemeGradientColorStop from './ThemeGradientColorStop.vue'

const props = withDefaults(
  defineProps<{
    open?: boolean
    value?: string
    allowGradient?: boolean
  }>(),
  {
    open: false,
    value: '',
    allowGradient: false,
  },
)

const emit = defineEmits<{
  'update:open': [value: boolean]
  apply: [value: string]
}>()

const pickerOpen = computed({
  get: () => props.open,
  set: (value) => emit('update:open', value),
})

const {
  currentValue,
  gradientEnabled,
  gradientType,
  gradientTypeItems,
  gradientAngle,
  solidHex,
  solidHexInput,
  solidAlphaEnabled,
  solidAlpha,
  gradientStartHex,
  gradientStartHexInput,
  gradientStartAlphaEnabled,
  gradientStartAlpha,
  gradientEndHex,
  gradientEndHexInput,
  gradientEndAlphaEnabled,
  gradientEndAlpha,
  screenPickerSupported,
  previewBackground,
  onFinalValueInput,
  onSolidPickerChange,
  onSolidHexInput,
  onSolidAlphaToggle,
  onSolidAlphaSlider,
  onSolidAlphaInput,
  onGradientToggle,
  onGradientTypeChange,
  onGradientAngleSlider,
  onGradientAngleInput,
  onGradientColorPickerChange,
  onGradientHexInput,
  onGradientAlphaToggle,
  onGradientAlphaSlider,
  onGradientAlphaInput,
  pickScreenColor,
} = useThemeColorPicker({
  value: computed(() => props.value),
  allowGradient: computed(() => props.allowGradient),
  apply: (value) => emit('apply', value),
})
</script>

<template>
  <UPopover v-model:open="pickerOpen" :content="{ align: 'start', side: 'bottom' }">
    <UButton color="neutral" variant="outline" class="h-8 px-2">
      <template #leading>
        <span
          class="size-4 rounded border border-[rgb(var(--border))]"
          :style="{ background: previewBackground }"
        ></span>
      </template>
      <span class="hidden text-xs sm:inline">Cor</span>
      <template #trailing>
        <UIcon name="i-lucide-pipette" class="size-4" />
      </template>
    </UButton>

    <template #content>
      <div
        class="space-y-3 p-3"
        :class="gradientEnabled ? 'w-[440px] max-w-[96vw]' : 'w-[320px] max-w-[92vw]'"
      >
        <div v-if="allowGradient" class="flex items-center justify-between gap-2">
          <div class="flex items-center gap-2">
            <USwitch :model-value="gradientEnabled" @update:model-value="onGradientToggle" />
            <span class="text-xs font-medium text-[rgb(var(--muted))]">Usar gradiente</span>
          </div>
          <span class="text-xs text-[rgb(var(--muted))]">
            {{ gradientEnabled ? 'Gradiente' : 'Cor solida' }}
          </span>
        </div>

        <div v-if="!gradientEnabled" class="space-y-2">
          <div class="flex items-center gap-2">
            <UColorPicker
              :model-value="solidHex"
              size="sm"
              @update:model-value="onSolidPickerChange"
            />
            <UInput
              :model-value="solidHexInput"
              placeholder="#0A84FF"
              class="w-full"
              @update:model-value="onSolidHexInput"
            />
            <UButton
              v-if="screenPickerSupported"
              icon="i-lucide-pipette"
              color="neutral"
              variant="outline"
              size="sm"
              @click="pickScreenColor('solid')"
            />
          </div>

          <div
            class="flex items-center gap-2 rounded-[var(--radius-sm)] border border-[rgb(var(--border))] px-2 py-1"
          >
            <USwitch :model-value="solidAlphaEnabled" @update:model-value="onSolidAlphaToggle" />
            <span class="text-xs text-[rgb(var(--muted))]">Transparencia</span>
          </div>

          <div v-if="solidAlphaEnabled" class="flex items-center gap-2">
            <USlider
              :model-value="solidAlpha"
              :min="0"
              :max="100"
              :step="1"
              class="w-full"
              @update:model-value="onSolidAlphaSlider"
            />
            <UInput
              :model-value="String(solidAlpha)"
              type="number"
              min="0"
              max="100"
              class="w-20"
              @update:model-value="onSolidAlphaInput"
            />
          </div>
        </div>

        <div v-else class="space-y-3">
          <div class="grid gap-2 sm:grid-cols-[140px_1fr] sm:items-center">
            <USelect
              :model-value="gradientType"
              :items="gradientTypeItems"
              class="w-full"
              @update:model-value="onGradientTypeChange"
            />

            <div v-if="gradientType !== 'radial'" class="flex items-center gap-2">
              <USlider
                :model-value="gradientAngle"
                :min="0"
                :max="360"
                :step="1"
                class="w-full"
                @update:model-value="onGradientAngleSlider"
              />
              <UInput
                :model-value="String(Math.round(gradientAngle))"
                type="number"
                min="0"
                max="360"
                class="w-14"
                @update:model-value="onGradientAngleInput"
              />
            </div>
          </div>

          <div class="grid gap-2 md:grid-cols-2">
            <ThemeGradientColorStop
              title="Cor 1"
              :hex="gradientStartHex"
              :hex-input="gradientStartHexInput"
              :alpha-enabled="gradientStartAlphaEnabled"
              :alpha="gradientStartAlpha"
              placeholder="#0A84FF"
              :screen-picker-supported="screenPickerSupported"
              @picker-change="onGradientColorPickerChange('start', $event)"
              @hex-input="onGradientHexInput('start', $event)"
              @alpha-toggle="onGradientAlphaToggle('start', $event)"
              @alpha-slider="onGradientAlphaSlider('start', $event)"
              @alpha-input="onGradientAlphaInput('start', $event)"
              @pick-screen="pickScreenColor('start')"
            />

            <ThemeGradientColorStop
              title="Cor 2"
              :hex="gradientEndHex"
              :hex-input="gradientEndHexInput"
              :alpha-enabled="gradientEndAlphaEnabled"
              :alpha="gradientEndAlpha"
              placeholder="#3B82F6"
              :screen-picker-supported="screenPickerSupported"
              @picker-change="onGradientColorPickerChange('end', $event)"
              @hex-input="onGradientHexInput('end', $event)"
              @alpha-toggle="onGradientAlphaToggle('end', $event)"
              @alpha-slider="onGradientAlphaSlider('end', $event)"
              @alpha-input="onGradientAlphaInput('end', $event)"
              @pick-screen="pickScreenColor('end')"
            />
          </div>
        </div>

        <div class="space-y-1">
          <p class="text-[11px] text-[rgb(var(--muted))]">Valor CSS final</p>
          <UInput
            :model-value="currentValue"
            placeholder="Ex: #0A84FF, rgba(...), linear-gradient(...)"
            @update:model-value="onFinalValueInput"
          />
        </div>
      </div>
    </template>
  </UPopover>
</template>
