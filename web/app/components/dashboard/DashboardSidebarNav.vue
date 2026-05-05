<script setup>
import { computed, onMounted, ref, watch } from "vue";
import {
  AlertTriangle,
  BarChart3,
  Blocks,
  Boxes,
  BrainCircuit,
  Building2,
  CalendarDays,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ClipboardList,
  Code2,
  Database,
  FileBarChart,
  FileText,
  FormInput,
  Gauge,
  Landmark,
  LayoutPanelLeft,
  Link2,
  ListChecks,
  ListTodo,
  Megaphone,
  MessageCircle,
  MessagesSquare,
  MonitorCog,
  PackageCheck,
  QrCode,
  SearchCheck,
  Settings,
  ShieldCheck,
  Store,
  Users,
  Wrench
} from "lucide-vue-next";
import { SIDEBAR_NAV_SECTIONS } from "~/utils/sidebar-nav";

const props = defineProps({
  activeWorkspace: {
    type: String,
    required: true
  },
  allowedWorkspaces: {
    type: Array,
    required: true
  }
});

const route = useRoute();
const openGroups = ref({});
const collapsed = ref(false);
const sidebarStorageKey = "dashboard-sidebar-collapsed";

const iconMap = {
  alert: AlertTriangle,
  audit: ShieldCheck,
  boxes: Boxes,
  brain: BrainCircuit,
  building: Building2,
  calendar: CalendarDays,
  chart: BarChart3,
  database: Database,
  feedback: MessageCircle,
  finance: Landmark,
  forms: FormInput,
  indicators: Gauge,
  integration: Blocks,
  link: Link2,
  manage: LayoutPanelLeft,
  megaphone: Megaphone,
  messages: MessagesSquare,
  monitoring: MonitorCog,
  page: FileText,
  qr: QrCode,
  queue: ListChecks,
  ranking: FileBarChart,
  reports: ClipboardList,
  script: Code2,
  settings: Settings,
  site: PackageCheck,
  stores: Store,
  tasks: ListTodo,
  team: Users,
  tools: Wrench,
  tracking: SearchCheck,
  user: Users,
  users: Users
};

const currentPath = computed(() => normalizePath(route.path));
const allowedWorkspaceSet = computed(() => new Set(props.allowedWorkspaces || []));
const visibleSections = computed(() =>
  SIDEBAR_NAV_SECTIONS.map((section) => ({
    ...section,
    items: (section.items || []).map(filterItem).filter(Boolean)
  })).filter((section) => section.items.length > 0)
);

function normalizePath(path) {
  const normalizedPath = String(path || "").replace(/\/+$/, "");
  return normalizedPath || "/";
}

function filterItem(item) {
  if (!isItemAllowed(item)) {
    return null;
  }

  if (!Array.isArray(item.children)) {
    return item;
  }

  const children = item.children.filter(isItemAllowed);
  if (!children.length) {
    return null;
  }

  return {
    ...item,
    children
  };
}

function isItemAllowed(item) {
  const workspaceId = String(item.workspaceId || "").trim();
  return !workspaceId || allowedWorkspaceSet.value.has(workspaceId);
}

function resolveIcon(icon) {
  return iconMap[icon] || LayoutPanelLeft;
}

function isItemActive(item) {
  const itemPath = normalizePath(item.path);

  if (String(item.workspaceId || "").trim() && props.activeWorkspace === item.workspaceId) {
    return true;
  }

  return currentPath.value === itemPath || currentPath.value.startsWith(`${itemPath}/`);
}

function isGroupActive(item) {
  return Array.isArray(item.children) && item.children.some(isItemActive);
}

function isGroupOpen(item) {
  return Boolean(openGroups.value[item.id]) || isGroupActive(item);
}

function toggleGroup(item) {
  if (collapsed.value) {
    collapsed.value = false;
    openGroups.value = {
      ...openGroups.value,
      [item.id]: true
    };
    return;
  }

  openGroups.value = {
    ...openGroups.value,
    [item.id]: !isGroupOpen(item)
  };
}

function toggleCollapsed() {
  collapsed.value = !collapsed.value;
}

watch(
  () => [route.path, visibleSections.value],
  () => {
    const nextOpenGroups = { ...openGroups.value };

    for (const section of visibleSections.value) {
      for (const item of section.items) {
        if (isGroupActive(item)) {
          nextOpenGroups[item.id] = true;
        }
      }
    }

    openGroups.value = nextOpenGroups;
  },
  { immediate: true }
);

watch(
  collapsed,
  (value) => {
    if (import.meta.client) {
      window.localStorage.setItem(sidebarStorageKey, value ? "1" : "0");
    }
  }
);

