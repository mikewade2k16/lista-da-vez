import { ref, watch } from "vue";
import { defineStore } from "pinia";

import { useAuthStore } from "~/stores/auth";
import { createApiRequest, getApiErrorMessage } from "~/utils/api-client";

function normalizeText(value) {
  return String(value || "").trim();
}

export const useAnalyticsStore = defineStore("analytics", () => {
  const runtimeConfig = useRuntimeConfig();
  const auth = useAuthStore();
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken);

  const ranking = ref(null);
  const data = ref(null);
  const intelligence = ref(null);
  const pending = ref(false);
  const ready = ref(false);
  const errorMessage = ref("");

  function clearState() {
    ranking.value = null;
    data.value = null;
    intelligence.value = null;
    ready.value = false;
    errorMessage.value = "";
  }

  function buildStoreQuery() {
    const storeId = normalizeText(auth.activeStoreId);
    return storeId ? `?storeId=${encodeURIComponent(storeId)}` : "";
  }

  async function ensureBase() {
    await auth.ensureSession();

    if (!auth.isAuthenticated || !normalizeText(auth.activeStoreId)) {
      clearState();
      return false;
    }

    return true;
  }

  async function fetchRanking() {
    if (!await ensureBase()) {
      return null;
    }

    pending.value = true;
    errorMessage.value = "";

    try {
      ranking.value = await apiRequest(`/v1/analytics/ranking${buildStoreQuery()}`);
      ready.value = true;
      return ranking.value;
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, "Nao foi possivel carregar o ranking.");
      throw error;
    } finally {
      pending.value = false;
    }
  }

  async function fetchData() {
    if (!await ensureBase()) {
      return null;
    }

    pending.value = true;
    errorMessage.value = "";

    try {
      data.value = await apiRequest(`/v1/analytics/data${buildStoreQuery()}`);
      ready.value = true;
      return data.value;
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, "Nao foi possivel carregar os dados operacionais.");
      throw error;
    } finally {
      pending.value = false;
    }
  }

  async function fetchIntelligence() {
    if (!await ensureBase()) {
      return null;
    }

    pending.value = true;
    errorMessage.value = "";

    try {
      intelligence.value = await apiRequest(`/v1/analytics/intelligence${buildStoreQuery()}`);
      ready.value = true;
      return intelligence.value;
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, "Nao foi possivel carregar a inteligencia operacional.");
      throw error;
    } finally {
      pending.value = false;
    }
  }

  async function ensureRanking() {
    if (ranking.value) {
      return ranking.value;
    }

    return fetchRanking();
  }

  async function ensureData() {
    if (data.value) {
      return data.value;
    }

    return fetchData();
  }

  async function ensureIntelligence() {
    if (intelligence.value) {
      return intelligence.value;
    }

    return fetchIntelligence();
  }

  if (import.meta.client) {
    watch(
      () => [auth.isAuthenticated, auth.activeStoreId],
      ([isAuthenticated, activeStoreId], [previousAuthenticated, previousStoreId]) => {
        if (!isAuthenticated || !normalizeText(activeStoreId)) {
          clearState();
          return;
        }

        if (!previousAuthenticated || previousStoreId !== activeStoreId) {
          clearState();
        }
      }
    );
  }

  return {
    ranking,
    data,
    intelligence,
    pending,
    ready,
    errorMessage,
    clearState,
    fetchRanking,
    fetchData,
    fetchIntelligence,
    ensureRanking,
    ensureData,
    ensureIntelligence
  };
});
