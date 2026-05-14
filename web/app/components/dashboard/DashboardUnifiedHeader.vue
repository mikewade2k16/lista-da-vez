<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { storeToRefs } from "pinia";
import AppSelectField from "~/components/ui/AppSelectField.vue";
import DashboardSidebarNav from "~/components/dashboard/DashboardSidebarNav.vue";
import FeedbackNotificationsDropdown from "~/components/feedback/FeedbackNotificationsDropdown.vue";
import { getRoleLabel } from "~/domain/utils/permissions";
import { useAuthStore } from "~/stores/auth";
import { useNavStore } from "~/stores/nav";
import { getApiBase } from "~/utils/api-client";

const ALL_STORES_VALUE = "__all_stores__";

const props = defineProps({
  state: {
    type: Object,
    required: true
  },
  activeWorkspace: {
    type: String,
    required: true
  },
  allowedWorkspaces: {
    type: Array,
    required: true
  }
});

const emit = defineEmits(["store-change", "profile-change"]);

const auth = useAuthStore();
const navStore = useNavStore();
const runtimeConfig = useRuntimeConfig();
const route = useRoute();
const { currentTheme, hasCustomTheme, initializeFromStorage, applyTheme, getThemeLabel } = useOmniTheme();
const { isAuthenticated, user, role, accessibleStoreIds, canUseAllStores, isAllStoresScope } = storeToRefs(auth);

const menuOpen = ref(false);
const profileMenuRef = ref(null);
const profileMenuOpen = ref(false);

const allowedWorkspaceSet = computed(() => new Set(props.allowedWorkspaces || []));
const currentPath = computed(() => normalizePath(route.path));
const activeServicesCount = computed(() => props.state.activeServices?.length || 0);

const availableStores = computed(() => {
  if (!isAuthenticated.value || !accessibleStoreIds.value.length) {
    return props.state.stores || [];
  }

  const allowedStoreIds = new Set(accessibleStoreIds.value);
  return (props.state.stores || []).filter((store) => allowedStoreIds.has(store.id));
});

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

const displayName = computed(() => String(user.value?.displayName || user.value?.name || "").trim());
const profileEmail = computed(() => String(user.value?.email || "").trim());
const profileRoleLabel = computed(() => getRoleLabel(role.value));
const avatarUrl = computed(() => {
  const avatarPath = String(user.value?.avatarPath || "").trim();
  if (!avatarPath) {
    return "";
  }

  return new URL(avatarPath, getApiBase(runtimeConfig)).toString();
});
const profileInitial = computed(() => (displayName.value || profileEmail.value || "m").charAt(0).toUpperCase());

const visibleSections = computed(() =>
  navStore.sections.map((section) => ({
    ...section,
    items: (section.items || []).map(filterItem).filter(Boolean)
  })).filter((section) => section.items.length > 0)
);

const headerItems = computed(() =>
  visibleSections.value.flatMap((section) =>
    section.items.map((item) => ({
      ...item,
      sectionLabel: section.label
    }))
  )
);

const themeButton = computed(() => {
  if (currentTheme.value === "dark") return { icon: "i-lucide-moon", label: getThemeLabel("dark") };
  if (currentTheme.value === "apple") return { icon: "i-lucide-sparkles", label: getThemeLabel("apple") };
  if (currentTheme.value === "custom") return { icon: "i-lucide-palette", label: getThemeLabel("custom") };
  return { icon: "i-lucide-sun", label: getThemeLabel("light") };
});

const themeItems = computed(() => [
  { value: "light", label: getThemeLabel("light"), icon: "i-lucide-sun" },
  { value: "dark", label: getThemeLabel("dark"), icon: "i-lucide-moon" },
  { value: "apple", label: getThemeLabel("apple"), icon: "i-lucide-sparkles" },
  { value: "custom", label: getThemeLabel("custom"), icon: "i-lucide-palette", disabled: !hasCustomTheme.value }
]);

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

function isItemActive(item) {
  const workspaceId = String(item.workspaceId || "").trim();
  const itemPath = normalizePath(item.path);

  if (workspaceId && props.activeWorkspace === workspaceId) {
    return true;
  }

  return Boolean(item.path) && (currentPath.value === itemPath || currentPath.value.startsWith(`${itemPath}/`));
}

