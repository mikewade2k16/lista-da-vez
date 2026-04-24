<script setup>
import { computed } from "vue";
import { storeToRefs } from "pinia";
import AppSelectField from "~/components/ui/AppSelectField.vue";
import { getRoleLabel } from "~/domain/utils/permissions";
import { useAuthStore } from "~/stores/auth";

const props = defineProps({
  state: {
    type: Object,
    required: true
  }
});

const emit = defineEmits(["store-change", "profile-change"]);
const auth = useAuthStore();
const { isAuthenticated, user, role, accessibleStoreIds } = storeToRefs(auth);

const availableStores = computed(() => {
  if (!isAuthenticated.value || !accessibleStoreIds.value.length) {
    return props.state.stores || [];
  }

  const allowedStoreIds = new Set(accessibleStoreIds.value);
  return (props.state.stores || []).filter((store) => allowedStoreIds.has(store.id));
});

const activeStore = computed(() =>
  availableStores.value.find((store) => store.id === props.state.activeStoreId) ||
  (props.state.stores || []).find((store) => store.id === props.state.activeStoreId) ||
  null
);
const activeServicesCount = computed(() => props.state.activeServices?.length || 0);
const storeSelectOptions = computed(() =>
  availableStores.value.map((store) => ({
    value: String(store.id || "").trim(),
    label: String(store.name || "").trim()
  }))
);
const profileSelectOptions = computed(() =>
  (props.state.profiles || []).map((profile) => ({
    value: String(profile.id || "").trim(),
    label: String(profile.name || "").trim()
  }))
);
const accountLabel = computed(() => {
  if (!isAuthenticated.value || !user.value) {
    return "";
  }

  return `${user.value.displayName} · ${getRoleLabel(role.value)}`;
});

function handleStoreChange(value) {
  emit("store-change", String(value || "").trim());
}

function handleProfileChange(value) {
  emit("profile-change", String(value || "").trim());
}

async function handleLogout() {
  await auth.logout();
  await navigateTo("/auth/login", { replace: true });
}
</script>

<template>
  <header class="app-header">
    <div class="brand-bar">
      <div class="brand">
        <span class="brand__name">{{ state.brandName }}</span>
        <span class="brand__sub">
          {{ state.pageTitle }}<template v-if="activeStore"> | {{ activeStore.name }}</template>
        </span>
      </div>
      <div class="brand__meta">
        <span class="summary-pill">{{ state.waitingList.length }} na fila</span>
        <span
          class="summary-pill"
          :class="{ 'summary-pill--active': activeServicesCount > 0 }"
        >
          {{ activeServicesCount }}/{{ state.settings.maxConcurrentServices }} em atendimento
        </span>
        <span class="summary-pill">{{ state.serviceHistory.length }} finalizados</span>
        <span v-if="isAuthenticated" class="summary-pill summary-pill--account">{{ accountLabel }}</span>
        <AppSelectField
          class="summary-select"
          :model-value="state.activeStoreId"
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
        <NuxtLink v-if="isAuthenticated" class="summary-action summary-action--ghost" to="/perfil">Perfil</NuxtLink>
        <button v-if="isAuthenticated" class="summary-action" type="button" @click="handleLogout">Sair</button>
      </div>
    </div>
  </header>
</template>
