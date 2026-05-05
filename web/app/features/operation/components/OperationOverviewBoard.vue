<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";

import { buildNickname } from "~/domain/utils/person-display";
import { formatClock, formatDuration } from "~/domain/utils/time";
import { useOperationsStore } from "~/stores/operations";
import { useUiStore } from "~/stores/ui";

const props = defineProps({
  overview: {
    type: Object,
    default: null
  },
  readOnly: {
    type: Boolean,
    default: false
  },
  filterStoreId: {
    type: String,
    default: ""
  }
});

const operationsStore = useOperationsStore();
const ui = useUiStore();
const now = ref(0);
let timerId = null;
const CLOCK_REFRESH_MS = 250;
const serverClockOffsetMs = computed(() => Number(operationsStore.state?.serverClockOffsetMs || 0) || 0);
const adjustedNow = computed(() => now.value + serverClockOffsetMs.value);

function shouldIncludeStore(storeId) {
  const filterStoreId = String(props.filterStoreId || "").trim();
  return !filterStoreId || String(storeId || "").trim() === filterStoreId;
}

const stores = computed(() =>
  (Array.isArray(props.overview?.stores) ? props.overview.stores : []).filter((store) => shouldIncludeStore(store.storeId))
);

const waitingList = computed(() =>
  (Array.isArray(props.overview?.waitingList) ? props.overview.waitingList : []).filter((item) => shouldIncludeStore(item.storeId))
);

const activeServices = computed(() =>
  (Array.isArray(props.overview?.activeServices) ? props.overview.activeServices : []).filter((item) => shouldIncludeStore(item.storeId))
);

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

function deriveSequentialServiceFinishedAt(targetService) {
  const targetStartedAt = Number(targetService?.serviceStartedAt || 0) || 0;
  let nextStartedAt = 0;

  activeServices.value.forEach((service) => {
    if (String(service?.serviceId || "").trim() === String(targetService?.serviceId || "").trim()) {
      return;
    }
    if (String(service?.personId || "").trim() !== String(targetService?.personId || "").trim()) {
      return;
    }
    if (!isSameSequentialGroup(targetService, service)) {
      return;
    }

    const candidateStartedAt = Number(service?.serviceStartedAt || 0) || 0;
    if (candidateStartedAt <= targetStartedAt) {
      return;
    }
    if (!nextStartedAt || candidateStartedAt < nextStartedAt) {
      nextStartedAt = candidateStartedAt;
    }
  });

  return nextStartedAt || 0;
}

function formatServiceDuration(service) {
  const effectiveFinishedAt = firstPositiveTimestamp([
    service?.effectiveFinishedAt,
    service?.stoppedAt,
    deriveSequentialServiceFinishedAt(service)
  ]);
  const finishedAt = effectiveFinishedAt > 0 ? effectiveFinishedAt : adjustedNow.value;
  return formatDuration(
    Math.max(0, finishedAt - Number(service?.serviceStartedAt || 0)),
    { roundUpPartialSecond: effectiveFinishedAt === 0 }
  );
}

const pausedEmployees = computed(() =>
  (Array.isArray(props.overview?.pausedEmployees) ? props.overview.pausedEmployees : []).filter((item) => shouldIncludeStore(item.storeId))
);

const availableConsultants = computed(() =>
  (Array.isArray(props.overview?.availableConsultants) ? props.overview.availableConsultants : []).filter((item) => shouldIncludeStore(item.storeId))
);

function pauseLabel(person) {
  return String(person?.pauseKind || "").trim() === "assignment" ? "Em tarefa" : "Pausado";
}

function displayName(person) {
  return buildNickname(person?.name || "");
}

async function assignTask(person) {
  const { confirmed, value } = await ui.prompt({
    title: "Enviar para tarefa",
    message: `Registre a tarefa ou reuniao para ${displayName(person)} em ${person.storeName}.`,
    inputLabel: "Motivo",
    inputPlaceholder: "Ex.: reuniao, apoio no caixa, conferencia de estoque",
    confirmLabel: "Salvar tarefa",
    required: true
  });

  if (!confirmed || !value) {
    return;
  }

  const result = await operationsStore.assignTask(person.personId, value, person.storeId);
  if (result?.ok === false) {
    ui.error(result.message);
    return;
  }

  ui.success("Consultor direcionado para tarefa.");
}

async function resumePerson(person) {
  const result = await operationsStore.resumeEmployee(person.personId, person.storeId);
  if (result?.ok === false) {
    ui.error(result.message);
    return;
  }

  ui.success("Consultor devolvido para a fila.");
}

