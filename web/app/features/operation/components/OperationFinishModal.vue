<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import OperationProductPicker from "~/features/operation/components/OperationProductPicker.vue";
import { useOperationsStore } from "~/stores/operations";
import { useUiStore } from "~/stores/ui";

const props = defineProps({
  state: {
    type: Object,
    required: true
  }
});

const operationsStore = useOperationsStore();
const ui = useUiStore();
const FINISH_MODAL_DRAFT_STORAGE_KEY = "ldv_finish_modal_drafts_v1";
const FINISH_MODAL_DRAFT_MAX_AGE_MS = 1000 * 60 * 60 * 24;
const PRODUCT_SEEN_NONE_DETAIL_KEY = "__none__";

function readDraftStorage() {
  if (import.meta.server) {
    return {};
  }

  try {
    const parsed = JSON.parse(window.sessionStorage.getItem(FINISH_MODAL_DRAFT_STORAGE_KEY) || "{}");
    return parsed && typeof parsed === "object" && parsed.drafts && typeof parsed.drafts === "object"
      ? parsed.drafts
      : {};
  } catch {
    return {};
  }
}

function writeDraftStorage(drafts) {
  if (import.meta.server) {
    return;
  }

  const now = Date.now();
  const normalizedDrafts = Object.fromEntries(
    Object.entries(drafts || {}).filter(([, entry]) =>
      entry && typeof entry === "object" && now - Number(entry.updatedAt || 0) <= FINISH_MODAL_DRAFT_MAX_AGE_MS
    )
  );

  if (Object.keys(normalizedDrafts).length === 0) {
    window.sessionStorage.removeItem(FINISH_MODAL_DRAFT_STORAGE_KEY);
    return;
  }

  window.sessionStorage.setItem(FINISH_MODAL_DRAFT_STORAGE_KEY, JSON.stringify({
    version: 1,
    drafts: normalizedDrafts
  }));
}

function removeStoredDraft(draftKey) {
  const normalizedKey = String(draftKey || "").trim();

  if (!normalizedKey) {
    return;
  }

  const drafts = readDraftStorage();
  delete drafts[normalizedKey];
  writeDraftStorage(drafts);
}

function createEmptyForm() {
  return {
    outcome: "",
    isExistingCustomer: false,
    productsSeen: [],
    productsClosed: [],
    productsSeenNone: false,
    productSeenNotes: "",
    customerName: "",
    customerPhone: "",
    customerEmail: "",
    customerProfessionId: "",
    visitReasonIds: [],
    visitReasonNotInformed: false,
    visitReasonDetails: {},
    customerSourceIds: [],
    customerSourceNotInformed: false,
    customerSourceDetails: {},
    queueJumpReasonId: "",
    lossReasonIds: [],
    lossReasonDetails: {},
    notes: ""
  };
}

function normalizeIdList(values = []) {
  return [...new Set((Array.isArray(values) ? values : []).map((value) => String(value || "").trim()).filter(Boolean))];
}

function syncSelectedDetails(itemIds = [], details = {}) {
  return Object.fromEntries(
    normalizeIdList(itemIds).map((itemId) => [itemId, String(details?.[itemId] || "")])
  );
}

function findOptionByLabel(options, label) {
  const normalizedLabel = String(label || "").trim().toLowerCase();

  if (!normalizedLabel) {
    return null;
  }

  return (options || []).find((item) => String(item?.label || "").trim().toLowerCase() === normalizedLabel) || null;
}

function normalizeProducts(items = []) {
  return (Array.isArray(items) ? items : []).map((item, index) => ({
    id: String(item?.id || `${item?.name || "produto"}-${index}`),
    name: String(item?.name || "").trim(),
    label: String(item?.label || item?.name || "").trim(),
    price: Math.max(0, Number(item?.price ?? item?.basePrice ?? 0) || 0),
    code: String(item?.code || "").trim(),
    isCustom: Boolean(item?.isCustom)
  }));
}

function getProductIdentity(product) {
  const code = String(product?.code || "").trim().toLowerCase();
  const id = String(product?.id || "").trim().toLowerCase();
  const name = String(product?.name || product?.label || "").trim().toLowerCase();

  return code ? `code:${code}` : id ? `id:${id}` : name ? `name:${name}` : "";
}

function mergeProductEntries(...groups) {
  const seen = new Set();
  const merged = [];

  groups.flat().forEach((product) => {
    const normalized = normalizeProducts([product])[0];
    const identity = getProductIdentity(normalized);

    if (!identity || seen.has(identity)) {
      return;
    }

    seen.add(identity);
    merged.push(normalized);
  });

  return merged;
}

function buildInitialForm(state, draft) {
  const currentDraft = draft || {};
  const selectedVisitReasonIds = normalizeIdList(currentDraft.visitReasonIds || currentDraft.visitReasons);
  const selectedSourceIds = normalizeIdList(
    Array.isArray(currentDraft.customerSourceIds) || Array.isArray(currentDraft.customerSources)
      ? currentDraft.customerSourceIds || currentDraft.customerSources
      : currentDraft.customerSource
        ? [currentDraft.customerSource]
        : []
  );
  const selectedProfession =
    (state.professionOptions || []).find((option) => option.id === String(currentDraft.customerProfessionId || "")) ||
    findOptionByLabel(state.professionOptions, currentDraft.customerProfession);
  const selectedQueueJumpReason =
    (state.queueJumpReasonOptions || []).find((option) => option.id === String(currentDraft.queueJumpReasonId || "")) ||
    findOptionByLabel(state.queueJumpReasonOptions, currentDraft.queueJumpReason);
  const selectedLossReasonIds = normalizeIdList(
    Array.isArray(currentDraft.lossReasonIds) || Array.isArray(currentDraft.lossReasons)
      ? currentDraft.lossReasonIds || currentDraft.lossReasons
      : currentDraft.lossReasonId
        ? [currentDraft.lossReasonId]
        : []
  );
  const selectedLossReason =
    selectedLossReasonIds[0]
      ? (state.lossReasonOptions || []).find((option) => option.id === selectedLossReasonIds[0]) || null
      : findOptionByLabel(state.lossReasonOptions, currentDraft.lossReason);
  const resolvedLossReasonIds = selectedLossReasonIds.length
    ? selectedLossReasonIds
    : selectedLossReason?.id
      ? [selectedLossReason.id]
      : [];

  return {
    outcome: String(currentDraft.outcome || ""),
    isExistingCustomer: Boolean(currentDraft.isExistingCustomer),
    productsSeen: normalizeProducts(currentDraft.productsSeen),
    productsClosed: normalizeProducts(currentDraft.productsClosed),
    productsSeenNone: Boolean(currentDraft.productsSeenNone),
    productSeenNotes: String(
      currentDraft.productSeenNotes
      || ((Array.isArray(currentDraft.productsSeen) && currentDraft.productsSeen.length) ? "" : currentDraft.productSeen)
      || ""
    ),
    customerName: String(currentDraft.customerName || ""),
    customerPhone: String(currentDraft.customerPhone || ""),
    customerEmail: String(currentDraft.customerEmail || ""),
    customerProfessionId: selectedProfession?.id || "",
    visitReasonIds: selectedVisitReasonIds,
    visitReasonNotInformed: Boolean(currentDraft.visitReasonsNotInformed) && selectedVisitReasonIds.length === 0,
    visitReasonDetails: syncSelectedDetails(selectedVisitReasonIds, currentDraft.visitReasonDetails),
    customerSourceIds: selectedSourceIds,
    customerSourceNotInformed: Boolean(currentDraft.customerSourcesNotInformed) && selectedSourceIds.length === 0,
    customerSourceDetails: syncSelectedDetails(
      selectedSourceIds,
      currentDraft.customerSourceDetails && typeof currentDraft.customerSourceDetails === "object"
        ? currentDraft.customerSourceDetails
        : selectedSourceIds[0]
          ? { [selectedSourceIds[0]]: String(currentDraft.customerSourceDetail || "") }
          : {}
    ),
    queueJumpReasonId: selectedQueueJumpReason?.id || "",
    lossReasonIds: String(currentDraft.outcome || "") === "nao-compra" ? resolvedLossReasonIds : [],
    lossReasonDetails: syncSelectedDetails(
      resolvedLossReasonIds,
      currentDraft.lossReasonDetails && typeof currentDraft.lossReasonDetails === "object"
        ? currentDraft.lossReasonDetails
        : {}
    ),
    notes: String(currentDraft.notes || "")
  };
}

