<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { ChevronDown, LayoutPanelLeft, LogOut, User, X } from "lucide-vue-next";
import { storeToRefs } from "pinia";
import AppSelectField from "~/components/ui/AppSelectField.vue";
import DashboardSidebarNav from "~/components/dashboard/DashboardSidebarNav.vue";
import FeedbackNotificationsDropdown from "~/components/feedback/FeedbackNotificationsDropdown.vue";
import { useDashboardNav } from "~/composables/useDashboardNav";
import { getRoleLabel } from "~/domain/utils/permissions";
import { useAuthStore } from "~/stores/auth";
import { getApiBase } from "~/utils/api-client";

const ALL_STORES_VALUE = "__all_stores__";

const props = defineProps({
  state: {
    type: Object,
    required: true
  },
  showOperationsContext: {
    type: Boolean,
    default: true
  },
  activeWorkspace: {
    type: String,
    default: ""
  },
  allowedWorkspaces: {
    type: Array,
    default: () => []
  }
});

const emit = defineEmits(["store-change", "profile-change"]);
const auth = useAuthStore();
const { isAuthenticated, user, role, accessibleStoreIds, canUseAllStores, isAllStoresScope } = storeToRefs(auth);
const runtimeConfig = useRuntimeConfig();
const sidebarOpen = ref(false);
const profileMenuRef = ref(null);
const profileMenuOpen = ref(false);
const route = useRoute();

const { headerItems, resolveIcon, isItemActive, isGroupActive } = useDashboardNav(
  computed(() => props.activeWorkspace),
  computed(() => props.allowedWorkspaces)
);

const availableStores = computed(() => {
  if (!isAuthenticated.value || !accessibleStoreIds.value.length) {
    return props.state.stores || [];
  }
  const allowedStoreIds = new Set(accessibleStoreIds.value);
  return (props.state.stores || []).filter((store) => allowedStoreIds.has(store.id));
});

const activeServicesCount = computed(() => props.state.activeServices?.length || 0);
const displayName = computed(() => String(user.value?.displayName || "").trim());
const profileEmail = computed(() => String(user.value?.email || "").trim());
const profileRoleLabel = computed(() => getRoleLabel(role.value));
const avatarUrl = computed(() => {
  const avatarPath = String(user.value?.avatarPath || "").trim();
  if (!avatarPath) return "";
  return new URL(avatarPath, getApiBase(runtimeConfig)).toString();
});
const profileInitial = computed(() => displayName.value.charAt(0).toUpperCase() || "U");
const selectedStoreValue = computed(() =>
  isAllStoresScope.value ? ALL_STORES_VALUE : String(props.state.activeStoreId || "").trim()
);
const storeSelectOptions = computed(() => {
  const options = availableStores.value.map((store) => ({
    value: String(store.id || "").trim(),
    label: String(store.name || "").trim(),
    meta: [String(store.code || "").trim(), String(store.city || "").trim()].filter(Boolean).join(" - ")
  }));
  if (canUseAllStores.value) {
    options.unshift({
      value: ALL_STORES_VALUE,
      label: "Todas as lojas",
      meta: "Mantem o contexto global para comparativo multi-loja"
    });
  }
  return options;
});
const profileSelectOptions = computed(() =>
  (props.state.profiles || []).map((profile) => ({
    value: String(profile.id || "").trim(),
    label: String(profile.name || "").trim()
  }))
);

function handleStoreChange(value) {
  const normalizedValue = String(value || "").trim();

  if (!normalizedValue) {
    return;
  }

  if (normalizedValue === ALL_STORES_VALUE) {
    auth.setStoreScopeMode("all");
    return;
  }

  auth.setStoreScopeMode("single");
  emit("store-change", normalizedValue);
}

function handleProfileChange(value) {
  emit("profile-change", String(value || "").trim());
}

function closeProfileMenu() {
  profileMenuOpen.value = false;
}

function closeSidebar() {
  sidebarOpen.value = false;
}

function toggleProfileMenu() {
  profileMenuOpen.value = !profileMenuOpen.value;
}