function isGroupActive(item) {
  return Array.isArray(item.children) && item.children.some(isItemActive);
}

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

function closeMenu() {
  menuOpen.value = false;
}

function toggleProfileMenu() {
  profileMenuOpen.value = !profileMenuOpen.value;
}

function selectTheme(value) {
  if (!value || value === "custom" && !hasCustomTheme.value) {
    return;
  }

  applyTheme(value);
}

async function toggleFullscreen() {
  if (!import.meta.client) {
    return;
  }

  if (document.fullscreenElement) {
    await document.exitFullscreen();
    return;
  }

  await document.documentElement.requestFullscreen();
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
    closeMenu();
  }
}

watch(
  () => route.fullPath,
  () => {
    closeProfileMenu();
    closeMenu();
  }
);

onMounted(() => {
  initializeFromStorage();
  document.addEventListener("pointerdown", handlePointerDown);
  document.addEventListener("keydown", handleEscape);
});

onBeforeUnmount(() => {
  document.removeEventListener("pointerdown", handlePointerDown);
  document.removeEventListener("keydown", handleEscape);
});
</script>

<template>
  <header class="dashboard-unified-header">
    <section class="dashboard-unified-header__brand-panel">
      <UButton
        icon="i-lucide-menu"
        color="neutral"
        variant="ghost"
        size="sm"
        aria-label="Abrir menu do sistema"
        @click="menuOpen = true"
      />
      <NuxtLink to="/tasks" class="dashboard-unified-header__brand-link" aria-label="Crow Visuals">
        <picture class="dashboard-unified-header__logo">
          <source srcset="/logo.avif" type="image/avif">
          <source srcset="/logo.webp" type="image/webp">
          <img src="/logo.png" alt="">
        </picture>
      </NuxtLink>
    </section>

    <section class="dashboard-unified-header__main-panel">
      <nav class="dashboard-unified-header__nav" aria-label="Menu principal">
        <template v-for="item in headerItems" :key="item.id">
          <UPopover
            v-if="item.children"
            :content="{ side: 'bottom', align: 'start' }"
          >
            <button
              class="dashboard-unified-header__nav-link"
              :class="{ 'is-active': isGroupActive(item) }"
              type="button"
            >
              <span>{{ item.label }}</span>
              <UIcon name="i-lucide-chevron-down" class="size-3.5" aria-hidden="true" />
            </button>

            <template #content>
              <div class="dashboard-unified-header__menu-popover">
                <NuxtLink
                  v-for="child in item.children"
                  :key="child.id"
                  :to="child.path"
                  class="dashboard-unified-header__menu-item"
                  :class="{ 'is-active': isItemActive(child) }"
                >
                  <span>{{ child.label }}</span>
                </NuxtLink>
              </div>
            </template>
          </UPopover>

          <NuxtLink
            v-else
            :to="item.path"
            class="dashboard-unified-header__nav-link"
            :class="{ 'is-active': isItemActive(item) }"
          >
            {{ item.label }}
          </NuxtLink>
        </template>
      </nav>

      <div class="dashboard-unified-header__actions">
        <div class="dashboard-unified-header__metrics" aria-label="Resumo da fila">
          <span class="summary-pill">{{ state.waitingList.length }} na fila</span>
          <span class="summary-pill summary-pill--active">{{ activeServicesCount }}/{{ state.settings.maxConcurrentServices }} em atendimento</span>
          <span class="summary-pill">{{ state.serviceHistory.length }} finalizados</span>
        </div>

        <AppSelectField
          class="summary-select dashboard-unified-header__store-select"
          :model-value="selectedStoreValue"
          :options="storeSelectOptions"
          placeholder="Selecionar loja"
          :show-leading-icon="false"
          compact
          @update:model-value="handleStoreChange"
        />

        <AppSelectField
          v-if="!isAuthenticated"
          class="summary-select dashboard-unified-header__profile-select"
          :model-value="state.activeProfileId"
          :options="profileSelectOptions"
          placeholder="Selecionar perfil"
          :show-leading-icon="false"
          compact
          @update:model-value="handleProfileChange"
        />

        <UPopover :content="{ side: 'bottom', align: 'end' }">
          <UButton :icon="themeButton.icon" color="neutral" variant="ghost" size="sm" :aria-label="themeButton.label" />
          <template #content>
            <div class="dashboard-unified-header__menu-popover dashboard-unified-header__menu-popover--sm">
              <button
                v-for="item in themeItems"
                :key="item.value"
                class="dashboard-unified-header__menu-item"
                :class="{ 'is-active': currentTheme === item.value }"
                type="button"
                :disabled="item.disabled"
                @click="selectTheme(item.value)"
              >
                <UIcon :name="item.icon" class="size-4" aria-hidden="true" />
                <span>{{ item.label }}</span>
              </button>
            </div>
          </template>
        </UPopover>

        <UButton icon="i-lucide-expand" color="neutral" variant="ghost" size="sm" aria-label="Tela cheia" @click="toggleFullscreen" />
        <FeedbackNotificationsDropdown v-if="isAuthenticated" />

        <div v-if="isAuthenticated" ref="profileMenuRef" class="dashboard-unified-header__profile-menu">
          <button
            class="dashboard-unified-header__profile-trigger"
            type="button"
            aria-haspopup="menu"
            :aria-expanded="profileMenuOpen ? 'true' : 'false'"
            aria-label="Abrir menu do perfil"
            @click="toggleProfileMenu"
          >
            <span class="dashboard-unified-header__profile-copy">
              <strong>{{ displayName || profileEmail || "Conta" }}</strong>
              <span>{{ profileRoleLabel }} / Root</span>
            </span>
            <span class="dashboard-unified-header__profile-avatar" aria-hidden="true">
              <img v-if="avatarUrl" :src="avatarUrl" alt="">
              <span v-else>{{ profileInitial }}</span>
            </span>
          </button>

          <Transition name="dashboard-unified-header-menu">
            <div v-if="profileMenuOpen" class="dashboard-unified-header__profile-dropdown" role="menu">
              <div class="dashboard-unified-header__profile-card">
                <span>{{ profileRoleLabel }}</span>
                <strong>{{ displayName || "Conta autenticada" }}</strong>
                <small>{{ profileEmail || "Sessao ativa" }}</small>
              </div>

              <NuxtLink class="dashboard-unified-header__menu-action" to="/perfil" role="menuitem" @click="closeProfileMenu">
                <UIcon name="i-lucide-user-round" class="size-4" aria-hidden="true" />
                <span>Pagina de perfil</span>
              </NuxtLink>

              <button class="dashboard-unified-header__menu-action dashboard-unified-header__menu-action--danger" type="button" role="menuitem" @click="handleLogout">
                <UIcon name="i-lucide-log-out" class="size-4" aria-hidden="true" />
                <span>Sair da plataforma</span>
              </button>
            </div>
          </Transition>
        </div>
      </div>
    </section>

    <USlideover v-model:open="menuOpen" side="left" title="Sistema" description="Menu completo do painel">
      <template #body>
        <DashboardSidebarNav
          class="dashboard-unified-header__drawer-nav"
          :active-workspace="activeWorkspace"
          :allowed-workspaces="allowedWorkspaces"
          always-expanded
        />
      </template>
    </USlideover>
  </header>
