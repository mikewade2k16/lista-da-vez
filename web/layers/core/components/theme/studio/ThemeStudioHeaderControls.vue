<script setup lang="ts">
import type { OmniThemeName } from '../../../composables/useOmniTheme'

const props = defineProps<{
  themeOrder: OmniThemeName[]
  themeLabels: Record<OmniThemeName, string>
  selectedTheme: OmniThemeName
  hasCustomTheme: boolean
  customThemeNameDraft: string
  showDetailedEditors: boolean
  search: string
}>()

const emit = defineEmits<{
  'update:selectedTheme': [value: OmniThemeName]
  'apply-selected-theme': []
  'reset-selected-theme': []
  'update:customThemeNameDraft': [value: string]
  'save-custom-theme-name': []
  'create-custom-from-selected': []
  'update:showDetailedEditors': [value: boolean]
  'update:search': [value: string]
}>()

function onDetailedToggle(value: boolean | 'indeterminate') {
  emit('update:showDetailedEditors', value === true)
}
</script>

<template>
  <div class="rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] p-4">
    <div class="flex flex-col gap-3 xl:flex-row xl:items-start xl:justify-between">
      <div class="space-y-3">
        <div class="flex flex-wrap gap-2">
          <button
            v-for="theme in props.themeOrder"
            :key="theme"
            type="button"
            class="rounded-[var(--radius-sm)] border px-3 py-2 text-sm font-medium transition"
            :class="
              props.selectedTheme === theme
                ? 'border-[rgb(var(--primary))] bg-[rgb(var(--primary))] text-white'
                : 'border-[rgb(var(--border))] bg-[rgb(var(--surface))] text-[rgb(var(--text))] hover:bg-[rgb(var(--surface-2))]'
            "
            :disabled="theme === 'custom' && !props.hasCustomTheme"
            @click="emit('update:selectedTheme', theme)"
          >
            {{ props.themeLabels[theme] }}
          </button>
        </div>

        <div class="flex flex-wrap gap-2">
          <UButton icon="i-lucide-check" label="Aplicar tema" color="primary" variant="solid" @click="emit('apply-selected-theme')" />
          <UButton icon="i-lucide-rotate-ccw" label="Resetar tema" color="neutral" variant="outline" @click="emit('reset-selected-theme')" />
        </div>
      </div>

      <div class="w-full max-w-xl space-y-3">
        <div class="grid gap-2 sm:grid-cols-[1fr_auto_auto]">
          <UInput
            :model-value="props.customThemeNameDraft"
            icon="i-lucide-tag"
            placeholder="Nome do tema custom"
            @update:model-value="emit('update:customThemeNameDraft', String($event ?? ''))"
          />
          <UButton icon="i-lucide-save" label="Salvar nome" color="neutral" variant="outline" @click="emit('save-custom-theme-name')" />
          <UButton icon="i-lucide-copy-plus" label="Criar custom" color="primary" variant="soft" @click="emit('create-custom-from-selected')" />
        </div>

        <div class="flex flex-wrap items-center justify-between gap-2 rounded-[var(--radius-sm)] border border-[rgb(var(--border))] bg-[rgb(var(--surface))] px-3 py-2">
          <span class="text-sm text-[rgb(var(--text))]">Modo detalhado (tokens individuais)</span>
          <USwitch :model-value="props.showDetailedEditors" @update:model-value="onDetailedToggle" />
        </div>
      </div>
    </div>

    <div class="mt-3">
      <p class="mb-2 text-xs text-[rgb(var(--muted))]">
        Alteracoes sao salvas automaticamente e aplicadas no painel em tempo real.
      </p>
      <UInput
        :model-value="props.search"
        icon="i-lucide-search"
        placeholder="Buscar variavel por nome/chave..."
        @update:model-value="emit('update:search', String($event ?? ''))"
      />
    </div>
  </div>
</template>
