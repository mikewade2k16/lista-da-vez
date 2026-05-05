<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import AppSelectField from "~/components/ui/AppSelectField.vue";
import { buildNickname } from "~/domain/utils/person-display";
import OperationActiveServiceCard from "~/features/operation/components/OperationActiveServiceCard.vue";
import { useOperationsStore } from "~/stores/operations";
import { useUiStore } from "~/stores/ui";

const props = defineProps({
  state: {
    type: Object,
    required: true
  },
  readOnly: {
    type: Boolean,
    default: false
  },
  integratedMode: {
    type: Boolean,
    default: false
  }
});

const operationsStore = useOperationsStore();
const ui = useUiStore();
const now = ref(0);
const isClockReady = ref(false);
let timerId = null;
const CLOCK_REFRESH_MS = 250;

const waitingList = computed(() => props.state.waitingList || []);
const activeServices = computed(() => props.state.activeServices || []);
const serviceHistory = computed(() => props.state.serviceHistory || []);
const serverClockOffsetMs = computed(() => Number(props.state?.serverClockOffsetMs || 0) || 0);
const adjustedNow = computed(() => now.value + serverClockOffsetMs.value);
const maxConcurrentServices = computed(() => props.state.settings?.maxConcurrentServices || 10);
const maxConcurrentPerConsultant = computed(() => props.state.settings?.maxConcurrentServicesPerConsultant || 1);
const cancelWindowSeconds = computed(() => Number(props.state.settings?.serviceCancelWindowSeconds || 30) || 30);
const modalConfig = computed(() => props.state.modalConfig || {});
const cancelReasonOptions = computed(() => Array.isArray(props.state.cancelReasonOptions) ? props.state.cancelReasonOptions : []);
const stopReasonOptions = computed(() => Array.isArray(props.state.stopReasonOptions) ? props.state.stopReasonOptions : []);
const isLimitReached = computed(() => activeServices.value.length >= maxConcurrentServices.value);
const actionModal = ref({
  open: false,
  action: "",
  service: null,
  reasonId: "",
  reasonText: "",
  submitting: false
});

const actionReasonOptions = computed(() => {
  const source = actionModal.value.action === "cancel" ? cancelReasonOptions.value : stopReasonOptions.value;
  return source.map((item) => ({
    value: String(item?.id || "").trim(),
    label: String(item?.label || "").trim()
  })).filter((item) => item.value && item.label);
});

const actionReasonMode = computed(() => {
  const rawMode = String(
    actionModal.value.action === "cancel"
      ? modalConfig.value?.cancelReasonInputMode
      : modalConfig.value?.stopReasonInputMode
  ).trim().toLowerCase();

  const normalized = rawMode === "select-with-other" || rawMode === "select_other" || rawMode === "select-other"
    ? "select-with-other"
    : rawMode === "select"
      ? "select"
      : "text";

  if (normalized !== "text" && actionReasonOptions.value.length === 0) {
    return "text";
  }

  return normalized;
});

const actionReasonLabel = computed(() => actionModal.value.action === "cancel"
  ? String(modalConfig.value?.cancelReasonLabel || "Motivo do cancelamento").trim()
  : String(modalConfig.value?.stopReasonLabel || "Motivo da parada").trim());

const actionReasonPlaceholder = computed(() => actionModal.value.action === "cancel"
  ? String(modalConfig.value?.cancelReasonPlaceholder || "Informe o motivo do cancelamento").trim()
  : String(modalConfig.value?.stopReasonPlaceholder || "Informe o motivo da parada").trim());

const actionOtherReasonLabel = computed(() => actionModal.value.action === "cancel"
  ? String(modalConfig.value?.cancelReasonOtherLabel || "Detalhe do cancelamento").trim()
  : String(modalConfig.value?.stopReasonOtherLabel || "Detalhe da parada").trim());

const actionOtherReasonPlaceholder = computed(() => actionModal.value.action === "cancel"
  ? String(modalConfig.value?.cancelReasonOtherPlaceholder || "Explique o motivo").trim()
  : String(modalConfig.value?.stopReasonOtherPlaceholder || "Explique o motivo").trim());

const actionReasonRequired = computed(() => false);

const showActionReasonField = computed(() => false);

const shouldShowOtherReason = computed(() => false);