async function handleLogout() {
  closeProfileMenu();
  await auth.logout();
  await navigateTo("/auth/login", { replace: true });
}

function handlePointerDown(event) {
  if (!profileMenuOpen.value) {
    return;
  }

  const target = event.target;
  if (profileMenuRef.value && !profileMenuRef.value.contains(target)) {
    closeProfileMenu();
  }
}

function handleEscape(event) {
  if (event.key === "Escape") {
    closeProfileMenu();
    closeSidebar();
  }
}

watch(
  () => route.fullPath,
  () => {
    closeProfileMenu();
    closeSidebar();
  }
);

onMounted(() => {
  document.addEventListener("pointerdown", handlePointerDown);
  document.addEventListener("keydown", handleEscape);
});

onBeforeUnmount(() => {
  document.removeEventListener("pointerdown", handlePointerDown);
  document.removeEventListener("keydown", handleEscape);
});
</script>

<template>
  <header class="app-header dashboard-header">
    <div class="brand-bar dashboard-header__bar">
      <div class="brand dashboard-header__brand">
        <UButton
          icon="i-lucide-menu"
          color="neutral"
          variant="ghost"
          size="sm"
          aria-label="Abrir sidebar"
          @click="sidebarOpen = true"
        />
        <picture class="dashboard-header__logo" aria-label="Logo da plataforma">
          <source srcset="/logo.avif" type="image/avif">
          <source srcset="/logo.webp" type="image/webp">
          <img src="/logo.png" alt="Logo da plataforma">
        </picture>
      </div>

      <nav class="dashboard-header__nav" aria-label="Menu principal">
        <template v-for="item in headerItems" :key="item.id">
          <div
            v-if="item.children"
            class="dashboard-header__nav-dropdown"
          >
            <button
              class="dashboard-header__nav-link"
              :class="{ 'is-active': isGroupActive(item) }"
              type="button"
            >
              <component
                :is="resolveIcon(item.icon)"
                class="dashboard-header__nav-icon"
                :size="16"
                :stroke-width="2.15"
                aria-hidden="true"
              />
              <span>{{ item.label }}</span>
              <ChevronDown
                class="dashboard-header__nav-chevron"
                :size="14"
                :stroke-width="2.25"
                aria-hidden="true"
              />
            </button>

            <div class="dashboard-header__nav-popover">
              <NuxtLink
                v-for="child in item.children"
                :key="child.id"
                :to="child.path"
                class="dashboard-header__nav-popover-item"
                :class="{ 'is-active': isItemActive(child) }"
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
          </div>

          <NuxtLink
            v-else
            :to="item.path"
            class="dashboard-header__nav-link"
            :class="{ 'is-active': isItemActive(item) }"
          >
            <component
              :is="resolveIcon(item.icon)"
              class="dashboard-header__nav-icon"
              :size="16"
              :stroke-width="2.15"
              aria-hidden="true"
            />
            <span>{{ item.label }}</span>
          </NuxtLink>
        </template>
      </nav>

      <div class="brand__meta dashboard-header__meta">
        <span v-if="showOperationsContext" class="summary-pill">{{ state.waitingList.length }} na fila</span>
        <span
          v-if="showOperationsContext"
          class="summary-pill"
          :class="{ 'summary-pill--active': activeServicesCount > 0 }"
        >
          {{ activeServicesCount }}/{{ state.settings.maxConcurrentServices }} em atendimento
        </span>
        <span v-if="showOperationsContext" class="summary-pill">{{ state.serviceHistory.length }} finalizados</span>
        <AppSelectField
          v-if="showOperationsContext"
          class="summary-select dashboard-header__store-select"
          :model-value="selectedStoreValue"
          :options="storeSelectOptions"
          placeholder="Selecionar loja"
          :show-leading-icon="false"
          compact
          @update:model-value="handleStoreChange"
        />
        <AppSelectField
          v-if="showOperationsContext && !isAuthenticated"
          class="summary-select"
          :model-value="state.activeProfileId"
          :options="profileSelectOptions"
          placeholder="Selecionar perfil"
          :show-leading-icon="false"
          compact
          @update:model-value="handleProfileChange"
        />
        <FeedbackNotificationsDropdown v-if="isAuthenticated" />
        <div v-if="isAuthenticated" ref="profileMenuRef" class="dashboard-header__profile-menu">
          <button
            class="dashboard-header__profile-trigger"
            type="button"
            aria-haspopup="menu"
            :aria-expanded="profileMenuOpen ? 'true' : 'false'"
            aria-label="Abrir menu do perfil"
            @click="toggleProfileMenu"
          >
            <span class="dashboard-header__profile-avatar" aria-hidden="true">
              <img v-if="avatarUrl" :src="avatarUrl" alt="">
              <span v-else>{{ profileInitial }}</span>
            </span>
          </button>

          <Transition name="dashboard-header-menu">
            <div v-if="profileMenuOpen" class="dashboard-header__profile-dropdown" role="menu">
              <div class="dashboard-header__profile-card">
                <span class="dashboard-header__profile-role">{{ profileRoleLabel }}</span>
                <strong class="dashboard-header__profile-fullname">{{ displayName || "Conta autenticada" }}</strong>
                <span class="dashboard-header__profile-email">{{ profileEmail || "Sessao ativa" }}</span>
              </div>

              <NuxtLink class="dashboard-header__menu-action" to="/perfil" role="menuitem" @click="closeProfileMenu">
                <User :size="16" :stroke-width="2.15" />
                <span>Pagina de perfil</span>
              </NuxtLink>

              <button class="dashboard-header__menu-action dashboard-header__menu-action--danger" type="button" role="menuitem" @click="handleLogout">
                <LogOut :size="16" :stroke-width="2.15" />
                <span>Sair da plataforma</span>
              </button>
            </div>
          </Transition>
        </div>
      </div>
    </div>

    <Teleport to="body">
      <Transition name="dashboard-sidebar-drawer">
        <div v-if="sidebarOpen" class="dashboard-header__sidebar-drawer" role="dialog" aria-modal="true" aria-label="Menu do sistema">
          <button class="dashboard-header__sidebar-backdrop" type="button" aria-label="Fechar sidebar" @click="closeSidebar" />
          <aside class="dashboard-header__sidebar-panel">
            <button class="dashboard-header__sidebar-close" type="button" aria-label="Fechar sidebar" @click="closeSidebar">
              <X :size="18" :stroke-width="2.2" aria-hidden="true" />
            </button>
            <div class="dashboard-header__sidebar-body">
              <DashboardSidebarNav
                class="dashboard-header__drawer-nav"
                :active-workspace="activeWorkspace"
                :allowed-workspaces="allowedWorkspaces"
                always-expanded
              />
            </div>
            <div class="dashboard-header__sidebar-footer">
              <FeedbackNotificationsDropdown v-if="isAuthenticated" />
              <NuxtLink
                v-if="isAuthenticated"
                class="dashboard-header__sidebar-profile"
                aria-label="Abrir perfil"
                to="/perfil"
                @click="closeSidebar"
              >
                <span class="dashboard-header__profile-avatar" aria-hidden="true">
                  <img v-if="avatarUrl" :src="avatarUrl" alt="">
                  <span v-else>{{ profileInitial }}</span>
                </span>
                <span class="dashboard-header__sidebar-profile-copy">
                  <strong>{{ displayName || "Conta autenticada" }}</strong>
                  <small>{{ profileRoleLabel }}</small>
                </span>
              </NuxtLink>
            </div>
          </aside>
        </div>
      </Transition>
    </Teleport>
  </header>