onMounted(() => {
  now.value = Date.now();
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
  <section class="operation-overview" data-testid="operation-overview-board">
    <div class="operation-overview__summary">
      <article
        v-for="store in stores"
        :key="store.storeId"
        class="operation-overview__summary-card"
      >
        <div class="operation-overview__summary-head">
          <strong>{{ store.storeName }}</strong>
          <span>{{ store.city || store.storeCode || "Loja" }}</span>
        </div>
        <div class="operation-overview__summary-grid">
          <span>Fila: <strong>{{ store.waitingCount }}</strong></span>
          <span>Atendimento: <strong>{{ store.activeCount }}</strong></span>
          <span>Pausa: <strong>{{ store.pausedCount }}</strong></span>
          <span>Disponiveis: <strong>{{ store.availableCount }}</strong></span>
        </div>
      </article>
    </div>

    <div class="operation-overview__columns">
      <section class="operation-overview__column">
        <header class="operation-overview__column-head">
          <strong>Em atendimento</strong>
          <span>{{ activeServices.length }}</span>
        </header>
        <div class="operation-overview__column-body">
          <article
            v-for="service in activeServices"
            :key="`${service.storeId}-${service.personId}-${service.serviceId}`"
            class="operation-overview__card"
          >
            <div class="operation-overview__card-top">
              <span class="operation-overview__store">{{ service.storeName }}</span>
              <span class="operation-overview__meta">ID {{ service.serviceId }}</span>
            </div>
            <div class="operation-overview__person">
              <span class="queue-card__avatar" :style="{ '--avatar-accent': service.color }">{{ service.initials }}</span>
              <div class="operation-overview__person-copy">
                <strong>{{ displayName(service) }}</strong>
                <span>{{ service.role }}</span>
              </div>
            </div>
            <div class="operation-overview__details">
              <span>{{ service.startMode === "queue-jump" ? "Fora da vez" : "Na vez" }}</span>
              <span>Inicio {{ formatClock(Math.max(0, Number(service.serviceStartedAt || 0) - serverClockOffsetMs)) }}</span>
              <strong>{{ formatServiceDuration(service) }}</strong>
            </div>
          </article>
          <div v-if="!activeServices.length" class="queue-empty">
            <strong class="queue-empty__title">Nenhum atendimento ativo</strong>
            <span class="queue-empty__text">A visao integrada mostra todos os atendimentos das lojas acessiveis.</span>
          </div>
        </div>
      </section>

      <section class="operation-overview__column">
        <header class="operation-overview__column-head">
          <strong>Na fila</strong>
          <span>{{ waitingList.length }}</span>
        </header>
        <div class="operation-overview__column-body">
          <article
            v-for="person in waitingList"
            :key="`${person.storeId}-${person.personId}`"
            class="operation-overview__card"
          >
            <div class="operation-overview__card-top">
              <span class="operation-overview__store">{{ person.storeName }}</span>
              <span class="operation-overview__meta">Posicao {{ person.queuePosition }}</span>
            </div>
            <div class="operation-overview__person">
              <span class="queue-card__avatar" :style="{ '--avatar-accent': person.color }">{{ person.initials }}</span>
              <div class="operation-overview__person-copy">
                <strong>{{ displayName(person) }}</strong>
                <span>{{ person.role }}</span>
              </div>
            </div>
            <div class="operation-overview__details">
              <span>Entrou {{ formatClock(person.queueJoinedAt) }}</span>
            </div>
            <button
              v-if="!readOnly"
              class="operation-overview__action"
              type="button"
              @click="assignTask(person)"
            >
              Tirar para tarefa
            </button>
          </article>
          <div v-if="!waitingList.length" class="queue-empty">
            <strong class="queue-empty__title">Fila sem pendencias</strong>
            <span class="queue-empty__text">Nenhum consultor aguardando nas lojas filtradas.</span>
          </div>
        </div>
      </section>

      <section class="operation-overview__column">
        <header class="operation-overview__column-head">
          <strong>Pausas e tarefas</strong>
          <span>{{ pausedEmployees.length }}</span>
        </header>
        <div class="operation-overview__column-body">
          <article
            v-for="person in pausedEmployees"
            :key="`${person.storeId}-${person.personId}`"
            class="operation-overview__card"
          >
            <div class="operation-overview__card-top">
              <span class="operation-overview__store">{{ person.storeName }}</span>
              <span class="operation-overview__meta">{{ pauseLabel(person) }}</span>
            </div>
            <div class="operation-overview__person">
              <span class="queue-card__avatar" :style="{ '--avatar-accent': person.color }">{{ person.initials }}</span>
              <div class="operation-overview__person-copy">
                <strong>{{ displayName(person) }}</strong>
                <span>{{ person.role }}</span>
              </div>
            </div>
            <div class="operation-overview__details">
              <span>{{ person.pauseReason }}</span>
              <span>{{ formatDuration(now - person.statusStartedAt) }}</span>
            </div>
            <button
              v-if="!readOnly"
              class="operation-overview__action"
              type="button"
              @click="resumePerson(person)"
            >
              Voltar para fila
            </button>
          </article>
          <div v-if="!pausedEmployees.length" class="queue-empty">
            <strong class="queue-empty__title">Nenhuma pausa ou tarefa</strong>
            <span class="queue-empty__text">Quando alguem sair para tarefa, reuniao ou pausa, aparece aqui.</span>
          </div>
        </div>
      </section>

      <section class="operation-overview__column">
        <header class="operation-overview__column-head">
          <strong>Disponiveis</strong>
          <span>{{ availableConsultants.length }}</span>
        </header>
        <div class="operation-overview__column-body">
          <article
            v-for="person in availableConsultants"
            :key="`${person.storeId}-${person.personId}`"
            class="operation-overview__card"
          >
            <div class="operation-overview__card-top">
              <span class="operation-overview__store">{{ person.storeName }}</span>
              <span class="operation-overview__meta">Disponivel</span>
            </div>
            <div class="operation-overview__person">
              <span class="queue-card__avatar" :style="{ '--avatar-accent': person.color }">{{ person.initials }}</span>
              <div class="operation-overview__person-copy">
                <strong>{{ displayName(person) }}</strong>
                <span>{{ person.role }}</span>
              </div>
            </div>
            <div class="operation-overview__details">
              <span>Pronto para atender</span>
            </div>
            <button
              v-if="!readOnly"
              class="operation-overview__action"
              type="button"
              @click="assignTask(person)"
            >
              Designar tarefa
            </button>
          </article>
          <div v-if="!availableConsultants.length" class="queue-empty">
            <strong class="queue-empty__title">Nenhum disponivel</strong>
            <span class="queue-empty__text">Todos os consultores filtrados estao em fila, pausa ou atendimento.</span>
          </div>
        </div>
      </section>
    </div>
  </section>
</template>

<style scoped>
.operation-overview {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.operation-overview__summary {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(14rem, 1fr));
  gap: 0.85rem;
}

.operation-overview__summary-card,
.operation-overview__card {
  border: 1px solid rgba(125, 146, 255, 0.16);
  border-radius: 1rem;
  background: rgba(13, 19, 36, 0.82);
  padding: 0.9rem;
}

.operation-overview__summary-head,
.operation-overview__card-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.operation-overview__summary-head span,
.operation-overview__meta {
  color: rgba(219, 226, 255, 0.68);
  font-size: 0.78rem;
}

.operation-overview__summary-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.45rem 0.8rem;
  margin-top: 0.7rem;
  color: rgba(244, 247, 255, 0.84);
  font-size: 0.82rem;
}

.operation-overview__columns {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0.9rem;
}

.operation-overview__column {
  display: flex;
  flex-direction: column;
  min-height: 18rem;
  border: 1px solid rgba(125, 146, 255, 0.14);
  border-radius: 1rem;
  background: rgba(8, 13, 26, 0.72);
}

.operation-overview__column-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.9rem 1rem;
  border-bottom: 1px solid rgba(125, 146, 255, 0.12);
}

