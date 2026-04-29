<script setup>
import { computed, ref } from "vue";
import { buildNickname } from "~/domain/utils/person-display";
import OperationPauseReasonDialog from "~/features/operation/components/OperationPauseReasonDialog.vue";
import { useOperationsStore } from "~/stores/operations";
import { useUiStore } from "~/stores/ui";

const props = defineProps({
  state: {
    type: Object,
    required: true
  },
  integratedMode: {
    type: Boolean,
    default: false
  }
});

const operationsStore = useOperationsStore();
const ui = useUiStore();

const employees = computed(() => props.state.roster || []);
const waitingIds = computed(() => new Set((props.state.waitingList || []).map((person) => person.id)));
const activeServiceIds = computed(() => new Set((props.state.activeServices || []).map((service) => service.id)));
const pausedByPersonId = computed(() =>
  new Map((props.state.pausedEmployees || []).map((item) => [item.personId, item]))
);
const pauseReasonOptions = computed(() => props.state.pauseReasonOptions || []);
const pauseDialogEmployee = ref(null);
const pausePending = ref(false);

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

function statusLabel(employeeId) {
  const status = statusFor(employeeId);
  const pausedItem = pausedByPersonId.value.get(employeeId);

  if (status === "service") return "Em atendimento";
  if (status === "queue") return "Na fila";
  if (status === "paused") {
    return String(pausedItem?.kind || "").trim() === "assignment" ? "Em tarefa" : "Pausado";
  }
  return "Disponivel";
}

function displayName(employee) {
  return buildNickname(employee?.name || "");
}

async function addToQueue(employee) {
  const result = await operationsStore.addToQueue(employee.id, props.integratedMode ? employee.storeId : "");

  if (result?.ok === false) {
    ui.error(result.message);
  }
}

function pauseEmployee(employee) {
  if (!pauseReasonOptions.value.length) {
    ui.error("Cadastre ao menos um motivo de pausa em Configuracoes > Pausas.");
    return;
  }

  pauseDialogEmployee.value = employee;
}

function closePauseDialog() {
  if (pausePending.value) {
    return;
  }

  pauseDialogEmployee.value = null;
}

async function confirmPauseEmployee(reason) {
  const employee = pauseDialogEmployee.value;
  const normalizedReason = String(reason || "").trim();

  if (!employee || !normalizedReason) {
    return;
  }

  pausePending.value = true;

  try {
    const result = await operationsStore.pauseEmployee(
      employee.id,
      normalizedReason,
      props.integratedMode ? employee.storeId : ""
    );

    if (result?.ok === false) {
      ui.error(result.message);
      return;
    }

    pauseDialogEmployee.value = null;
    ui.success("Consultor pausado.");
  } finally {
    pausePending.value = false;
  }
}

async function assignTask(employee) {
  const { confirmed, value } = await ui.prompt({
    title: "Direcionar para tarefa",
    message: "Registre a tarefa ou reuniao para tirar este consultor da fila temporariamente.",
    inputLabel: "Motivo",
    inputPlaceholder: "Ex.: reuniao, apoio no caixa, estoque, suporte",
    confirmLabel: "Salvar tarefa",
    required: true
  });

  if (!confirmed || !value) {
    return;
  }

  const result = await operationsStore.assignTask(employee.id, value, props.integratedMode ? employee.storeId : "");

  if (result?.ok === false) {
    ui.error(result.message);
    return;
  }

  ui.success("Consultor direcionado para tarefa.");
}

async function resumeEmployee(employee) {
  const result = await operationsStore.resumeEmployee(employee.id, props.integratedMode ? employee.storeId : "");

  if (result?.ok === false) {
    ui.error(result.message);
    return;
  }

  ui.success("Consultor retomado.");
}
</script>

<template>
  <footer class="employee-strip" data-testid="operation-consultant-strip">
    <!--<div class="employee-strip__header">
      <strong class="employee-strip__title">Consultores</strong>
    </div>-->
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
          <span class="employee__name">{{ displayName(employee) }}</span>
          <span v-if="integratedMode && employee.storeName" class="employee__store">{{ employee.storeName }}</span>
          <span class="employee__status">{{ statusLabel(employee.id) }}</span>
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
            @click="addToQueue(employee)"
          >
            <span class="material-icons-round">login</span>
          </button>
          <button
            class="employee__action employee__action--secondary"
            type="button"
            title="Direcionar para tarefa"
            :data-testid="`operation-assign-task-${employee.id}`"
            @click="assignTask(employee)"
          >
            <span class="material-icons-round">assignment</span>
          </button>
          <button
            class="employee__action employee__action--secondary"
            type="button"
            title="Pausar"
            :data-testid="`operation-pause-${employee.id}`"
            @click="pauseEmployee(employee)"
          >
            <span class="material-icons-round">pause</span>
          </button>
        </div>

        <div v-else-if="statusFor(employee.id) === 'queue'" class="employee__actions">
          <button
            class="employee__action employee__action--secondary"
            type="button"
            title="Direcionar para tarefa"
            :data-testid="`operation-assign-task-${employee.id}`"
            @click="assignTask(employee)"
          >
            <span class="material-icons-round">assignment</span>
          </button>
          <button
            class="employee__action employee__action--secondary"
            type="button"
            title="Pausar"
            :data-testid="`operation-pause-${employee.id}`"
            @click="pauseEmployee(employee)"
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
            @click="resumeEmployee(employee)"
          >
            <span class="material-icons-round">play_arrow</span>
          </button>
        </div>
      </div>
    </div>

    <OperationPauseReasonDialog
      :open="Boolean(pauseDialogEmployee)"
      :employee="pauseDialogEmployee"
      :options="pauseReasonOptions"
      :pending="pausePending"
      @close="closePauseDialog"
      @confirm="confirmPauseEmployee"
    />
  </footer>
</template>