</template>

<style scoped>
.dashboard-header {
  position: relative;
  z-index: 9500;
  flex: 0 0 auto;
  overflow: visible;
  background: var(--admin-header-panel-bg);
  border-bottom: 1px solid var(--admin-header-border);
  box-shadow: var(--admin-header-shell-shadow);
  color: var(--admin-header-text);
  backdrop-filter: blur(var(--admin-header-panel-blur));
}

.dashboard-header__bar {
  width: min(100%, 1400px);
  width: min(100%, 95%);
  gap: 1rem;
  padding: 0.85rem 1rem;
  overflow: visible;
}

.dashboard-header__brand {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  min-width: 0;
  flex: 0 0 auto;
}

.dashboard-header__logo {
  display: inline-flex;
  width: clamp(5.5rem, 10vw, 7.1rem);
  max-width: 100%;
}

.dashboard-header__logo img {
  display: block;
  width: 100%;
  height: auto;
}

.dashboard-header__nav {
  min-width: 0;
  flex: 1 1 auto;
  display: flex;
  align-items: center;
  gap: 0.24rem;
  overflow: visible;
  scrollbar-width: none;
}

.dashboard-header__nav::-webkit-scrollbar {
  display: none;
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
  box-shadow: none;
  color: var(--admin-header-muted);
  font-size: 0.82rem;
  font-weight: 800;
  text-decoration: none;
  white-space: nowrap;
  cursor: pointer;
  transition: color 0.16s ease;
}

