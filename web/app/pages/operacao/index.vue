<script setup>
import { computed, onMounted, ref, watch } from "vue";
import OperationWorkspace from "~/features/operation/components/OperationWorkspace.vue";
import { storeToRefs } from "pinia";
import { useAuthStore } from "~/stores/auth";
import { useOperationsStore } from "~/stores/operations";
import { useOperationsRealtime } from "~/composables/useOperationsRealtime";
import { canUseAllStoresScope } from "~/domain/utils/permissions";
import { getApiErrorMessage } from "~/utils/api-client";

definePageMeta({
  layout: "dashboard",
  workspaceId: "operacao",
  supportsAllStoresScope: true
});

const auth = useAuthStore();
const operationsStore = useOperationsStore();
const loadError = ref("");
const integratedStoreId = ref("");
const storeOptions = computed(() => auth.storeContext || []);
const { isAllStoresScope } = storeToRefs(auth);

const canSeeIntegrated = computed(() => canUseAllStoresScope(auth.accessibleStoreIds));
const scopeMode = computed(() => {
  if (!canSeeIntegrated.value) {
    return "single";
  }

  return isAllStoresScope.value ? "all" : "single";
});

useOperationsRealtime({ scopeMode });

async function loadOperationView() {
  if (!auth.isAuthenticated) {
    return;
  }

  try {
    loadError.value = "";

    if (scopeMode.value === "all" && canSeeIntegrated.value) {
      await operationsStore.refreshOverview();
      return;
    }

    operationsStore.clearOverview();
    await operationsStore.refreshActiveStore();
  } catch (error) {
    loadError.value = getApiErrorMessage(error, "Nao foi possivel carregar a operacao.");
  }
}

onMounted(async () => {
  await auth.ensureSession();
  await loadOperationView();
});

const { state, overview, overviewPending, overviewError } = storeToRefs(operationsStore);

const isRemoteRosterReady = computed(() => {
  if (scopeMode.value === "all" && canSeeIntegrated.value) {
    return !overviewPending.value || Boolean(overview.value);
  }

  if (!auth.isAuthenticated || loadError.value) {
    return false;
  }

  const activeStoreId = String(auth.activeStoreId || state.value?.activeStoreId || "").trim();
  const roster = Array.isArray(state.value?.roster) ? state.value.roster : [];

  if (!activeStoreId) {
    return false;
  }

  if (roster.length === 0) {
    return true;
  }

  return roster.every((consultant) => String(consultant?.storeId || "").trim() === activeStoreId);
});

const pageErrorMessage = computed(() => {
  if (loadError.value) {
    return loadError.value;
  }

  if (scopeMode.value === "all" && overviewError.value) {
    return overviewError.value;
  }

  return "";
});

watch(
  [scopeMode, () => auth.activeStoreId, () => auth.isAuthenticated],
  () => {
    void loadOperationView();
  }
);

watch(
  storeOptions,
  (stores) => {
    const normalizedFilter = String(integratedStoreId.value || "").trim();
    if (!normalizedFilter) {
      return;
    }

    const exists = (stores || []).some((store) => String(store?.id || "").trim() === normalizedFilter);
    if (!exists) {
      integratedStoreId.value = "";
    }
  },
  { immediate: true }
);

watch(scopeMode, (nextMode) => {
  if (nextMode !== "all") {
    integratedStoreId.value = "";
  }
});

function handleIntegratedStoreChange(storeId) {
  integratedStoreId.value = String(storeId || "").trim();
}
</script>

<template>
  <div class="workspace-host">
    <div v-if="pageErrorMessage" class="loading-state">
      <strong class="loading-state__title">Nao foi possivel carregar a operacao</strong>
      <p class="workspace__text">{{ pageErrorMessage }}</p>
    </div>
    <div v-else-if="!isRemoteRosterReady" class="loading-state">
      <strong class="loading-state__title">Carregando operacao...</strong>
      <p class="workspace__text">
        {{ scopeMode === "all" ? "Sincronizando a operacao integrada das lojas acessiveis." : "Sincronizando consultores, fila e atendimento da loja ativa." }}
      </p>
    </div>
    <OperationWorkspace
      v-else
      :state="state"
      :overview="overview"
      :scope-mode="scopeMode"
      :can-see-integrated="canSeeIntegrated"
      :stores="storeOptions"
      :integrated-store-id="integratedStoreId"
      @integrated-store-change="handleIntegratedStoreChange"
    />
  </div>
</template>
