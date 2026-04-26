<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { LogOut, User } from "lucide-vue-next";
import { storeToRefs } from "pinia";
import AppSelectField from "~/components/ui/AppSelectField.vue";
import { getRoleLabel } from "~/domain/utils/permissions";
import { useAuthStore } from "~/stores/auth";
import { getApiBase } from "~/utils/api-client";

const ALL_STORES_VALUE = "__all_stores__";

const props = defineProps({
  state: {
    type: Object,
    required: true
  }
});

const emit = defineEmits(["store-change", "profile-change"]);
const auth = useAuthStore();
const { isAuthenticated, user, role, accessibleStoreIds, canUseAllStores, isAllStoresScope } = storeToRefs(auth);
const runtimeConfig = useRuntimeConfig();
const route = useRoute();
const profileMenuRef = ref(null);
const profileMenuOpen = ref(false);

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
  if (!avatarPath) {
    return "";
  }

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
  }
}

watch(
  () => route.fullPath,
  () => {
    closeProfileMenu();
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
        <picture class="dashboard-header__logo" aria-label="Logo da plataforma">
          <source srcset="/logo.avif" type="image/avif">
          <source srcset="/logo.webp" type="image/webp">
          <img src="/logo.png" alt="Logo da plataforma">
        </picture>
      </div>
      <div class="brand__meta dashboard-header__meta">
        <span class="summary-pill">{{ state.waitingList.length }} na fila</span>
        <span
          class="summary-pill"
          :class="{ 'summary-pill--active': activeServicesCount > 0 }"
        >
          {{ activeServicesCount }}/{{ state.settings.maxConcurrentServices }} em atendimento
        </span>
        <span class="summary-pill">{{ state.serviceHistory.length }} finalizados</span>
        <AppSelectField
          class="summary-select dashboard-header__store-select"
          :model-value="selectedStoreValue"
          :options="storeSelectOptions"
          placeholder="Selecionar loja"
          :show-leading-icon="false"
          compact
          @update:model-value="handleStoreChange"
        />
        <AppSelectField
          v-if="!isAuthenticated"
          class="summary-select"
          :model-value="state.activeProfileId"
          :options="profileSelectOptions"
          placeholder="Selecionar perfil"
          :show-leading-icon="false"
          compact
          @update:model-value="handleProfileChange"
        />
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
  </header>
</template>

<style scoped>
.dashboard-header {
  background: linear-gradient(180deg, rgba(5, 10, 18, 0.98) 0%, rgba(10, 16, 28, 0.98) 100%);
  border-bottom: 1px solid rgba(137, 151, 185, 0.18);
  box-shadow: 0 16px 34px rgba(0, 0, 0, 0.24);
}

.dashboard-header__bar {
  width: min(100%, 1240px);
  gap: 1rem;
  padding: 0.85rem 1rem;
}

.dashboard-header__brand {
  min-width: 0;
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

.dashboard-header__meta {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.7rem;
  flex-wrap: wrap;
}

.dashboard-header__store-select {
  min-width: 13.5rem;
}

.dashboard-header__store-select :deep(.app-select-field__trigger) {
  min-height: 2.6rem;
  padding: 0 0.85rem;
  border-radius: 999px;
  border-color: rgba(255, 255, 255, 0.14);
  background: rgba(255, 255, 255, 0.08);
  color: #f8fafc;
}

.dashboard-header__store-select :deep(.app-select-field__trigger:hover),
.dashboard-header__store-select :deep(.app-select-field__trigger.is-open),
.dashboard-header__store-select :deep(.app-select-field__trigger.is-filled) {
  border-color: rgba(118, 138, 255, 0.42);
  background: rgba(255, 255, 255, 0.12);
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
  border: 1px solid rgba(255, 255, 255, 0.14);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.08);
  color: #f8fafc;
  cursor: pointer;
  transition: border-color 0.18s ease, background 0.18s ease, transform 0.18s ease;
}

.dashboard-header__profile-trigger:hover,
.dashboard-header__profile-trigger[aria-expanded="true"] {
  border-color: rgba(118, 138, 255, 0.42);
  background: rgba(255, 255, 255, 0.12);
}

.dashboard-header__profile-avatar {
  display: grid;
  place-items: center;
  width: 2.3rem;
  height: 2.3rem;
  border-radius: 999px;
  overflow: hidden;
  background: linear-gradient(135deg, rgba(118, 138, 255, 0.92), rgba(45, 212, 191, 0.9));
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
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 1rem;
  background: rgba(8, 13, 24, 0.96);
  box-shadow: 0 22px 48px rgba(0, 0, 0, 0.32);
  backdrop-filter: blur(18px);
}

.dashboard-header__profile-card {
  display: grid;
  gap: 0.24rem;
  padding: 0.8rem 0.85rem;
  border: 1px solid rgba(118, 138, 255, 0.16);
  border-radius: 0.85rem;
  background: linear-gradient(180deg, rgba(20, 28, 46, 0.92), rgba(12, 18, 31, 0.94));
}

.dashboard-header__profile-role {
  color: rgba(147, 197, 253, 0.9);
  font-size: 0.64rem;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.dashboard-header__profile-fullname {
  color: #f8fafc;
  font-size: 0.95rem;
  font-weight: 700;
}

.dashboard-header__profile-email {
  color: rgba(226, 232, 240, 0.68);
  font-size: 0.75rem;
  word-break: break-word;
}

.dashboard-header__menu-action {
  display: flex;
  align-items: center;
  gap: 0.7rem;
  width: 100%;
  padding: 0.82rem 0.88rem;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 0.85rem;
  background: rgba(255, 255, 255, 0.04);
  color: #f8fafc;
  text-decoration: none;
  font-size: 0.82rem;
  font-weight: 700;
  cursor: pointer;
  transition: border-color 0.18s ease, background 0.18s ease, color 0.18s ease;
}

.dashboard-header__menu-action:hover {
  border-color: rgba(118, 138, 255, 0.28);
  background: rgba(118, 138, 255, 0.1);
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

@media (max-width: 900px) {
  .dashboard-header__bar {
    align-items: stretch;
    flex-direction: column;
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
