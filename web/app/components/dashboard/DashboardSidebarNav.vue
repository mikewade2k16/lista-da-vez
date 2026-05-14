<script setup>
import { computed, onMounted, ref, watch } from "vue";
import { ChevronDown, ChevronLeft, ChevronRight, LayoutPanelLeft } from "lucide-vue-next";
import { useDashboardNav } from "~/composables/useDashboardNav";

const props = defineProps({
  activeWorkspace: {
    type: String,
    required: true
  },
  allowedWorkspaces: {
    type: Array,
    required: true
  },
  alwaysExpanded: {
    type: Boolean,
    default: false
  }
});

const route = useRoute();
const openGroups = ref({});
const collapsed = ref(false);
const sidebarStorageKey = "dashboard-sidebar-collapsed";

const { visibleSections, resolveIcon, isItemActive, isGroupActive } = useDashboardNav(
  computed(() => props.activeWorkspace),
  computed(() => props.allowedWorkspaces)
);

const isCollapsed = computed(() => collapsed.value && !props.alwaysExpanded);

function isGroupOpen(item) {
  return Boolean(openGroups.value[item.id]) || isGroupActive(item);
}

function toggleGroup(item) {
  if (isCollapsed.value) {
    collapsed.value = false;
    openGroups.value = { ...openGroups.value, [item.id]: true };
    return;
  }
  openGroups.value = { ...openGroups.value, [item.id]: !isGroupOpen(item) };
}

function toggleCollapsed() {
  if (props.alwaysExpanded) { collapsed.value = false; return; }
  collapsed.value = !collapsed.value;
}

watch(
  () => [route.path, visibleSections.value],
  () => {
    const nextOpenGroups = { ...openGroups.value };
    for (const section of visibleSections.value) {
      for (const item of section.items) {
        if (isGroupActive(item)) nextOpenGroups[item.id] = true;
      }
    }
    openGroups.value = nextOpenGroups;
  },
  { immediate: true }
);

watch(collapsed, (value) => {
  if (import.meta.client && !props.alwaysExpanded) {
    window.localStorage.setItem(sidebarStorageKey, value ? "1" : "0");
  }
});

onMounted(() => {
  if (!import.meta.client) return;
  collapsed.value = props.alwaysExpanded ? false : window.localStorage.getItem(sidebarStorageKey) === "1";
});
</script>

<template>
  <aside
    class="dashboard-sidebar"
    :class="{ 'is-collapsed': isCollapsed }"
    aria-label="Paginas do sistema"
  >
    <div class="dashboard-sidebar__head">
      <NuxtLink v-if="!isCollapsed" to="/operacao" class="dashboard-sidebar__brand" aria-label="Crow Visuals">
        <picture class="dashboard-sidebar__logo">
          <source srcset="/logo.avif" type="image/avif">
          <source srcset="/logo.webp" type="image/webp">
          <img src="/logo.png" alt="">
        </picture>
      </NuxtLink>
      <span v-else class="dashboard-sidebar__head-icon" aria-hidden="true">
        <LayoutPanelLeft :size="17" :stroke-width="2.2" />
      </span>
      <button
        v-if="!alwaysExpanded"
        class="dashboard-sidebar__collapse-btn"
        type="button"
        :title="isCollapsed ? 'Expandir sidebar' : 'Recolher sidebar'"
        :aria-label="isCollapsed ? 'Expandir sidebar' : 'Recolher sidebar'"
        :aria-pressed="isCollapsed ? 'true' : 'false'"
        @click="toggleCollapsed"
      >
        <ChevronRight
          v-if="isCollapsed"
          :size="16"
          :stroke-width="2.2"
          aria-hidden="true"
        />
        <ChevronLeft
          v-else
          :size="16"
          :stroke-width="2.2"
          aria-hidden="true"
        />
      </button>
    </div>

    <div class="dashboard-sidebar__scroll">
      <div
        v-for="section in visibleSections"
        :key="section.id"
        class="dashboard-sidebar__section"
      >
      <span v-if="!isCollapsed" class="dashboard-sidebar__section-label">{{ section.label }}</span>

        <div class="dashboard-sidebar__items">
          <template v-for="item in section.items" :key="item.id">
            <button
              v-if="item.children"
              class="dashboard-sidebar__item dashboard-sidebar__item--group"
              :class="{ 'is-active': isGroupActive(item), 'is-open': isGroupOpen(item) }"
              type="button"
              :title="item.label"
              :aria-label="item.label"
              :aria-expanded="isGroupOpen(item) ? 'true' : 'false'"
              @click="toggleGroup(item)"
            >
              <component
                :is="resolveIcon(item.icon)"
                class="dashboard-sidebar__icon"
                :size="17"
                :stroke-width="2.15"
                aria-hidden="true"
              />
              <span class="dashboard-sidebar__label">{{ item.label }}</span>
              <ChevronDown
                v-if="!isCollapsed"
                class="dashboard-sidebar__chevron"
                :size="16"
                :stroke-width="2.2"
                aria-hidden="true"
              />
            </button>

            <Transition name="dashboard-sidebar-submenu">
              <div
                v-if="item.children && isGroupOpen(item) && !isCollapsed"
                class="dashboard-sidebar__submenu"
              >
                <NuxtLink
                  v-for="child in item.children"
                  :key="child.id"
                  :to="child.path"
                  class="dashboard-sidebar__subitem"
                  :class="{ 'is-active': isItemActive(child) }"
                  :title="child.label"
                  :aria-label="child.label"
                >
                  <component
                    :is="resolveIcon(child.icon)"
                    class="dashboard-sidebar__subicon"
                    :size="15"
                    :stroke-width="2.1"
                    aria-hidden="true"
                  />
                  <span>{{ child.label }}</span>
                </NuxtLink>
              </div>
            </Transition>

            <NuxtLink
              v-if="!item.children"
              :to="item.path"
              class="dashboard-sidebar__item"
              :class="{ 'is-active': isItemActive(item) }"
              :title="item.label"
              :aria-label="item.label"
            >
              <component
                :is="resolveIcon(item.icon)"
                class="dashboard-sidebar__icon"
                :size="17"
                :stroke-width="2.15"
                aria-hidden="true"
              />
              <span class="dashboard-sidebar__label">{{ item.label }}</span>
            </NuxtLink>
          </template>
        </div>
      </div>
    </div>
  </aside>