function formatCurrency(value) {
  return Number(value || 0).toLocaleString("pt-BR", { style: "currency", currency: "BRL" });
}

function formatPhoneMask(value) {
  const digits = String(value || "").replace(/\D/g, "").slice(0, 11);

  if (!digits) {
    return "";
  }

  if (digits.length <= 2) {
    return `(${digits}`;
  }

  if (digits.length <= 6) {
    return `(${digits.slice(0, 2)}) ${digits.slice(2)}`;
  }

  if (digits.length <= 10) {
    return `(${digits.slice(0, 2)}) ${digits.slice(2, 6)}-${digits.slice(6)}`;
  }

  return `(${digits.slice(0, 2)}) ${digits.slice(2, 7)}-${digits.slice(7)}`;
}

function handleCustomerPhoneInput(event) {
  const maskedValue = formatPhoneMask(event?.target?.value || form.customerPhone);
  form.customerPhone = maskedValue;

  if (event?.target) {
    event.target.value = maskedValue;
  }
}

function mapOptionToPickerItem(option, meta = "") {
  return {
    id: String(option?.id || ""),
    label: String(option?.label || option?.name || "").trim(),
    meta: String(meta || "").trim()
  };
}

function resolveModalText(value, fallback) {
  const normalizedValue = String(value || "").trim();
  return normalizedValue || fallback;
}

function resolveModalBoolean(value, fallback = false) {
  return typeof value === "boolean" ? value : fallback;
}

function resolveModalNumber(value, fallback = 0, minimum = 0) {
  return Math.max(minimum, Number(value ?? fallback) || fallback || 0);
}

const modalConfig = computed(() => props.state.modalConfig || {});
const showCustomerNameField = computed(() => resolveModalBoolean(modalConfig.value.showCustomerNameField, true));
const showCustomerPhoneField = computed(() => resolveModalBoolean(modalConfig.value.showCustomerPhoneField, true));
const showEmailField = computed(() => resolveModalBoolean(modalConfig.value.showEmailField, true));
const showProfessionField = computed(() => resolveModalBoolean(modalConfig.value.showProfessionField, true));
const showNotesField = computed(() => resolveModalBoolean(modalConfig.value.showNotesField, true));
const showProductSeenField = computed(() => resolveModalBoolean(modalConfig.value.showProductSeenField, true));
const showProductSeenNotesField = computed(() => resolveModalBoolean(modalConfig.value.showProductSeenNotesField, true));
const showProductClosedField = computed(() => resolveModalBoolean(modalConfig.value.showProductClosedField, true));
const showVisitReasonField = computed(() => resolveModalBoolean(modalConfig.value.showVisitReasonField, true));
const showCustomerSourceField = computed(() => resolveModalBoolean(modalConfig.value.showCustomerSourceField, true));
const showExistingCustomerField = computed(() => resolveModalBoolean(modalConfig.value.showExistingCustomerField, true));
const showQueueJumpReasonField = computed(() => resolveModalBoolean(modalConfig.value.showQueueJumpReasonField, true));
const showLossReasonField = computed(() => resolveModalBoolean(modalConfig.value.showLossReasonField, true));
const requireCustomerNameField = computed(() =>
  resolveModalBoolean(modalConfig.value.requireCustomerNameField, resolveModalBoolean(modalConfig.value.requireCustomerNamePhone, true))
);
const requireCustomerPhoneField = computed(() =>
  resolveModalBoolean(modalConfig.value.requireCustomerPhoneField, resolveModalBoolean(modalConfig.value.requireCustomerNamePhone, true))
);
const requireEmailField = computed(() => resolveModalBoolean(modalConfig.value.requireEmailField, false));
const requireProfessionField = computed(() => resolveModalBoolean(modalConfig.value.requireProfessionField, false));
const requireNotesField = computed(() => resolveModalBoolean(modalConfig.value.requireNotesField, false));
const requireProductSeenField = computed(() =>
  resolveModalBoolean(modalConfig.value.requireProductSeenField, resolveModalBoolean(modalConfig.value.requireProduct, true))
);
const requireProductSeenNotesField = computed(() =>
  resolveModalBoolean(modalConfig.value.requireProductSeenNotesField, false)
);
const requireProductClosedField = computed(() =>
  resolveModalBoolean(modalConfig.value.requireProductClosedField, resolveModalBoolean(modalConfig.value.requireProduct, true))
);
const requireVisitReasonField = computed(() => resolveModalBoolean(modalConfig.value.requireVisitReason, true));
const requireCustomerSourceField = computed(() => resolveModalBoolean(modalConfig.value.requireCustomerSource, true));
const allowProductSeenNone = computed(() => resolveModalBoolean(modalConfig.value.allowProductSeenNone, true));
const requireProductSeenNotesWhenNone = computed(() =>
  resolveModalBoolean(modalConfig.value.requireProductSeenNotesWhenNone, true)
);
const productSeenNotesMinChars = computed(() => resolveModalNumber(modalConfig.value.productSeenNotesMinChars, 20, 1));
const requireQueueJumpReasonField = computed(() => resolveModalBoolean(modalConfig.value.requireQueueJumpReasonField, true));
const requireLossReasonField = computed(() => resolveModalBoolean(modalConfig.value.requireLossReasonField, true));
const showCustomerSection = computed(() =>
  showCustomerNameField.value
  || showCustomerPhoneField.value
  || showEmailField.value
  || showProfessionField.value
  || showExistingCustomerField.value
  || showCustomerSourceField.value
  || showNotesField.value
);
const visitReasonSelectionMode = computed(() =>
  modalConfig.value.visitReasonSelectionMode === "single" ? "single" : "multiple"
);
const lossReasonSelectionMode = computed(() =>
  modalConfig.value.lossReasonSelectionMode === "multiple" ? "multiple" : "single"
);
const customerSourceSelectionMode = computed(() =>
  modalConfig.value.customerSourceSelectionMode === "multiple" ? "multiple" : "single"
);
const isVisitReasonMultiple = computed(() => visitReasonSelectionMode.value === "multiple");
const isLossReasonMultiple = computed(() => lossReasonSelectionMode.value === "multiple");
const isCustomerSourceMultiple = computed(() => customerSourceSelectionMode.value === "multiple");
const visitReasonConfiguredDetailMode = computed(() => {
  const configuredMode = modalConfig.value.visitReasonDetailMode;

  if (["off", "shared", "per-item"].includes(configuredMode)) {
    return configuredMode;
  }

  return modalConfig.value.showVisitReasonDetails === false ? "off" : "shared";
});
const lossReasonConfiguredDetailMode = computed(() => {
  const configuredMode = modalConfig.value.lossReasonDetailMode;

  if (["off", "shared", "per-item"].includes(configuredMode)) {
    return configuredMode;
  }

  return "off";
});
const customerSourceConfiguredDetailMode = computed(() => {
  const configuredMode = modalConfig.value.customerSourceDetailMode;

  if (["off", "shared", "per-item"].includes(configuredMode)) {
    return configuredMode;
  }

  return modalConfig.value.showCustomerSourceDetails === false ? "off" : "shared";
});
const visitReasonDetailsEnabled = computed(() => visitReasonConfiguredDetailMode.value !== "off");
const lossReasonDetailsEnabled = computed(() => lossReasonConfiguredDetailMode.value !== "off");
const customerSourceDetailsEnabled = computed(() => customerSourceConfiguredDetailMode.value !== "off");
const visitReasonPickerDetailMode = computed(() =>
  visitReasonConfiguredDetailMode.value === "per-item" ? "per-item" : "shared"
);
const lossReasonPickerDetailMode = computed(() =>
  lossReasonConfiguredDetailMode.value === "per-item" ? "per-item" : "shared"
);
const customerSourcePickerDetailMode = computed(() =>
  customerSourceConfiguredDetailMode.value === "per-item" ? "per-item" : "shared"
);
const service = computed(() =>
  (props.state.activeServices || []).find((item) => item.id === props.state.finishModalPersonId) || null
);
const draft = computed(() => props.state.finishModalDraft || null);
const serviceDraftKey = computed(() => {
  const currentService = service.value;
  const storeId = String(props.state.activeStoreId || "").trim();
  const serviceId = String(currentService?.serviceId || "").trim();

  return storeId && serviceId ? `${storeId}:${serviceId}` : "";
});
const hasRestoredDraft = computed(() =>
  Boolean(restoredDraftKey.value && restoredDraftKey.value === serviceDraftKey.value)
);
const isClosedOutcome = computed(() => form.outcome === "compra" || form.outcome === "reserva");
const trimmedProductSeenNotes = computed(() => String(form.productSeenNotes || "").trim());
const productSeenNotesLabel = computed(() => resolveModalText(modalConfig.value.productSeenNotesLabel, "Observação dos interesses"));
const productSeenNotesPlaceholder = computed(() =>
  resolveModalText(
    modalConfig.value.productSeenNotesPlaceholder,
    "Descreva referência, pedido específico, contexto do cliente ou justificativa quando não houver interesse identificado."
  )
);
const canUseProductSeenNotes = computed(() =>
  showProductSeenField.value
  && showProductSeenNotesField.value
  && (
    form.productsSeen.length > 0
    || (allowProductSeenNone.value && form.productsSeenNone)
  )
);
const isProductSeenNoneSelected = computed(() =>
  showProductSeenField.value
  && showProductSeenNotesField.value
  && allowProductSeenNone.value
  && form.productsSeenNone
  && form.productsSeen.length === 0
);
const productSeenNotesForPayload = computed(() =>
  canUseProductSeenNotes.value ? trimmedProductSeenNotes.value : ""
);
const isProductSeenNotesRequired = computed(() =>
  isProductSeenNoneSelected.value && (requireProductSeenNotesField.value || requireProductSeenNotesWhenNone.value)
);
const isProductSeenNotesValid = computed(() =>
  !isProductSeenNotesRequired.value || trimmedProductSeenNotes.value.length >= productSeenNotesMinChars.value
);
const productSeenNotesHelperText = computed(() => {
  if (isProductSeenNotesRequired.value) {
    return `Obrigatório quando nenhum interesse for identificado. Informe pelo menos ${productSeenNotesMinChars.value} caracteres.`;
  }

  return "Use este campo para detalhar referência, gosto, pedido especial ou algo que ainda não existe em loja.";
});
const productSeenDetailMap = computed(() => {
  const note = trimmedProductSeenNotes.value;

  if (!canUseProductSeenNotes.value || isProductSeenNoneSelected.value || !note) {
    return {};
  }

  return Object.fromEntries(
    normalizeProducts(form.productsSeen)
      .map((item) => String(item.id || "").trim())
      .filter(Boolean)
      .map((itemId) => [itemId, note])
  );
});
const closedProductLabel = computed(() => {
  const configuredLabel = String(modalConfig.value.productClosedLabel || "").trim();
  if (configuredLabel && !["produto reservado/comprado", "produto fechado"].includes(configuredLabel.toLowerCase())) {
    return configuredLabel;
  }

  if (form.outcome === "compra") {
    return "Compra";
  }

  if (form.outcome === "reserva") {
    return "Reserva";
  }

  return "Fechamento";
});
const closedProductHelperText = computed(() => {
  if (form.outcome === "compra") {
    return "";
  }

  if (form.outcome === "reserva") {
    return "";
  }

  return "Registre o item fechado quando o atendimento terminar em compra ou reserva.";
});
const selectedProfessionLabel = computed(
  () => props.state.professionOptions.find((option) => option.id === form.customerProfessionId)?.label || ""
);
const selectedVisitReasonIdSet = computed(() => new Set(normalizeIdList(form.visitReasonIds)));
const selectedLossReasonIdSet = computed(() => new Set(normalizeIdList(form.lossReasonIds)));
const selectedCustomerSourceIdSet = computed(() => new Set(normalizeIdList(form.customerSourceIds)));
const closedTotal = computed(() =>
  form.productsClosed.reduce((sum, product) => sum + (Number(product.price) || 0), 0)
);

