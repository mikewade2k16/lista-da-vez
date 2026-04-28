import { computed, ref } from "vue";
import { defineStore } from "pinia";

import { useAuthStore } from "~/stores/auth";
import { createApiRequest, getApiErrorMessage } from "~/utils/api-client";

interface ErpStoreScope {
  tenantId: string;
  storeId: string;
  storeCode: string;
  storeName: string;
  storeCity?: string;
  storeCnpj?: string;
}

interface ErpSyncRunSummary {
  id: string;
  dataType: string;
  mode: string;
  status: string;
  filesSeen: number;
  filesImported: number;
  filesSkipped: number;
  rowsRead: number;
  rowsImported: number;
  sourcePath?: string;
  errorMessage?: string;
  startedAt: string;
  finishedAt?: string;
  storeCnpj?: string;
}

interface ErpSyncFileSummary {
  id: string;
  dataType: string;
  sourceName: string;
  sourceKind: string;
  checksumSha256: string;
  recordCount: number;
  importedAt: string;
  storeCnpj?: string;
}

interface ErpStatusResponse {
  store: ErpStoreScope;
  supportedTypes: string[];
  functionalTypes: string[];
  placeholderTypes: string[];
  productCurrent: number;
  rawItemRows: number;
  typeStats?: ErpTypeStatus[];
  lastRun?: ErpSyncRunSummary | null;
  lastImportedFile?: ErpSyncFileSummary | null;
}

interface ErpTypeStatus {
  dataType: string;
  totalRows: number;
  currentRows?: number;
  rawRows?: number;
  lastRun?: ErpSyncRunSummary | null;
  lastImportedFile?: ErpSyncFileSummary | null;
}

interface ErpProductRow {
  sku: string;
  identifier: string;
  name: string;
  description?: string;
  supplierReference?: string;
  brandName?: string;
  seasonName?: string;
  category1?: string;
  category2?: string;
  category3?: string;
  size?: string;
  color?: string;
  unit?: string;
  priceRaw?: string;
  priceCents?: number | null;
  sourceCreatedAt?: string | null;
  sourceUpdatedAt?: string | null;
  sourceFileName?: string;
  sourceBatchDate?: string;
}

interface ErpProductsResponse {
  store: ErpStoreScope;
  identifierPrefix?: string;
  search?: string;
  page: number;
  pageSize: number;
  total: number;
  items: ErpProductRow[];
}

interface ErpRawRecordsResponse {
  store: ErpStoreScope;
  dataType: string;
  search?: string;
  specificSearch?: string;
  page: number;
  pageSize: number;
  total: number;
  items: Array<Record<string, unknown>>;
}

interface BootstrapResponse {
  ok: boolean;
  runId: string;
  store: ErpStoreScope;
  dataType: string;
  sourcePath: string;
  filesSeen: number;
  filesImported: number;
  filesSkipped: number;
  rowsRead: number;
  rowsImported: number;
  startedAt: string;
  finishedAt: string;
  storeCnpj?: string;
}

function normalizeText(value: unknown) {
  return String(value || "").trim();
}

function resolveRecordsEndpoint(dataType: string) {
  switch (dataType) {
    case "customer":
      return "/v1/erp/customers";
    case "employee":
      return "/v1/erp/employees";
    case "order":
      return "/v1/erp/orders";
    case "ordercanceled":
      return "/v1/erp/orders/canceled";
    default:
      return "/v1/erp/records";
  }
}

