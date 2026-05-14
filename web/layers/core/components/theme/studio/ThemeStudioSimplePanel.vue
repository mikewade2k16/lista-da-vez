<script setup lang="ts">
import ThemeColorInput from '../ThemeColorInput.vue'
import type { FontItem, ThemeStudioHeaderState, ThemeStudioPageHeaderState } from '../../../composables/useThemeStudio'

const props = defineProps<{
  fontItems: FontItem[]
  fontValue: string
  radiusKeys: readonly string[]
  radiusValueMap: Record<string, number>
  shadowColor: string
  shadowGlowColor: string
  accentValues: {
    primary: string
    primary600: string
    success: string
    danger: string
    ring: string
  }
  headerValues: {
    brandBg: string
    panelBg: string
    brandBlur: number
    panelBlur: number
    border: string
    separator: string
    text: string
    hover: string
    active: string
    shellShadow: string
    fadeTop: string
    fadeBottom: string
    fadeTopSize: number
    fadeBottomSize: number
  }
  headerState: ThemeStudioHeaderState
  pageHeaderState: ThemeStudioPageHeaderState
  headerTextLinkValue: string
}>()

const emit = defineEmits<{
  'update-font': [value: string | number | undefined]
  'update-radius': [payload: { key: string, value: string | number | undefined }]
  'update-shadow-color': [value: string | number | undefined]
  'update-shadow-glow-color': [value: string | number | undefined]
  'update-accent': [payload: { key: string, value: string | number | undefined }]
  'update-header-brand-bg': [value: string | number | undefined]
  'update-header-panel-bg': [value: string | number | undefined]
  'toggle-header-backgrounds': [value: boolean]
  'update-header-brand-blur': [value: string | number | undefined]
  'update-header-panel-blur': [value: string | number | undefined]
  'toggle-header-blurs': [value: boolean]
  'update-header-border': [value: string | number | undefined]
  'update-header-separator': [value: string | number | undefined]
  'toggle-header-dividers': [value: boolean]
  'toggle-header-text-follow': [value: boolean]
  'update-header-text': [value: string | number | undefined]
  'toggle-header-primary-actions': [value: boolean]
  'toggle-header-interactions': [value: boolean]
  'update-header-hover': [value: string | number | undefined]
  'update-header-active': [value: string | number | undefined]
  'update-header-shell-shadow': [value: string | number | undefined]
  'update-header-fade-top': [value: string | number | undefined]
  'update-header-fade-bottom': [value: string | number | undefined]
  'update-header-fade-top-size': [value: string | number | undefined]
  'update-header-fade-bottom-size': [value: string | number | undefined]
  'toggle-page-header-eyebrow': [value: boolean]
  'toggle-page-header-title': [value: boolean]
  'toggle-page-header-description': [value: boolean]
  'disable-page-header-all': []
}>()

function toBool(value: boolean | 'indeterminate') {
  return value === true
}
</script>