.dashboard-header__nav-link:hover {
  background: transparent;
  box-shadow: none;
  color: var(--admin-header-text);
}

.dashboard-header__nav-link.is-active {
  background: transparent;
  box-shadow: none;
  color: var(--admin-header-text);
}

.dashboard-header__nav-link::after {
  content: "";
  position: absolute;
  inset-inline: 0.6rem;
  bottom: 0.18rem;
  height: 2px;
  border-radius: 999px;
  background: rgb(var(--primary));
  transform: scaleX(0);
  transform-origin: left center;
  transition: transform 0.18s ease;
}

.dashboard-header__nav-link:hover::after,
.dashboard-header__nav-link:focus-visible::after,
.dashboard-header__nav-link.is-active::after,
.dashboard-header__nav-dropdown:hover .dashboard-header__nav-link::after,
.dashboard-header__nav-dropdown:focus-within .dashboard-header__nav-link::after {
  transform: scaleX(1);
}

.dashboard-header__nav-icon,
.dashboard-header__nav-chevron {
  flex-shrink: 0;
}

.dashboard-header__nav-chevron {
  width: 0.86rem;
  height: 0.86rem;
  color: var(--admin-header-muted);
  transition: transform 0.16s ease, color 0.16s ease;
}

.dashboard-header__nav-dropdown {
  position: relative;
  flex: 0 0 auto;
  padding-block: 0.4rem;
}

.dashboard-header__nav-dropdown:hover .dashboard-header__nav-link,
.dashboard-header__nav-dropdown:focus-within .dashboard-header__nav-link {
  background: transparent;
  box-shadow: none;
  color: var(--admin-header-text);
}

.dashboard-header__nav-dropdown:hover .dashboard-header__nav-chevron,
.dashboard-header__nav-dropdown:focus-within .dashboard-header__nav-chevron {
  transform: rotate(180deg);
  color: rgb(var(--primary));
}

