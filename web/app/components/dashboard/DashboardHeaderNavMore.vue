<script setup lang="ts">
import { ref } from 'vue'
import { ChevronDown, MoreHorizontal } from 'lucide-vue-next'
import type { NavItem } from '~/stores/nav'
import { useDropdownDismiss } from '~/composables/useDropdownDismiss'

defineProps<{
  // Itens top-level que nao couberam na barra (vao para este popover "Mais").
  items: NavItem[]
  resolveIcon: (icon: string) => unknown
  isItemActive: (item: NavItem) => boolean
}>()

const rootRef = ref<HTMLElement | null>(null)
const open = ref(false)

function toggle() {
  open.value = !open.value
}

function close() {
  open.value = false
}

useDropdownDismiss(() => open.value, close, { rootRef })
</script>

<template>
  <div
    ref="rootRef"
    class="dashboard-header__nav-dropdown dashboard-header__nav-more"
    :class="{ 'is-open': open }"
  >
    <button
      class="dashboard-header__nav-link"
      type="button"
      :aria-expanded="open ? 'true' : 'false'"
      aria-haspopup="true"
      aria-label="Mais itens do menu"
      @click="toggle"
    >
      <MoreHorizontal
        class="dashboard-header__nav-icon"
        :size="16"
        :stroke-width="2.15"
        aria-hidden="true"
      />
      <span>Mais</span>
      <ChevronDown
        class="dashboard-header__nav-chevron"
        :size="14"
        :stroke-width="2.25"
        aria-hidden="true"
      />
    </button>

    <div class="dashboard-header__nav-popover dashboard-header__nav-popover--more">
      <template v-for="item in items" :key="item.id">
        <NuxtLink
          v-if="!item.children"
          :to="item.path"
          class="dashboard-header__nav-popover-item"
          :class="{ 'is-active': isItemActive(item) }"
          @click="close"
        >
          <component
            :is="resolveIcon(item.icon)"
            class="dashboard-header__nav-popover-icon"
            :size="16"
            :stroke-width="2.1"
            aria-hidden="true"
          />
          <span>{{ item.label }}</span>
        </NuxtLink>

        <div v-else class="dashboard-header__nav-more-group">
          <span class="dashboard-header__nav-more-group-label">{{ item.label }}</span>
          <NuxtLink
            v-for="child in item.children"
            :key="child.id"
            :to="child.path"
            class="dashboard-header__nav-popover-item"
            :class="{ 'is-active': isItemActive(child) }"
            @click="close"
          >
            <component
              :is="resolveIcon(child.icon)"
              class="dashboard-header__nav-popover-icon"
              :size="16"
              :stroke-width="2.1"
              aria-hidden="true"
            />
            <span>{{ child.label }}</span>
          </NuxtLink>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.dashboard-header__nav-dropdown {
  position: relative;
  flex: 0 0 auto;
  padding-block: 0.4rem;
}

.dashboard-header__nav-more {
  margin-left: auto;
}

.dashboard-header__nav-link {
  appearance: none;
  position: relative;
  min-height: 2.45rem;
  display: inline-flex;
  align-items: center;
  gap: 0.42rem;
  flex: 0 0 auto;
  border: 0;
  border-radius: var(--radius-sm);
  padding: 0 0.65rem;
  background: transparent;
  color: var(--admin-header-muted);
  font-size: 0.82rem;
  font-weight: 800;
  white-space: nowrap;
  cursor: pointer;
  transition: color 0.16s ease;
}

.dashboard-header__nav-icon,
.dashboard-header__nav-chevron {
  flex-shrink: 0;
}

.dashboard-header__nav-chevron {
  width: 0.86rem;
  height: 0.86rem;
  color: var(--admin-header-muted);
  transition:
    transform 0.16s ease,
    color 0.16s ease;
}

.dashboard-header__nav-more:hover .dashboard-header__nav-link,
.dashboard-header__nav-more.is-open .dashboard-header__nav-link {
  color: var(--admin-header-text);
}

.dashboard-header__nav-more:hover .dashboard-header__nav-chevron,
.dashboard-header__nav-more.is-open .dashboard-header__nav-chevron {
  transform: rotate(180deg);
  color: rgb(var(--primary));
}

.dashboard-header__nav-more:hover .dashboard-header__nav-popover,
.dashboard-header__nav-more.is-open .dashboard-header__nav-popover {
  opacity: 1;
  visibility: visible;
  transform: translateY(0);
  pointer-events: auto;
}

.dashboard-header__nav-popover {
  position: absolute;
  top: calc(100% - 0.3rem);
  left: 0;
  z-index: 9600;
  min-width: 13rem;
  display: grid;
  gap: 0.2rem;
  border: 1px solid var(--admin-header-border);
  border-radius: var(--radius-sm);
  background: var(--admin-header-panel-bg);
  box-shadow: var(--shadow-md);
  padding: 0.35rem;
  backdrop-filter: blur(var(--admin-header-panel-blur));
  opacity: 0;
  visibility: hidden;
  transform: translateY(-0.35rem);
  pointer-events: none;
  transition:
    opacity 0.14s ease,
    transform 0.14s ease,
    visibility 0.14s ease;
}

.dashboard-header__nav-popover--more {
  left: auto;
  right: 0;
  max-height: min(70vh, 30rem);
  overflow-y: auto;
}

.dashboard-header__nav-popover-item {
  min-height: 2.2rem;
  display: flex;
  align-items: center;
  gap: 0.55rem;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  padding: 0 0.65rem;
  color: var(--admin-header-text);
  font-size: 0.82rem;
  font-weight: 750;
  text-decoration: none;
}

.dashboard-header__nav-popover-item:hover,
.dashboard-header__nav-popover-item.is-active {
  border-color: rgb(var(--ring) / 0.22);
  background: var(--admin-header-hover-bg);
}

.dashboard-header__nav-popover-icon {
  flex-shrink: 0;
  color: var(--admin-header-muted);
}

.dashboard-header__nav-popover-item:hover .dashboard-header__nav-popover-icon,
.dashboard-header__nav-popover-item.is-active .dashboard-header__nav-popover-icon {
  color: rgb(var(--primary));
}

.dashboard-header__nav-more-group {
  display: grid;
  gap: 0.2rem;
  padding-top: 0.2rem;
}

.dashboard-header__nav-more-group-label {
  padding: 0.25rem 0.65rem 0.1rem;
  color: var(--admin-header-muted);
  font-size: 0.62rem;
  font-weight: 800;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}
</style>