const formStep1Quality = computed(() => {
  const checks = {
    outcome: !!form.outcome
  };

  if (showProductSeenField.value && requireProductSeenField.value) {
    checks.productSeen = form.productsSeen.length > 0 || form.productsSeenNone;
  }

  if (isProductSeenNotesRequired.value) {
    checks.productSeenNotes = isProductSeenNotesValid.value;
  }

  if (isClosedOutcome.value && showProductClosedField.value && requireProductClosedField.value) {
    checks.productClosed = form.productsClosed.length > 0;
  }

  const total = Object.keys(checks).length;
  const filled = Object.values(checks).filter(Boolean).length;
  const isComplete = filled === total;

  return { checks, filled, total, isComplete };
});

const formQuality = computed(() => {
  const hasText = (v) => String(v || "").trim().length > 0;

  const checks = {};

  if (showCustomerNameField.value && requireCustomerNameField.value) {
    checks.customerName = hasText(form.customerName);
  }

  if (showCustomerPhoneField.value && requireCustomerPhoneField.value) {
    checks.customerPhone = hasText(form.customerPhone);
  }

  if (showProductSeenField.value && requireProductSeenField.value) {
    checks.productSeen = form.productsSeen.length > 0 || form.productsSeenNone;
  }

  if (isProductSeenNotesRequired.value) {
    checks.productSeenNotes = isProductSeenNotesValid.value;
  }

  if (isClosedOutcome.value && showProductClosedField.value && requireProductClosedField.value) {
    checks.productClosed = form.productsClosed.length > 0;
  }

  if (showVisitReasonField.value && requireVisitReasonField.value) {
    checks.visitReasons = form.visitReasonIds.length > 0 || form.visitReasonNotInformed;
  }

  if (showCustomerSourceField.value && requireCustomerSourceField.value) {
    checks.customerSources = form.customerSourceIds.length > 0 || form.customerSourceNotInformed;
  }

  if (service.value?.startMode === "queue-jump" && showQueueJumpReasonField.value && requireQueueJumpReasonField.value) {
    checks.queueJumpReason = Boolean(selectedQueueJumpReasonLabel.value);
  }

  if (form.outcome === "nao-compra" && showLossReasonField.value && requireLossReasonField.value) {
    checks.lossReason = form.lossReasonIds.length > 0;
  }

  if (showEmailField.value && requireEmailField.value) {
    checks.customerEmail = hasText(form.customerEmail);
  }

  if (showProfessionField.value && requireProfessionField.value) {
    checks.customerProfession = !!form.customerProfessionId;
  }

  if (showNotesField.value && requireNotesField.value) {
    checks.notes = hasText(form.notes);
  }

  const coreTotal = Object.keys(checks).length;
  const coreFilledCount = Object.values(checks).filter(Boolean).length;
  const hasNotes = hasText(form.notes) && showNotesField.value;
  const isCoreComplete = coreFilledCount === coreTotal;
  const level = isCoreComplete ? (hasNotes ? "excellent" : "complete") : "incomplete";
  const levelLabels = { excellent: "Excelente", complete: "Completo", incomplete: "Incompleto" };

  return { checks, coreFilledCount, coreTotal, hasNotes, isCoreComplete, level, levelLabel: levelLabels[level] };
});
const customProducts = ref([]);
const restoredDraftKey = ref("");
let isApplyingDraft = false;
const productCatalogItems = computed(() =>
  (props.state.productCatalog || []).map((product) => ({
    id: String(product.id || ""),
    label: String(product.name || "").trim(),
    name: String(product.name || "").trim(),
    category: String(product.category || "").trim(),
    code: String(product.code || "").trim(),
    price: Math.max(0, Number(product.basePrice || 0)),
    basePrice: Math.max(0, Number(product.basePrice || 0))
  }))
);
const productPickerOptions = computed(() =>
  mergeProductEntries(productCatalogItems.value, customProducts.value)
);
const professionPickerOptions = computed(() =>
  (props.state.professionOptions || []).map((option) => mapOptionToPickerItem(option))
);
const professionSelectedItems = computed({
  get: () => professionPickerOptions.value.filter((option) => option.id === form.customerProfessionId),
  set: (items) => {
    form.customerProfessionId = items[0]?.id || "";
  }
});
const visitReasonPickerOptions = computed(() =>
  (props.state.visitReasonOptions || []).map((option) => mapOptionToPickerItem(option))
);
const visitReasonSelectedItems = computed({
  get: () => visitReasonPickerOptions.value.filter((option) => selectedVisitReasonIdSet.value.has(option.id)),
  set: (items) => {
    form.visitReasonIds = normalizeIdList(items.map((item) => item.id));
    form.visitReasonDetails = syncSelectedDetails(form.visitReasonIds, form.visitReasonDetails);
    form.visitReasonNotInformed = false;
  }
});
const customerSourcePickerOptions = computed(() =>
  (props.state.customerSourceOptions || []).map((option) => mapOptionToPickerItem(option))
);
const customerSourceSelectedItems = computed({
  get: () => customerSourcePickerOptions.value.filter((option) => selectedCustomerSourceIdSet.value.has(option.id)),
  set: (items) => {
    form.customerSourceIds = normalizeIdList(items.map((item) => item.id));
    form.customerSourceDetails = syncSelectedDetails(form.customerSourceIds, form.customerSourceDetails);
    form.customerSourceNotInformed = false;
  }
});
const queueJumpReasonPickerOptions = computed(() =>
  (props.state.queueJumpReasonOptions || []).map((option) => mapOptionToPickerItem(option))
);
const lossReasonPickerOptions = computed(() =>
  (props.state.lossReasonOptions || []).map((option) => mapOptionToPickerItem(option))
);
const selectedQueueJumpReasonLabel = computed(
  () => (props.state.queueJumpReasonOptions || []).find((option) => option.id === form.queueJumpReasonId)?.label || ""
);
const selectedLossReasonLabels = computed(() =>
  lossReasonPickerOptions.value
    .filter((option) => selectedLossReasonIdSet.value.has(option.id))
    .map((option) => option.label)
    .filter(Boolean)
);
const selectedLossReasonLabel = computed(() => selectedLossReasonLabels.value[0] || "");
const selectedLossReasonSummary = computed(() => selectedLossReasonLabels.value.join(", "));
const modalTitle = computed(() => resolveModalText(modalConfig.value.title, "Fechar atendimento"));
const productSeenLabel = computed(() => resolveModalText(modalConfig.value.productSeenLabel, "Interesses do cliente"));
const productSeenPlaceholder = computed(() => resolveModalText(modalConfig.value.productSeenPlaceholder, "Busque e selecione interesses"));
const customerSectionLabel = computed(() => resolveModalText(modalConfig.value.customerSectionLabel, "Dados do cliente"));
const customerNameLabel = computed(() => resolveModalText(modalConfig.value.customerNameLabel, "Nome do cliente"));
const customerPhoneLabel = computed(() => resolveModalText(modalConfig.value.customerPhoneLabel, "Telefone"));
const customerEmailLabel = computed(() => resolveModalText(modalConfig.value.customerEmailLabel, "E-mail"));
const customerProfessionLabel = computed(() => resolveModalText(modalConfig.value.customerProfessionLabel, "Profissão"));
const existingCustomerLabel = computed(() => resolveModalText(modalConfig.value.existingCustomerLabel, "Já era cliente"));
const visitReasonLabel = computed(() => resolveModalText(modalConfig.value.visitReasonLabel, "Motivo da visita"));
const customerSourceLabel = computed(() => resolveModalText(modalConfig.value.customerSourceLabel, "Origem do cliente"));
const notesLabel = computed(() => resolveModalText(modalConfig.value.notesLabel, "Observações"));
const notesPlaceholder = computed(() => resolveModalText(modalConfig.value.notesPlaceholder, "Detalhes adicionais do atendimento"));
const queueJumpReasonLabel = computed(() => resolveModalText(modalConfig.value.queueJumpReasonLabel, "Motivo do atendimento fora da vez"));
const queueJumpReasonPlaceholder = computed(() => resolveModalText(modalConfig.value.queueJumpReasonPlaceholder, "Busque e selecione o motivo fora da vez"));
const lossReasonLabel = computed(() => resolveModalText(modalConfig.value.lossReasonLabel, "Motivo da perda"));
const lossReasonPlaceholder = computed(() => resolveModalText(modalConfig.value.lossReasonPlaceholder, "Busque e selecione o motivo da perda"));
const queueJumpReasonSelectedItems = computed({
  get: () => queueJumpReasonPickerOptions.value.filter((option) => option.id === form.queueJumpReasonId),
  set: (items) => {
    form.queueJumpReasonId = items[0]?.id || "";
  }
});
const lossReasonSelectedItems = computed({
  get: () => lossReasonPickerOptions.value.filter((option) => selectedLossReasonIdSet.value.has(option.id)),
  set: (items) => {
    form.lossReasonIds = normalizeIdList(items.map((item) => item.id));
    form.lossReasonDetails = syncSelectedDetails(form.lossReasonIds, form.lossReasonDetails);
  }
});

