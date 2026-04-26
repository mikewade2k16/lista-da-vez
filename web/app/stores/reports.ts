import { computed, ref, watch } from "vue";
import { defineStore, storeToRefs } from "pinia";
import { normalizeReportFilters } from "~/domain/utils/reports";
import { useAuthStore } from "~/stores/auth";
import { useAppRuntimeStore } from "~/stores/app-runtime";
import { createApiRequest, getApiErrorMessage } from "~/utils/api-client";

const RESULTS_PAGE_SIZE = 200;

export const useReportsStore = defineStore("reports", () => {
  const runtimeConfig = useRuntimeConfig();
  const runtime = useAppRuntimeStore();
  const auth = useAuthStore();
  const { state } = storeToRefs(runtime);
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken);
  const reportFilters = computed(() => normalizeReportFilters(state.value.reportFilters || {}));
  const overview = ref(null);
  const results = ref(null);
  const recentServices = ref(null);
  const multiStoreOverview = ref(null);
  const pending = ref(false);
  const ready = ref(false);
  const errorMessage = ref("");
  const lastLoadedKey = ref("");
  const integratedScope = ref(false);
  let refreshTimer = null;

  const activeStoreId = computed(() =>
    String(auth.activeStoreId || state.value.activeStoreId || "").trim()
  );
  const activeTenantId = computed(() =>
    String(auth.activeTenantId || auth.tenantContext?.[0]?.id || "").trim()
  );
  const activeStoreName = computed(() => {
    if (integratedScope.value) {
      return "Todas as lojas";
    }

    const storeId = activeStoreId.value;
    return (state.value.stores || []).find((store) => store.id === storeId)?.name || "";
  });

  async function resolveActiveStoreId() {
    await runtime.ensure();

    if (auth.isAuthenticated) {
      await auth.ensureSession();
    }

    return String(auth.activeStoreId || runtime.state.activeStoreId || "").trim();
  }

  function clearRemoteState() {
    overview.value = null;
    results.value = null;
    recentServices.value = null;
    multiStoreOverview.value = null;
    ready.value = false;
    errorMessage.value = "";
    lastLoadedKey.value = "";
  }

  function appendFilterParams(params) {
    const filters = reportFilters.value;

    if (filters.dateFrom) {
      params.set("dateFrom", filters.dateFrom);
    }

    if (filters.dateTo) {
      params.set("dateTo", filters.dateTo);
    }

    [
      "consultantIds",
      "outcomes",
      "sourceIds",
      "visitReasonIds",
      "startModes",
      "existingCustomerModes",
      "completionLevels",
      "campaignIds"
    ].forEach((key) => {
      (filters[key] || []).forEach((value) => {
        if (String(value || "").trim()) {
          params.append(key, String(value).trim());
        }
      });
    });

    if (String(filters.minSaleAmount || "").trim()) {
      params.set("minSaleAmount", String(filters.minSaleAmount).trim());
    }

    if (String(filters.maxSaleAmount || "").trim()) {
      params.set("maxSaleAmount", String(filters.maxSaleAmount).trim());
    }

    if (String(filters.search || "").trim()) {
      params.set("search", String(filters.search).trim());
    }

    return params;
  }

  function buildRequestParams(storeId, pageSize = RESULTS_PAGE_SIZE) {
    const params = appendFilterParams(new URLSearchParams());

    params.set("storeId", storeId);
    params.set("page", "1");
    params.set("pageSize", String(pageSize));

    return params;
  }

  function buildMultiStoreRequestParams(tenantId, pageSize = RESULTS_PAGE_SIZE) {
    const params = appendFilterParams(new URLSearchParams());

    params.set("tenantId", tenantId);
    params.set("page", "1");
    params.set("pageSize", String(pageSize));

    return params;
  }

  function buildRefreshKey(scopeType, scopeId) {
    return JSON.stringify({
      scopeType,
      scopeId,
      filters: reportFilters.value
    });
  }

  async function refreshReports() {
    const storeId = await resolveActiveStoreId();

    if (!storeId || !auth.isAuthenticated) {
      clearRemoteState();
      return null;
    }

    const refreshKey = buildRefreshKey("store", storeId);

    pending.value = true;
    errorMessage.value = "";

    try {
      const [overviewResponse, resultsResponse, recentServicesResponse] = await Promise.all([
        apiRequest(`/v1/reports/overview?${buildRequestParams(storeId, RESULTS_PAGE_SIZE).toString()}`),
        apiRequest(`/v1/reports/results?${buildRequestParams(storeId, RESULTS_PAGE_SIZE).toString()}`),
        apiRequest(`/v1/reports/recent-services?${buildRequestParams(storeId, 12).toString()}`)
      ]);

      overview.value = overviewResponse;
      results.value = resultsResponse;
      recentServices.value = recentServicesResponse;
      multiStoreOverview.value = null;
      ready.value = true;
      lastLoadedKey.value = refreshKey;
      return { overview: overviewResponse, results: resultsResponse, recentServices: recentServicesResponse };
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, "Nao foi possivel carregar os relatórios.");
      throw error;
    } finally {
      pending.value = false;
    }
  }

  async function refreshIntegratedReports() {
    await runtime.ensure();

    if (auth.isAuthenticated) {
      await auth.ensureSession();
    }

    const tenantId = activeTenantId.value;

    if (!tenantId || !auth.isAuthenticated) {
      clearRemoteState();
      return null;
    }

    const refreshKey = buildRefreshKey("tenant", tenantId);

    pending.value = true;
    errorMessage.value = "";

    try {
      const reportParams = buildMultiStoreRequestParams(tenantId, RESULTS_PAGE_SIZE).toString();
      const recentParams = buildMultiStoreRequestParams(tenantId, 12).toString();
      const [overviewResponse, resultsResponse, recentServicesResponse, multiStoreResponse] = await Promise.all([
        apiRequest(`/v1/reports/overview?${reportParams}`),
        apiRequest(`/v1/reports/results?${reportParams}`),
        apiRequest(`/v1/reports/recent-services?${recentParams}`),
        apiRequest(`/v1/reports/multistore-overview?${reportParams}`)
      ]);

      overview.value = overviewResponse;
      results.value = resultsResponse;
      recentServices.value = recentServicesResponse;
      multiStoreOverview.value = multiStoreResponse;
      ready.value = true;
      lastLoadedKey.value = refreshKey;
      return {
        overview: overviewResponse,
        results: resultsResponse,
        recentServices: recentServicesResponse,
        multiStoreOverview: multiStoreResponse
      };
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, "Nao foi possivel carregar os relatórios.");
      throw error;
    } finally {
      pending.value = false;
    }
  }

  async function refreshCurrentScope() {
    if (integratedScope.value) {
      return refreshIntegratedReports();
    }

    return refreshReports();
  }

  function scheduleRefresh() {
    if (refreshTimer) {
      clearTimeout(refreshTimer);
    }

    refreshTimer = setTimeout(() => {
      refreshTimer = null;
      void refreshCurrentScope().catch(() => {});
    }, 220);
  }

  async function ensureLoaded() {
    if (integratedScope.value) {
      const tenantId = activeTenantId.value;

      if (!tenantId || !auth.isAuthenticated) {
        clearRemoteState();
        return false;
      }

      if (ready.value && lastLoadedKey.value === buildRefreshKey("tenant", tenantId)) {
        return true;
      }

      try {
        await refreshIntegratedReports();
        return true;
      } catch {
        return false;
      }
    }

    const storeId = await resolveActiveStoreId();

    if (!storeId || !auth.isAuthenticated) {
      clearRemoteState();
      return false;
    }

    if (ready.value && lastLoadedKey.value === buildRefreshKey("store", storeId)) {
      return true;
    }

    try {
      await refreshReports();
      return true;
    } catch {
      return false;
    }
  }

  if (import.meta.client) {
    watch(
      () => [auth.isAuthenticated, activeStoreId.value, activeTenantId.value, integratedScope.value],
      ([isAuthenticated, storeId, tenantId, isIntegrated], [previousAuthenticated, previousStoreId, previousTenantId, previousIntegrated]) => {
        if (!isAuthenticated || (isIntegrated ? !tenantId : !storeId)) {
          clearRemoteState();
          return;
        }

        if (
          !previousAuthenticated ||
          previousStoreId !== storeId ||
          previousTenantId !== tenantId ||
          previousIntegrated !== isIntegrated
        ) {
          void refreshCurrentScope().catch(() => {});
        }
      }
    );
  }

  return {
    state,
    reportFilters,
    overview,
    results,
    recentServices,
    multiStoreOverview,
    pending,
    ready,
    errorMessage,
    integratedScope,
    activeStoreId,
    activeStoreName,
    ensure: runtime.ensure,
    ensureLoaded,
    refreshReports,
    refreshIntegratedReports,
    setIntegratedScope(value) {
      integratedScope.value = Boolean(value);
    },
    async updateReportFilter(filterId, value) {
      const result = await runtime.run("updateReportFilter", filterId, value);
      scheduleRefresh();
      return result;
    },
    async resetReportFilters() {
      const result = await runtime.run("resetReportFilters");
      await ensureLoaded();
      return result;
    }
  };
});
