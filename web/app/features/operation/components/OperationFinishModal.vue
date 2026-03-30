<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import OperationProductPicker from "~/features/operation/components/OperationProductPicker.vue";
import { useDashboardStore } from "~/stores/dashboard";
import { useUiStore } from "~/stores/ui";

const props = defineProps({
  state: {
    type: Object,
    required: true
  }
});

const dashboard = useDashboardStore();
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
    visitReasonId: "",
    visitReasonNotInformed: false,
    visitReasonDetail: "",
    customerSourceId: "",
    customerSourceNotInformed: false,
    customerSourceDetail: "",
    queueJumpReason: "",
    notes: ""
  };
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
  const selectedVisitReasonId = Array.isArray(currentDraft.visitReasons) ? currentDraft.visitReasons[0] || "" : "";
  const selectedSourceId = Array.isArray(currentDraft.customerSources) ? currentDraft.customerSources[0] || "" : "";
  const selectedProfession = findOptionByLabel(state.professionOptions, currentDraft.customerProfession);

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
    visitReasonId: selectedVisitReasonId,
    visitReasonNotInformed: Boolean(currentDraft.visitReasonsNotInformed) && !selectedVisitReasonId,
    visitReasonDetail: selectedVisitReasonId ? String(currentDraft.visitReasonDetails?.[selectedVisitReasonId] || "") : "",
    customerSourceId: selectedSourceId,
    customerSourceNotInformed: Boolean(currentDraft.customerSourcesNotInformed) && !selectedSourceId,
    customerSourceDetail: selectedSourceId ? String(currentDraft.customerSourceDetails?.[selectedSourceId] || "") : "",
    queueJumpReason: String(currentDraft.queueJumpReason || ""),
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