const actionModalTitle = computed(() => actionModal.value.action === "cancel" ? "Cancelar atendimento" : "Parar atendimento");
const actionModalDescription = computed(() => {
  return actionModal.value.action === "cancel"
    ? "Esse atendimento sera desfeito e o consultor volta para a posicao que estava na fila."
    : "O tempo do atendimento ficara pausado. O consultor ainda precisara encerrar o atendimento.";
});

function isSameSequentialGroup(targetService, candidate) {
  const targetGroupId = String(targetService?.parallelGroupId || "").trim();
  const targetServiceId = String(targetService?.serviceId || "").trim();

  if (targetGroupId) {
    return String(candidate?.parallelGroupId || "").trim() === targetGroupId;
  }

  const siblingServiceIds = Array.isArray(candidate?.siblingServiceIds) ? candidate.siblingServiceIds : [];
  return siblingServiceIds.includes(targetServiceId);
}

function firstPositiveTimestamp(values) {
  return values
    .map((value) => Number(value || 0) || 0)
    .filter((value) => value > 0)
    .sort((left, right) => left - right)[0] || 0;
}

function deriveSequentialServiceFinishedAt(targetService, groupedServices) {
  const targetStartedAt = Number(targetService?.serviceStartedAt || 0) || 0;
  const explicitFinishedAt = Number(targetService?.effectiveFinishedAt || 0) || 0;
  let nextStartedAt = 0;

  const consider = (candidateStartedAt) => {
    const normalizedStartedAt = Number(candidateStartedAt || 0) || 0;
    if (normalizedStartedAt <= targetStartedAt) {
      return;
    }
    if (!nextStartedAt || normalizedStartedAt < nextStartedAt) {
      nextStartedAt = normalizedStartedAt;
    }
  };

  groupedServices.forEach((service) => {
    if (String(service?.serviceId || "").trim() === String(targetService?.serviceId || "").trim()) {
      return;
    }
    if (!isSameSequentialGroup(targetService, service)) {
      return;
    }
    consider(service?.serviceStartedAt);
  });

  serviceHistory.value.forEach((entry) => {
    if (String(entry?.personId || "").trim() !== String(targetService?.id || "").trim()) {
      return;
    }
    if (!isSameSequentialGroup(targetService, entry)) {
      return;
    }
    consider(entry?.startedAt);
  });

  return firstPositiveTimestamp([explicitFinishedAt, nextStartedAt]);
}

const servicesGroupedByConsultant = computed(() => {
  const grouped = new Map();
  activeServices.value.forEach((service) => {
    if (!grouped.has(service.id)) {
      grouped.set(service.id, []);
    }
    grouped.get(service.id).push(service);
  });

  grouped.forEach((services, consultantId) => {
    const sortedServices = [...services].sort((left, right) => {
      const leftSequence = Number(left.parallelStartIndex || 0);
      const rightSequence = Number(right.parallelStartIndex || 0);

      if (leftSequence > 0 && rightSequence > 0 && leftSequence !== rightSequence) {
        return leftSequence - rightSequence;
      }

      if (left.serviceStartedAt !== right.serviceStartedAt) {
        return Number(left.serviceStartedAt || 0) - Number(right.serviceStartedAt || 0);
      }

      return String(left.serviceId || "").localeCompare(String(right.serviceId || ""));
    });

    grouped.set(
      consultantId,
      sortedServices.map((service) => ({
        ...service,
        effectiveFinishedAt: deriveSequentialServiceFinishedAt(service, sortedServices)
      }))
    );
  });

  // Ordenar consultores: mais recente (ultimo serviceStartedAt) em cima
  const sortedEntries = [...grouped.entries()].sort(([, servicesA], [, servicesB]) => {
    const latestA = servicesA.reduce((max, s) => Math.max(max, Number(s.serviceStartedAt || 0)), 0);
    const latestB = servicesB.reduce((max, s) => Math.max(max, Number(s.serviceStartedAt || 0)), 0);
    return latestB - latestA;
  });

  return new Map(sortedEntries);
});

function actionHint(index) {
  const skippedCount = index;
  return `Passa na frente de ${skippedCount} ${skippedCount === 1 ? "pessoa" : "pessoas"}`;
}

function displayName(person) {
  return buildNickname(person?.name || "");
}

