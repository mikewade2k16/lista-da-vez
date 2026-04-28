<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";

import SettingsTabs from "~/components/settings/SettingsTabs.vue";
import ErpProductsTable from "~/components/erp/ErpProductsTable.vue";
import { hasPermission } from "~/domain/utils/permissions";
import { useAuthStore } from "~/stores/auth";
import { useErpStore } from "~/stores/erp";
import { useUiStore } from "~/stores/ui";

const auth = useAuthStore();
const erpStore = useErpStore();
const ui = useUiStore();

const activeTab = ref("produtos");
const activeBancoTab = ref("geral");
const searchValue = ref("");
const recordsSearchValue = ref("");
const recordsSpecificSearchValue = ref("");
const identifierPrefixValue = ref("");
const erpStoreCode = ref("");
const erpStoreCodeInput = ref("");

const tabs = [
  { id: "produtos", label: "Produtos", icon: "inventory_2" },
  { id: "pedidos", label: "Compras", icon: "receipt_long" },
  { id: "clientes", label: "Clientes", icon: "groups" },
  { id: "cancelados", label: "Cancelados", icon: "event_busy" },
  { id: "funcionarios", label: "Funcionarios", icon: "badge" },
  { id: "banco", label: "Banco", icon: "storage" }
];

const bancoTabs = [
  { id: "geral", label: "Visao geral", icon: "dashboard" },
  { id: "produtos", label: "Produtos", icon: "inventory_2" },
  { id: "clientes", label: "Clientes", icon: "groups" },
  { id: "pedidos", label: "Compras", icon: "receipt_long" },
  { id: "cancelados", label: "Cancelados", icon: "event_busy" },
  { id: "funcionarios", label: "Funcionarios", icon: "badge" },
  { id: "outbox", label: "Outbox", icon: "send" }
];

const availableStores = computed(() =>
  Array.isArray(auth.storeContext)
    ? (auth.storeContext as Array<{ id: string; code: string; name: string }>).filter((s) => s?.code)
    : []
);

type ErpGridColumn = {
  id: string;
  label: string;
  width: string;
  align: string;
  locked?: boolean;
  defaultVisible?: boolean;
};

const columns: ErpGridColumn[] = [
  { id: "sku", label: "SKU", width: "120px", align: "left", locked: true },
  { id: "identifier", label: "Identificador", width: "140px", align: "left" },
  { id: "name", label: "Produto", width: "minmax(320px, 2.2fr)", align: "left", locked: true },
  { id: "description", label: "Descricao", width: "minmax(260px, 1.5fr)", align: "left" },
  { id: "supplierReference", label: "Ref. fornecedor", width: "150px", align: "left" },
  { id: "brandName", label: "Marca", width: "140px", align: "left" },
  { id: "seasonName", label: "Colecao", width: "140px", align: "left" },
  { id: "category1", label: "Categoria", width: "150px", align: "left" },
  { id: "category2", label: "Subcategoria", width: "170px", align: "left" },
  { id: "category3", label: "Linha", width: "150px", align: "left" },
  { id: "size", label: "Tam.", width: "90px", align: "center" },
  { id: "color", label: "Cor", width: "110px", align: "left" },
  { id: "unit", label: "Un.", width: "80px", align: "center" },
  { id: "priceRaw", label: "Preco", width: "120px", align: "right", locked: true },
  { id: "sourceUpdatedAt", label: "Atualizado", width: "160px", align: "left" }
];