</template>

<style scoped>
.dashboard-unified-header {
  position: sticky;
  top: 0;
  z-index: 60;
  display: grid;
  grid-template-columns: minmax(15rem, 18rem) minmax(0, 1fr);
  gap: 0.75rem;
  padding: 0.75rem;
  background: rgb(var(--bg) / 0.92);
  backdrop-filter: blur(16px);
}

.dashboard-unified-header__brand-panel,
.dashboard-unified-header__main-panel {
  min-height: 4rem;
  display: flex;
  align-items: center;
  border: 1px solid var(--admin-header-border);
  border-radius: var(--radius-md);
  background: var(--admin-header-panel-bg);
  box-shadow: var(--admin-header-shell-shadow);
  color: var(--admin-header-text);
  backdrop-filter: blur(var(--admin-header-panel-blur));
}

.dashboard-unified-header__brand-panel {
  gap: 0.75rem;
  padding: 0 1rem;
}

.dashboard-unified-header__brand-link {
  display: inline-flex;
  align-items: center;
  min-width: 0;
  text-decoration: none;
}

.dashboard-unified-header__logo {
  display: inline-flex;
  width: clamp(5.6rem, 9vw, 7.25rem);
}

.dashboard-unified-header__logo img {
  width: 100%;
  height: auto;
  display: block;
}

