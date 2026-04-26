import { computed, ref, watch } from "vue";
import { defineStore, storeToRefs } from "pinia";
import { normalizeServiceHistoryList } from "~/stores/dashboard/runtime/state";
import { useAuthStore } from "~/stores/auth";
import { useAppRuntimeStore } from "~/stores/app-runtime";
import { createApiRequest, getApiErrorMessage } from "~/utils/api-client";

function normalizeText(value) {
  return String(value || "").trim();
}

export const useCampaignsStore = defineStore("campaigns", () => {
  const runtimeConfig = useRuntimeConfig();
  const runtime = useAppRuntimeStore();
  const auth = useAuthStore();
  const { state } = storeToRefs(runtime);
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken);
  const integratedHistory = ref([]);
  const integratedPending = ref(false);
  const integratedReady = ref(false);
  const integratedError = ref("");
  const integratedScopeKey = ref("");

  const campaigns = computed(() => state.value.campaigns || []);
  const accessibleStores = computed(() => {
    const allowedStoreIds = new Set(
      Array.isArray(auth.accessibleStoreIds)
        ? auth.accessibleStoreIds.map((storeId) => normalizeText(storeId)).filter(Boolean)
        : []
    );

    return (auth.storeContext || []).filter((store) => {
      const storeId = normalizeText(store?.id);
      return !allowedStoreIds.size || allowedStoreIds.has(storeId);
    });
  });
  const activeTenantId = computed(() =>
    normalizeText(auth.activeTenantId || auth.tenantContext?.[0]?.id)
  );

  function clearIntegratedHistory() {
    integratedHistory.value = [];
    integratedPending.value = false;
    integratedReady.value = false;
    integratedError.value = "";
    integratedScopeKey.value = "";
  }

  function buildIntegratedScopeKey() {
    return JSON.stringify({
      tenantId: activeTenantId.value,
      storeIds: accessibleStores.value
        .map((store) => normalizeText(store?.id))
        .filter(Boolean)
        .sort()
    });
  }

  async function refreshIntegratedHistory() {
    if (auth.isAuthenticated) {
      await auth.ensureSession();
    }

    const tenantId = activeTenantId.value;
    const stores = accessibleStores.value;

    if (!tenantId || !auth.isAuthenticated || !stores.length) {
      clearIntegratedHistory();
      return [];
    }

    integratedPending.value = true;
    integratedError.value = "";

    try {
      const scopeKey = buildIntegratedScopeKey();
      const snapshots = await Promise.all(stores.map(async (store) => ({
        store,
        snapshot: await apiRequest(`/v1/operations/snapshot?storeId=${encodeURIComponent(store.id)}`)
      })));

      integratedHistory.value = snapshots.flatMap(({ store, snapshot }) =>
        normalizeServiceHistoryList(snapshot?.serviceHistory, store.id, store.name, Date.now())
      );
      integratedReady.value = true;
      integratedScopeKey.value = scopeKey;

      return integratedHistory.value;
    } catch (error) {
      integratedError.value = getApiErrorMessage(error, "Nao foi possivel carregar o historico consolidado das campanhas.");
      throw error;
    } finally {
      integratedPending.value = false;
    }
  }

  async function ensureIntegratedHistory() {
    const scopeKey = buildIntegratedScopeKey();

    if (integratedReady.value && integratedScopeKey.value === scopeKey) {
      return integratedHistory.value;
    }

    try {
      return await refreshIntegratedHistory();
    } catch {
      return [];
    }
  }

  if (import.meta.client) {
    watch(
      () => [auth.isAuthenticated, activeTenantId.value, accessibleStores.value.length],
      ([isAuthenticated, tenantId, storeCount], [previousAuthenticated, previousTenantId, previousStoreCount]) => {
        if (!isAuthenticated || !tenantId || storeCount < 1) {
          clearIntegratedHistory();
          return;
        }

        if (
          !previousAuthenticated ||
          previousTenantId !== tenantId ||
          previousStoreCount !== storeCount
        ) {
          clearIntegratedHistory();
        }
      }
    );
  }

  return {
    state,
    campaigns,
    integratedHistory,
    integratedPending,
    integratedReady,
    integratedError,
    ensure: runtime.ensure,
    ensureIntegratedHistory,
    refreshIntegratedHistory,
    clearIntegratedHistory,
    createCampaign(payload) {
      return runtime.run("createCampaign", payload);
    },
    updateCampaign(campaignId, patch) {
      return runtime.run("updateCampaign", campaignId, patch);
    },
    removeCampaign(campaignId) {
      return runtime.run("removeCampaign", campaignId);
    }
  };
});
