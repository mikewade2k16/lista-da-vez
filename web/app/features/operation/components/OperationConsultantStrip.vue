<script setup>
import { computed } from "vue";
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

const employees = computed(() => props.state.roster || []);
const waitingIds = computed(() => new Set((props.state.waitingList || []).map((person) => person.id)));
const activeServiceIds = computed(() => new Set((props.state.activeServices || []).map((service) => service.id)));
const pausedByPersonId = computed(() =>
  new Map((props.state.pausedEmployees || []).map((item) => [item.personId, item]))
);

function statusFor(employeeId) {
  if (activeServiceIds.value.has(employeeId)) {
    return "service";
  }

  if (pausedByPersonId.value.has(employeeId)) {
    return "paused";
  }

  if (waitingIds.value.has(employeeId)) {
    return "queue";
  }

  return "available";
}

function statusLabel(status) {
  if (status === "service") return "Em atendimento";
  if (status === "queue") return "Na fila";
  if (status === "paused") return "Pausado";
  return "Disponivel";
}

function addToQueue(personId) {
  void dashboard.addToQueue(personId);
}

async function pauseEmployee(personId) {
  const { confirmed, value } = await ui.prompt({
    title: "Pausar consultor",
    message: "Informe o motivo da pausa para registrar no painel.",
    inputLabel: "Motivo da pausa",
    inputPlaceholder: "Ex.: almoco, atendimento externo, suporte interno",
    confirmLabel: "Pausar",
    required: true
  });

  if (confirmed && value) {
    await dashboard.pauseEmployee(personId, value);
    ui.success("Consultor pausado.");
  }
}

async function resumeEmployee(personId) {
  await dashboard.resumeEmployee(personId);
  ui.success("Consultor retomado.");
}
</script>

<template>
  <footer class="employee-strip" data-testid="operation-consultant-strip">
    <div class="employee-strip__header">
      <strong class="employee-strip__title">Consultores</strong>
      <span class="employee-strip__text">Entrar na fila, pausar e retomar ficam por aqui</span>
    </div>
    <div class="employee-strip__list">
      <div
        v-for="employee in employees"
        :key="employee.id"
        class="employee"
        :class="`employee--${statusFor(employee.id)}`"
        :data-testid="`operation-consultant-${employee.id}`"
      >
        <span class="employee__avatar" :style="{ '--avatar-accent': employee.color }">
          {{ employee.initials }}
        </span>
        <div class="employee__info">
          <span class="employee__name">{{ employee.name }}</span>
          <span class="employee__status">{{ statusLabel(statusFor(employee.id)) }}</span>
          <span v-if="pausedByPersonId.get(employee.id)" class="employee__pause-reason">
            {{ pausedByPersonId.get(employee.id).reason }}
          </span>
        </div>

        <div v-if="statusFor(employee.id) === 'available'" class="employee__actions">
          <button
            class="employee__action employee__action--primary"
            type="button"
            title="Entrar na fila"
            :data-testid="`operation-add-to-queue-${employee.id}`"
            @click="addToQueue(employee.id)"
          >
            <span class="material-icons-round">login</span>
          </button>
          <button
            class="employee__action employee__action--secondary"
            type="button"
            title="Pausar"
            :data-testid="`operation-pause-${employee.id}`"
            @click="pauseEmployee(employee.id)"
          >
            <span class="material-icons-round">pause</span>
          </button>
        </div>

        <div v-else-if="statusFor(employee.id) === 'queue'" class="employee__actions">
          <button
            class="employee__action employee__action--secondary"
            type="button"
            title="Pausar"
            :data-testid="`operation-pause-${employee.id}`"
            @click="pauseEmployee(employee.id)"
          >
            <span class="material-icons-round">pause</span>
          </button>
        </div>

        <div v-else-if="statusFor(employee.id) === 'paused'" class="employee__actions">
          <button
            class="employee__action employee__action--primary"
            type="button"
            title="Retomar"
            :data-testid="`operation-resume-${employee.id}`"
            @click="resumeEmployee(employee.id)"
          >
            <span class="material-icons-round">play_arrow</span>
          </button>
        </div>
      </div>
    </div>
  </footer>
</template>