const recordsColumnsByTab: Record<string, ErpGridColumn[]> = {
  clientes: [
    { id: "name", label: "Nome", width: "minmax(240px, 1.8fr)", align: "left", locked: true },
    { id: "nickname", label: "Apelido", width: "160px", align: "left" },
    { id: "cpf", label: "CPF", width: "150px", align: "left" },
    { id: "email", label: "Email", width: "minmax(230px, 1.5fr)", align: "left" },
    { id: "phone", label: "Telefone", width: "150px", align: "left" },
    { id: "mobile", label: "Celular", width: "150px", align: "left" },
    { id: "gender", label: "Genero", width: "110px", align: "center" },
    { id: "birthday_raw", label: "Nascimento", width: "140px", align: "left" },
    { id: "street", label: "Endereco", width: "minmax(220px, 1.4fr)", align: "left" },
    { id: "number", label: "Numero", width: "100px", align: "left" },
    { id: "complement", label: "Complemento", width: "150px", align: "left" },
    { id: "neighborhood", label: "Bairro", width: "160px", align: "left" },
    { id: "city", label: "Cidade", width: "170px", align: "left" },
    { id: "uf", label: "UF", width: "90px", align: "center" },
    { id: "country", label: "Pais", width: "100px", align: "center" },
    { id: "zipcode", label: "CEP", width: "130px", align: "left" },
    { id: "employee_id", label: "Funcionario", width: "130px", align: "left" },
    { id: "registered_at_raw", label: "Cadastro", width: "170px", align: "left" },
    { id: "original_id", label: "ID original", width: "140px", align: "left" },
    { id: "identifier", label: "Identificador", width: "140px", align: "left" },
    { id: "tags", label: "Tags", width: "minmax(180px, 1fr)", align: "left" }
  ],
  funcionarios: [
    { id: "name", label: "Nome", width: "minmax(240px, 1.8fr)", align: "left", locked: true },
    { id: "original_id", label: "ID original", width: "150px", align: "left" },
    { id: "street", label: "Endereco", width: "minmax(220px, 1.3fr)", align: "left" },
    { id: "complement", label: "Complemento", width: "150px", align: "left" },
    { id: "city", label: "Cidade", width: "170px", align: "left" },
    { id: "uf", label: "UF", width: "90px", align: "center" },
    { id: "zipcode", label: "CEP", width: "130px", align: "left" },
    { id: "is_active_raw", label: "Ativo", width: "110px", align: "center" }
  ],
  pedidos: [
    { id: "order_id", label: "Compra", width: "160px", align: "left", locked: true },
    { id: "identifier", label: "Identificador", width: "140px", align: "left" },
    { id: "customer_id", label: "Cliente", width: "130px", align: "left" },
    { id: "order_date_raw", label: "Data", width: "140px", align: "left" },
    { id: "total_amount_raw", label: "Total compra", width: "130px", align: "right" },
    { id: "product_return_raw", label: "Devolucao", width: "120px", align: "right" },
    { id: "sku", label: "SKU", width: "130px", align: "left" },
    { id: "amount_raw", label: "Valor item", width: "120px", align: "right" },
    { id: "quantity_raw", label: "Qtd", width: "90px", align: "right" },
    { id: "employee_id", label: "Funcionario", width: "130px", align: "left" },
    { id: "payment_type", label: "Pagamento", width: "140px", align: "left" },
    { id: "total_exclusion_raw", label: "Exclusao", width: "120px", align: "right" },
    { id: "total_debit_raw", label: "Debito", width: "120px", align: "right" }
  ],
  cancelados: [
    { id: "order_id", label: "Compra", width: "160px", align: "left", locked: true },
    { id: "identifier", label: "Identificador", width: "140px", align: "left" },
    { id: "customer_id", label: "Cliente", width: "130px", align: "left" },
    { id: "order_date_raw", label: "Data", width: "140px", align: "left" },
    { id: "total_amount_raw", label: "Total compra", width: "130px", align: "right" },
    { id: "product_return_raw", label: "Devolucao", width: "120px", align: "right" },
    { id: "sku", label: "SKU", width: "130px", align: "left" },
    { id: "amount_raw", label: "Valor item", width: "120px", align: "right" },
    { id: "quantity_raw", label: "Qtd", width: "90px", align: "right" },
    { id: "employee_id", label: "Funcionario", width: "130px", align: "left" },
    { id: "payment_type", label: "Pagamento", width: "140px", align: "left" },
    { id: "total_exclusion_raw", label: "Exclusao", width: "120px", align: "right" },
    { id: "total_debit_raw", label: "Debito", width: "120px", align: "right" }
  ]
};

const pageSizeOptions = [25, 50, 100, 200];
const recordsDataTypeByTab: Record<string, string> = {
  clientes: "customer",
  funcionarios: "employee",
  pedidos: "order",
  cancelados: "ordercanceled"
};
const recordsLabelByTab: Record<string, string> = {
  clientes: "clientes",
  funcionarios: "funcionarios",
  pedidos: "compras",
  cancelados: "cancelados"
};
const recordsBootstrapLabelByTab: Record<string, string> = {
  clientes: "Bootstrap clientes 184",
  funcionarios: "Bootstrap funcionarios 184",
  pedidos: "Bootstrap compras 184",
  cancelados: "Bootstrap cancelados 184"
};
const recordsSpecificSearchByTab: Record<string, { label: string; placeholder: string }> = {
  clientes: { label: "CPF (comeca com)", placeholder: "Ex: 123.456.789-00" },
  funcionarios: { label: "ID funcionario (comeca com)", placeholder: "Ex: 315" },
  pedidos: { label: "Compra (comeca com)", placeholder: "Ex: 315578" },
  cancelados: { label: "Compra cancelada (comeca com)", placeholder: "Ex: 315578" }
};
const recordsGeneralSearchPlaceholderByTab: Record<string, string> = {
  clientes: "Busca geral (nome, email, telefone, cidade, tags...)",
  funcionarios: "Busca geral (nome, cidade, UF, endereco, status...)",
  pedidos: "Busca geral (compra, cliente, SKU, valor, funcionario, pagamento...)",
  cancelados: "Busca geral (compra cancelada, cliente, SKU, valor, funcionario...)"
};

const currentStore = computed(() => erpStore.activeStore);
const status = computed(() => erpStore.status);
const currentProductCount = computed(() => Number(status.value?.productCurrent || 0));
const rawItemRows = computed(() => Number(status.value?.rawItemRows || 0));
const lastRun = computed(() => status.value?.lastRun || null);
const lastImportedFile = computed(() => status.value?.lastImportedFile || null);
const resolvedStoreCode = computed(() => erpStoreCode.value || status.value?.store?.storeCode || currentStore.value?.code || "");

const subLojaLabel = computed(() => {
  if (resolvedStoreCode.value === "184") return "Pérola";
  if (resolvedStoreCode.value === "905") return "Pérola Ribeirão";
  return resolvedStoreCode.value ? `Loja ${resolvedStoreCode.value}` : "—";
});
const canSync = computed(() => {
  if (auth.permissionsResolved) {
    return hasPermission(auth.permissionKeys, "workspace.erp.edit");
  }

  return auth.role === "platform_admin" || auth.role === "owner";
});
const activeRecordsDataType = computed(() => recordsDataTypeByTab[activeTab.value] || "");
const activeRecordsColumns = computed(() => recordsColumnsByTab[activeTab.value] || []);
const activeRecordsBootstrapLabel = computed(() => recordsBootstrapLabelByTab[activeTab.value] || "Bootstrap registros 184");
const activeRecordsSpecificSearch = computed(() => recordsSpecificSearchByTab[activeTab.value] || { label: "Campo especifico", placeholder: "Digite para filtrar" });
const activeRecordsGeneralSearchPlaceholder = computed(() => recordsGeneralSearchPlaceholderByTab[activeTab.value] || "Busca geral (campos do tipo selecionado)");
const activeTypeStatus = computed(() => {
  const dataType = activeRecordsDataType.value;
  return (Array.isArray(status.value?.typeStats) ? status.value.typeStats : []).find((item) => item?.dataType === dataType) || null;
});
const activeRecordsTotal = computed(() => Number(activeTypeStatus.value?.totalRows || erpStore.totalRecords || 0));
const activeRecordsLastRun = computed(() => activeTypeStatus.value?.lastRun || null);
const activeRecordsLastImportedFile = computed(() => activeTypeStatus.value?.lastImportedFile || null);