onMounted(() => {
  if (!import.meta.client) {
    return;
  }

  collapsed.value = window.localStorage.getItem(sidebarStorageKey) === "1";
});
</script>

<template>
  <aside
    class="dashboard-sidebar"
    :class="{ 'is-collapsed': collapsed }"
    aria-label="Paginas do sistema"
  >
    <div class="dashboard-sidebar__head">
      <span class="dashboard-sidebar__head-icon" aria-hidden="true">
        <LayoutPanelLeft :size="17" :stroke-width="2.2" />
      </span>
      <div v-if="!collapsed" class="dashboard-sidebar__head-copy">
        <strong>Sistema</strong>
        <span>{{ visibleSections.length }} grupos</span>
      </div>
      <button
        class="dashboard-sidebar__collapse-btn"
        type="button"
        :title="collapsed ? 'Expandir sidebar' : 'Recolher sidebar'"
        :aria-label="collapsed ? 'Expandir sidebar' : 'Recolher sidebar'"
        :aria-pressed="collapsed ? 'true' : 'false'"
        @click="toggleCollapsed"
      >
        <ChevronRight
          v-if="collapsed"
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
        <span v-if="!collapsed" class="dashboard-sidebar__section-label">{{ section.label }}</span>

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
                v-if="!collapsed"
                class="dashboard-sidebar__chevron"
                :size="16"
                :stroke-width="2.2"
                aria-hidden="true"
              />
            </button>

            <Transition name="dashboard-sidebar-submenu">
              <div
                v-if="item.children && isGroupOpen(item) && !collapsed"
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
  border: 1px solid rgba(148, 163, 184, 0.14);
  border-radius: 16px;
  background: linear-gradient(180deg, rgba(13, 18, 29, 0.94), rgba(8, 12, 20, 0.96));
  box-shadow: 0 18px 44px rgba(2, 6, 23, 0.26);
  transition: width 0.2s ease, border-radius 0.2s ease;
}

.dashboard-sidebar.is-collapsed {
  width: 5.9rem;
}

.dashboard-sidebar__head {
  display: flex;
  align-items: center;
  gap: 0.72rem;
  padding: 0.86rem 0.92rem;
  border-bottom: 1px solid rgba(148, 163, 184, 0.12);
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
  background: rgba(129, 140, 248, 0.14);
  color: #c7d2fe;
  flex-shrink: 0;
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
  color: #f8fafc;
  font-size: 0.9rem;
  line-height: 1.1;
}

.dashboard-sidebar__head-copy span {
  color: rgba(148, 163, 184, 0.86);
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
  border: 1px solid rgba(148, 163, 184, 0.16);
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.04);
  color: #cbd5e1;
  cursor: pointer;
  transition: border-color 0.16s ease, background 0.16s ease, color 0.16s ease;
}

.dashboard-sidebar__collapse-btn:hover {
  border-color: rgba(129, 140, 248, 0.32);
  background: rgba(129, 140, 248, 0.12);
  color: #eef2ff;
}

.dashboard-sidebar.is-collapsed .dashboard-sidebar__collapse-btn {
  margin-left: 0;
}

.dashboard-sidebar__scroll {
  min-height: 0;
  overflow-y: auto;
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
  color: rgba(148, 163, 184, 0.72);
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
  color: rgba(226, 232, 240, 0.78);
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
  border-color: rgba(129, 140, 248, 0.2);
  background: rgba(129, 140, 248, 0.09);
  color: #eef2ff;
}

.dashboard-sidebar__item.is-active,
.dashboard-sidebar__subitem.is-active {
  border-color: rgba(129, 140, 248, 0.32);
  background: linear-gradient(135deg, rgba(129, 140, 248, 0.22), rgba(45, 212, 191, 0.08));
  color: #ffffff;
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
  color: rgba(226, 232, 240, 0.84);
  font-size: 0.61rem;
  font-weight: 800;
  line-height: 1.05;
  text-align: center;
}

.dashboard-sidebar.is-collapsed .dashboard-sidebar__item.is-active .dashboard-sidebar__label,
.dashboard-sidebar.is-collapsed .dashboard-sidebar__item:hover .dashboard-sidebar__label {
  color: #ffffff;
}

.dashboard-sidebar__chevron {
  margin-left: auto;
  flex-shrink: 0;
  color: rgba(148, 163, 184, 0.72);
  transition: transform 0.16s ease, color 0.16s ease;
}

.dashboard-sidebar__item.is-open .dashboard-sidebar__chevron {
  transform: rotate(180deg);
  color: #c7d2fe;
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
  background: rgba(148, 163, 184, 0.14);
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