const form = reactive(createEmptyForm());
const step = ref(1);

function updateProfessionSelectedItems(items) {
  professionSelectedItems.value = items;
}

function updateVisitReasonSelectedItems(items) {
  visitReasonSelectedItems.value = items;
}

function updateCustomerSourceSelectedItems(items) {
  customerSourceSelectedItems.value = items;
}

function updateQueueJumpReasonSelectedItems(items) {
  queueJumpReasonSelectedItems.value = items;
}

function updateLossReasonSelectedItems(items) {
  lossReasonSelectedItems.value = items;
}

function buildDraftPayload() {
  return {
    outcome: form.outcome,
    isExistingCustomer: form.isExistingCustomer,
    productsSeen: normalizeProducts(form.productsSeen),
    productsClosed: normalizeProducts(form.productsClosed),
    productsSeenNone: form.productsSeenNone,
    productSeenNotes: productSeenNotesForPayload.value,
    customerName: form.customerName,
    customerPhone: form.customerPhone,
    customerEmail: form.customerEmail,
    customerProfessionId: form.customerProfessionId,
    customerProfession: selectedProfessionLabel.value,
    visitReasonIds: normalizeIdList(form.visitReasonIds),
    visitReasons: normalizeIdList(form.visitReasonIds),
    visitReasonsNotInformed: form.visitReasonNotInformed,
    visitReasonDetails: { ...form.visitReasonDetails },
    customerSourceIds: normalizeIdList(form.customerSourceIds),
    customerSources: normalizeIdList(form.customerSourceIds),
    customerSourcesNotInformed: form.customerSourceNotInformed,
    customerSourceDetails: { ...form.customerSourceDetails },
    queueJumpReasonId: form.queueJumpReasonId,
    queueJumpReason: selectedQueueJumpReasonLabel.value,
    lossReasonIds: normalizeIdList(form.lossReasonIds),
    lossReasons: normalizeIdList(form.lossReasonIds),
    lossReasonDetails: { ...form.lossReasonDetails },
    lossReasonId: normalizeIdList(form.lossReasonIds)[0] || "",
    lossReason: selectedLossReasonSummary.value,
    notes: form.notes
  };
}

function hasDraftContent(payload, products = []) {
  if (!payload || typeof payload !== "object") {
    return false;
  }

  return Boolean(
    payload.outcome ||
    payload.isExistingCustomer ||
    payload.productsSeen?.length ||
    payload.productsClosed?.length ||
    payload.productsSeenNone ||
    payload.productSeenNotes ||
    payload.customerName ||
    payload.customerPhone ||
    payload.customerEmail ||
    payload.customerProfessionId ||
    payload.visitReasonIds?.length ||
    payload.visitReasonsNotInformed ||
    Object.keys(payload.visitReasonDetails || {}).length ||
    payload.customerSourceIds?.length ||
    payload.customerSourcesNotInformed ||
    Object.keys(payload.customerSourceDetails || {}).length ||
    payload.queueJumpReasonId ||
    payload.lossReasonIds?.length ||
    Object.keys(payload.lossReasonDetails || {}).length ||
    payload.notes ||
    products.length
  );
}