const bancoSectionByTab: Record<string, { title: string; text: string; note: string; cards: Array<{ table: string; label: string; desc: string; badge: string }> }> = {
  geral: {
    title: "Estrutura geral do modulo ERP",
    text: "O desenho separa controle de execucao, espelho raw e projecao de leitura. Isso garante trilha auditavel sem perder performance nas consultas do painel.",
    note: "Sub-lojas entram por linha em store_cnpj nas tabelas raw. A projecao atual prioriza SKU por tenant/loja e pode receber filtro por CNPJ na proxima camada.",
    cards: [
      { table: "erp_sync_runs", label: "Runs de sincronizacao", desc: "Controle de cada execucao, status e contadores por tipo.", badge: "controle" },
      { table: "erp_sync_files", label: "Lotes processados", desc: "Checksum por arquivo para idempotencia e reprocessamento seguro.", badge: "controle" },
      { table: "erp_item_raw", label: "Raw de produtos", desc: "Historico bruto linha a linha vindo do consolidado markdown.", badge: "raw" },
      { table: "erp_item_current", label: "Catalogo atual", desc: "Projecao deduplicada por SKU para consultas de produtos.", badge: "projecao" }
    ]
  },
  produtos: {
    title: "Banco da frente de Produtos",
    text: "Produtos possuem pipeline completo: gravacao raw e atualizacao de projecao atual com upsert por SKU.",
    note: "A tabela erp_item_current alimenta a grade principal e ja respeita paginacao/busca administrativa.",
    cards: [
      { table: "erp_item_raw", label: "Itens raw", desc: "Espelho integral das linhas de item importadas por lote.", badge: "raw" },
      { table: "erp_item_current", label: "Itens atuais", desc: "Camada otimizada para leitura no painel, 1 registro por SKU.", badge: "projecao" },
      { table: "erp_sync_files", label: "Arquivos de item", desc: "Metadados e checksum dos lotes que atualizaram produtos.", badge: "controle" }
    ]
  },
  clientes: {
    title: "Banco da frente de Clientes",
    text: "Clientes usam tabela raw dedicada com todos os campos de origem para auditoria e busca administrativa.",
    note: "A leitura da aba Clientes vem de erp_customer_raw via endpoint paginado do modulo ERP.",
    cards: [
      { table: "erp_customer_raw", label: "Clientes raw", desc: "Nome, CPF, email, contato e identificador por linha de origem.", badge: "raw" },
      { table: "erp_sync_files", label: "Lotes de customer", desc: "Controle de importacao e deduplicacao por checksum.", badge: "controle" }
    ]
  },
  pedidos: {
    title: "Banco da frente de Compras",
    text: "Compras ativas ficam em tabela raw propria com valores brutos e campos normalizados de apoio.",
    note: "A aba Compras consulta erp_order_raw e preserva referencia de lote, linha e tipo de pagamento.",
    cards: [
      { table: "erp_order_raw", label: "Compras raw", desc: "Compra, cliente, SKU, valores e metadados de origem.", badge: "raw" },
      { table: "erp_sync_files", label: "Lotes de order", desc: "Historico de arquivos importados para compras.", badge: "controle" }
    ]
  },
  cancelados: {
    title: "Banco da frente de Cancelados",
    text: "Cancelados seguem o mesmo contrato de compras, em tabela separada para governanca e filtros dedicados.",
    note: "Separar order e ordercanceled evita ambiguidades em indicadores e trilhas de reconciliacao.",
    cards: [
      { table: "erp_order_canceled_raw", label: "Compras canceladas raw", desc: "Mesma base de campos de order, isolada para cancelamentos.", badge: "raw" },
      { table: "erp_sync_files", label: "Lotes de ordercanceled", desc: "Controle de lotes importados para cancelados.", badge: "controle" }
    ]
  },
  funcionarios: {
    title: "Banco da frente de Funcionarios",
    text: "Funcionarios entram em tabela raw especifica com dados cadastrais e status de atividade.",
    note: "A consulta da aba Funcionarios usa erp_employee_raw com paginação e busca textual.",
    cards: [
      { table: "erp_employee_raw", label: "Funcionarios raw", desc: "ID original, nome, cidade, UF e indicador de ativo.", badge: "raw" },
      { table: "erp_sync_files", label: "Lotes de employee", desc: "Trilha de importacoes da frente de funcionarios.", badge: "controle" }
    ]
  },
  outbox: {
    title: "Banco da frente de Outbox",
    text: "Outbox prepara integracao incremental com outros bancos/servicos sem acoplar na importacao principal.",
    note: "Quando ativado, processa eventos pendentes com retries e controle de disponibilidade.",
    cards: [
      { table: "erp_export_outbox", label: "Outbox de exportacao", desc: "Fila de eventos ERP para sincronizacoes futuras.", badge: "outbox" },
      { table: "erp_sync_runs", label: "Relacionamento com runs", desc: "Permite rastrear de qual ciclo de importacao partiu o evento.", badge: "controle" }
    ]
  }
};