</template>

<style scoped>
.dashboard-sidebar {
  width: 16rem;
  min-width: 0;
  min-height: 0;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  overflow: hidden;
  border: 1px solid var(--admin-header-border);
  border-radius: 16px;
  background: var(--admin-header-panel-bg);
  box-shadow: var(--shadow-md);
  color: var(--admin-header-text);
  backdrop-filter: blur(var(--admin-header-panel-blur));
  transition: width 0.2s ease, border-radius 0.2s ease;
}

.dashboard-sidebar.is-collapsed {
  width: 5.9rem;
}

.dashboard-sidebar__head {
  display: flex;
  align-items: center;
  gap: 0.72rem;
  min-height: 4.25rem;
  padding: 0.72rem 0.92rem;
  border-bottom: 1px solid var(--admin-header-separator);
}

.dashboard-sidebar.is-collapsed .dashboard-sidebar__head {
  justify-content: center;
  gap: 0.4rem;
  padding: 0.72rem 0.55rem;
}

.dashboard-sidebar__head-icon {
  display: inline-grid;
  place-items: center;
  width: 2.05rem;
  height: 2.05rem;
  border-radius: 10px;
  background: var(--admin-header-active-bg);
  color: rgb(var(--primary));
  flex-shrink: 0;
}

.dashboard-sidebar__brand {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  color: var(--admin-header-text);
  text-decoration: none;
}

.dashboard-sidebar__logo {
  display: inline-flex;
  width: clamp(5.65rem, 9vw, 7.4rem);
}

.dashboard-sidebar__logo img {
  display: block;
  width: 100%;
  height: auto;
}

.dashboard-sidebar.is-collapsed .dashboard-sidebar__head-icon {
  display: none;
}

.dashboard-sidebar__head-copy {
  min-width: 0;
  display: grid;
  gap: 0.1rem;
}

.dashboard-sidebar__head-copy strong {
  color: var(--admin-header-text);
  font-size: 0.9rem;
  line-height: 1.1;
}

.dashboard-sidebar__head-copy span {
  color: var(--admin-header-muted);
  font-size: 0.72rem;
  line-height: 1.1;
}

.dashboard-sidebar__collapse-btn {
  display: inline-grid;
  place-items: center;
  width: 2rem;
  height: 2rem;
  margin-left: auto;
  padding: 0;
  border: 1px solid var(--admin-header-border);
  border-radius: 10px;
  background: transparent;
  color: var(--admin-header-muted);
  cursor: pointer;
  transition: border-color 0.16s ease, background 0.16s ease, color 0.16s ease;
}

.dashboard-sidebar__collapse-btn:hover {
  border-color: rgb(var(--ring) / 0.32);
  background: var(--admin-header-hover-bg);
  color: var(--admin-header-text);
}

.dashboard-sidebar.is-collapsed .dashboard-sidebar__collapse-btn {
  margin-left: 0;
}

.dashboard-sidebar__scroll {
  min-height: 0;
  overflow-y: scroll;
  scrollbar-gutter: stable;
  padding: 0.72rem;
}

.dashboard-sidebar.is-collapsed .dashboard-sidebar__scroll {
  padding: 0.55rem;
  overflow-x: hidden;
}

.dashboard-sidebar__section {
  display: grid;
  gap: 0.45rem;
}