function loadStoredDraft(currentService) {
  const key = serviceDraftKey.value;

  if (!key || !currentService) {
    return null;
  }

  const stored = readDraftStorage()[key];

  if (!stored || typeof stored !== "object") {
    return null;
  }

  if (stored.serviceId !== currentService.serviceId || stored.personId !== currentService.id) {
    removeStoredDraft(key);
    return null;
  }

  return stored;
}

function saveActiveDraft() {
  if (isApplyingDraft || !service.value || !serviceDraftKey.value) {
    return;
  }

  const payload = buildDraftPayload();
  const normalizedCustomProducts = normalizeProducts(customProducts.value).filter((product) => product.isCustom);

  if (!hasDraftContent(payload, normalizedCustomProducts)) {
    removeStoredDraft(serviceDraftKey.value);
    restoredDraftKey.value = "";
    return;
  }

  const drafts = readDraftStorage();
  drafts[serviceDraftKey.value] = {
    version: 1,
    storeId: String(props.state.activeStoreId || "").trim(),
    serviceId: service.value.serviceId,
    personId: service.value.id,
    updatedAt: Date.now(),
    form: payload,
    customProducts: normalizedCustomProducts
  };
  writeDraftStorage(drafts);
}

function registerCustomProducts(items = []) {
  const nextCustomProducts = normalizeProducts(items).filter((product) => product.isCustom);

  if (!nextCustomProducts.length) {
    return;
  }

  customProducts.value = mergeProductEntries(customProducts.value, nextCustomProducts);
}

function updateProductsSeen(items) {
  const nextItems = normalizeProducts(items);
  const wasNoneSelected = form.productsSeenNone;
  registerCustomProducts(nextItems);
  form.productsSeen = nextItems;

  if (nextItems.length > 0) {
    form.productsSeenNone = false;
    if (wasNoneSelected) {
      form.productSeenNotes = "";
    }
    return;
  }

  form.productSeenNotes = "";
}

function updateProductsSeenNone(nextValue) {
  const normalizedValue = Boolean(nextValue);

  if (form.productsSeenNone === normalizedValue) {
    return;
  }

  form.productsSeenNone = normalizedValue;

  if (normalizedValue) {
    form.productsSeen = [];
    form.productSeenNotes = "";
    return;
  }

  form.productSeenNotes = "";
}

function updateProductSeenDetails(details = {}) {
  const normalizedDetails = details && typeof details === "object" ? details : {};

  if (isProductSeenNoneSelected.value) {
    form.productSeenNotes = String(normalizedDetails[PRODUCT_SEEN_NONE_DETAIL_KEY] || "").trim();
    return;
  }

  const selectedProductIds = normalizeProducts(form.productsSeen)
    .map((item) => String(item.id || "").trim())
    .filter(Boolean);

  form.productSeenNotes = selectedProductIds
    .map((itemId) => String(normalizedDetails[itemId] || "").trim())
    .find(Boolean) || "";
}

function updateProductsClosed(items) {
  const nextItems = normalizeProducts(items);
  registerCustomProducts(nextItems);
  form.productsClosed = nextItems;
}

function clearCurrentDraft() {
  const key = serviceDraftKey.value;

  if (key) {
    removeStoredDraft(key);
  }

  isApplyingDraft = true;
  restoredDraftKey.value = "";
  customProducts.value = [];
  step.value = 1;
  Object.assign(form, createEmptyForm());
  normalizeFormForModalConfig();
  isApplyingDraft = false;
}

function normalizeFormForModalConfig() {
  form.visitReasonIds = normalizeIdList(form.visitReasonIds);
  form.lossReasonIds = normalizeIdList(form.lossReasonIds);
  form.customerSourceIds = normalizeIdList(form.customerSourceIds);
  form.visitReasonDetails = syncSelectedDetails(form.visitReasonIds, form.visitReasonDetails);
  form.lossReasonDetails = syncSelectedDetails(form.lossReasonIds, form.lossReasonDetails);
  form.customerSourceDetails = syncSelectedDetails(form.customerSourceIds, form.customerSourceDetails);

  if (!allowProductSeenNone.value) {
    form.productsSeenNone = false;
  }

  if (form.productsSeen.length) {
    form.productsSeenNone = false;
  }

  if (!canUseProductSeenNotes.value) {
    form.productSeenNotes = "";
  }

  if (form.visitReasonIds.length) {
    form.visitReasonNotInformed = false;
  }

  if (form.customerSourceIds.length) {
    form.customerSourceNotInformed = false;
  }
}

function resetForm() {
  const currentService = service.value;
  const storedDraft = loadStoredDraft(currentService);
  const initialDraft = storedDraft?.form || draft.value;

  isApplyingDraft = true;
  step.value = 1;
  customProducts.value = mergeProductEntries(storedDraft?.customProducts || [], initialDraft?.customProducts || []);
  restoredDraftKey.value = storedDraft ? serviceDraftKey.value : "";
  Object.assign(form, createEmptyForm(), buildInitialForm(props.state, initialDraft));
  normalizeFormForModalConfig();
  isApplyingDraft = false;
}

function goToStep1() {
  step.value = 1;
}

async function goToStep2() {
  if (!form.outcome) {
    await ui.alert("Selecione como o atendimento terminou.");
    return;
  }

  if (showProductSeenField.value && requireProductSeenField.value && form.productsSeen.length === 0 && !form.productsSeenNone) {
    await ui.alert("Selecione pelo menos um interesse do cliente ou use a opcao de nenhum.");
    return;
  }

  if (isProductSeenNotesRequired.value && !isProductSeenNotesValid.value) {
    await ui.alert(`Preencha os detalhes dos interesses com pelo menos ${productSeenNotesMinChars.value} caracteres.`);
    return;
  }

  if (isClosedOutcome.value && showProductClosedField.value && requireProductClosedField.value && form.productsClosed.length === 0) {
    await ui.alert("Selecione o item de compra ou reserva.");
    return;
  }

  step.value = 2;
}

function closeModal() {
  void operationsStore.closeFinishModal();
}