.dashboard-header__nav-dropdown:hover .dashboard-header__nav-popover,
.dashboard-header__nav-dropdown:focus-within .dashboard-header__nav-popover {
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
  transition: opacity 0.14s ease, transform 0.14s ease, visibility 0.14s ease;
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

.dashboard-header__meta {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.7rem;
  flex-wrap: wrap;
  flex: 0 0 auto;
}

.dashboard-header__store-select {
  min-width: 13.5rem;
}

.dashboard-header__store-select :deep(.app-select-field__trigger) {
  min-height: 2.6rem;
  padding: 0 0.85rem;
  border-radius: 999px;
  border-color: var(--admin-header-border);
  background: var(--admin-header-hover-bg);
  color: var(--admin-header-text);
}

.dashboard-header__store-select :deep(.app-select-field__trigger:hover),
.dashboard-header__store-select :deep(.app-select-field__trigger.is-open),
.dashboard-header__store-select :deep(.app-select-field__trigger.is-filled) {
  border-color: rgb(var(--ring) / 0.42);
  background: var(--admin-header-active-bg);
}

.dashboard-header__profile-menu {
  position: relative;
}

.dashboard-header__profile-trigger {
  display: grid;
  place-items: center;
  width: 3rem;
  height: 3rem;
  padding: 0;
  border: 1px solid var(--admin-header-border);
  border-radius: 999px;
  background: var(--admin-header-hover-bg);
  color: var(--admin-header-text);
  cursor: pointer;
  transition: border-color 0.18s ease, background 0.18s ease, transform 0.18s ease;
}

.dashboard-header__profile-trigger:hover,
.dashboard-header__profile-trigger[aria-expanded="true"] {
  border-color: rgb(var(--ring) / 0.42);
  background: var(--admin-header-active-bg);
}

.dashboard-header__profile-avatar {
  display: grid;
  place-items: center;
  width: 2.3rem;
  height: 2.3rem;
  border-radius: 999px;
  overflow: hidden;
  background: linear-gradient(135deg, rgb(var(--primary) / 0.92), rgb(var(--success) / 0.9));
  color: #f8fafc;
  font-size: 0.86rem;
  font-weight: 800;
  text-transform: uppercase;
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.18);
}

.dashboard-header__profile-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.dashboard-header__profile-dropdown {
  position: absolute;
  top: calc(100% + 0.55rem);
  right: 0;
  z-index: 30;
  display: grid;
  gap: 0.55rem;
  width: min(18.5rem, calc(100vw - 2rem));
  padding: 0.8rem;
  border: 1px solid var(--admin-header-border);
  border-radius: 1rem;
  background: var(--admin-header-panel-bg);
  box-shadow: var(--shadow-md);
  backdrop-filter: blur(var(--admin-header-panel-blur));
}

.dashboard-header__profile-card {
  display: grid;
  gap: 0.24rem;
  padding: 0.8rem 0.85rem;
  border: 1px solid rgb(var(--primary) / 0.16);
  border-radius: 0.85rem;
  background: var(--admin-header-brand-bg);
}

.dashboard-header__profile-role {
  color: rgb(var(--primary) / 0.9);
  font-size: 0.64rem;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.dashboard-header__profile-fullname {
  color: var(--admin-header-text);
  font-size: 0.95rem;
  font-weight: 700;
}

.dashboard-header__profile-email {
  color: var(--admin-header-muted);
  font-size: 0.75rem;
  word-break: break-word;
}

.dashboard-header__menu-action {
  display: flex;
  align-items: center;
  gap: 0.7rem;
  width: 100%;
  padding: 0.82rem 0.88rem;
  border: 1px solid var(--admin-header-border);
  border-radius: 0.85rem;
  background: transparent;
  color: var(--admin-header-text);
  text-decoration: none;
  font-size: 0.82rem;
  font-weight: 700;
  cursor: pointer;
  transition: border-color 0.18s ease, background 0.18s ease, color 0.18s ease;
}

.dashboard-header__menu-action:hover {
  border-color: rgb(var(--ring) / 0.28);
  background: var(--admin-header-hover-bg);
}

.dashboard-header__menu-action--danger {
  color: #fecaca;
  background: rgba(127, 29, 29, 0.18);
  border-color: rgba(248, 113, 113, 0.16);
}

.dashboard-header__menu-action--danger:hover {
  border-color: rgba(248, 113, 113, 0.32);
  background: rgba(127, 29, 29, 0.28);
}

.dashboard-header-menu-enter-active,
.dashboard-header-menu-leave-active {
  transition: opacity 0.18s ease, transform 0.18s ease;
}

.dashboard-header-menu-enter-from,
.dashboard-header-menu-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}

.dashboard-header__drawer-nav {
  width: 100%;
  height: 100%;
  border-radius: 0;
  border: 0;
  box-shadow: none;
}

.dashboard-header__sidebar-drawer {
  position: fixed;
  inset: 0;
  z-index: 10000;
  pointer-events: none;
}

