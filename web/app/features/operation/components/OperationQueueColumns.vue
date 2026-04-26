<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
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

const waitingList = computed(() => props.state.waitingList || []);
const activeServices = computed(() => props.state.activeServices || []);
const maxConcurrentServices = computed(() => props.state.settings?.maxConcurrentServices || 10);
const isLimitReached = computed(() => activeServices.value.length >= maxConcurrentServices.value);

function actionHint(index) {
  const skippedCount = index;
  return `Passa na frente de ${skippedCount} ${skippedCount === 1 ? "pessoa" : "pessoas"}`;
}

async function startFirstService() {
  const result = await operationsStore.startService();

  if (result?.ok === false) {
    ui.error(result.message);
  }
}

async function startSpecificService(personId) {
  const result = await operationsStore.startService(personId);

  if (result?.ok === false) {
    ui.error(result.message);
  }
}

function openFinishModal(personId) {
  void operationsStore.openFinishModal(personId);
}

async function assignTask(person) {
  const { confirmed, value } = await ui.prompt({
    title: "Direcionar para tarefa",
    message: `Registre a tarefa ou reuniao para ${person.name}${person.storeName ? ` em ${person.storeName}` : ""}.`,
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
  }, 1000);
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
                <strong class="queue-card__name">{{ person.name }}</strong>
                <span v-if="props.integratedMode && person.storeName" class="queue-card__store-badge">{{ person.storeName }}</span>
              </span>
              <span class="queue-card__role">{{ person.role }}</span>
              <span class="queue-card__note">{{ index === 0 ? "Aguardando" : "Aguardando na fila" }}</span>
            </span>
            <div class="queue-card__actions">
              <span v-if="(index === 0 && !props.integratedMode) || props.readOnly" class="queue-card__badge">{{ index === 0 ? "Na vez" : "Na fila" }}</span>
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
          <OperationActiveServiceCard
            v-for="service in activeServices"
            :key="service.serviceId"
            :service="service"
            :now="now"
            :clock-ready="isClockReady"
            :read-only="props.readOnly"
            :integrated-mode="props.integratedMode"
            @finish="openFinishModal"
          />
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