export const useErpStore = defineStore("erp", () => {
  const runtimeConfig = useRuntimeConfig();
  const auth = useAuthStore();
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken);
  let productsRequestSeq = 0;
  let recordsRequestSeq = 0;

  const status = ref<ErpStatusResponse | null>(null);
  const products = ref<ErpProductRow[]>([]);
  const totalProducts = ref(0);
  const page = ref(1);
  const pageSize = ref(50);
  const loadingStatus = ref(false);
  const loadingProducts = ref(false);
  const loadingRecords = ref(false);
  const syncing = ref(false);
  const error = ref("");
  const records = ref<Array<Record<string, unknown>>>([]);
  const totalRecords = ref(0);
  const recordsPage = ref(1);
  const recordsPageSize = ref(50);

  const activeStore = computed(() => {
    const activeStoreId = normalizeText(auth.activeStoreId);
    return (Array.isArray(auth.storeContext) ? auth.storeContext : []).find(
      (store) => normalizeText(store?.id) === activeStoreId
    ) || null;
  });

  const activeTenantId = computed(() =>
    normalizeText(auth.activeTenantId || activeStore.value?.tenantId || auth.tenantContext?.[0]?.id)
  );

  const activeStoreCode = computed(() => normalizeText(activeStore.value?.code));

  async function fetchStatus(payload: { tenantId?: string; storeCode?: string } = {}) {
    try {
      loadingStatus.value = true;
      error.value = "";
      await auth.ensureSession();

      const tenantId = normalizeText(payload.tenantId || activeTenantId.value);
      const storeCode = normalizeText(payload.storeCode || activeStoreCode.value);
      if (!storeCode) {
        return { ok: false, message: "Nenhuma loja ativa selecionada para o ERP." };
      }

      const params = new URLSearchParams({ storeCode });
      if (tenantId) {
        params.set("tenantId", tenantId);
      }

      const response = await apiRequest(`/v1/erp/status?${params.toString()}`);
      status.value = response as ErpStatusResponse;
      return { ok: true, data: status.value };
    } catch (err) {
      const message = getApiErrorMessage(err, "Erro ao carregar o status do ERP.");
      error.value = message;
      return { ok: false, message };
    } finally {
      loadingStatus.value = false;
    }
  }

  async function fetchProducts(payload: { tenantId?: string; storeCode?: string; identifierPrefix?: string; search?: string; page?: number; pageSize?: number } = {}) {
    const requestSeq = ++productsRequestSeq;
    try {
      loadingProducts.value = true;
      error.value = "";
      await auth.ensureSession();

      const tenantId = normalizeText(payload.tenantId || activeTenantId.value);
      const storeCode = normalizeText(payload.storeCode || activeStoreCode.value);
      if (!storeCode) {
        return { ok: false, message: "Nenhuma loja ativa selecionada para o ERP." };
      }

      const nextPage = Math.max(1, Number(payload.page || page.value || 1) || 1);
      const nextPageSize = Math.max(1, Number(payload.pageSize || pageSize.value || 50) || 50);
      const params = new URLSearchParams({
        storeCode,
        page: String(nextPage),
        pageSize: String(nextPageSize)
      });

      if (tenantId) {
        params.set("tenantId", tenantId);
      }

      const identifierPrefix = normalizeText(payload.identifierPrefix);
      if (identifierPrefix) {
        params.set("identifierPrefix", identifierPrefix);
      }

      const search = normalizeText(payload.search);
      if (search) {
        params.set("search", search);
      }

      const response = await apiRequest(`/v1/erp/products?${params.toString()}`) as ErpProductsResponse;
      if (requestSeq !== productsRequestSeq) {
        return { ok: true, data: response, stale: true };
      }

      products.value = Array.isArray(response.items) ? response.items : [];
      totalProducts.value = Number(response.total || 0) || 0;
      page.value = Number(response.page || nextPage) || nextPage;
      pageSize.value = Number(response.pageSize || nextPageSize) || nextPageSize;
      if (response.store && (!status.value || status.value.store.storeId !== response.store.storeId)) {
        status.value = {
          ...(status.value || {
            supportedTypes: [],
            functionalTypes: [],
            placeholderTypes: [],
            productCurrent: 0,
            rawItemRows: 0,
            lastRun: null,
            lastImportedFile: null
          }),
          store: response.store
        };
      }
      return { ok: true, data: response };
    } catch (err) {
      const message = getApiErrorMessage(err, "Erro ao carregar os produtos do ERP.");
      if (requestSeq !== productsRequestSeq) {
        return { ok: true, stale: true };
      }

      error.value = message;
      return { ok: false, message };
    } finally {
      if (requestSeq === productsRequestSeq) {
        loadingProducts.value = false;
      }
    }
  }

  async function bootstrapItems(payload: { tenantId?: string; storeCode?: string; sourcePath?: string } = {}) {
    try {
      syncing.value = true;
      error.value = "";
      await auth.ensureSession();

      const tenantId = normalizeText(payload.tenantId || activeTenantId.value);
      const storeCode = normalizeText(payload.storeCode || activeStoreCode.value);
      if (!storeCode) {
        return { ok: false, message: "Nenhuma loja ativa selecionada para o ERP." };
      }

      const response = await apiRequest("/v1/erp/bootstrap/items", {
        method: "POST",
        body: {
          tenantId,
          storeCode,
          sourcePath: normalizeText(payload.sourcePath)
        }
      }) as BootstrapResponse;

      return { ok: true, data: response };
    } catch (err) {
      const message = getApiErrorMessage(err, "Erro ao iniciar o bootstrap de produtos do ERP.");
      error.value = message;
      return { ok: false, message };
    } finally {
      syncing.value = false;
    }
  }

  async function bootstrapDataType(payload: { tenantId?: string; storeCode?: string; dataType: string; sourcePath?: string }) {
    try {
      syncing.value = true;
      error.value = "";
      await auth.ensureSession();

      const tenantId = normalizeText(payload.tenantId || activeTenantId.value);
      const storeCode = normalizeText(payload.storeCode || activeStoreCode.value);
      const dataType = normalizeText(payload.dataType).toLowerCase();
      if (!storeCode) {
        return { ok: false, message: "Nenhuma loja ativa selecionada para o ERP." };
      }
      if (!dataType) {
        return { ok: false, message: "Tipo de dado ERP nao informado." };
      }

      const response = await apiRequest("/v1/erp/bootstrap", {
        method: "POST",
        body: {
          tenantId,
          storeCode,
          dataType,
          sourcePath: normalizeText(payload.sourcePath)
        }
      }) as BootstrapResponse;

      return { ok: true, data: response };
    } catch (err) {
      const message = getApiErrorMessage(err, "Erro ao iniciar o bootstrap do ERP.");
      error.value = message;
      return { ok: false, message };
    } finally {
      syncing.value = false;
    }
  }

  async function fetchRecords(payload: {
    tenantId?: string;
    storeCode?: string;
    dataType: string;
    search?: string;
    specificSearch?: string;
    page?: number;
    pageSize?: number;
  }) {
    const requestSeq = ++recordsRequestSeq;
    try {
      loadingRecords.value = true;
      error.value = "";
      await auth.ensureSession();

      const tenantId = normalizeText(payload.tenantId || activeTenantId.value);
      const storeCode = normalizeText(payload.storeCode || activeStoreCode.value);
      if (!storeCode) {
        return { ok: false, message: "Nenhuma loja ativa selecionada para o ERP." };
      }

      const dataType = normalizeText(payload.dataType).toLowerCase();
      if (!dataType) {
        return { ok: false, message: "Tipo de dado ERP não informado." };
      }

      const nextPage = Math.max(1, Number(payload.page || recordsPage.value || 1) || 1);
      const nextPageSize = Math.max(1, Number(payload.pageSize || recordsPageSize.value || 50) || 50);
      const params = new URLSearchParams({
        storeCode,
        page: String(nextPage),
        pageSize: String(nextPageSize)
      });

      if (tenantId) {
        params.set("tenantId", tenantId);
      }

      const search = normalizeText(payload.search);
      if (search) {
        params.set("search", search);
      }

      const specificSearch = normalizeText(payload.specificSearch);
      if (specificSearch) {
        params.set("specificSearch", specificSearch);
      }

      let response: ErpRawRecordsResponse;
      const endpoint = resolveRecordsEndpoint(dataType);
      try {
        response = await apiRequest(`${endpoint}?${params.toString()}`) as ErpRawRecordsResponse;
      } catch (err) {
        // Backward compatibility path when only the generic endpoint is exposed.
        const statusCode = Number((err as { statusCode?: number })?.statusCode || 0);
        if (statusCode !== 404 || endpoint === "/v1/erp/records") {
          throw err;
        }
        params.set("dataType", dataType);
        response = await apiRequest(`/v1/erp/records?${params.toString()}`) as ErpRawRecordsResponse;
      }

      if (requestSeq !== recordsRequestSeq) {
        return { ok: true, data: response, stale: true };
      }

      records.value = Array.isArray(response.items) ? response.items : [];
      totalRecords.value = Number(response.total || 0) || 0;
      recordsPage.value = Number(response.page || nextPage) || nextPage;
      recordsPageSize.value = Number(response.pageSize || nextPageSize) || nextPageSize;
      return { ok: true, data: response };
    } catch (err) {
      const message = getApiErrorMessage(err, "Erro ao carregar os registros ERP.");
      if (requestSeq !== recordsRequestSeq) {
        return { ok: true, stale: true };
      }

      error.value = message;
      return { ok: false, message };
    } finally {
      if (requestSeq === recordsRequestSeq) {
        loadingRecords.value = false;
      }
    }
  }

  function reset() {
    productsRequestSeq += 1;
    recordsRequestSeq += 1;
    status.value = null;
    products.value = [];
    totalProducts.value = 0;
    page.value = 1;
    pageSize.value = 50;
    records.value = [];
    totalRecords.value = 0;
    recordsPage.value = 1;
    recordsPageSize.value = 50;
    error.value = "";
  }

  return {
    status,
    products,
    totalProducts,
    page,
    pageSize,
    loadingStatus,
    loadingProducts,
    loadingRecords,
    syncing,
    error,
    records,
    totalRecords,
    recordsPage,
    recordsPageSize,
    activeStore,
    activeTenantId,
    activeStoreCode,
    fetchStatus,
    fetchProducts,
    fetchRecords,
    bootstrapItems,
    bootstrapDataType,
    reset
  };
});