.dashboard-header__sidebar-backdrop {
  position: absolute;
  inset: 0;
  border: 0;
  background: rgba(2, 6, 23, 0.48);
  pointer-events: auto;
}

.dashboard-header__sidebar-panel {
  position: absolute;
  inset-block: 0;
  left: 0;
  width: min(18rem, calc(100vw - 2rem));
  padding: 0;
  display: grid;
  grid-template-rows: minmax(0, 1fr) auto;
  background: var(--admin-header-panel-bg);
  border-right: 1px solid var(--admin-header-border);
  box-shadow: var(--shadow-md);
  pointer-events: auto;
}

.dashboard-header__sidebar-body {
  min-height: 0;
  overflow: hidden;
}

.dashboard-header__sidebar-close {
  position: absolute;
  top: 0.95rem;
  right: 0.95rem;
  z-index: 2;
  display: grid;
  place-items: center;
  width: 2rem;
  height: 2rem;
  border: 1px solid var(--admin-header-border);
  border-radius: 10px;
  background: var(--admin-header-hover-bg);
  color: var(--admin-header-muted);
  cursor: pointer;
  transition: color 0.16s ease, background 0.16s ease, border-color 0.16s ease;
}

.dashboard-header__sidebar-close:hover {
  border-color: rgb(var(--ring) / 0.32);
  color: var(--admin-header-text);
}

.dashboard-header__sidebar-footer {
  min-height: 4.4rem;
  display: flex;
  align-items: center;
  gap: 0.75rem;
  border-top: 1px solid var(--admin-header-separator);
  padding: 0.8rem 1rem;
  background: var(--admin-header-panel-bg);
}

.dashboard-header__sidebar-profile {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  gap: 0.7rem;
  border: 0;
  background: transparent;
  color: var(--admin-header-text);
  cursor: pointer;
  text-decoration: none;
}

.dashboard-header__sidebar-profile-copy {
  min-width: 0;
  display: grid;
  gap: 0.15rem;
  text-align: left;
}

.dashboard-header__sidebar-profile-copy strong,
.dashboard-header__sidebar-profile-copy small {
  max-width: 10rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dashboard-header__sidebar-profile-copy strong {
  color: var(--admin-header-text);
  font-size: 0.82rem;
  font-weight: 800;
}

.dashboard-header__sidebar-profile-copy small {
  color: var(--admin-header-muted);
  font-size: 0.72rem;
  font-weight: 700;
}

.dashboard-sidebar-drawer-enter-active,
.dashboard-sidebar-drawer-leave-active {
  transition: opacity 0.18s ease;
}

.dashboard-sidebar-drawer-enter-active .dashboard-header__sidebar-panel,
.dashboard-sidebar-drawer-leave-active .dashboard-header__sidebar-panel {
  transition: transform 0.18s ease;
}

.dashboard-sidebar-drawer-enter-from,
.dashboard-sidebar-drawer-leave-to {
  opacity: 0;
}

.dashboard-sidebar-drawer-enter-from .dashboard-header__sidebar-panel,
.dashboard-sidebar-drawer-leave-to .dashboard-header__sidebar-panel {
  transform: translateX(-100%);
}

@media (max-width: 900px) {
  .dashboard-header__bar {
    align-items: stretch;
    flex-direction: column;
  }

  .dashboard-header__brand {
    justify-content: space-between;
  }

  .dashboard-header__nav {
    min-height: 2.6rem;
    overflow-x: auto;
    overflow-y: visible;
  }

  .dashboard-header__meta {
    justify-content: flex-start;
  }
}

@media (max-width: 640px) {
  .dashboard-header__bar {
    padding: 0.8rem 0.85rem;
  }

  .dashboard-header__store-select {
    min-width: min(100%, 16rem);
    width: 100%;
  }

  .dashboard-header__profile-menu {
    width: auto;
  }

  .dashboard-header__profile-trigger {
    width: 3rem;
  }

  .dashboard-header__profile-dropdown {
    left: 0;
    right: auto;
    width: min(100%, 18.5rem);
  }
}
</style>