function isWithinCancelWindow(service) {
  if (!service || Number(service?.stoppedAt || 0) > 0) {
    return false;
  }

  const windowMs = Math.max(0, cancelWindowSeconds.value) * 1000;
  if (!windowMs) {
    return false;
  }

  return Math.max(0, adjustedNow.value - Number(service?.serviceStartedAt || 0)) <= windowMs;
}

function resolveService(serviceOrId) {
  if (serviceOrId && typeof serviceOrId === "object") {
    return serviceOrId;
  }

  const targetId = String(serviceOrId || "").trim();
  return activeServices.value.find((item) => String(item?.serviceId || "").trim() === targetId) || null;
}

function closeActionModal() {
  actionModal.value = {
    open: false,
    action: "",
    service: null,
    reasonId: "",
    reasonText: "",
    submitting: false
  };
}

function openActionModal(service, action) {
  actionModal.value = {
    open: true,
    action,
    service,
    reasonId: "",
    reasonText: "",
    submitting: false
  };
}

function resolveActionReason() {
  if (!showActionReasonField.value) {
    return "";
  }

  if (actionReasonMode.value === "text") {
    return String(actionModal.value.reasonText || "").trim();
  }

  if (actionReasonMode.value === "select-with-other" && actionModal.value.reasonId === "__other__") {
    return String(actionModal.value.reasonText || "").trim();
  }

  return actionReasonOptions.value.find((item) => item.value === actionModal.value.reasonId)?.label || "";
}

async function startFirstService() {
  const result = await operationsStore.startService();

  if (result?.ok === false) {
    ui.error(result.message);
  } else {
    ui.success("Atendimento iniciado!");
  }
}

async function startSpecificService(personId) {
  const result = await operationsStore.startService(personId);

  if (result?.ok === false) {
    ui.error(result.message);
  } else {
    ui.success("Atendimento iniciado!");
  }
}

async function openFinishModal(serviceOrId) {
  const service = resolveService(serviceOrId);
  if (!service) {
    return;
  }

  if (isWithinCancelWindow(service)) {
    const result = await operationsStore.serviceAction(
      service.serviceId,
      "cancel",
      "",
      { storeId: service.storeId || "" }
    );
    if (result?.ok === false) {
      ui.error(result.message || "Nao foi possivel cancelar o atendimento.");
    } else {
      ui.success("Atendimento cancelado e consultor devolvido para a fila.");
    }
    return;
  }

  void operationsStore.openFinishModal(service.serviceId);
}

function openStopModal(serviceOrId) {
  const service = resolveService(serviceOrId);
  if (!service) {
    return;
  }

  openActionModal(service, "stop");
}

async function startParallelService(personId) {
  const consultant = props.state.roster?.find((item) => item.id === personId);
  const consultantName = displayName(consultant) || "Consultor";
  const result = await operationsStore.startParallelService(personId, consultant?.storeId || "");

  if (result?.ok === false) {
    ui.error(result.message);
  } else {
    const parallelCount = (activeServices.value.filter((item) => item.id === personId).length || 0) + 1;
    ui.success(`Abrindo ${parallelCount}o atendimento em aberto de ${consultantName}`);
  }
}

async function submitActionModal() {
  if (!actionModal.value.service || !actionModal.value.action || actionModal.value.submitting) {
    return;
  }

  const reason = resolveActionReason();
  if (showActionReasonField.value && actionReasonRequired.value && !reason) {
    ui.error(`${actionReasonLabel.value || "Justificativa"} e obrigatorio.`);
    return;
  }

  actionModal.value = {
    ...actionModal.value,
    submitting: true
  };

  const result = await operationsStore.serviceAction(
    actionModal.value.service.serviceId,
    actionModal.value.action,
    reason,
    {
      storeId: actionModal.value.service.storeId || ""
    }
  );

  if (result?.ok === false) {
    actionModal.value = {
      ...actionModal.value,
      submitting: false
    };
    ui.error(result.message || "Nao foi possivel concluir a acao.");
    return;
  }

  const successMessage = actionModal.value.action === "cancel"
    ? "Atendimento cancelado e consultor devolvido para a fila."
    : "Atendimento parado com tempo congelado.";
  closeActionModal();
  ui.success(successMessage);
}

