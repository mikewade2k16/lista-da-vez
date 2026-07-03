<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { normalizeAppRole } from '~/domain/utils/permissions'

type SwitcherTheme = 'light' | 'dark' | 'liquidglass'

const USER_THEME_STORAGE_KEY = 'omni-ui-user-theme'
const THEME_OPTIONS: Array<{ value: SwitcherTheme; icon: string }> = [
  { value: 'light', icon: 'i-lucide-sun' },
  { value: 'dark', icon: 'i-lucide-moon' },
  { value: 'liquidglass', icon: 'i-lucide-sparkles' },
]
const THEME_ICONS: Record<string, string> = {
  light: 'i-lucide-sun',
  dark: 'i-lucide-moon',
  liquidglass: 'i-lucide-sparkles',
  apple: 'i-lucide-palette',
  custom: 'i-lucide-palette',
}

const auth = useAuthStore()
const { role, allowedWorkspaces } = storeToRefs(auth)
const { currentTheme, initializeFromStorage, applyTheme, getThemeLabel } = useOmniTheme()

const canOpenThemeStudio = computed(
  () =>
    normalizeAppRole(role.value) === 'platform_admin' || allowedWorkspaces.value.includes('themes'),
)

const themeButton = computed(() => ({
  icon: THEME_ICONS[currentTheme.value] || 'i-lucide-palette',
  label: getThemeLabel(currentTheme.value),
}))

const themeItems = computed(() =>
  THEME_OPTIONS.map((option) => ({
    ...option,
    label: getThemeLabel(option.value),
  })),
)

function selectTheme(value: SwitcherTheme) {
  // applyTheme com persist: platform_admin grava o tema GLOBAL da plataforma;
  // usuário comum grava local. Para os temas-base (light/dark) também guardamos a
  // preferência pessoal, que sobrepõe o tema global base no próximo load.
  applyTheme(value, true)
  if (import.meta.client && (value === 'light' || value === 'dark')) {
    window.localStorage.setItem(USER_THEME_STORAGE_KEY, value)
  }
}

onMounted(() => {
  initializeFromStorage()
})
</script>

<template>
  <UPopover :content="{ side: 'bottom', align: 'end' }">
    <UButton
      :icon="themeButton.icon"
      color="neutral"
      variant="ghost"
      size="sm"
      :aria-label="`Tema: ${themeButton.label}`"
      title="Tema"
    />

    <template #content>
      <div class="dashboard-theme-switcher">
        <button
          v-for="item in themeItems"
          :key="item.value"
          class="dashboard-theme-switcher__item"
          :class="{ 'is-active': currentTheme === item.value }"
          type="button"
          @click="selectTheme(item.value)"
        >
          <UIcon :name="item.icon" class="size-4" aria-hidden="true" />
          <span>{{ item.label }}</span>
          <UIcon
            v-if="currentTheme === item.value"
            name="i-lucide-check"
            class="dashboard-theme-switcher__check"
            aria-hidden="true"
          />
        </button>

        <NuxtLink v-if="canOpenThemeStudio" class="dashboard-theme-switcher__studio" to="/themes">
          <UIcon name="i-lucide-palette" class="size-4" aria-hidden="true" />
          <span>Theme Studio</span>
        </NuxtLink>
      </div>
    </template>
  </UPopover>
</template>

<style scoped>
.dashboard-theme-switcher {
  min-width: 12.5rem;
  display: grid;
  gap: 0.25rem;
  padding: 0.35rem;
  border: 1px solid var(--admin-header-border);
  border-radius: var(--radius-sm);
  background: var(--admin-header-panel-bg);
  box-shadow: var(--shadow-md);
  color: var(--admin-header-text);
  backdrop-filter: blur(var(--admin-header-panel-blur));
}

.dashboard-theme-switcher__item,
.dashboard-theme-switcher__studio {
  min-height: 2.35rem;
  display: flex;
  align-items: center;
  gap: 0.55rem;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  padding: 0 0.7rem;
  background: transparent;
  color: var(--admin-header-text);
  font-size: 0.82rem;
  font-weight: 750;
  text-align: left;
  text-decoration: none;
  cursor: pointer;
}

.dashboard-theme-switcher__item:hover,
.dashboard-theme-switcher__item.is-active,
.dashboard-theme-switcher__studio:hover {
  border-color: rgb(var(--ring) / 0.2);
  background: var(--admin-header-hover-bg);
}

.dashboard-theme-switcher__check {
  margin-left: auto;
  color: rgb(var(--primary));
}

.dashboard-theme-switcher__studio {
  margin-top: 0.2rem;
  border-top-color: var(--admin-header-separator);
  color: rgb(var(--primary));
}
</style>