const activeBancoSection = computed(() => bancoSectionByTab[activeBancoTab.value] || bancoSectionByTab.geral);

function formatDateTime(value?: string | null) {
  const normalized = String(value || "").trim();
  if (!normalized) {
    return "-";
  }

  const parsed = new Date(normalized);
  if (Number.isNaN(parsed.getTime())) {
    return normalized;
  }

  const datePart = parsed.toLocaleDateString("pt-BR", {
    day: "numeric",
    month: "short",
    year: "numeric"
  }).replace(/\. de /g, " ").replace(/\.$/, "");

  const timePart = parsed.toLocaleTimeString("pt-BR", {
    hour: "2-digit",
    minute: "2-digit"
  });

  return `${datePart} às ${timePart}`;
}

function formatNumber(value: number | null | undefined) {
  const n = Number(value || 0);
  return n.toLocaleString("pt-BR");
}

// Converts a source filename like "20260413043245_184-cnpj-item-20260406.csv"
// into a human-readable date string. Falls back to the raw name.
function formatSourceFileName(sourceName?: string | null): string {
  if (!sourceName) return "-";
  const match = sourceName.match(/^(\d{4})(\d{2})(\d{2})(\d{2})(\d{2})(\d{2})/);
  if (!match) return sourceName;
  const [, year, month, day, hour, minute] = match;
  const parsed = new Date(`${year}-${month}-${day}T${hour}:${minute}:00`);
  if (Number.isNaN(parsed.getTime())) return sourceName;
  return formatDateTime(parsed.toISOString());
}

function formatPrice(rawValue?: string, cents?: number | null) {
  const numericCents = Number.isFinite(Number(cents)) ? Number(cents) : Number(rawValue || 0);
  if (!numericCents) {
    return "-";
  }

  return new Intl.NumberFormat("pt-BR", {
    style: "currency",
    currency: "BRL"
  }).format(numericCents / 100);
}

async function loadStatus() {
  const storeCode = erpStoreCode.value;
  if (!storeCode) {
    return;
  }

  const result = await erpStore.fetchStatus({ storeCode });
  if (!result.ok && result.message) {
    ui.error(result.message);
  }
}

async function loadProducts(payload: { page?: number; pageSize?: number } = {}) {
  if (activeTab.value !== "produtos") {
    return;
  }

  const storeCode = erpStoreCode.value;
  if (!storeCode) {
    return;
  }

  const result = await erpStore.fetchProducts({
    storeCode,
    identifierPrefix: identifierPrefixValue.value,
    search: searchValue.value,
    page: payload.page || erpStore.page || 1,
    pageSize: payload.pageSize || erpStore.pageSize || 50
  });
  if (!result.ok && result.message) {
    ui.error(result.message);
  }
}

async function loadRecords(payload: { page?: number; pageSize?: number } = {}) {
  if (!activeRecordsDataType.value) {
    return;
  }

  const storeCode = erpStoreCode.value;
  if (!storeCode) {
    return;
  }

  const result = await erpStore.fetchRecords({
    storeCode,
    dataType: activeRecordsDataType.value,
    search: recordsSearchValue.value,
    specificSearch: recordsSpecificSearchValue.value,
    page: payload.page || erpStore.recordsPage || 1,
    pageSize: payload.pageSize || erpStore.recordsPageSize || 50
  });
  if (!result.ok && result.message) {
    ui.error(result.message);
  }
}

async function handlePageChange(nextPage: number) {
  await loadProducts({ page: nextPage, pageSize: erpStore.pageSize });
}

async function handlePageSizeChange(nextPageSize: number) {
  await loadProducts({ page: 1, pageSize: nextPageSize });
}

async function reloadWorkspace() {
  await loadStatus();
  if (activeTab.value === "produtos") {
    await loadProducts();
    return;
  }
  if (activeTab.value !== "banco") {
    await loadRecords();
  }
}

async function handleRecordsPageChange(nextPage: number) {
  await loadRecords({ page: nextPage, pageSize: erpStore.recordsPageSize });
}

async function handleRecordsPageSizeChange(nextPageSize: number) {
  await loadRecords({ page: 1, pageSize: nextPageSize });
}

async function handleBootstrap() {
  const storeCode = erpStoreCode.value;
  if (!storeCode) {
    ui.error("Selecione uma loja antes de iniciar o bootstrap.");
    return;
  }

  const result = await erpStore.bootstrapItems({ storeCode });
  if (!result.ok) {
    ui.error(result.message || "Nao foi possivel iniciar o bootstrap do ERP.");
    return;
  }

  ui.success(`Bootstrap ERP concluido: ${result.data?.rowsImported || 0} linhas importadas em ${result.data?.filesImported || 0} lotes.`);
  await reloadWorkspace();
}

async function handleRecordsBootstrap() {
  const storeCode = erpStoreCode.value;
  const dataType = activeRecordsDataType.value;
  const label = recordsLabelByTab[activeTab.value] || "registros";
  if (!storeCode) {
    ui.error("Selecione uma loja antes de iniciar o bootstrap.");
    return;
  }
  if (!dataType) {
    ui.error("Selecione uma aba ERP valida antes de iniciar o bootstrap.");
    return;
  }

  const result = await erpStore.bootstrapDataType({ storeCode, dataType });
  if (!result.ok) {
    ui.error(result.message || "Nao foi possivel iniciar o bootstrap do ERP.");
    return;
  }

  ui.success(`Bootstrap ERP de ${label} concluido: ${result.data?.rowsImported || 0} linhas importadas em ${result.data?.filesImported || 0} lotes.`);
  await reloadWorkspace();
}