.dashboard-unified-header__main-panel {
  min-width: 0;
  gap: 0.85rem;
  padding: 0 0.8rem 0 1rem;
}

.dashboard-unified-header__nav {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 0.25rem;
  overflow-x: auto;
  scrollbar-width: none;
}

.dashboard-unified-header__nav::-webkit-scrollbar {
  display: none;
}

.dashboard-unified-header__nav-link {
  position: relative;
  min-height: 2.65rem;
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  flex: 0 0 auto;
  padding: 0 0.68rem;
  border: 0;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--admin-header-muted);
  font-size: 0.84rem;
  font-weight: 800;
  text-decoration: none;
  cursor: pointer;
  white-space: nowrap;
  transition: color 0.16s ease, background 0.16s ease;
}

.dashboard-unified-header__nav-link:hover {
  color: var(--admin-header-text);
  background: var(--admin-header-hover-bg);
}

.dashboard-unified-header__nav-link.is-active {
  color: var(--admin-header-text);
}

.dashboard-unified-header__nav-link.is-active::after {
  content: "";
  position: absolute;
  inset-inline: 0.65rem;
  bottom: 0.2rem;
  height: 2px;
  border-radius: 999px;
  background: rgb(var(--primary));
}

.dashboard-unified-header__actions {
  margin-left: auto;
  min-width: max-content;
  display: flex;
  align-items: center;
  gap: 0.45rem;
}

.dashboard-unified-header__metrics {
  display: flex;
  align-items: center;
  gap: 0.42rem;
}

.dashboard-unified-header__store-select {
  width: 13rem;
}

.dashboard-unified-header__profile-select {
  width: 12rem;
}

.dashboard-unified-header__store-select :deep(.app-select-field__trigger),
.dashboard-unified-header__profile-select :deep(.app-select-field__trigger) {
  min-height: 2.55rem;
  padding: 0 0.85rem;
  border-radius: 999px;
  border-color: var(--admin-header-border);
  background: var(--admin-header-hover-bg);
  color: var(--admin-header-text);
}

.dashboard-unified-header__menu-popover {
  min-width: 13rem;
  display: grid;
  gap: 0.2rem;
  padding: 0.35rem;
  border: 1px solid var(--admin-header-border);
  border-radius: var(--radius-sm);
  background: var(--admin-header-panel-bg);
  box-shadow: var(--shadow-md);
  backdrop-filter: blur(var(--admin-header-panel-blur));
}

.dashboard-unified-header__menu-popover--sm {
  min-width: 11.5rem;
}

.dashboard-unified-header__menu-item {
  min-height: 2.25rem;
  display: flex;
  align-items: center;
  gap: 0.55rem;
  border: 0;
  border-radius: var(--radius-sm);
  padding: 0 0.65rem;
  background: transparent;
  color: var(--admin-header-text);
  font-size: 0.82rem;
  font-weight: 750;
  text-align: left;
  text-decoration: none;
  cursor: pointer;
}

.dashboard-unified-header__menu-item:hover,
.dashboard-unified-header__menu-item.is-active {
  background: var(--admin-header-hover-bg);
}

.dashboard-unified-header__menu-item:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.dashboard-unified-header__profile-menu {
  position: relative;
}

.dashboard-unified-header__profile-trigger {
  display: inline-flex;
  align-items: center;
  gap: 0.65rem;
  min-height: 2.65rem;
  border: 0;
  border-left: 1px solid var(--admin-header-separator);
  padding: 0 0 0 0.85rem;
  background: transparent;
  color: var(--admin-header-text);
  cursor: pointer;
}

.dashboard-unified-header__profile-copy {
  display: grid;
  gap: 0.1rem;
  text-align: right;
}