.operation-overview__column-body {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 0.75rem;
  padding: 0.9rem;
}

.operation-overview__store {
  display: inline-flex;
  align-items: center;
  min-height: 1.7rem;
  padding: 0 0.55rem;
  border-radius: 999px;
  background: rgba(125, 146, 255, 0.18);
  color: #dce3ff;
  font-size: 0.72rem;
  font-weight: 700;
}

.operation-overview__person {
  display: flex;
  align-items: center;
  gap: 0.7rem;
  margin-top: 0.8rem;
}

.operation-overview__person-copy {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 0.15rem;
}

.operation-overview__person-copy strong {
  font-size: 0.96rem;
}

.operation-overview__person-copy span,
.operation-overview__details {
  color: rgba(219, 226, 255, 0.76);
  font-size: 0.8rem;
}

.operation-overview__details {
  display: flex;
  flex-direction: column;
  gap: 0.18rem;
  margin-top: 0.8rem;
}

.operation-overview__action {
  margin-top: 0.8rem;
  min-height: 2.45rem;
  border: 1px solid rgba(125, 146, 255, 0.24);
  border-radius: 0.85rem;
  background: rgba(125, 146, 255, 0.18);
  color: #f4f7ff;
  font: inherit;
  font-weight: 700;
  cursor: pointer;
}

.operation-overview__action:hover {
  background: rgba(125, 146, 255, 0.26);
}

@media (max-width: 1250px) {
  .operation-overview__columns {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 760px) {
  .operation-overview__columns {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