function pickStoreCode(code: string) {
  erpStoreCode.value = code;
  erpStoreCodeInput.value = code;
}

function applyManualStoreCode() {
  const trimmed = erpStoreCodeInput.value.trim();
  if (trimmed) {
    erpStoreCode.value = trimmed;
  }
}

function resolveInitialStoreCode() {
  const stores = availableStores.value;
  const has184 = stores.find((s) => s.code === "184");
  const fallback = stores[0]?.code || (currentStore.value?.code) || "";
  const initial = has184?.code || fallback;
  if (initial) {
    pickStoreCode(initial);
  }
}

watch(
  () => [auth.isAuthenticated, auth.activeTenantId, auth.activeStoreId],
  (currentScope, previousScope) => {
    const [isAuthenticated, tenantId, storeId] = currentScope;
    const [previousAuthenticated, previousTenantId, previousStoreId] = previousScope ?? [];

    if (!isAuthenticated) {
      erpStore.reset();
      return;
    }

    if (
      isAuthenticated !== previousAuthenticated ||
      tenantId !== previousTenantId ||
      storeId !== previousStoreId
    ) {
      resolveInitialStoreCode();
      void reloadWorkspace();
    }
  },
  { immediate: true }
);

watch(erpStoreCode, () => {
  void reloadWorkspace();
});

watch(activeTab, () => {
  if (activeTab.value === "banco") {
    activeBancoTab.value = "geral";
    return;
  }
  if (activeTab.value === "produtos") {
    void loadProducts();
    return;
  }
  if (activeTab.value !== "banco") {
    recordsSpecificSearchValue.value = "";
    void loadRecords({ page: 1 });
  }
});

watch(searchValue, () => {
  if (activeTab.value === "produtos") {
    void loadProducts({ page: 1 });
  }
});

watch(identifierPrefixValue, () => {
  if (activeTab.value === "produtos") {
    void loadProducts({ page: 1 });
  }
});

watch(recordsSearchValue, () => {
  if (activeRecordsDataType.value) {
    void loadRecords({ page: 1 });
  }
});

watch(recordsSpecificSearchValue, () => {
  if (activeRecordsDataType.value) {
    void loadRecords({ page: 1 });
  }
});

onMounted(() => {
  if (auth.isAuthenticated) {
    resolveInitialStoreCode();
    void reloadWorkspace();
  }
});
</script>

