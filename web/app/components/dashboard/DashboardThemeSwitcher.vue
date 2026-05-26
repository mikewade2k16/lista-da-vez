<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { normalizeAppRole } from '~/domain/utils/permissions'

type HeaderTheme = 'light' | 'dark'

const USER_THEME_STORAGE_KEY = 'omni-ui-user-theme'
const USER_THEME_OPTIONS: Array<{ value: HeaderTheme; icon: string }> = [
  { value: 'light', icon: 'i-lucide-sun' },
  { value: 'dark', icon: 'i-lucide-moon' },
]

const auth = useAuthStore()
const { role, allowedWorkspaces } = storeToRefs(auth)
const { currentTheme, initializeFromStorage, applyTheme, getThemeLabel } = useOmniTheme()

const canOpenThemeStudio = computed(
  () =>
    normalizeAppRole(role.value) === 'platform_admin' || allowedWorkspaces.value.includes('themes'),
)

const activeHeaderTheme = computed<HeaderTheme>(() =>
  currentTheme.value === 'dark' ? 'dark' : 'light',
)

const themeButton = computed(() => {
  if (activeHeaderTheme.value === 'dark') {
    return { icon: 'i-lucide-moon', label: getThemeLabel('dark') }
  }

  return { icon: 'i-lucide-sun', label: getThemeLabel('light') }
})

const themeItems = computed(() =>
  USER_THEME_OPTIONS.map((option) => ({
    ...option,
    label: getThemeLabel(option.value),
  })),
)

function selectTheme(value: HeaderTheme) {
  if (value !== 'light' && value !== 'dark') {
    return
  }

  applyTheme(value, false)

  if (import.meta.client) {
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
          :class="{ 'is-active': activeHeaderTheme === item.value }"
          type="button"
          @click="selectTheme(item.value)"
        >
          <UIcon :name="item.icon" class="size-4" aria-hidden="true" />
          <span>{{ item.label }}</span>
          <UIcon
            v-if="activeHeaderTheme === item.value"
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
