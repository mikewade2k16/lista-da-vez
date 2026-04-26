import { computed, ref, watch } from "vue";
import { defineStore, storeToRefs } from "pinia";

import { buildRankingRows } from "~/domain/utils/admin-metrics";
import { useAuthStore } from "~/stores/auth";
import { useAppRuntimeStore } from "~/stores/app-runtime";
import { normalizeServiceHistoryList } from "~/stores/dashboard/runtime/state";
import { createApiRequest, getApiErrorMessage } from "~/utils/api-client";
import { hydrateRuntimeStoreContext } from "~/utils/runtime-remote";

function normalizeText(value) {
  return String(value || "").trim();
}

function normalizeConsultantList(consultants = [], fallbackStore = {}) {
  return (Array.isArray(consultants) ? consultants : []).map((consultant) => ({
    id: normalizeText(consultant?.id),
    storeId: normalizeText(consultant?.storeId) || normalizeText(fallbackStore?.id),
    storeName: normalizeText(fallbackStore?.name),
    storeCode: normalizeText(fallbackStore?.code),
    storeCity: normalizeText(fallbackStore?.city),
    name: normalizeText(consultant?.name),
    role: normalizeText(consultant?.role) || "Atendimento",
    initials: normalizeText(consultant?.initials),
    color: normalizeText(consultant?.color) || "#168aad",
    monthlyGoal: Math.max(0, Number(consultant?.monthlyGoal || 0) || 0),
    commissionRate: Math.max(0, Number(consultant?.commissionRate || 0) || 0),
    conversionGoal: Math.max(0, Number(consultant?.conversionGoal || 0) || 0),
    avgTicketGoal: Math.max(0, Number(consultant?.avgTicketGoal || 0) || 0),
    paGoal: Math.max(0, Number(consultant?.paGoal || 0) || 0),
    active: Boolean(consultant?.active ?? true),
    access: consultant?.access && typeof consultant.access === "object"
      ? {
          userId: normalizeText(consultant.access?.userId),
          email: normalizeText(consultant.access?.email).toLowerCase(),
          active: Boolean(consultant.access?.active ?? false)
        }
      : null
  })).filter((consultant) => consultant.id && consultant.name);
}

function buildIntegratedRankingResponse(tenantId, roster = [], serviceHistory = []) {
  const rosterByConsultantId = new Map(
    (Array.isArray(roster) ? roster : []).map((consultant) => [normalizeText(consultant?.id), consultant])
  );
  const mapRows = (rows) =>
    rows.map((row) => {
      const consultant = rosterByConsultantId.get(normalizeText(row?.consultantId));

      return {
        ...row,
        storeId: normalizeText(consultant?.storeId),
        storeName: normalizeText(consultant?.storeName)
      };
    });

  return {
    storeId: "",
    tenantId: normalizeText(tenantId),
    monthlyRows: mapRows(buildRankingRows({ history: serviceHistory, roster, scope: "month" })),
    dailyRows: mapRows(buildRankingRows({ history: serviceHistory, roster, scope: "today" })),
    alerts: []
  };
}