<template>
  <section class="admin-panel erp-panel" data-testid="erp-panel">
    <header class="erp-panel__header">
      <div>
        <h2 class="erp-panel__title">ERP FTP Store 184 MVP</h2>
        <p class="erp-panel__text">
          Produtos funcionais com raw exato do FTP, projeção rápida para busca e trilha de sync por lote.
        </p>
      </div>

      <div class="erp-panel__store-selector">
        <div class="erp-panel__selectors-row">
          <div class="erp-panel__selector-group">
            <label class="erp-panel__store-label">Loja ERP</label>
            <select
              v-if="availableStores.length"
              class="erp-panel__store-select"
              :value="erpStoreCode"
              @change="(e) => pickStoreCode((e.target as HTMLSelectElement).value)"
            >
              <option value="">Selecionar loja...</option>
              <option v-for="store in availableStores" :key="store.id" :value="store.code">
                {{ store.code }} – {{ store.name }}
              </option>
            </select>
            <template v-else>
              <div class="erp-panel__store-controls">
                <input
                  v-model="erpStoreCodeInput"
                  class="erp-panel__store-input"
                  placeholder="Código (ex: 184)"
                  @keydown.enter="applyManualStoreCode"
                />
                <button class="erp-panel__toolbar-btn erp-panel__toolbar-btn--ghost" type="button" @click="applyManualStoreCode">
                  Buscar
                </button>
              </div>
            </template>
          </div>
        </div>
      </div>
    </header>

    <SettingsTabs :tabs="tabs" :active-tab="activeTab" @update:active-tab="activeTab = $event" />

    <template v-if="activeTab === 'produtos'">
      <div class="erp-panel__stats">
        <article class="erp-panel__stat-card">
          <span class="erp-panel__stat-label">Produtos atuais</span>
          <strong class="erp-panel__stat-value">{{ formatNumber(currentProductCount) }}</strong>
          <small>projeção em <code>erp_item_current</code></small>
        </article>
        <article class="erp-panel__stat-card">
          <span class="erp-panel__stat-label">Linhas raw</span>
          <strong class="erp-panel__stat-value">{{ formatNumber(rawItemRows) }}</strong>
          <small>histórico bruto importado</small>
        </article>
        <article class="erp-panel__stat-card">
          <span class="erp-panel__stat-label">Último lote importado</span>
          <strong class="erp-panel__stat-value erp-panel__stat-value--small">{{ formatSourceFileName(lastImportedFile?.sourceName) }}</strong>
          <small>registrado {{ formatDateTime(lastImportedFile?.importedAt) }}</small>
        </article>
        <article class="erp-panel__stat-card">
          <span class="erp-panel__stat-label">Último run</span>
          <strong class="erp-panel__stat-value erp-panel__stat-value--small">{{ lastRun?.status || "sem execução" }}</strong>
          <small>{{ formatDateTime(lastRun?.finishedAt || lastRun?.startedAt) }}</small>
        </article>
      </div>

      <div class="erp-panel__run-meta">
        <div class="erp-panel__run-box">
          <span class="erp-panel__run-label">Arquivos processados</span>
          <strong>{{ formatNumber(lastRun?.filesImported) }}</strong>
          <small>{{ formatNumber(lastRun?.filesSkipped) }} ignorados por checksum</small>
        </div>
        <div class="erp-panel__run-box">
          <span class="erp-panel__run-label">Linhas importadas</span>
          <strong>{{ formatNumber(lastRun?.rowsImported) }}</strong>
          <small>{{ formatNumber(lastRun?.rowsRead) }} lidas do consolidado</small>
        </div>
        <div class="erp-panel__run-box">
          <span class="erp-panel__run-label">Concluído em</span>
          <strong class="erp-panel__run-date">{{ formatDateTime(lastRun?.finishedAt) }}</strong>
          <small>iniciado {{ formatDateTime(lastRun?.startedAt) }}</small>
        </div>
      </div>

      <ErpProductsTable
        :columns="columns"
        :rows="erpStore.products"
        :row-key="(row) => `${row.sku}-${row.identifier}`"
        :total="erpStore.totalProducts"
        :page="erpStore.page"
        :page-size="erpStore.pageSize"
        :page-size-options="pageSizeOptions"
        :search-value="searchValue"
        :identifier-search-value="identifierPrefixValue"
        :loading="erpStore.loadingProducts || erpStore.loadingStatus"
        general-search-placeholder="Busca geral (nome, descricao, SKU, identificador, valor, categoria...)"
        identifier-search-label="Identificador (comeca com)"
        identifier-search-placeholder="Ex: 153"
        :show-identifier-search="true"
        :show-refresh-action="true"
        :show-bootstrap-action="canSync"
        :can-bootstrap="canSync"
        :syncing="erpStore.syncing"
        bootstrap-label="Bootstrap produtos 184"
        bootstrap-busy-label="Sincronizando..."
        empty-title="Nenhum produto no ERP"
        empty-text="Dispare o bootstrap da loja 184 ou ajuste a busca para preencher a grade administrativa."
        storage-key="erp-products-grid-columns-v2"
        testid="erp-products-grid"
        @update:search-value="searchValue = $event"
        @update:identifier-search-value="identifierPrefixValue = $event"
        @update:page="handlePageChange"
        @update:page-size="handlePageSizeChange"
        @refresh="reloadWorkspace"
        @bootstrap="handleBootstrap"
      >
        <template #cell-name="{ row }">
          <div class="erp-panel__name-cell">
            <strong>{{ row.name }}</strong>
            <span>{{ row.description || row.brandName || "Sem descricao complementar" }}</span>
          </div>
        </template>

        <template #cell-priceRaw="{ row }">
          <span class="erp-panel__price-cell">{{ formatPrice(row.priceRaw, row.priceCents) }}</span>
        </template>

        <template #cell-sourceUpdatedAt="{ row }">
          <span class="erp-panel__muted-cell">{{ formatDateTime(row.sourceUpdatedAt || row.sourceCreatedAt) }}</span>
        </template>
      </ErpProductsTable>
    </template>

    <section v-else-if="activeTab === 'banco'" class="erp-banco">
      <div class="erp-banco__intro">
        <h3>{{ activeBancoSection.title }}</h3>
        <p>{{ activeBancoSection.text }}</p>
      </div>

      <SettingsTabs :tabs="bancoTabs" :active-tab="activeBancoTab" @update:active-tab="activeBancoTab = $event" />

      <div class="erp-banco__grid">
        <article
          v-for="item in activeBancoSection.cards"
          :key="item.table"
          class="erp-banco__card"
          :class="`erp-banco__card--${item.badge.split(' ')[0]}`"
        >
          <div class="erp-banco__card-head">
            <code class="erp-banco__table-name">{{ item.table }}</code>
            <span class="erp-banco__badge">{{ item.badge }}</span>
          </div>
          <strong class="erp-banco__card-label">{{ item.label }}</strong>
          <p class="erp-banco__card-desc">{{ item.desc }}</p>
          <div v-if="item.table === 'erp_item_current'" class="erp-banco__live">
            <span class="erp-banco__live-label">Registros atuais</span>
            <strong class="erp-banco__live-count">{{ currentProductCount.toLocaleString("pt-BR") }}</strong>
          </div>
          <div v-if="item.table === 'erp_item_raw'" class="erp-banco__live">
            <span class="erp-banco__live-label">Linhas brutas importadas</span>
            <strong class="erp-banco__live-count">{{ rawItemRows.toLocaleString("pt-BR") }}</strong>
          </div>
        </article>
      </div>

      <div class="erp-banco__note">
        {{ activeBancoSection.note }}
      </div>
    </section>

    <template v-else>
      <div class="erp-panel__stats">
        <article class="erp-panel__stat-card">
          <span class="erp-panel__stat-label">Registros atuais</span>
          <strong class="erp-panel__stat-value">{{ formatNumber(activeRecordsTotal) }}</strong>
          <small>linhas cadastradas nesta aba</small>
        </article>
        <article class="erp-panel__stat-card">
          <span class="erp-panel__stat-label">Ultimo lote importado</span>
          <strong class="erp-panel__stat-value erp-panel__stat-value--small">
            {{ formatNumber(activeRecordsLastImportedFile?.recordCount) }} registros
          </strong>
          <small>registrado {{ formatDateTime(activeRecordsLastImportedFile?.importedAt) }}</small>
        </article>
        <article class="erp-panel__stat-card">
          <span class="erp-panel__stat-label">Ultimo run</span>
          <strong class="erp-panel__stat-value erp-panel__stat-value--small">{{ activeRecordsLastRun?.status || "sem execucao" }}</strong>
          <small>{{ formatDateTime(activeRecordsLastRun?.finishedAt || activeRecordsLastRun?.startedAt) }}</small>
        </article>
        <article class="erp-panel__stat-card">
          <span class="erp-panel__stat-label">Linhas importadas</span>
          <strong class="erp-panel__stat-value">{{ formatNumber(activeRecordsLastRun?.rowsImported) }}</strong>
          <small>{{ formatNumber(activeRecordsLastRun?.filesImported) }} lotes processados</small>
        </article>
      </div>

      <ErpProductsTable
        :columns="activeRecordsColumns"
        :rows="erpStore.records"
        :row-key="(row, index) => String(row.id || row.order_id || row.original_id || row.identifier || row.cpf || index)"
        :total="erpStore.totalRecords"
        :page="erpStore.recordsPage"
        :page-size="erpStore.recordsPageSize"
        :page-size-options="pageSizeOptions"
        :search-value="recordsSearchValue"
        :identifier-search-value="recordsSpecificSearchValue"
        :loading="erpStore.loadingRecords || erpStore.loadingStatus"
        :show-identifier-search="true"
        :identifier-search-label="activeRecordsSpecificSearch.label"
        :identifier-search-placeholder="activeRecordsSpecificSearch.placeholder"
        :show-bootstrap-action="canSync"
        :can-bootstrap="canSync"
        :syncing="erpStore.syncing"
        :bootstrap-label="activeRecordsBootstrapLabel"
        bootstrap-busy-label="Sincronizando..."
        :general-search-placeholder="activeRecordsGeneralSearchPlaceholder"
        empty-title="Nenhum registro encontrado"
        empty-text="Nao ha linhas importadas para este tipo na loja selecionada. Use o bootstrap da aba para carregar o consolidado."
        :storage-key="`erp-${activeTab}-grid-columns-v3`"
        :testid="`erp-${activeTab}-grid`"
        @update:search-value="recordsSearchValue = $event"
        @update:identifier-search-value="recordsSpecificSearchValue = $event"
        @update:page="handleRecordsPageChange"
        @update:page-size="handleRecordsPageSizeChange"
        @refresh="reloadWorkspace"
        @bootstrap="handleRecordsBootstrap"
      />
    </template>
  </section>