const modalConfig = computed(() => props.state.modalConfig || {});
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
const availableVisitReasons = computed(() =>
  (props.state.visitReasonOptions || []).filter((option) => {
    const allowedOutcomes = Array.isArray(option.outcomes) ? option.outcomes : [];

    if (!form.outcome || allowedOutcomes.length === 0) {
      return true;
    }

    return allowedOutcomes.includes(form.outcome) || option.id === form.visitReasonId;
  })
);
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
    visitReasons: !!(form.visitReasonId || form.visitReasonNotInformed),
    customerSources: !!(form.customerSourceId || form.customerSourceNotInformed)
  };

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
  availableVisitReasons.value.map((option) =>
    mapOptionToPickerItem(
      option,
      Array.isArray(option.outcomes) && option.outcomes.length ? option.outcomes.join(" / ") : ""
    )
  )
);
const visitReasonSelectedItems = computed({
  get: () => visitReasonPickerOptions.value.filter((option) => option.id === form.visitReasonId),
  set: (items) => {
    form.visitReasonId = items[0]?.id || "";
    form.visitReasonNotInformed = false;
  }
});
const customerSourcePickerOptions = computed(() =>
  (props.state.customerSourceOptions || []).map((option) => mapOptionToPickerItem(option))
);
const customerSourceSelectedItems = computed({
  get: () => customerSourcePickerOptions.value.filter((option) => option.id === form.customerSourceId),
  set: (items) => {
    form.customerSourceId = items[0]?.id || "";
    form.customerSourceNotInformed = false;
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

function resetForm() {
  step.value = 1;
  Object.assign(form, createEmptyForm(), buildInitialForm(props.state, draft.value));
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
  void dashboard.closeFinishModal();
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

  if (modalConfig.value.requireVisitReason && !form.visitReasonId && !form.visitReasonNotInformed) {
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

  if (modalConfig.value.requireCustomerSource && !form.customerSourceId && !form.customerSourceNotInformed) {
    await ui.alert("Selecione uma origem do cliente ou marque 'Nao informado'.");
    return;
  }

  if (service.value.startMode === "queue-jump" && !form.queueJumpReason.trim()) {
    await ui.alert("Preencha o motivo do atendimento fora da vez.");
    return;
  }

  await dashboard.finishService(service.value.id, {
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
    visitReasons: form.visitReasonId ? [form.visitReasonId] : [],
    visitReasonsNotInformed: form.visitReasonNotInformed,
    visitReasonDetails: form.visitReasonId && form.visitReasonDetail.trim()
      ? { [form.visitReasonId]: form.visitReasonDetail.trim() }
      : {},
    customerSources: form.customerSourceId ? [form.customerSourceId] : [],
    customerSourcesNotInformed: form.customerSourceNotInformed,
    customerSourceDetails: form.customerSourceId && form.customerSourceDetail.trim()
      ? { [form.customerSourceId]: form.customerSourceDetail.trim() }
      : {},
    saleAmount: isClosedOutcome.value ? closedTotal.value : 0,
    queueJumpReason: form.queueJumpReason.trim(),
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

watch(() => form.visitReasonId, (nextValue) => {
  if (nextValue) {
    form.visitReasonNotInformed = false;
    return;
  }

  form.visitReasonDetail = "";
});

watch(() => form.customerSourceId, (nextValue) => {
  if (nextValue) {
    form.customerSourceNotInformed = false;
    return;
  }

  form.customerSourceDetail = "";
});

watch(() => form.visitReasonNotInformed, (nextValue) => {
  if (!nextValue) {
    return;
  }

  form.visitReasonId = "";
  form.visitReasonDetail = "";
});

watch(() => form.customerSourceNotInformed, (nextValue) => {
  if (!nextValue) {
    return;
  }

  form.customerSourceId = "";
  form.customerSourceDetail = "";
});

watch(() => form.outcome, (nextValue) => {
  if (nextValue === "compra" || nextValue === "reserva") {
    return;
  }

  form.isGift = false;
});

function handleEscape(event) {
  if (event.key !== "Escape") return;
  if (!service.value) return;
  if (document.querySelector(".product-pick__dropdown.is-open")) return;
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
                  :multiple="false"
                  :none-selected="form.visitReasonNotInformed"
                  allow-none
                  none-label="Nao informado"
                  none-state-label="Nao informado"
                  trigger-label="Selecionar motivo"
                  search-placeholder="Busque e selecione o motivo"
                  empty-selected-label="Nenhum motivo selecionado"
                  testid-prefix="operation-visit-reason"
                  @update:selected-items="updateVisitReasonSelectedItems"
                  @update:none-selected="form.visitReasonNotInformed = $event"
                />
                <input
                  v-if="modalConfig.showVisitReasonDetails && form.visitReasonId"
                  v-model="form.visitReasonDetail"
                  class="finish-form__input"
                  type="text"
                  placeholder="Detalhe opcional"
                  data-testid="operation-visit-reason-detail"
                >
              </section>

              <section class="finish-form__section operation-modal__picker-cell">
                <OperationProductPicker
                  label="De onde o cliente veio"
                  :options="customerSourcePickerOptions"
                  :selected-items="customerSourceSelectedItems"
                  :multiple="false"
                  :none-selected="form.customerSourceNotInformed"
                  allow-none
                  none-label="Nao informado"
                  none-state-label="Nao informado"
                  trigger-label="Selecionar origem"
                  search-placeholder="Busque e selecione a origem"
                  empty-selected-label="Nenhuma origem selecionada"
                  testid-prefix="operation-customer-source"
                  @update:selected-items="updateCustomerSourceSelectedItems"
                  @update:none-selected="form.customerSourceNotInformed = $event"
                />
                <input
                  v-if="modalConfig.showCustomerSourceDetails && form.customerSourceId"
                  v-model="form.customerSourceDetail"
                  class="finish-form__input"
                  type="text"
                  placeholder="Detalhe opcional"
                  data-testid="operation-customer-source-detail"
                >
              </section>
            </div>

            <section v-if="service.startMode === 'queue-jump'" class="finish-form__section">
              <label class="finish-form__label" for="queue-jump-reason">
                {{ modalConfig.queueJumpReasonLabel }}
              </label>
              <textarea
                id="queue-jump-reason"
                v-model="form.queueJumpReason"
                class="finish-form__textarea"
                rows="2"
                :placeholder="modalConfig.queueJumpReasonPlaceholder"
                data-testid="operation-queue-jump-reason"
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