async function submitForm() {
  if (step.value !== 2) {
    await goToStep2();
    return;
  }

  if (!service.value?.id || !form.outcome) {
    await ui.alert("Selecione como o atendimento terminou.");
    return;
  }

  if (showVisitReasonField.value && requireVisitReasonField.value && form.visitReasonIds.length === 0 && !form.visitReasonNotInformed) {
    await ui.alert("Selecione um motivo da visita ou marque 'Nao informado'.");
    return;
  }

  if (showProductSeenField.value && requireProductSeenField.value && form.productsSeen.length === 0 && !form.productsSeenNone) {
    await ui.alert("Selecione pelo menos um interesse do cliente ou use a opcao de nenhum.");
    return;
  }

  if (isProductSeenNotesRequired.value && !isProductSeenNotesValid.value) {
    await ui.alert(`Preencha os detalhes dos interesses com pelo menos ${productSeenNotesMinChars.value} caracteres.`);
    return;
  }

  if (isClosedOutcome.value && showProductClosedField.value && requireProductClosedField.value && form.productsClosed.length === 0) {
    await ui.alert("Selecione o item de compra ou reserva.");
    return;
  }

  if (showCustomerNameField.value && requireCustomerNameField.value && !form.customerName.trim()) {
    await ui.alert("Nome do cliente e obrigatorio.");
    return;
  }

  if (showCustomerPhoneField.value && requireCustomerPhoneField.value && !form.customerPhone.trim()) {
    await ui.alert("Telefone do cliente e obrigatorio.");
    return;
  }

  if (showEmailField.value && requireEmailField.value && !form.customerEmail.trim()) {
    await ui.alert("E-mail do cliente é obrigatório.");
    return;
  }

  if (showProfessionField.value && requireProfessionField.value && !form.customerProfessionId) {
    await ui.alert("Selecione a profissao do cliente.");
    return;
  }

  if (showCustomerSourceField.value && requireCustomerSourceField.value && form.customerSourceIds.length === 0 && !form.customerSourceNotInformed) {
    await ui.alert("Selecione uma origem do cliente ou marque 'Nao informado'.");
    return;
  }

  if (showNotesField.value && requireNotesField.value && !form.notes.trim()) {
    await ui.alert("Observações são obrigatórias para concluir o atendimento.");
    return;
  }

  if (service.value.startMode === "queue-jump" && showQueueJumpReasonField.value && requireQueueJumpReasonField.value && !selectedQueueJumpReasonLabel.value) {
    if (!queueJumpReasonPickerOptions.value.length) {
      await ui.alert("Cadastre pelo menos um motivo de atendimento fora da vez em Configuracoes.");
      return;
    }

    await ui.alert("Selecione o motivo do atendimento fora da vez.");
    return;
  }

  if (form.outcome === "nao-compra" && showLossReasonField.value && requireLossReasonField.value && form.lossReasonIds.length === 0) {
    if (!lossReasonPickerOptions.value.length) {
      await ui.alert("Cadastre pelo menos um motivo da perda em Configuracoes.");
      return;
    }

    await ui.alert("Selecione o motivo da perda.");
    return;
  }

  const currentService = service.value;
  const productSeenSummary = [
    form.productsSeen.length ? form.productsSeen.map((item) => item.name).filter(Boolean).join(", ") : "",
    form.productsSeenNone ? "Nenhum interesse identificado" : "",
    productSeenNotesForPayload.value
  ].filter(Boolean).join(" | ");
  const result = await operationsStore.finishService(currentService.id, {
    outcome: form.outcome,
    productSeen: productSeenSummary,
    productClosed: isClosedOutcome.value ? form.productsClosed[0]?.name || "" : "",
    productsSeen: form.productsSeen,
    productsClosed: isClosedOutcome.value ? form.productsClosed : [],
    productsSeenNone: form.productsSeenNone,
    productSeenNotes: productSeenNotesForPayload.value,
    productDetails: (isClosedOutcome.value ? form.productsClosed[0]?.name : "") || productSeenSummary || "",
    customerName: form.customerName.trim(),
    customerPhone: form.customerPhone.trim(),
    customerEmail: form.customerEmail.trim(),
    customerProfession: selectedProfessionLabel.value,
    isExistingCustomer: form.isExistingCustomer,
    visitReasons: normalizeIdList(form.visitReasonIds),
    visitReasonsNotInformed: form.visitReasonNotInformed,
    visitReasonDetails: visitReasonDetailsEnabled.value
      ? Object.fromEntries(
      normalizeIdList(form.visitReasonIds)
        .map((reasonId) => [reasonId, String(form.visitReasonDetails?.[reasonId] || "").trim()])
        .filter(([, detail]) => detail)
      )
      : {},
    customerSources: normalizeIdList(form.customerSourceIds),
    customerSourcesNotInformed: form.customerSourceNotInformed,
    customerSourceDetails: customerSourceDetailsEnabled.value
      ? Object.fromEntries(
      normalizeIdList(form.customerSourceIds)
        .map((sourceId) => [sourceId, String(form.customerSourceDetails?.[sourceId] || "").trim()])
        .filter(([, detail]) => detail)
      )
      : {},
    lossReasons: form.outcome === "nao-compra" ? normalizeIdList(form.lossReasonIds) : [],
    lossReasonDetails: lossReasonDetailsEnabled.value && form.outcome === "nao-compra"
      ? Object.fromEntries(
      normalizeIdList(form.lossReasonIds)
        .map((reasonId) => [reasonId, String(form.lossReasonDetails?.[reasonId] || "").trim()])
        .filter(([, detail]) => detail)
      )
      : {},
    lossReasonId: form.outcome === "nao-compra" ? normalizeIdList(form.lossReasonIds)[0] || "" : "",
    lossReason: form.outcome === "nao-compra" ? selectedLossReasonSummary.value : "",
    saleAmount: isClosedOutcome.value ? closedTotal.value : 0,
    queueJumpReason: service.value.startMode === "queue-jump" ? selectedQueueJumpReasonLabel.value : "",
    notes: form.notes.trim()
  });

  if (result?.ok === false) {
    ui.error(result.message || "Nao foi possivel encerrar o atendimento.");
    return;
  }

  removeStoredDraft(`${String(props.state.activeStoreId || "").trim()}:${currentService.serviceId}`);
  restoredDraftKey.value = "";
  customProducts.value = [];
  ui.success("Atendimento encerrado.");
}

watch(serviceDraftKey, () => {
  resetForm();
}, { immediate: true });

watch(draft, () => {
  if (!hasRestoredDraft.value) {
    resetForm();
  }
});

watch(() => [...form.visitReasonIds], (nextValue) => {
  if (nextValue.length) {
    form.visitReasonNotInformed = false;
  }

  form.visitReasonDetails = syncSelectedDetails(nextValue, form.visitReasonDetails);
}, { deep: true });

watch(() => [...form.customerSourceIds], (nextValue) => {
  if (nextValue.length) {
    form.customerSourceNotInformed = false;
  }

  form.customerSourceDetails = syncSelectedDetails(nextValue, form.customerSourceDetails);
}, { deep: true });

watch(() => [...form.lossReasonIds], (nextValue) => {
  form.lossReasonDetails = syncSelectedDetails(nextValue, form.lossReasonDetails);
}, { deep: true });

watch(() => form.visitReasonNotInformed, (nextValue) => {
  if (!nextValue) {
    return;
  }

  form.visitReasonIds = [];
  form.visitReasonDetails = {};
});

watch(() => form.customerSourceNotInformed, (nextValue) => {
  if (!nextValue) {
    return;
  }

  form.customerSourceIds = [];
  form.customerSourceDetails = {};
});

watch([isVisitReasonMultiple, visitReasonDetailsEnabled], () => {
  normalizeFormForModalConfig();
});

watch([isLossReasonMultiple, lossReasonDetailsEnabled], () => {
  normalizeFormForModalConfig();
});

watch([isCustomerSourceMultiple, customerSourceDetailsEnabled], () => {
  normalizeFormForModalConfig();
});

watch([allowProductSeenNone, showProductSeenNotesField], () => {
  normalizeFormForModalConfig();
});

watch(() => form.outcome, (nextValue) => {
  if (nextValue !== "nao-compra") {
    form.lossReasonIds = [];
    form.lossReasonDetails = {};
  }
});

watch(form, () => {
  saveActiveDraft();
}, { deep: true });

watch(customProducts, () => {
  saveActiveDraft();
}, { deep: true });

function handleEscape(event) {
  if (event.key !== "Escape") return;
  if (!service.value) return;
  if (document.querySelector(".product-pick__dropdown.is-open")) return;
  if (document.querySelector(".product-pick__detail-popover")) return;
  closeModal();
}

onMounted(() => {
  document.addEventListener("keydown", handleEscape);
});

onBeforeUnmount(() => {
  document.removeEventListener("keydown", handleEscape);
});
</script>