</template>

<style scoped>
.erp-panel {
  display: grid;
  gap: 1rem;
  align-content: start;
}

.erp-panel__header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 1rem;
  flex-wrap: wrap;
}

.erp-panel__title {
  margin: 0;
  font-size: 1.5rem;
  font-weight: 700;
  color: var(--text-main);
}

.erp-panel__text {
  margin: 0.35rem 0 0;
  max-width: 52rem;
  color: var(--text-muted);
  line-height: 1.55;
}

.erp-panel__store-chip {
  min-width: 16rem;
  display: grid;
  gap: 0.2rem;
  padding: 0.85rem 1rem;
  border-radius: 1rem;
  border: 1px solid rgba(98, 129, 255, 0.22);
  background: linear-gradient(135deg, rgba(20, 28, 42, 0.96), rgba(12, 40, 47, 0.92));
  box-shadow: var(--shadow-card);
}

.erp-panel__store-chip strong {
  color: var(--text-main);
}

.erp-panel__store-chip span {
  color: var(--text-muted);
  font-size: 0.86rem;
}

.erp-panel__stats,
.erp-panel__run-meta {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 0.85rem;
}

.erp-panel__stat-card,
.erp-panel__run-box {
  display: grid;
  gap: 0.35rem;
  padding: 1rem;
  border-radius: 1rem;
  border: 1px solid var(--line-soft);
  background: rgba(13, 18, 29, 0.88);
  box-shadow: var(--shadow-card);
}

