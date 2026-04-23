import { computed } from "vue";
import { defineStore, storeToRefs } from "pinia";

import { useAuthStore } from "~/stores/auth";
import { useAppRuntimeStore } from "~/stores/app-runtime";
import { createApiRequest, getApiErrorMessage } from "~/utils/api-client";
import { hydrateRuntimeStoreContext } from "~/utils/runtime-remote";

export const useConsultantsStore = defineStore("consultants", () => {
  const runtimeConfig = useRuntimeConfig();
  const runtime = useAppRuntimeStore();
  const auth = useAuthStore();
  const { state } = storeToRefs(runtime);
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken);

  const roster = computed(() => state.value.roster || []);
  const selectedConsultantId = computed(() => state.value.selectedConsultantId || null);

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

    return hydrateRuntimeStoreContext(runtime, apiRequest, storeId);
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

      await hydrateRuntimeStoreContext(runtime, apiRequest, storeId);
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

      await hydrateRuntimeStoreContext(runtime, apiRequest, storeId);
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

      await hydrateRuntimeStoreContext(runtime, apiRequest, storeId);
      return { ok: true };
    } catch (error) {
      return {
        ok: false,
        message: getApiErrorMessage(error, "Nao foi possivel arquivar consultor.")
      };
    }
  }

  return {
    state,
    roster,
    selectedConsultantId,
    ensure: runtime.ensure,
    refreshActiveStore,
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
