<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
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
const isLimitReached = computed(() => activeServices.value.length >= maxConcurrentServices.value);

function isSameSequentialGroup(targetService, candidate) {
  const targetGroupId = String(targetService?.parallelGroupId || "").trim();
  const targetServiceId = String(targetService?.serviceId || "").trim();

  if (targetGroupId) {
    return String(candidate?.parallelGroupId || "").trim() === targetGroupId;
  }

  const siblingServiceIds = Array.isArray(candidate?.siblingServiceIds) ? candidate.siblingServiceIds : [];
  return siblingServiceIds.includes(targetServiceId);
}

function deriveSequentialServiceFinishedAt(targetService, groupedServices) {
  const targetStartedAt = Number(targetService?.serviceStartedAt || 0) || 0;
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

  return nextStartedAt || 0;
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

  return grouped;
});

function actionHint(index) {
  const skippedCount = index;
  return `Passa na frente de ${skippedCount} ${skippedCount === 1 ? "pessoa" : "pessoas"}`;
}

function displayName(person) {
  return buildNickname(person?.name || "");
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

function openFinishModal(serviceId) {
  void operationsStore.openFinishModal(serviceId);
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
    <section class="queue-column" data-testid="operation-waiting-column">
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

    <section class="queue-column" data-testid="operation-service-column">
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
              @finish="openFinishModal"
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
</template>