.dashboard-sidebar__section + .dashboard-sidebar__section {
  margin-top: 0.9rem;
}

.dashboard-sidebar.is-collapsed .dashboard-sidebar__section {
  gap: 0.3rem;
}

.dashboard-sidebar.is-collapsed .dashboard-sidebar__section + .dashboard-sidebar__section {
  margin-top: 0.35rem;
}

.dashboard-sidebar__section-label {
  padding: 0 0.35rem;
  color: var(--admin-header-muted);
  font-size: 0.64rem;
  font-weight: 800;
  line-height: 1;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.dashboard-sidebar__items,
.dashboard-sidebar__submenu {
  display: grid;
  gap: 0.3rem;
}

.dashboard-sidebar__item,
.dashboard-sidebar__subitem {
  min-width: 0;
  width: 100%;
  display: flex;
  align-items: center;
  gap: 0.62rem;
  border: 1px solid transparent;
  background: transparent;
  color: var(--admin-header-muted);
  text-decoration: none;
  text-align: left;
  cursor: pointer;
  transition: border-color 0.16s ease, background 0.16s ease, color 0.16s ease, transform 0.16s ease;
}

.dashboard-sidebar__item {
  min-height: 2.45rem;
  padding: 0 0.68rem;
  border-radius: 10px;
  font-size: 0.82rem;
  font-weight: 750;
}

.dashboard-sidebar.is-collapsed .dashboard-sidebar__item {
  flex-direction: column;
  justify-content: center;
  gap: 0.28rem;
  min-height: 3.55rem;
  padding: 0.38rem 0.32rem;
}

.dashboard-sidebar__item:hover,
.dashboard-sidebar__subitem:hover {
  border-color: rgb(var(--ring) / 0.2);
  background: var(--admin-header-hover-bg);
  color: var(--admin-header-text);
}

.dashboard-sidebar__item.is-active,
.dashboard-sidebar__subitem.is-active {
  border-color: rgb(var(--ring) / 0.32);
  background: var(--admin-header-active-bg);
  color: var(--admin-header-text);
}

.dashboard-sidebar__item--group {
  appearance: none;
}

.dashboard-sidebar__icon,
.dashboard-sidebar__subicon {
  flex-shrink: 0;
}

.dashboard-sidebar.is-collapsed .dashboard-sidebar__icon {
  width: 1.08rem;
  height: 1.08rem;
}

.dashboard-sidebar__label {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dashboard-sidebar.is-collapsed .dashboard-sidebar__label {
  width: 100%;
  flex: 0 1 auto;
  color: var(--admin-header-muted);
  font-size: 0.61rem;
  font-weight: 800;
  line-height: 1.05;
  text-align: center;
}

.dashboard-sidebar.is-collapsed .dashboard-sidebar__item.is-active .dashboard-sidebar__label,
.dashboard-sidebar.is-collapsed .dashboard-sidebar__item:hover .dashboard-sidebar__label {
  color: var(--admin-header-text);
}

.dashboard-sidebar__chevron {
  margin-left: auto;
  flex-shrink: 0;
  color: var(--admin-header-muted);
  transition: transform 0.16s ease, color 0.16s ease;
}

.dashboard-sidebar__item.is-open .dashboard-sidebar__chevron {
  transform: rotate(180deg);
  color: rgb(var(--primary));
}

.dashboard-sidebar__submenu {
  position: relative;
  margin-left: 1.04rem;
  padding-left: 0.62rem;
}

.dashboard-sidebar__submenu::before {
  content: "";
  position: absolute;
  top: 0.16rem;
  bottom: 0.16rem;
  left: 0;
  width: 1px;
  background: var(--admin-header-separator);
}

.dashboard-sidebar__subitem {
  min-height: 2.1rem;
  padding: 0 0.55rem;
  border-radius: 9px;
  font-size: 0.77rem;
  font-weight: 700;
}

.dashboard-sidebar-submenu-enter-active,
.dashboard-sidebar-submenu-leave-active {
  transition: opacity 0.15s ease, transform 0.15s ease;
}

.dashboard-sidebar-submenu-enter-from,
.dashboard-sidebar-submenu-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}

@media (max-width: 980px) {
  .dashboard-sidebar {
    max-height: 20rem;
  }

  .dashboard-sidebar__scroll {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.8rem;
    align-items: start;
  }

  .dashboard-sidebar__section + .dashboard-sidebar__section {
    margin-top: 0;
  }
}

@media (max-width: 640px) {
  .dashboard-sidebar {
    max-height: 18rem;
    border-radius: 14px;
  }

  .dashboard-sidebar__head {
    padding: 0.72rem;
  }

  .dashboard-sidebar__scroll {
    grid-template-columns: minmax(0, 1fr);
    padding: 0.62rem;
  }
}
</style>