export const useConsultantsStore = defineStore("consultants", () => {
  const runtimeConfig = useRuntimeConfig();
  const runtime = useAppRuntimeStore();
  const auth = useAuthStore();
  const { state } = storeToRefs(runtime);
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken);
  const integratedRoster = ref([]);
  const integratedRanking = ref(null);
  const integratedOverview = ref(null);
  const integratedPending = ref(false);
  const integratedReady = ref(false);
  const integratedError = ref("");
  const integratedScopeKey = ref("");

  const roster = computed(() => state.value.roster || []);
  const selectedConsultantId = computed(() => state.value.selectedConsultantId || null);
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

  async function resolveActiveStoreId() {
    await runtime.ensure();

    if (auth.isAuthenticated) {
      await auth.ensureSession();
    }

    return String(auth.activeStoreId || runtime.state.activeStoreId || "").trim();
  }

  function canArchiveConsultantLocally(consultantId) {
    const currentState = runtime.state;
    const isInQueue = (currentState.waitingList || []).some((item) => item.id === consultantId);
    const isInService = (currentState.activeServices || []).some((item) => item.id === consultantId);
    const isPaused = (currentState.pausedEmployees || []).some((item) => item.personId === consultantId);

    if (isInQueue || isInService || isPaused) {
      return {
        ok: false,
        message: "Retire o consultor de fila, atendimento ou pausa antes de arquivar."
      };
    }

    return { ok: true };
  }

  function normalizeConsultantPayload(payload = {}) {
    return {
      name: String(payload?.name || "").trim(),
      role: String(payload?.role || "").trim(),
      color: String(payload?.color || "").trim(),
      monthlyGoal: Math.max(0, Number(payload?.monthlyGoal || 0) || 0),
      commissionRate: Math.max(0, Number(payload?.commissionRate || 0) || 0),
      conversionGoal: Math.max(0, Number(payload?.conversionGoal || 0) || 0),
      avgTicketGoal: Math.max(0, Number(payload?.avgTicketGoal || 0) || 0),
      paGoal: Math.max(0, Number(payload?.paGoal || 0) || 0)
    };
  }

  async function refreshActiveStore() {
    const storeId = await resolveActiveStoreId();

    if (!storeId || !auth.isAuthenticated) {
      return null;
    }

    return hydrateRuntimeStoreContext(runtime, apiRequest, storeId, activeTenantId.value);
  }

  function clearIntegratedView() {
    integratedRoster.value = [];
    integratedRanking.value = null;
    integratedOverview.value = null;
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

  async function refreshIntegratedView() {
    await runtime.ensure();

    if (auth.isAuthenticated) {
      await auth.ensureSession();
    }

    const tenantId = activeTenantId.value;
    const stores = accessibleStores.value;

    if (!tenantId || !auth.isAuthenticated || !stores.length) {
      clearIntegratedView();
      return null;
    }

    integratedPending.value = true;
    integratedError.value = "";

    try {
      const scopeKey = buildIntegratedScopeKey();
      const [overviewResponse, storeResponses] = await Promise.all([
        apiRequest("/v1/operations/overview"),
        Promise.all(stores.map(async (store) => {
          const [consultantsResponse, snapshotResponse] = await Promise.all([
            apiRequest(`/v1/consultants?storeId=${encodeURIComponent(store.id)}`),
            apiRequest(`/v1/operations/snapshot?storeId=${encodeURIComponent(store.id)}`)
          ]);

          return {
            store,
            consultants: Array.isArray(consultantsResponse?.consultants) ? consultantsResponse.consultants : [],
            snapshot: snapshotResponse || {}
          };
        }))
      ]);

      integratedRoster.value = storeResponses.flatMap(({ store, consultants }) =>
        normalizeConsultantList(consultants, store)
      );
      integratedRanking.value = buildIntegratedRankingResponse(
        tenantId,
        integratedRoster.value,
        storeResponses.flatMap(({ store, snapshot }) =>
          normalizeServiceHistoryList(snapshot?.serviceHistory, store.id, store.name, Date.now())
        )
      );
      integratedOverview.value = overviewResponse;
      integratedReady.value = true;
      integratedScopeKey.value = scopeKey;

      return {
        roster: integratedRoster.value,
        ranking: integratedRanking.value,
        overview: integratedOverview.value
      };
    } catch (error) {
      integratedError.value = getApiErrorMessage(error, "Nao foi possivel carregar o comparativo dos consultores.");
      throw error;
    } finally {
      integratedPending.value = false;
    }
  }

  async function ensureIntegratedView() {
    const scopeKey = buildIntegratedScopeKey();

    if (integratedReady.value && integratedScopeKey.value === scopeKey) {
      return {
        roster: integratedRoster.value,
        ranking: integratedRanking.value,
        overview: integratedOverview.value
      };
    }

    try {
      return await refreshIntegratedView();
    } catch {
      return null;
    }
  }

  async function createConsultantProfile(payload) {
    const storeId = await resolveActiveStoreId();

    if (!storeId || !auth.isAuthenticated) {
      return { ok: false, message: "Sessao ou loja ativa indisponivel." };
    }

    try {
      const response = await apiRequest("/v1/consultants", {
        method: "POST",
        body: {
          storeId,
          ...normalizeConsultantPayload(payload)
        }
      });

      await hydrateRuntimeStoreContext(runtime, apiRequest, storeId, activeTenantId.value);
      return {
        ok: true,
        consultant: response?.consultant || null,
        access: response?.access || null
      };
    } catch (error) {
      return {
        ok: false,
        message: getApiErrorMessage(error, "Nao foi possivel criar consultor.")
      };
    }
  }

  async function updateConsultantProfile(consultantId, payload) {
    const storeId = await resolveActiveStoreId();

    if (!storeId || !auth.isAuthenticated) {
      return { ok: false, message: "Sessao ou loja ativa indisponivel." };
    }

    try {
      await apiRequest(`/v1/consultants/${consultantId}`, {
        method: "PATCH",
        body: normalizeConsultantPayload(payload)
      });

      await hydrateRuntimeStoreContext(runtime, apiRequest, storeId, activeTenantId.value);
      return { ok: true };
    } catch (error) {
      return {
        ok: false,
        message: getApiErrorMessage(error, "Nao foi possivel atualizar consultor.")
      };
    }
  }

  async function archiveConsultantProfile(consultantId) {
    const storeId = await resolveActiveStoreId();

    if (!storeId || !auth.isAuthenticated) {
      return { ok: false, message: "Sessao ou loja ativa indisponivel." };
    }

    const localValidation = canArchiveConsultantLocally(consultantId);
    if (localValidation.ok === false) {
      return localValidation;
    }

    try {
      await apiRequest(`/v1/consultants/${consultantId}/archive`, {
        method: "POST"
      });

      await hydrateRuntimeStoreContext(runtime, apiRequest, storeId, activeTenantId.value);
      return { ok: true };
    } catch (error) {
      return {
        ok: false,
        message: getApiErrorMessage(error, "Nao foi possivel arquivar consultor.")
      };
    }
  }

  if (import.meta.client) {
    watch(
      () => [auth.isAuthenticated, activeTenantId.value, accessibleStores.value.length],
      ([isAuthenticated, tenantId, storeCount], [previousAuthenticated, previousTenantId, previousStoreCount]) => {
        if (!isAuthenticated || !tenantId || storeCount < 1) {
          clearIntegratedView();
          return;
        }

        if (
          !previousAuthenticated ||
          previousTenantId !== tenantId ||
          previousStoreCount !== storeCount
        ) {
          clearIntegratedView();
        }
      }
    );
  }

  return {
    state,
    roster,
    selectedConsultantId,
    integratedRoster,
    integratedRanking,
    integratedOverview,
    integratedPending,
    integratedReady,
    integratedError,
    ensure: runtime.ensure,
    refreshActiveStore,
    refreshIntegratedView,
    ensureIntegratedView,
    clearIntegratedView,
    setSelectedConsultant(personId) {
      return runtime.run("setSelectedConsultant", personId);
    },
    setConsultantSimulationAdditionalSales(amount) {
      return runtime.run("setConsultantSimulationAdditionalSales", amount);
    },
    createConsultantProfile,
    updateConsultantProfile,
    archiveConsultantProfile
  };
});