.erp-panel__stat-label,
.erp-panel__run-label {
  color: var(--text-muted);
  font-size: 0.78rem;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

.erp-panel__stat-value {
  color: var(--text-main);
  font-size: 1.7rem;
  font-variant-numeric: tabular-nums;
  line-height: 1;
}

.erp-panel__stat-value--small {
  font-size: 0.95rem;
  line-height: 1.3;
  word-break: break-word;
}

.erp-panel__run-date {
  color: var(--text-main);
  font-size: 0.9rem;
  line-height: 1.4;
  word-break: break-word;
}

.erp-panel__stat-card small,
.erp-panel__run-box small {
  color: var(--text-muted);
}

.erp-panel__toolbar-actions {
  display: flex;
  gap: 0.55rem;
  flex-wrap: wrap;
}

.erp-panel__prefix-filter {
  display: grid;
  gap: 0.2rem;
  min-width: min(100%, 260px);
}

.erp-panel__prefix-filter span {
  font-size: 0.72rem;
  color: var(--text-muted);
}

.erp-panel__prefix-input {
  width: 100%;
  min-height: 2.45rem;
  padding: 0 0.8rem;
  border-radius: 0.8rem;
  border: 1px solid var(--line-soft);
  background: rgba(17, 24, 39, 0.92);
  color: var(--text-main);
}

.erp-panel__toolbar-btn {
  min-height: 2.45rem;
  padding: 0 0.95rem;
  border-radius: 0.8rem;
  border: 1px solid var(--line-soft);
  background: rgba(17, 24, 39, 0.92);
  color: var(--text-main);
  font-weight: 600;
  cursor: pointer;
  transition: transform 0.16s ease, border-color 0.16s ease, background-color 0.16s ease;
}

.erp-panel__toolbar-btn:hover:not(:disabled) {
  transform: translateY(-1px);
  border-color: rgba(98, 129, 255, 0.35);
}

.erp-panel__toolbar-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.erp-panel__toolbar-btn--primary {
  border-color: rgba(83, 198, 160, 0.32);
  background: linear-gradient(135deg, rgba(13, 102, 87, 0.92), rgba(14, 73, 67, 0.94));
}

.erp-panel__name-cell {
  display: grid;
  gap: 0.2rem;
}

.erp-panel__name-cell strong {
  color: var(--text-main);
  font-size: 0.9rem;
}

.erp-panel__name-cell span,
.erp-panel__muted-cell {
  color: var(--text-muted);
  font-size: 0.78rem;
}

.erp-panel__price-cell {
  color: #b9ffd2;
  font-weight: 700;
}

.erp-placeholder {
  display: grid;
  gap: 0.7rem;
  padding: 1.3rem;
  border-radius: 1rem;
  border: 1px dashed rgba(98, 129, 255, 0.32);
  background: linear-gradient(135deg, rgba(18, 24, 38, 0.92), rgba(24, 33, 58, 0.82));
}

.erp-placeholder__badge {
  width: fit-content;
  padding: 0.28rem 0.65rem;
  border-radius: 999px;
  background: rgba(98, 129, 255, 0.16);
  color: #d8e1ff;
  font-size: 0.74rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.erp-placeholder h3 {
  margin: 0;
  color: var(--text-main);
}

.erp-placeholder p {
  margin: 0;
  max-width: 52rem;
  color: var(--text-muted);
  line-height: 1.6;
}

/* ── store selector ── */
.erp-panel__store-selector {
  display: grid;
  gap: 0.4rem;
}

.erp-panel__selectors-row {
  display: flex;
  gap: 0.75rem;
  flex-wrap: wrap;
  align-items: flex-end;
}

.erp-panel__selector-group {
  display: grid;
  gap: 0.3rem;
  min-width: 14rem;
}

.erp-panel__store-label {
  font-size: 0.74rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--text-muted);
}

.erp-panel__store-controls {
  display: flex;
  gap: 0.5rem;
  align-items: center;
}

.erp-panel__store-select,
.erp-panel__store-input {
  width: 100%;
  min-height: 2.4rem;
  padding: 0 0.75rem;
  border-radius: 0.8rem;
  border: 1px solid var(--line-soft);
  background: rgba(17, 24, 39, 0.92);
  color: var(--text-main);
  font-size: 0.9rem;
}

.erp-panel__store-select--disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

/* ── banco tab ── */
.erp-banco {
  display: grid;
  gap: 1.2rem;
}

.erp-banco__intro h3 {
  margin: 0 0 0.5rem;
  color: var(--text-main);
}

.erp-banco__intro p {
  margin: 0;
  max-width: 64rem;
  color: var(--text-muted);
  line-height: 1.6;
}

.erp-banco__grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 0.85rem;
}

.erp-banco__card {
  display: grid;
  gap: 0.4rem;
  padding: 1rem;
  border-radius: 1rem;
  border: 1px solid var(--line-soft);
  background: rgba(13, 18, 29, 0.9);
}

.erp-banco__card--raw { border-color: rgba(120, 80, 200, 0.25); }
.erp-banco__card--projecao { border-color: rgba(83, 198, 160, 0.3); }
.erp-banco__card--controle { border-color: rgba(98, 129, 255, 0.25); }
.erp-banco__card--outbox { border-color: rgba(200, 170, 60, 0.25); }

.erp-banco__card-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 0.5rem;
}

.erp-banco__table-name {
  font-size: 0.8rem;
  color: #b8d0ff;
  word-break: break-all;
}

.erp-banco__badge {
  font-size: 0.68rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.07em;
  padding: 0.18rem 0.5rem;
  border-radius: 999px;
  background: rgba(98, 129, 255, 0.14);
  color: #c8d8ff;
  white-space: nowrap;
}

.erp-banco__card-label {
  color: var(--text-main);
  font-size: 0.9rem;
}

.erp-banco__card-desc {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.8rem;
  line-height: 1.5;
}

.erp-banco__live {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 0.3rem;
  padding-top: 0.5rem;
  border-top: 1px solid var(--line-soft);
}

.erp-banco__live-label {
  color: var(--text-muted);
  font-size: 0.75rem;
}

.erp-banco__live-count {
  color: #b9ffd2;
  font-size: 1.1rem;
}

.erp-banco__note {
  padding: 0.9rem 1.1rem;
  border-radius: 0.9rem;
  border: 1px solid rgba(200, 170, 60, 0.25);
  background: rgba(180, 140, 30, 0.1);
  color: var(--text-muted);
  font-size: 0.82rem;
  line-height: 1.6;
}

.erp-banco__note code {
  color: #c8d8ff;
  font-size: 0.78rem;
}

@media (max-width: 720px) {
  .erp-panel__header {
    align-items: stretch;
  }

  .erp-panel__store-selector {
    min-width: 0;
    width: 100%;
  }

  .erp-panel__toolbar-actions {
    width: 100%;
  }

  .erp-panel__prefix-filter {
    width: 100%;
    min-width: 0;
  }

  .erp-panel__toolbar-btn {
    flex: 1 1 12rem;
  }
}
</style>
