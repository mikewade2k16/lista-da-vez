import { computed, ref } from "vue";
import { defineStore } from "pinia";

import { useAuthStore } from "~/stores/auth";
import { createApiRequest, getApiErrorMessage } from "~/utils/api-client";

type CRMSummary = {
  orders: number;
  units: number;
  salesCents: number;
  ticketAverageCents: number;
  valuePerProductCents: number;
  paScore: number;
  monthlyGoalCents: number;
  goalProgress: number;
  remainingToGoalCents: number;
  unmappedSalesCents?: number;
};

type CRMStoreMetric = {
  storeSlug: string;
  storeLabel: string;
  storeCode?: string;
  storeName?: string;
  storeCnpjs?: string[];
  mapped: boolean;
  orders: number;
  units: number;
  salesCents: number;
  ticketAverageCents: number;
  valuePerProductCents: number;
  paScore: number;
  monthlyGoalCents: number;
  avgTicketGoalCents: number;
  paGoal: number;
  goalProgress: number;
  remainingToGoalCents: number;
};

type CRMConsultantMetric = {
  consultantId: string;
  consultantName: string;
  storeSlug: string;
  storeLabel: string;
  storeCnpj?: string;
  mapped: boolean;
  orders: number;
  units: number;
  salesCents: number;
  ticketAverageCents: number;
  valuePerProductCents: number;
  paScore: number;
};

type CRMOverviewResponse = {
  store?: Record<string, unknown> | null;
  dateFrom: string;
  dateTo: string;
  summary: CRMSummary;
  stores: CRMStoreMetric[];
  consultants: CRMConsultantMetric[];
  warnings?: string[];
};

function formatDateInput(date: Date) {
  return [
    date.getUTCFullYear(),
    String(date.getUTCMonth() + 1).padStart(2, "0"),
    String(date.getUTCDate()).padStart(2, "0")
  ].join("-");
}

function buildCurrentMonthRange() {
  const now = new Date();
  const monthStart = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 1));
  const monthEnd = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() + 1, 0));

  return {
    dateFrom: formatDateInput(monthStart),
    dateTo: formatDateInput(monthEnd)
  };
}

function normalizeText(value: unknown) {
  return String(value || "").trim();
}

function createEmptyOverview(dateFrom: string, dateTo: string): CRMOverviewResponse {
  return {
    store: null,
    dateFrom,
    dateTo,
    summary: {
      orders: 0,
      units: 0,
      salesCents: 0,
      ticketAverageCents: 0,
      valuePerProductCents: 0,
      paScore: 0,
      monthlyGoalCents: 0,
      goalProgress: 0,
      remainingToGoalCents: 0,
      unmappedSalesCents: 0
    },
    stores: [],
    consultants: [],
    warnings: []
  };
}

export const useCrmStore = defineStore("crm", () => {
  const runtimeConfig = useRuntimeConfig();
  const auth = useAuthStore();
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken);
  const defaultRange = buildCurrentMonthRange();

  const overview = ref<CRMOverviewResponse | null>(null);
  const pending = ref(false);
  const ready = ref(false);
  const errorMessage = ref("");
  const lastLoadedKey = ref("");
  const dateFrom = ref(defaultRange.dateFrom);
  const dateTo = ref(defaultRange.dateTo);

  const activeTenantId = computed(() =>
    normalizeText(auth.activeTenantId || auth.tenantContext?.[0]?.id)
  );

  function buildRequestKey() {
    return JSON.stringify({
      tenantId: activeTenantId.value,
      dateFrom: dateFrom.value,
      dateTo: dateTo.value
    });
  }

  function clearState() {
    overview.value = createEmptyOverview(dateFrom.value, dateTo.value);
    pending.value = false;
    ready.value = false;
    errorMessage.value = "";
    lastLoadedKey.value = "";
  }

  function resetCurrentMonth() {
    const nextRange = buildCurrentMonthRange();
    dateFrom.value = nextRange.dateFrom;
    dateTo.value = nextRange.dateTo;
  }

  async function refreshOverview() {
    if (!auth.isAuthenticated) {
      clearState();
      return null;
    }

    pending.value = true;
    errorMessage.value = "";

    try {
      await auth.ensureSession();

      if (!auth.isAuthenticated) {
        clearState();
        return null;
      }

      const params = new URLSearchParams();
      if (activeTenantId.value) {
        params.set("tenantId", activeTenantId.value);
      }
      if (dateFrom.value) {
        params.set("dateFrom", dateFrom.value);
      }
      if (dateTo.value) {
        params.set("dateTo", dateTo.value);
      }

      const response = await apiRequest(`/v1/erp/crm?${params.toString()}`) as CRMOverviewResponse;
      overview.value = response;
      ready.value = true;
      lastLoadedKey.value = buildRequestKey();
      return response;
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, "Nao foi possivel carregar o CRM do ERP.");
      throw error;
    } finally {
      pending.value = false;
    }
  }

  async function ensureLoaded() {
    if (!auth.isAuthenticated) {
      clearState();
      return false;
    }

    if (ready.value && lastLoadedKey.value === buildRequestKey()) {
      return true;
    }

    try {
      await refreshOverview();
      return true;
    } catch {
      return false;
    }
  }

  async function applyFilters() {
    return refreshOverview();
  }

  return {
    overview,
    pending,
    ready,
    errorMessage,
    dateFrom,
    dateTo,
    ensureLoaded,
    refreshOverview,
    applyFilters,
    resetCurrentMonth,
    clearState
  };
});