<template>
  <div class="grid gap-4 xl:grid-cols-5">
    <article class="rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface))] p-4">
      <h2 class="text-sm font-semibold uppercase tracking-wide text-[rgb(var(--muted))]">Typography</h2>
      <div class="mt-3 space-y-3">
        <USelect :model-value="props.fontValue" :items="props.fontItems" class="w-full"
          @update:model-value="emit('update-font', $event)" />
      
        <h2 class="text-sm font-semibold uppercase tracking-wide text-[rgb(var(--muted))] pt-3">Radius</h2>
        <div class="mt-3 space-y-3">
          <div v-for="key in props.radiusKeys" :key="key"
            class="grid gap-2 sm:grid-cols-[90px_1fr_auto] sm:items-center">
            <p class="text-xs font-medium uppercase tracking-wide text-[rgb(var(--muted))]">{{ key }}</p>
            <UInput :model-value="String(props.radiusValueMap[key] ?? 0)" type="number" min="0" max="64"
              @update:model-value="emit('update-radius', { key, value: $event })" />
            <div class="h-10 w-10 border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))]"
              :style="{ borderRadius: `${props.radiusValueMap[key] ?? 0}px` }" />
          </div>
        </div>
      </div>
    </article>

    <article class="rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface))] p-4">
      <h2 class="text-sm font-semibold uppercase tracking-wide text-[rgb(var(--muted))]">Page Headers</h2>
      <div class="mt-3 space-y-3">
        <div class="flex items-center justify-between gap-2 rounded-[var(--radius-sm)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] px-3 py-2">
          <p class="text-xs font-medium uppercase tracking-wide text-[rgb(var(--muted))]">Eyebrow</p>
          <USwitch
            :model-value="props.pageHeaderState.showEyebrow"
            @update:model-value="emit('toggle-page-header-eyebrow', toBool($event))"
          />
        </div>

        <div class="flex items-center justify-between gap-2 rounded-[var(--radius-sm)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] px-3 py-2">
          <p class="text-xs font-medium uppercase tracking-wide text-[rgb(var(--muted))]">Title</p>
          <USwitch
            :model-value="props.pageHeaderState.showTitle"
            @update:model-value="emit('toggle-page-header-title', toBool($event))"
          />
        </div>

        <div class="flex items-center justify-between gap-2 rounded-[var(--radius-sm)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] px-3 py-2">
          <p class="text-xs font-medium uppercase tracking-wide text-[rgb(var(--muted))]">Description</p>
          <USwitch
            :model-value="props.pageHeaderState.showDescription"
            @update:model-value="emit('toggle-page-header-description', toBool($event))"
          />
        </div>

        <p class="text-[11px] text-[rgb(var(--muted))]">
          Controle global do cabecalho das paginas admin (eyebrow, titulo e descricao).
        </p>
        <UButton
          icon="i-lucide-eye-off"
          label="Desativar tudo"
          color="neutral"
          variant="soft"
          size="sm"
          @click="emit('disable-page-header-all')"
        />
      </div>
    </article>

    <article class="rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface))] p-4">
      <h2 class="text-sm font-semibold uppercase tracking-wide text-[rgb(var(--muted))]">Header</h2>
      <div class="mt-3 space-y-3">
        <div class="space-y-2">
          <div class="flex items-center justify-between gap-2">
            <p class="text-xs font-medium uppercase tracking-wide text-[rgb(var(--muted))]">Background</p>
            <div class="flex items-center gap-2">
              <span class="text-xs text-[rgb(var(--muted))]">Link Brand/Panel</span>
              <USwitch :model-value="props.headerState.linkBackgrounds"
                @update:model-value="emit('toggle-header-backgrounds', toBool($event))" />
            </div>
          </div>
          <ThemeColorInput :model-value="props.headerValues.brandBg" allow-gradient
            @update:model-value="emit('update-header-brand-bg', $event)" />
          <ThemeColorInput v-if="!props.headerState.linkBackgrounds" :model-value="props.headerValues.panelBg"
            allow-gradient @update:model-value="emit('update-header-panel-bg', $event)" />
        </div>

        <div class="space-y-2">
          <div class="flex items-center justify-between gap-2">
            <p class="text-xs font-medium uppercase tracking-wide text-[rgb(var(--muted))]">Blur</p>
            <div class="flex items-center gap-2">
              <span class="text-xs text-[rgb(var(--muted))]">Link Brand/Panel</span>
              <USwitch :model-value="props.headerState.linkBlurs"
                @update:model-value="emit('toggle-header-blurs', toBool($event))" />
            </div>
          </div>

          <div class="grid gap-2 sm:grid-cols-2">
            <div class="space-y-1">
              <p class="text-[11px] uppercase tracking-wide text-[rgb(var(--muted))]">Brand Blur (px)</p>
              <UInput :model-value="String(props.headerValues.brandBlur ?? 0)" type="number" min="0" max="120"
                @update:model-value="emit('update-header-brand-blur', $event)" />
            </div>

            <div v-if="!props.headerState.linkBlurs" class="space-y-1">
              <p class="text-[11px] uppercase tracking-wide text-[rgb(var(--muted))]">Panel Blur (px)</p>
              <UInput :model-value="String(props.headerValues.panelBlur ?? 0)" type="number" min="0" max="120"
                @update:model-value="emit('update-header-panel-blur', $event)" />
            </div>
          </div>
          <p class="text-[11px] text-[rgb(var(--muted))]">Use 0 para desativar blur.</p>
        </div>

        <div class="space-y-2">
          <div class="flex items-center justify-between gap-2">
            <p class="text-xs font-medium uppercase tracking-wide text-[rgb(var(--muted))]">Border/Separator</p>
            <div class="flex items-center gap-2">
              <span class="text-xs text-[rgb(var(--muted))]">Link</span>
              <USwitch :model-value="props.headerState.linkDividers"
                @update:model-value="emit('toggle-header-dividers', toBool($event))" />
            </div>
          </div>
          <ThemeColorInput :model-value="props.headerValues.border"
            @update:model-value="emit('update-header-border', $event)" />
          <ThemeColorInput v-if="!props.headerState.linkDividers" :model-value="props.headerValues.separator"
            @update:model-value="emit('update-header-separator', $event)" />
        </div>

        <div class="space-y-2">
          <div class="flex items-center justify-between gap-2">
            <p class="text-xs font-medium uppercase tracking-wide text-[rgb(var(--muted))]">Text</p>
            <div class="flex items-center gap-2">
              <span class="text-xs text-[rgb(var(--muted))]">Follow theme text</span>
              <USwitch :model-value="props.headerState.textFollowTheme"
                @update:model-value="emit('toggle-header-text-follow', toBool($event))" />
            </div>
          </div>
          <ThemeColorInput v-if="!props.headerState.textFollowTheme" :model-value="props.headerValues.text"
            @update:model-value="emit('update-header-text', $event)" />
          <p v-else class="text-xs text-[rgb(var(--muted))]">Usando automaticamente:
            <code>{{ props.headerTextLinkValue }}</code>
          </p>
        </div>

        <div class="space-y-2">
          <div class="flex items-center justify-between gap-2">
            <p class="text-xs font-medium uppercase tracking-wide text-[rgb(var(--muted))]">Hover/Active</p>
            <div class="flex items-center gap-2">
              <span class="text-xs text-[rgb(var(--muted))]">Use primary</span>
              <USwitch :model-value="props.headerState.actionFollowPrimary"
                @update:model-value="emit('toggle-header-primary-actions', toBool($event))" />
            </div>
          </div>
          <div class="flex items-center justify-end gap-2">
            <span class="text-xs text-[rgb(var(--muted))]">Link hover/active</span>
            <USwitch :model-value="props.headerState.linkInteractions"
              @update:model-value="emit('toggle-header-interactions', toBool($event))" />
          </div>
          <ThemeColorInput :model-value="props.headerValues.hover"
            @update:model-value="emit('update-header-hover', $event)" />
          <ThemeColorInput v-if="!props.headerState.linkInteractions" :model-value="props.headerValues.active"
            @update:model-value="emit('update-header-active', $event)" />
        </div>

        <div class="space-y-2">
          <p class="text-xs font-medium uppercase tracking-wide text-[rgb(var(--muted))]">Header FX</p>

          <div class="space-y-1">
            <p class="text-[11px] uppercase tracking-wide text-[rgb(var(--muted))]">Box Shadow (header inteiro)</p>
            <UInput :model-value="props.headerValues.shellShadow" placeholder="none ou 0 10px 24px rgba(...)"
              @update:model-value="emit('update-header-shell-shadow', $event)" />
          </div>

          <div class="grid gap-2 sm:grid-cols-2">
            <div class="space-y-1">
              <p class="text-[11px] uppercase tracking-wide text-[rgb(var(--muted))]">Fade Top Size (px)</p>
              <UInput :model-value="String(props.headerValues.fadeTopSize ?? 0)" type="number" min="0" max="80"
                @update:model-value="emit('update-header-fade-top-size', $event)" />
            </div>
            <div class="space-y-1">
              <p class="text-[11px] uppercase tracking-wide text-[rgb(var(--muted))]">Fade Bottom Size (px)</p>
              <UInput :model-value="String(props.headerValues.fadeBottomSize ?? 0)" type="number" min="0" max="80"
                @update:model-value="emit('update-header-fade-bottom-size', $event)" />
            </div>
          </div>

          <div class="space-y-1">
            <p class="text-[11px] uppercase tracking-wide text-[rgb(var(--muted))]">Before (top gradient)</p>
            <ThemeColorInput :model-value="props.headerValues.fadeTop" allow-gradient
              @update:model-value="emit('update-header-fade-top', $event)" />
          </div>

          <div class="space-y-1">
            <p class="text-[11px] uppercase tracking-wide text-[rgb(var(--muted))]">After (bottom gradient)</p>
            <ThemeColorInput :model-value="props.headerValues.fadeBottom" allow-gradient
              @update:model-value="emit('update-header-fade-bottom', $event)" />
          </div>
        </div>
      </div>
    </article>

    <article class="rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface))] p-4">
      <h2 class="text-sm font-semibold uppercase tracking-wide text-[rgb(var(--muted))]">Shadow</h2>
      <div class="mt-3 space-y-3">
        <div class="space-y-2">
          <p class="text-xs font-medium uppercase tracking-wide text-[rgb(var(--muted))]">Shadow Base Color</p>
          <ThemeColorInput :model-value="props.shadowColor" @update:model-value="emit('update-shadow-color', $event)" />
        </div>
        <div class="space-y-2">
          <p class="text-xs font-medium uppercase tracking-wide text-[rgb(var(--muted))]">Shadow Glow Color</p>
          <ThemeColorInput :model-value="props.shadowGlowColor"
            @update:model-value="emit('update-shadow-glow-color', $event)" />
        </div>
      </div>
    </article>

    <article class="rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface))] p-4">
      <h2 class="text-sm font-semibold uppercase tracking-wide text-[rgb(var(--muted))]">Acentos</h2>
      <div class="mt-3 space-y-3">
        <div class="space-y-2">
          <p class="text-xs font-medium uppercase tracking-wide text-[rgb(var(--muted))]">Primary</p>
          <ThemeColorInput
            :model-value="props.accentValues.primary"
            @update:model-value="emit('update-accent', { key: 'primary', value: $event })"
          />
        </div>
        <div class="space-y-2">
          <p class="text-xs font-medium uppercase tracking-wide text-[rgb(var(--muted))]">Primary 600</p>
          <ThemeColorInput
            :model-value="props.accentValues.primary600"
            @update:model-value="emit('update-accent', { key: 'primary-600', value: $event })"
          />
        </div>
        <div class="space-y-2">
          <p class="text-xs font-medium uppercase tracking-wide text-[rgb(var(--muted))]">Success</p>
          <ThemeColorInput
            :model-value="props.accentValues.success"
            @update:model-value="emit('update-accent', { key: 'success', value: $event })"
          />
        </div>
        <div class="space-y-2">
          <p class="text-xs font-medium uppercase tracking-wide text-[rgb(var(--muted))]">Danger</p>
          <ThemeColorInput
            :model-value="props.accentValues.danger"
            @update:model-value="emit('update-accent', { key: 'danger', value: $event })"
          />
        </div>
        <div class="space-y-2">
          <p class="text-xs font-medium uppercase tracking-wide text-[rgb(var(--muted))]">Ring</p>
          <ThemeColorInput
            :model-value="props.accentValues.ring"
            @update:model-value="emit('update-accent', { key: 'ring', value: $event })"
          />
        </div>
      </div>
    </article>


  </div>
</template>