.dashboard-unified-header__profile-copy strong {
  max-width: 9rem;
  overflow: hidden;
  color: var(--admin-header-text);
  font-size: 0.82rem;
  line-height: 1;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dashboard-unified-header__profile-copy span {
  max-width: 9rem;
  overflow: hidden;
  color: var(--admin-header-muted);
  font-size: 0.72rem;
  line-height: 1.1;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dashboard-unified-header__profile-avatar {
  display: grid;
  place-items: center;
  width: 2.55rem;
  height: 2.55rem;
  overflow: hidden;
  border-radius: 999px;
  background: linear-gradient(135deg, rgb(var(--primary)), rgb(var(--success)));
  color: #f8fafc;
  font-size: 0.9rem;
  font-weight: 900;
  box-shadow: 0 0 0 3px rgb(var(--primary) / 0.28);
}

.dashboard-unified-header__profile-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.dashboard-unified-header__profile-dropdown {
  position: absolute;
  top: calc(100% + 0.55rem);
  right: 0;
  z-index: 30;
  display: grid;
  gap: 0.55rem;
  width: min(18.5rem, calc(100vw - 2rem));
  padding: 0.8rem;
  border: 1px solid var(--admin-header-border);
  border-radius: var(--radius-md);
  background: var(--admin-header-panel-bg);
  box-shadow: var(--shadow-md);
  backdrop-filter: blur(var(--admin-header-panel-blur));
}

.dashboard-unified-header__profile-card {
  display: grid;
  gap: 0.24rem;
  padding: 0.8rem 0.85rem;
  border: 1px solid rgb(var(--primary) / 0.16);
  border-radius: var(--radius-sm);
  background: var(--admin-header-hover-bg);
}

.dashboard-unified-header__profile-card span {
  color: rgb(var(--primary));
  font-size: 0.64rem;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.dashboard-unified-header__profile-card strong {
  color: var(--admin-header-text);
  font-size: 0.95rem;
  font-weight: 800;
}

.dashboard-unified-header__profile-card small {
  color: var(--admin-header-muted);
  font-size: 0.75rem;
  word-break: break-word;
}

.dashboard-unified-header__menu-action {
  display: flex;
  align-items: center;
  gap: 0.65rem;
  min-height: 2.5rem;
  border: 1px solid var(--admin-header-border);
  border-radius: var(--radius-sm);
  padding: 0 0.75rem;
  background: transparent;
  color: var(--admin-header-text);
  font-size: 0.82rem;
  font-weight: 750;
  text-decoration: none;
  cursor: pointer;
}

.dashboard-unified-header__menu-action:hover {
  background: var(--admin-header-hover-bg);
}

.dashboard-unified-header__menu-action--danger {
  color: #fecaca;
  background: rgba(127, 29, 29, 0.18);
  border-color: rgba(248, 113, 113, 0.16);
}

.dashboard-unified-header__drawer-nav {
  width: 100%;
  height: min(78vh, 48rem);
}

.dashboard-unified-header-menu-enter-active,
.dashboard-unified-header-menu-leave-active {
  transition: opacity 0.18s ease, transform 0.18s ease;
}

.dashboard-unified-header-menu-enter-from,
.dashboard-unified-header-menu-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}

@media (max-width: 1280px) {
  .dashboard-unified-header__metrics {
    display: none;
  }
}

@media (max-width: 1080px) {
  .dashboard-unified-header {
    grid-template-columns: minmax(0, 1fr);
  }

  .dashboard-unified-header__brand-panel {
    min-height: 3.6rem;
  }

  .dashboard-unified-header__main-panel {
    min-height: 3.5rem;
  }
}

@media (max-width: 760px) {
  .dashboard-unified-header {
    padding: 0.6rem;
  }

  .dashboard-unified-header__main-panel {
    align-items: stretch;
    flex-direction: column;
    padding: 0.65rem;
  }

  .dashboard-unified-header__actions {
    width: 100%;
    min-width: 0;
    justify-content: space-between;
  }

  .dashboard-unified-header__store-select,
  .dashboard-unified-header__profile-select {
    width: min(14rem, 100%);
  }

  .dashboard-unified-header__profile-copy {
    display: none;
  }
}
</style>
