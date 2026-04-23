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

function createEmptyForm() {
  return {
    outcome: "",
    isWindowService: false,
    isGift: false,
    isExistingCustomer: false,
    productsSeen: [],
    productsClosed: [],
    productsSeenNone: false,
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

function buildInitialForm(state, draft) {
  const currentDraft = draft || {};
  const selectedVisitReasonIds = normalizeIdList(currentDraft.visitReasons);
  const selectedSourceIds = normalizeIdList(
    Array.isArray(currentDraft.customerSources)
      ? currentDraft.customerSources
      : currentDraft.customerSource
        ? [currentDraft.customerSource]
        : []
  );
  const selectedProfession = findOptionByLabel(state.professionOptions, currentDraft.customerProfession);
  const selectedQueueJumpReason =
    (state.queueJumpReasonOptions || []).find((option) => option.id === String(currentDraft.queueJumpReasonId || "")) ||
    findOptionByLabel(state.queueJumpReasonOptions, currentDraft.queueJumpReason);
  const selectedLossReasonIds = normalizeIdList(
    Array.isArray(currentDraft.lossReasons)
      ? currentDraft.lossReasons
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
    isWindowService: Boolean(currentDraft.isWindowService),
    isGift: Boolean(currentDraft.isGift),
    isExistingCustomer: Boolean(currentDraft.isExistingCustomer),
    productsSeen: normalizeProducts(currentDraft.productsSeen),
    productsClosed: normalizeProducts(currentDraft.productsClosed),
    productsSeenNone: Boolean(currentDraft.productsSeenNone),
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

const modalConfig = computed(() => props.state.modalConfig || {});
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
const isClosedOutcome = computed(() => form.outcome === "compra" || form.outcome === "reserva");
const closedProductLabel = computed(() => {
  if (form.outcome === "compra") {
    return "Produto comprado";
  }

  if (form.outcome === "reserva") {
    return "Produto reservado";
  }

  return "Produto comprado/reservado";
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
    outcome: !!form.outcome,
    productSeen: form.productsSeen.length > 0 || form.productsSeenNone
  };

  if (isClosedOutcome.value) {
    checks.productClosed = form.productsClosed.length > 0;
  }

  const total = Object.keys(checks).length;
  const filled = Object.values(checks).filter(Boolean).length;
  const isComplete = filled === total;

  return { checks, filled, total, isComplete };
});

const formQuality = computed(() => {
  const hasText = (v) => String(v || "").trim().length > 0;

  const checks = {
    customerName: hasText(form.customerName),
    customerPhone: hasText(form.customerPhone),
    product: form.productsSeen.length > 0 || form.productsClosed.length > 0 || form.productsSeenNone,
    visitReasons: form.visitReasonIds.length > 0 || form.visitReasonNotInformed,
    customerSources: form.customerSourceIds.length > 0 || form.customerSourceNotInformed
  };

  if (service.value?.startMode === "queue-jump") {
    checks.queueJumpReason = Boolean(selectedQueueJumpReasonLabel.value);
  }

  if (form.outcome === "nao-compra") {
    checks.lossReason = form.lossReasonIds.length > 0;
  }

  if (modalConfig.value.showEmailField) {
    checks.customerEmail = hasText(form.customerEmail);
  }

  if (modalConfig.value.showProfessionField) {
    checks.customerProfession = !!form.customerProfessionId;
  }

  const coreTotal = Object.keys(checks).length;
  const coreFilledCount = Object.values(checks).filter(Boolean).length;
  const hasNotes = hasText(form.notes) && Boolean(modalConfig.value.showNotesField);
  const isCoreComplete = coreFilledCount === coreTotal;
  const level = isCoreComplete ? (hasNotes ? "excellent" : "complete") : "incomplete";
  const levelLabels = { excellent: "Excelente", complete: "Completo", incomplete: "Incompleto" };

  return { checks, coreFilledCount, coreTotal, hasNotes, isCoreComplete, level, levelLabel: levelLabels[level] };
});
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

function normalizeFormForModalConfig() {
  if (!isVisitReasonMultiple.value && form.visitReasonIds.length > 1) {
    form.visitReasonIds = form.visitReasonIds.slice(0, 1);
  }

  if (!isLossReasonMultiple.value && form.lossReasonIds.length > 1) {
    form.lossReasonIds = form.lossReasonIds.slice(0, 1);
  }

  if (!isCustomerSourceMultiple.value && form.customerSourceIds.length > 1) {
    form.customerSourceIds = form.customerSourceIds.slice(0, 1);
  }

  form.visitReasonIds = normalizeIdList(form.visitReasonIds);
  form.lossReasonIds = normalizeIdList(form.lossReasonIds);
  form.customerSourceIds = normalizeIdList(form.customerSourceIds);
  form.visitReasonDetails = visitReasonDetailsEnabled.value
    ? syncSelectedDetails(form.visitReasonIds, form.visitReasonDetails)
    : {};
  form.lossReasonDetails = lossReasonDetailsEnabled.value
    ? syncSelectedDetails(form.lossReasonIds, form.lossReasonDetails)
    : {};
  form.customerSourceDetails = customerSourceDetailsEnabled.value
    ? syncSelectedDetails(form.customerSourceIds, form.customerSourceDetails)
    : {};

  if (form.visitReasonIds.length) {
    form.visitReasonNotInformed = false;
  }

  if (form.customerSourceIds.length) {
    form.customerSourceNotInformed = false;
  }
}

function resetForm() {
  step.value = 1;
  Object.assign(form, createEmptyForm(), buildInitialForm(props.state, draft.value));
  normalizeFormForModalConfig();
}

function goToStep1() {
  step.value = 1;
}

async function goToStep2() {
  if (!form.outcome) {
    await ui.alert("Selecione como o atendimento terminou.");
    return;
  }

  if (modalConfig.value.requireProduct && form.productsSeen.length === 0 && !form.productsSeenNone) {
    await ui.alert("Selecione pelo menos um produto visto ou marque 'Nenhum'.");
    return;
  }

  if (isClosedOutcome.value && modalConfig.value.requireProduct && form.productsClosed.length === 0) {
    await ui.alert("Selecione o produto comprado/reservado.");
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

  if (modalConfig.value.requireVisitReason && form.visitReasonIds.length === 0 && !form.visitReasonNotInformed) {
    await ui.alert("Selecione um motivo da visita ou marque 'Nao informado'.");
    return;
  }

  if (modalConfig.value.requireProduct && form.productsSeen.length === 0 && !form.productsSeenNone) {
    await ui.alert("Selecione pelo menos um produto visto ou marque 'Nenhum'.");
    return;
  }

  if (isClosedOutcome.value && modalConfig.value.requireProduct && form.productsClosed.length === 0) {
    await ui.alert("Selecione o produto comprado/reservado.");
    return;
  }

  if (modalConfig.value.requireCustomerNamePhone && (!form.customerName.trim() || !form.customerPhone.trim())) {
    await ui.alert("Nome e telefone do cliente sao obrigatorios.");
    return;
  }

  if (modalConfig.value.requireCustomerSource && form.customerSourceIds.length === 0 && !form.customerSourceNotInformed) {
    await ui.alert("Selecione uma origem do cliente ou marque 'Nao informado'.");
    return;
  }

  if (service.value.startMode === "queue-jump" && !selectedQueueJumpReasonLabel.value) {
    if (!queueJumpReasonPickerOptions.value.length) {
      await ui.alert("Cadastre pelo menos um motivo de atendimento fora da vez em Configuracoes.");
      return;
    }

    await ui.alert("Selecione o motivo do atendimento fora da vez.");
    return;
  }

  if (form.outcome === "nao-compra" && form.lossReasonIds.length === 0) {
    if (!lossReasonPickerOptions.value.length) {
      await ui.alert("Cadastre pelo menos um motivo da perda em Configuracoes.");
      return;
    }

    await ui.alert("Selecione o motivo da perda.");
    return;
  }

  await operationsStore.finishService(service.value.id, {
    outcome: form.outcome,
    isWindowService: form.isWindowService,
    isGift: isClosedOutcome.value ? form.isGift : false,
    productSeen: form.productsSeen[0]?.name || "",
    productClosed: isClosedOutcome.value ? form.productsClosed[0]?.name || "" : "",
    productsSeen: form.productsSeen,
    productsClosed: isClosedOutcome.value ? form.productsClosed : [],
    productsSeenNone: form.productsSeenNone,
    productDetails: (isClosedOutcome.value ? form.productsClosed[0]?.name : "") || form.productsSeen[0]?.name || "",
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
  ui.success("Atendimento encerrado.");
}

watch(service, () => {
  resetForm();
}, { immediate: true });

watch(draft, () => {
  resetForm();
});

watch(() => [...form.visitReasonIds], (nextValue) => {
  if (nextValue.length) {
    form.visitReasonNotInformed = false;
  }

  form.visitReasonDetails = visitReasonDetailsEnabled.value
    ? syncSelectedDetails(nextValue, form.visitReasonDetails)
    : {};
}, { deep: true });

watch(() => [...form.customerSourceIds], (nextValue) => {
  if (nextValue.length) {
    form.customerSourceNotInformed = false;
  }

  form.customerSourceDetails = customerSourceDetailsEnabled.value
    ? syncSelectedDetails(nextValue, form.customerSourceDetails)
    : {};
}, { deep: true });

watch(() => [...form.lossReasonIds], (nextValue) => {
  form.lossReasonDetails = lossReasonDetailsEnabled.value
    ? syncSelectedDetails(nextValue, form.lossReasonDetails)
    : {};
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

watch(() => form.outcome, (nextValue) => {
  if (nextValue !== "nao-compra") {
    form.lossReasonIds = [];
    form.lossReasonDetails = {};
  }

  if (nextValue === "compra" || nextValue === "reserva") {
    return;
  }

  form.isGift = false;
});

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
            <h2 id="finish-modal-title" class="finish-modal__title">{{ modalConfig.title }}</h2>
            <p class="finish-modal__subtitle">{{ service.name }} | ID {{ service.serviceId }}</p>
          </div>
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

            <section class="finish-form__section finish-form__grid">
              <label class="modal-checkbox">
                <input v-model="form.isWindowService" type="checkbox">
                <span>Atendimento de vitrine</span>
              </label>
              <label v-if="isClosedOutcome" class="modal-checkbox">
                <input v-model="form.isGift" type="checkbox">
                <span>Foi para presente</span>
              </label>
              <label class="modal-checkbox">
                <input v-model="form.isExistingCustomer" type="checkbox">
                <span>Ja era cliente</span>
              </label>
            </section>

            <OperationProductPicker
              :label="modalConfig.productSeenLabel || 'Produto visto pelo cliente'"
              :options="productCatalogItems"
              :selected-items="form.productsSeen"
              :none-selected="form.productsSeenNone"
              :search-placeholder="modalConfig.productSeenPlaceholder || 'Busque e selecione um produto'"
              trigger-label="Selecionar produto"
              empty-selected-label="Nenhum produto selecionado"
              allow-none
              allow-custom
              testid-prefix="operation-products-seen"
              @update:selected-items="form.productsSeen = $event"
              @update:none-selected="form.productsSeenNone = $event"
            />

            <OperationProductPicker
              v-if="isClosedOutcome"
              :label="closedProductLabel"
              :options="productCatalogItems"
              :selected-items="form.productsClosed"
              :search-placeholder="modalConfig.productClosedPlaceholder || 'Busque e selecione o produto fechado'"
              trigger-label="Selecionar produto"
              empty-selected-label="Nenhum produto selecionado"
              allow-custom
              mode="closed"
              testid-prefix="operation-products-closed"
              @update:selected-items="form.productsClosed = $event"
            />

            <div class="finish-form__quality" :class="formStep1Quality.isComplete ? 'finish-form__quality--complete' : 'finish-form__quality--incomplete'">
              <div class="finish-form__quality-dots">
                <span class="finish-form__quality-dot" :class="{ 'is-filled': formStep1Quality.checks.outcome }" title="Como terminou"></span>
                <span class="finish-form__quality-dot" :class="{ 'is-filled': formStep1Quality.checks.productSeen }" title="Produto visto"></span>
                <span v-if="isClosedOutcome" class="finish-form__quality-dot" :class="{ 'is-filled': formStep1Quality.checks.productClosed }" title="Produto fechado"></span>
              </div>
              <span class="finish-form__quality-text">
                {{ formStep1Quality.filled }}/{{ formStep1Quality.total }} obrigatórios
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
            <section class="finish-form__section">
              <strong class="finish-form__label">{{ modalConfig.customerSectionLabel }}</strong>
            </section>

            <section class="finish-form__section finish-form__grid finish-form__grid--customer">
              <label class="finish-form__field">
                <span class="finish-form__label">Nome do cliente</span>
                <input
                  v-model="form.customerName"
                  class="finish-form__input"
                  type="text"
                  placeholder="Nome"
                  data-testid="operation-customer-name"
                >
              </label>
              <label class="finish-form__field">
                <span class="finish-form__label">Telefone</span>
                <input
                  v-model="form.customerPhone"
                  class="finish-form__input"
                  type="tel"
                  placeholder="Telefone"
                  data-testid="operation-customer-phone"
                >
              </label>
              <label v-if="modalConfig.showEmailField" class="finish-form__field">
                <span class="finish-form__label">Email</span>
                <input
                  v-model="form.customerEmail"
                  class="finish-form__input"
                  type="email"
                  placeholder="Email opcional"
                  data-testid="operation-customer-email"
                >
              </label>
            </section>

            <div class="operation-modal__select-grid">
              <section v-if="modalConfig.showProfessionField" class="finish-form__section operation-modal__picker-cell">
                <OperationProductPicker
                  label="Profissao"
                  :options="professionPickerOptions"
                  :selected-items="professionSelectedItems"
                  :multiple="false"
                  trigger-label="Selecionar profissao"
                  search-placeholder="Busque e selecione a profissao"
                  empty-selected-label="Nenhuma profissao selecionada"
                  testid-prefix="operation-customer-profession"
                  @update:selected-items="updateProfessionSelectedItems"
                />
              </section>

              <section class="finish-form__section operation-modal__picker-cell">
                <OperationProductPicker
                  label="Motivo da visita"
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

              <section class="finish-form__section operation-modal__picker-cell">
                <OperationProductPicker
                  label="De onde o cliente veio"
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

            <section v-if="service.startMode === 'queue-jump'" class="finish-form__section operation-modal__picker-cell">
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

            <section v-if="form.outcome === 'nao-compra'" class="finish-form__section operation-modal__picker-cell">
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

            <section v-if="modalConfig.showNotesField" class="finish-form__section">
              <label class="finish-form__label" for="finish-notes">{{ modalConfig.notesLabel }}</label>
              <textarea
                id="finish-notes"
                v-model="form.notes"
                class="finish-form__textarea"
                rows="3"
                :placeholder="modalConfig.notesPlaceholder"
                data-testid="operation-notes"
              />
            </section>

            <section v-if="isClosedOutcome" class="finish-form__section operation-modal__summary">
              <span class="finish-form__label">Valor da venda derivado dos produtos fechados</span>
              <strong>{{ formatCurrency(closedTotal) }}</strong>
            </section>

            <div class="finish-form__quality" :class="`finish-form__quality--${formQuality.level}`">
              <div class="finish-form__quality-dots">
                <span class="finish-form__quality-dot" :class="{ 'is-filled': formQuality.checks.customerName }" title="Nome"></span>
                <span class="finish-form__quality-dot" :class="{ 'is-filled': formQuality.checks.customerPhone }" title="Telefone"></span>
                <span class="finish-form__quality-dot" :class="{ 'is-filled': formQuality.checks.product }" title="Produto visto"></span>
                <span class="finish-form__quality-dot" :class="{ 'is-filled': formQuality.checks.visitReasons }" title="Motivo da visita"></span>
                <span class="finish-form__quality-dot" :class="{ 'is-filled': formQuality.checks.customerSources }" title="Origem do cliente"></span>
                <span v-if="form.outcome === 'nao-compra'" class="finish-form__quality-dot" :class="{ 'is-filled': formQuality.checks.lossReason }" title="Motivo da perda"></span>
                <span v-if="service.startMode === 'queue-jump'" class="finish-form__quality-dot" :class="{ 'is-filled': formQuality.checks.queueJumpReason }" title="Motivo fora da vez"></span>
                <span v-if="modalConfig.showEmailField" class="finish-form__quality-dot" :class="{ 'is-filled': formQuality.checks.customerEmail }" title="Email"></span>
                <span v-if="modalConfig.showProfessionField" class="finish-form__quality-dot" :class="{ 'is-filled': formQuality.checks.customerProfession }" title="Profissao"></span>
                <span v-if="modalConfig.showNotesField" class="finish-form__quality-dot finish-form__quality-dot--notes" :class="{ 'is-filled': formQuality.hasNotes }" title="Observacoes"></span>
              </div>
              <span class="finish-form__quality-text">
                {{ formQuality.coreFilledCount }}/{{ formQuality.coreTotal }} campos · {{ formQuality.levelLabel }}
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
