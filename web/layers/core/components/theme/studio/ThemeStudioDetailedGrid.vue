<script setup lang="ts">
import ThemeColorInput from '../ThemeColorInput.vue'
import type { FontItem } from '../../../composables/useThemeStudio'
import type { OmniThemeVariable } from '../../../composables/useOmniTheme'

interface ThemeSection {
  key: string
  items: OmniThemeVariable[]
}

const props = defineProps<{
  sections: ThemeSection[]
  fontItems: FontItem[]
  groupLabelBy: (group: string) => string
  variableValueMap: Record<string, string>
  radiusValueMap: Record<string, number>
  tripletModelValueMap: Record<string, string>
  radiusKeys: readonly string[]
  shadowColorKeys: readonly string[]
}>()

const emit = defineEmits<{
  'update-font': [value: string | number | undefined]
  'update-radius': [payload: { key: string, value: string | number | undefined }]
  'update-css': [payload: { key: string, value: string | number | undefined }]
  'update-triplet': [payload: { key: string, value: string | number | undefined }]
  'update-text': [payload: { key: string, value: string | number | undefined }]
}>()

function isRadiusKey(key: string) {
  return props.radiusKeys.includes(key)
}

function isShadowColorKey(key: string) {
  return props.shadowColorKeys.includes(key)
}
</script>

<template>
  <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
    <article
      v-for="section in props.sections"
      :key="section.key"
      class="rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface))] p-4"
    >
      <h2 class="text-sm font-semibold uppercase tracking-wide text-[rgb(var(--muted))]">{{ props.groupLabelBy(section.key) }}</h2>

      <div class="mt-3 space-y-3">
        <div
          v-for="variable in section.items"
          :key="variable.key"
          class="rounded-[var(--radius-sm)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] p-3"
        >
          <div class="mb-2 flex items-center justify-between gap-2">
            <p class="text-sm font-semibold text-[rgb(var(--text))]">{{ variable.label }}</p>
            <code class="text-xs text-[rgb(var(--muted))]">--{{ variable.key }}</code>
          </div>

          <div v-if="variable.key === 'font-sans'" class="space-y-2">
            <USelect
              :model-value="props.variableValueMap['font-sans']"
              :items="props.fontItems"
              class="w-full"
              @update:model-value="emit('update-font', $event)"
            />
            <p class="text-sm" :style="{ fontFamily: props.variableValueMap['font-sans'] }">
              The quick brown fox jumps over the lazy dog.
            </p>
          </div>

          <div v-else-if="isRadiusKey(variable.key)" class="grid gap-2 sm:grid-cols-[1fr_auto] sm:items-center">
            <UInput
              :model-value="String(props.radiusValueMap[variable.key] ?? 0)"
              type="number"
              min="0"
              max="64"
              @update:model-value="emit('update-radius', { key: variable.key, value: $event })"
            />
            <div
              class="h-10 w-10 border border-[rgb(var(--border))] bg-[rgb(var(--surface))]"
              :style="{ borderRadius: `${props.radiusValueMap[variable.key] ?? 0}px` }"
            />
          </div>

          <ThemeColorInput
            v-else-if="isShadowColorKey(variable.key)"
            :model-value="props.variableValueMap[variable.key] ?? ''"
            @update:model-value="emit('update-css', { key: variable.key, value: $event })"
          />

          <ThemeColorInput
            v-else-if="variable.kind === 'rgb-triplet'"
            :model-value="props.tripletModelValueMap[variable.key] ?? ''"
            @update:model-value="emit('update-triplet', { key: variable.key, value: $event })"
          />

          <ThemeColorInput
            v-else-if="variable.kind === 'css-color' || variable.kind === 'css-gradient'"
            :model-value="props.variableValueMap[variable.key] ?? ''"
            :allow-gradient="variable.kind === 'css-gradient'"
            @update:model-value="emit('update-css', { key: variable.key, value: $event })"
          />

          <UInput
            v-else
            :model-value="props.variableValueMap[variable.key] ?? ''"
            placeholder="Digite o valor da variavel"
            @update:model-value="emit('update-text', { key: variable.key, value: $event })"
          />
        </div>
      </div>
    </article>
  </div>
</template>