async function assignTask(person) {
  const { confirmed, value } = await ui.prompt({
    title: "Direcionar para tarefa",
    message: `Registre a tarefa ou reuniao para ${displayName(person)}${person.storeName ? ` em ${person.storeName}` : ""}.`,
    inputLabel: "Motivo",
    inputPlaceholder: "Ex.: reuniao, apoio no caixa, estoque, suporte",
    confirmLabel: "Remover da fista da fila",
    required: true
  });

  if (!confirmed || !value) {
    return;
  }

  const result = await operationsStore.assignTask(person.id, value, props.integratedMode ? person.storeId : "");

  if (result?.ok === false) {
    ui.error(result.message);
    return;
  }

  ui.success("Consultor direcionado para tarefa.");
}

onMounted(() => {
  now.value = Date.now();
  isClockReady.value = true;
  timerId = window.setInterval(() => {
    now.value = Date.now();
  }, CLOCK_REFRESH_MS);
});

watch(activeServices, () => {
  now.value = Date.now();
});

onBeforeUnmount(() => {
  if (timerId) {
    window.clearInterval(timerId);
  }
});
</script>

<template>
  <div class="workspace__intro" style="display: none;">
    <h1 class="workspace__title">Lista da vez</h1>
    <p class="workspace__text">
      Toque em um funcionario abaixo para entrar na fila. O atendimento normal sai pelo botao do rodape
      e o fora da vez fica no card do consultor. Ao encerrar, abrimos o fechamento completo no modal.
    </p>
  </div>

  <div class="queue-grid" data-testid="operation-board">
      <section class="queue-column queue-column--waiting" data-testid="operation-waiting-column">
      <header class="queue-column__header">Lista da vez</header>
      <div v-if="waitingList.length > 0 && !props.readOnly && !props.integratedMode" class="queue-column__action-bar">
        <button
          class="column-action column-action--primary"
          type="button"
          :disabled="isLimitReached"
          data-testid="operation-start-first"
          @click="startFirstService"
        >
          {{ isLimitReached ? `Limite de ${maxConcurrentServices} atendimentos ativos` : "Atender primeiro da fila" }}
        </button>
      </div>
      <div class="queue-column__body queue-column__body--waiting">
        <template v-if="waitingList.length > 0">
          <article
            v-for="(person, index) in waitingList"
            :key="person.id"
            class="queue-card"
            :class="{ 'queue-card--next': index === 0 }"
            :data-testid="`operation-waiting-${person.id}`"
          >
            <span class="queue-card__position">{{ index + 1 }}</span>
            <span class="queue-card__avatar" :style="{ '--avatar-accent': person.color }">
              {{ person.initials }}
            </span>
            <span class="queue-card__content">
              <span class="queue-card__headline">
                <strong class="queue-card__name">{{ displayName(person) }}</strong>
                <span v-if="props.integratedMode && person.storeName" class="queue-card__store-badge">{{ person.storeName }}</span>
              </span>
              <span class="queue-card__role">{{ person.role }}</span>
              <span class="queue-card__note">{{ index === 0 ? "Aguardando" : "Aguardando na fila" }}</span>
            </span>
            <div class="queue-card__actions">
              <template v-if="(index === 0 && !props.integratedMode) || props.readOnly">
                <button
                  v-if="index === 0 && !props.readOnly"
                  class="queue-card__badge queue-card__badge--button"
                  type="button"
                  :data-testid="`operation-start-next-${person.id}`"
                  @click="startFirstService"
                >
                  Na vez
                </button>
                <span v-else class="queue-card__badge">{{ index === 0 ? "Na vez" : "Na fila" }}</span>
              </template>
              <template v-else-if="props.integratedMode">
                <button
                  class="queue-card__task-btn"
                  type="button"
                  :data-testid="`operation-assign-task-${person.id}`"
                  @click="assignTask(person)"
                >
                  Remover da lista
                </button>
              </template>
              <template v-else>
                <div class="queue-card__action-wrap">
                  <button
                    class="queue-card__action"
                    type="button"
                    title="Atender fora da vez"
                    :disabled="isLimitReached"
                    :data-testid="`operation-start-specific-${person.id}`"
                    @click="startSpecificService(person.id)"
                  >
                    <span class="material-icons-round">bolt</span>
                  </button>
                  <span class="queue-card__action-hint">{{ actionHint(index) }}</span>
                </div>
              </template>
            </div>
          </article>
        </template>
        <div v-else class="queue-empty">
          <span class="queue-empty__icon">!</span>
          <strong class="queue-empty__title">Fila vazia</strong>
          <span class="queue-empty__text">
            {{ props.readOnly ? "Acompanhe por aqui quando a operacao da loja iniciar novos atendimentos." : "Use a barra de Consultores abaixo para colocar alguem na lista." }}
          </span>
        </div>
      </div>
    </section>

    <section class="queue-column queue-column--service" data-testid="operation-service-column">
      <header class="queue-column__header">Em atendimento</header>
      <div class="queue-column__body queue-column__body--service">
        <template v-if="activeServices.length > 0">
          <div
            v-for="[consultantId, services] in servicesGroupedByConsultant"
            :key="consultantId"
            class="service-group"
          >
            <OperationActiveServiceCard
              :services="services"
              :now="adjustedNow"
              :clock-ready="isClockReady"
              :server-clock-offset-ms="serverClockOffsetMs"
              :read-only="props.readOnly"
              :integrated-mode="props.integratedMode"
              :max-concurrent-per-consultant="maxConcurrentPerConsultant"
              :cancel-window-seconds="cancelWindowSeconds"
              @finish="openFinishModal"
              @stop="openStopModal"
              @start-parallel="startParallelService"
            />
          </div>
        </template>
        <div v-else class="queue-empty">
          <span class="queue-empty__icon">...</span>
          <strong class="queue-empty__title">Nenhum atendimento em andamento</strong>
          <span class="queue-empty__text">Quando iniciar um atendimento, o tempo passa a ser contado aqui.</span>
        </div>
      </div>
    </section>
  </div>

  <Teleport to="body">
    <div
      v-if="actionModal.open && actionModal.service"
      class="modal-backdrop"
      data-testid="operation-service-action-backdrop"
      @click.self="closeActionModal"
    >
      <div class="finish-modal finish-modal--compact" role="dialog" aria-modal="true" data-testid="operation-service-action-modal">
        <div class="finish-modal__header">
          <div>
            <h2 class="finish-modal__title">{{ actionModalTitle }}</h2>
            <p class="finish-modal__subtitle">{{ displayName(actionModal.service) }}</p>
          </div>
          <div class="finish-modal__header-actions">
            <button class="finish-modal__close" type="button" aria-label="Fechar" @click="closeActionModal">x</button>
          </div>
        </div>

        <div class="service-action-modal__body">
          <p class="service-action-modal__description">{{ actionModalDescription }}</p>

          <div v-if="showActionReasonField" class="service-action-modal__field">
            <label v-if="actionReasonMode !== 'text'" class="settings-card__text">{{ actionReasonLabel }}</label>
            <AppSelectField
              v-if="actionReasonMode !== 'text'"
              v-model="actionModal.reasonId"
              class="settings-field"
              :options="[
                ...actionReasonOptions,
                ...(actionReasonMode === 'select-with-other' ? [{ value: '__other__', label: 'Outro' }] : [])
              ]"
              :placeholder="actionReasonPlaceholder"
              :searchable="actionReasonOptions.length >= 8"
              :show-leading-icon="false"
              testid="operation-service-action-reason-select"
            />
            <label v-if="shouldShowOtherReason" class="settings-field service-action-modal__textarea-wrap">
              <span class="settings-card__text">{{ actionReasonMode === 'text' ? actionReasonLabel : actionOtherReasonLabel }}</span>
              <textarea
                v-model="actionModal.reasonText"
                class="service-action-modal__textarea"
                :placeholder="actionReasonMode === 'text' ? actionReasonPlaceholder : actionOtherReasonPlaceholder"
                rows="4"
                data-testid="operation-service-action-reason-text"
              />
            </label>
          </div>
        </div>

        <div class="service-action-modal__footer">
          <button class="column-action column-action--secondary" type="button" @click="closeActionModal">
            Fechar
          </button>
          <button
            class="column-action column-action--primary"
            type="button"
            :disabled="actionModal.submitting"
            data-testid="operation-service-action-submit"
            @click="submitActionModal"
          >
            {{ actionModal.action === 'cancel' ? 'Confirmar cancelamento' : 'Confirmar parada' }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
