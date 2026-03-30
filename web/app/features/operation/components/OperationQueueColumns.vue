<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { formatClock, formatDuration } from "@core/utils/time";
import { useDashboardStore } from "~/stores/dashboard";

const props = defineProps({
  state: {
    type: Object,
    required: true
  }
});

const dashboard = useDashboardStore();
const now = ref(Date.now());
let timerId = null;

const waitingList = computed(() => props.state.waitingList || []);
const activeServices = computed(() => props.state.activeServices || []);
const maxConcurrentServices = computed(() => props.state.settings?.maxConcurrentServices || 10);
const isLimitReached = computed(() => activeServices.value.length >= maxConcurrentServices.value);

function serviceLabel(service) {
  const skippedCount = service.skippedPeople?.length || 0;
  const typeLabel = service.startMode === "queue-jump" ? "Fora da vez" : "Na vez";

  return skippedCount > 0
    ? `${typeLabel}, passou ${skippedCount} ${skippedCount === 1 ? "pessoa" : "pessoas"}`
    : typeLabel;
}

function actionHint(index) {
  const skippedCount = index;
  return `Passa na frente de ${skippedCount} ${skippedCount === 1 ? "pessoa" : "pessoas"}`;
}

function startFirstService() {
  void dashboard.startService();
}

function startSpecificService(personId) {
  void dashboard.startService(personId);
}

function openFinishModal(personId) {
  void dashboard.openFinishModal(personId);
}

onMounted(() => {
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
      <div v-if="waitingList.length > 0" class="queue-column__action-bar">
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
              <strong class="queue-card__name">{{ person.name }}</strong>
              <span class="queue-card__role">{{ person.role }}</span>
              <span class="queue-card__note">{{ index === 0 ? "Aguardando" : "Aguardando na fila" }}</span>
            </span>
            <div class="queue-card__actions">
              <span v-if="index === 0" class="queue-card__badge">Na vez</span>
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
          <span class="queue-empty__text">Use a barra de Consultores abaixo para colocar alguem na lista.</span>
        </div>
      </div>
    </section>

    <section class="queue-column" data-testid="operation-service-column">
      <header class="queue-column__header">Em atendimento</header>
      <div class="queue-column__body queue-column__body--service">
        <template v-if="activeServices.length > 0">
          <article
            v-for="service in activeServices"
            :key="service.serviceId"
            class="service-card"
            :data-testid="`operation-service-${service.id}`"
          >
            <div class="service-card__header">
              <span class="service-card__eyebrow">Atendimento em andamento</span>
              <span class="queue-card__note">Iniciado as {{ formatClock(service.serviceStartedAt) }}</span>
              <span class="queue-card__note">ID {{ service.serviceId }}</span>
            </div>
            <div class="service-card__body">
              <span class="queue-card__avatar queue-card__avatar--large" :style="{ '--avatar-accent': service.color }">
                {{ service.initials }}
              </span>
              <div class="service-card__content">
                <strong class="queue-card__name">{{ service.name }}</strong>
                <span class="queue-card__role">{{ service.role }}</span>
                <span class="queue-card__note">{{ serviceLabel(service) }}</span>
              </div>
              <strong class="service-card__timer">
                {{ formatDuration(now - service.serviceStartedAt) }}
              </strong>
            </div>
            <button
              class="column-action column-action--secondary"
              type="button"
              :data-testid="`operation-finish-${service.id}`"
              @click="openFinishModal(service.id)"
            >
              Encerrar atendimento
            </button>
          </article>
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