<template>
  <Teleport to="body">
    <div
      v-if="service"
      class="modal-backdrop"
      data-testid="operation-finish-modal-backdrop"
      @click.self.prevent
    >
      <div
        class="finish-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="finish-modal-title"
        data-testid="operation-finish-modal"
      >
        <div class="finish-modal__header">
          <div>
            <h2 id="finish-modal-title" class="finish-modal__title">{{ modalTitle }}</h2>
            <p class="finish-modal__subtitle">{{ service.name }} | ID {{ service.serviceId }}</p>
          </div>
          <div class="finish-modal__header-actions">
            <button
              v-if="hasRestoredDraft"
              class="finish-modal__draft-clear"
              type="button"
              data-testid="operation-finish-clear-draft"
              @click="clearCurrentDraft"
            >
              Limpar modal
            </button>
            <button
              class="finish-modal__close"
              type="button"
              aria-label="Fechar"
              data-testid="operation-finish-close"
              @click="closeModal"
            >
              X
            </button>
          </div>
        </div>

        <div class="finish-modal__steps">
          <div class="finish-modal__step">
            <span
              class="finish-modal__step-dot"
              :class="{ 'is-active': step === 1, 'is-done': step > 1 }"
            >1</span>
            <span class="finish-modal__step-label" :class="{ 'is-active': step === 1 }">Atendimento</span>
          </div>
          <div class="finish-modal__step-line" :class="{ 'is-done': step > 1 }" />
          <div class="finish-modal__step">
            <span
              class="finish-modal__step-dot"
              :class="{ 'is-active': step === 2 }"
            >2</span>
            <span class="finish-modal__step-label" :class="{ 'is-active': step === 2 }">Cliente</span>
          </div>
        </div>

        <form class="finish-form" @submit.prevent="submitForm">
          <template v-if="step === 1">
            <section class="finish-form__section">
              <strong class="finish-form__label">Como terminou</strong>
              <div class="finish-form__options">
                <label class="modal-radio">
                  <input
                    v-model="form.outcome"
                    type="radio"
                    name="finish-outcome"
                    value="reserva"
                    data-testid="operation-outcome-reserva"
                  >
                  <span>Reserva</span>
                </label>
                <label class="modal-radio">
                  <input
                    v-model="form.outcome"
                    type="radio"
                    name="finish-outcome"
                    value="compra"
                    data-testid="operation-outcome-compra"
                  >
                  <span>Compra</span>
                </label>
                <label class="modal-radio">
                  <input
                    v-model="form.outcome"
                    type="radio"
                    name="finish-outcome"
                    value="nao-compra"
                    data-testid="operation-outcome-nao-compra"
                  >
                  <span>Nao compra</span>
                </label>
              </div>
            </section>

            <OperationProductPicker
              v-if="isClosedOutcome && showProductClosedField"
              key="products-closed-picker"
              :label="closedProductLabel"
              :helper-text="closedProductHelperText"
              :options="productPickerOptions"
              :selected-items="form.productsClosed"
              :search-placeholder="modalConfig.productClosedPlaceholder || 'Busque e selecione o produto fechado'"
              trigger-label="Selecionar item"
              empty-selected-label="Nenhum item selecionado"
              allow-custom
              mode="closed"
              testid-prefix="operation-products-closed"
              @update:selected-items="updateProductsClosed"
            />

            <OperationProductPicker
              v-if="showProductSeenField"
              key="products-seen-picker"
              :label="productSeenLabel"
              helper-text=""
              :options="productPickerOptions"
              :selected-items="form.productsSeen"
              :none-selected="form.productsSeenNone"
              :search-placeholder="productSeenPlaceholder"
              trigger-label="Selecionar interesse"
              empty-selected-label="Nenhum interesse selecionado"
              :allow-none="allowProductSeenNone"
              none-placement="dropdown"
              none-label="Nenhum interesse identificado"
              none-state-label="Nenhum interesse identificado"
              :enable-item-details="showProductSeenNotesField"
              item-detail-mode="shared"
              :item-details="productSeenDetailMap"
              :item-detail-label="productSeenNotesLabel"
              :item-detail-placeholder="productSeenNotesPlaceholder"
              item-detail-testid="operation-product-seen-notes"
              testid-prefix="operation-products-seen"
              @update:selected-items="updateProductsSeen"
              @update:item-details="updateProductSeenDetails"
              @update:none-selected="updateProductsSeenNone"
            />

            <section v-if="isProductSeenNoneSelected" class="finish-form__section">
              <label class="finish-form__label" for="finish-product-seen-notes">{{ productSeenNotesLabel }}</label>
              <textarea
                id="finish-product-seen-notes"
                v-model="form.productSeenNotes"
                class="finish-form__textarea"
                rows="3"
                :placeholder="productSeenNotesPlaceholder"
                data-testid="operation-product-seen-notes"
              />
              <div class="finish-form__field-note" :class="{ 'finish-form__field-note--error': isProductSeenNotesRequired && !isProductSeenNotesValid }">
                <span>{{ productSeenNotesHelperText }}</span>
                <strong>{{ trimmedProductSeenNotes.length }}/{{ productSeenNotesMinChars }} caracteres</strong>
              </div>
            </section>

            <div class="finish-form__quality" :class="formStep1Quality.isComplete ? 'finish-form__quality--complete' : 'finish-form__quality--incomplete'">
              <div class="finish-form__quality-dots">
                <span class="finish-form__quality-dot" :class="{ 'is-filled': formStep1Quality.checks.outcome }" title="Como terminou"></span>
                <span v-if="isClosedOutcome && showProductClosedField && requireProductClosedField" class="finish-form__quality-dot" :class="{ 'is-filled': formStep1Quality.checks.productClosed }" title="Compra / reserva"></span>
                <span v-if="showProductSeenField && requireProductSeenField" class="finish-form__quality-dot" :class="{ 'is-filled': formStep1Quality.checks.productSeen }" title="Interesses do cliente"></span>
                <span v-if="isProductSeenNotesRequired" class="finish-form__quality-dot finish-form__quality-dot--notes" :class="{ 'is-filled': formStep1Quality.checks.productSeenNotes }" title="Detalhes dos interesses"></span>
              </div>
              <span class="finish-form__quality-text">
                {{ formStep1Quality.filled }}/{{ formStep1Quality.total }} obrigatorios
                · {{ formStep1Quality.isComplete ? 'Pronto para avançar' : 'Preencha antes de continuar' }}
              </span>
            </div>

            <div class="finish-form__actions">
              <button
                class="column-action column-action--secondary"
                type="button"
                data-testid="operation-finish-cancel"
                @click="closeModal"
              >
                Cancelar
              </button>
              <button
                class="column-action column-action--primary"
                type="button"
                data-testid="operation-step-next"
                @click="goToStep2"
              >
                Próximo
              </button>
            </div>
          </template>

          <template v-if="step === 2">
            <section v-if="showCustomerSection" class="finish-form__section">
              <strong class="finish-form__label">{{ customerSectionLabel }}</strong>
            </section>

            <section v-if="showExistingCustomerField" class="finish-form__section finish-form__grid">
              <label class="modal-checkbox">
                <input v-model="form.isExistingCustomer" type="checkbox">
                <span>{{ existingCustomerLabel }}</span>
              </label>
            </section>

            <section class="finish-form__section finish-form__grid finish-form__grid--customer">
              <label v-if="showCustomerNameField" class="finish-form__field">
                <span class="finish-form__label">{{ customerNameLabel }}</span>
                <input
                  v-model="form.customerName"
                  class="finish-form__input"
                  type="text"
                  placeholder="Nome Completo"
                  data-testid="operation-customer-name"
                >
              </label>
              <label v-if="showCustomerPhoneField" class="finish-form__field">
                <span class="finish-form__label">{{ customerPhoneLabel }}</span>
                <input
                  v-model="form.customerPhone"
                  class="finish-form__input"
                  type="tel"
                  placeholder="(11) 99999-9999"
                  data-testid="operation-customer-phone"
                  @input="handleCustomerPhoneInput"
                >
              </label>
              <label v-if="showEmailField" class="finish-form__field">
                <span class="finish-form__label">{{ customerEmailLabel }}</span>
                <input
                  v-model="form.customerEmail"
                  class="finish-form__input"
                  type="email"
                  placeholder="E-mail"
                  data-testid="operation-customer-email"
                >
              </label>
            </section>

            <div class="operation-modal__select-grid">
              <section v-if="showProfessionField" class="finish-form__section operation-modal__picker-cell">
                <OperationProductPicker
                  :label="customerProfessionLabel"
                  :options="professionPickerOptions"
                  :selected-items="professionSelectedItems"
                  :multiple="false"
                  trigger-label="Selecionar profissão"
                  search-placeholder="Busque e selecione a profissão"
                  empty-selected-label="Nenhuma profissão selecionada"
                  testid-prefix="operation-customer-profession"
                  @update:selected-items="updateProfessionSelectedItems"
                />
              </section>

              <section v-if="showVisitReasonField" class="finish-form__section operation-modal__picker-cell">
                <OperationProductPicker
                  :label="visitReasonLabel"
                  :options="visitReasonPickerOptions"
                  :selected-items="visitReasonSelectedItems"
                  :multiple="isVisitReasonMultiple"
                  :enable-item-details="visitReasonDetailsEnabled"
                  :item-detail-mode="visitReasonPickerDetailMode"
                  :item-details="form.visitReasonDetails"
                  item-detail-label="Descricao"
                  item-detail-placeholder="Digite a descricao que deseja salvar"
                  item-detail-testid="operation-visit-reason-detail"
                  :none-selected="form.visitReasonNotInformed"
                  allow-none
                  none-label="Nao informado"
                  none-state-label="Nao informado"
                  trigger-label="Selecionar motivo"
                  search-placeholder="Busque e selecione o motivo"
                  empty-selected-label="Nenhum motivo selecionado"
                  testid-prefix="operation-visit-reason"
                  @update:selected-items="updateVisitReasonSelectedItems"
                  @update:item-details="form.visitReasonDetails = syncSelectedDetails(form.visitReasonIds, $event)"
                  @update:none-selected="form.visitReasonNotInformed = $event"
                />
              </section>

              <section v-if="showCustomerSourceField" class="finish-form__section operation-modal__picker-cell">
                <OperationProductPicker
                  :label="customerSourceLabel"
                  :options="customerSourcePickerOptions"
                  :selected-items="customerSourceSelectedItems"
                  :multiple="isCustomerSourceMultiple"
                  :enable-item-details="customerSourceDetailsEnabled"
                  :item-detail-mode="customerSourcePickerDetailMode"
                  :item-details="form.customerSourceDetails"
                  item-detail-label="Descricao"
                  item-detail-placeholder="Digite a descricao da origem"
                  item-detail-testid="operation-customer-source-detail"
                  :none-selected="form.customerSourceNotInformed"
                  allow-none
                  none-label="Nao informado"
                  none-state-label="Nao informado"
                  trigger-label="Selecionar origem"
                  search-placeholder="Busque e selecione a origem"
                  empty-selected-label="Nenhuma origem selecionada"
                  testid-prefix="operation-customer-source"
                  @update:selected-items="updateCustomerSourceSelectedItems"
                  @update:item-details="form.customerSourceDetails = syncSelectedDetails(form.customerSourceIds, $event)"
                  @update:none-selected="form.customerSourceNotInformed = $event"
                />
              </section>
            </div>

            <section v-if="service.startMode === 'queue-jump' && showQueueJumpReasonField" class="finish-form__section operation-modal__picker-cell">
              <OperationProductPicker
                :label="queueJumpReasonLabel"
                :options="queueJumpReasonPickerOptions"
                :selected-items="queueJumpReasonSelectedItems"
                :multiple="false"
                trigger-label="Selecionar motivo"
                :search-placeholder="queueJumpReasonPlaceholder"
                empty-selected-label="Nenhum motivo selecionado"
                testid-prefix="operation-queue-jump-reason"
                @update:selected-items="updateQueueJumpReasonSelectedItems"
              />
            </section>

            <section v-if="form.outcome === 'nao-compra' && showLossReasonField" class="finish-form__section operation-modal__picker-cell">
              <OperationProductPicker
                :label="lossReasonLabel"
                :options="lossReasonPickerOptions"
                :selected-items="lossReasonSelectedItems"
                :multiple="isLossReasonMultiple"
                :enable-item-details="lossReasonDetailsEnabled"
                :item-detail-mode="lossReasonPickerDetailMode"
                :item-details="form.lossReasonDetails"
                item-detail-label="Descricao"
                item-detail-placeholder="Digite a descricao do motivo da perda"
                item-detail-testid="operation-loss-reason-detail"
                trigger-label="Selecionar motivo"
                :search-placeholder="lossReasonPlaceholder"
                empty-selected-label="Nenhum motivo selecionado"
                testid-prefix="operation-loss-reason"
                @update:selected-items="updateLossReasonSelectedItems"
                @update:item-details="form.lossReasonDetails = syncSelectedDetails(form.lossReasonIds, $event)"
              />
            </section>

            <section v-if="showNotesField" class="finish-form__section">
              <label class="finish-form__label" for="finish-notes">{{ notesLabel }}</label>
              <textarea
                id="finish-notes"
                v-model="form.notes"
                class="finish-form__textarea"
                rows="3"
                :placeholder="notesPlaceholder"
                data-testid="operation-notes"
              />
            </section>

            <section v-if="isClosedOutcome && showProductClosedField" class="finish-form__section operation-modal__summary">
              <span class="finish-form__label">Valor da venda derivado dos produtos fechados</span>
              <strong>{{ formatCurrency(closedTotal) }}</strong>
            </section>

            <div class="finish-form__quality" :class="`finish-form__quality--${formQuality.level}`">
              <div class="finish-form__quality-dots">
                <span v-if="showCustomerNameField && requireCustomerNameField" class="finish-form__quality-dot" :class="{ 'is-filled': formQuality.checks.customerName }" title="Nome"></span>
                <span v-if="showCustomerPhoneField && requireCustomerPhoneField" class="finish-form__quality-dot" :class="{ 'is-filled': formQuality.checks.customerPhone }" title="Telefone"></span>
                <span v-if="isClosedOutcome && showProductClosedField && requireProductClosedField" class="finish-form__quality-dot" :class="{ 'is-filled': formQuality.checks.productClosed }" title="Compra / reserva"></span>
                <span v-if="showProductSeenField && requireProductSeenField" class="finish-form__quality-dot" :class="{ 'is-filled': formQuality.checks.productSeen }" title="Interesses do cliente"></span>
                <span v-if="isProductSeenNotesRequired" class="finish-form__quality-dot finish-form__quality-dot--notes" :class="{ 'is-filled': formQuality.checks.productSeenNotes }" title="Detalhes dos interesses"></span>
                <span v-if="showVisitReasonField && requireVisitReasonField" class="finish-form__quality-dot" :class="{ 'is-filled': formQuality.checks.visitReasons }" title="Motivo da visita"></span>
                <span v-if="showCustomerSourceField && requireCustomerSourceField" class="finish-form__quality-dot" :class="{ 'is-filled': formQuality.checks.customerSources }" title="Origem do cliente"></span>
                <span v-if="form.outcome === 'nao-compra' && showLossReasonField && requireLossReasonField" class="finish-form__quality-dot" :class="{ 'is-filled': formQuality.checks.lossReason }" title="Motivo da perda"></span>
                <span v-if="service.startMode === 'queue-jump' && showQueueJumpReasonField && requireQueueJumpReasonField" class="finish-form__quality-dot" :class="{ 'is-filled': formQuality.checks.queueJumpReason }" title="Motivo fora da vez"></span>
                <span v-if="showEmailField && requireEmailField" class="finish-form__quality-dot" :class="{ 'is-filled': formQuality.checks.customerEmail }" title="E-mail"></span>
                <span v-if="showProfessionField && requireProfessionField" class="finish-form__quality-dot" :class="{ 'is-filled': formQuality.checks.customerProfession }" title="Profissão"></span>
                <span v-if="showNotesField && requireNotesField" class="finish-form__quality-dot finish-form__quality-dot--notes" :class="{ 'is-filled': formQuality.checks.notes }" title="Observações"></span>
              </div>
              <span class="finish-form__quality-text">
                {{ formQuality.coreFilledCount }}/{{ formQuality.coreTotal }} obrigatorios · {{ formQuality.levelLabel }}
              </span>
            </div>

            <div class="finish-form__actions">
              <button
                class="column-action column-action--secondary"
                type="button"
                data-testid="operation-step-back"
                @click="goToStep1"
              >
                ← Voltar
              </button>
              <button
                class="column-action column-action--primary"
                type="submit"
                data-testid="operation-finish-submit"
              >
                Salvar e encerrar
              </button>
            </div>
          </template>
        </form>
      </div>
    </div>
  </Teleport>
</template>
