<script setup>
import { computed, ref, watch } from "vue";
import { buildNickname } from "~/domain/utils/person-display";
import OperationProductPicker from "~/features/operation/components/OperationProductPicker.vue";

const props = defineProps({
  open: {
    type: Boolean,
    default: false
  },
  employee: {
    type: Object,
    default: null
  },
  options: {
    type: Array,
    default: () => []
  },
  pending: {
    type: Boolean,
    default: false
  }
});

const emit = defineEmits(["close", "confirm"]);
const selectedItems = ref([]);

const pickerOptions = computed(() =>
  (props.options || []).map((option) => ({
    id: String(option?.id || "").trim(),
    label: String(option?.label || "").trim()
  })).filter((option) => option.id && option.label)
);
const selectedReason = computed(() => selectedItems.value[0] || null);

watch(
  () => [props.open, props.employee?.id, pickerOptions.value.map((option) => option.id).join(",")],
  () => {
    selectedItems.value = [];
  },
  { immediate: true }
);

function closeDialog() {
  if (props.pending) {
    return;
  }

  emit("close");
}

function submitDialog() {
  if (!selectedReason.value?.label || props.pending) {
    return;
  }

  emit("confirm", selectedReason.value.label);
}

function displayName(employee) {
  return buildNickname(employee?.name || "");
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      class="ui-dialog-backdrop"
      data-testid="operation-pause-dialog-backdrop"
      @click.self="closeDialog"
    >
      <div
        class="ui-dialog operation-pause-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="operation-pause-dialog-title"
        data-testid="operation-pause-dialog"
      >
        <header class="ui-dialog__header">
          <h2 id="operation-pause-dialog-title" class="ui-dialog__title">Pausar consultor</h2>
          <button
            class="ui-dialog__close"
            type="button"
            aria-label="Fechar"
            data-testid="operation-pause-dialog-close"
            @click="closeDialog"
          >
            X
          </button>
        </header>

        <form class="ui-dialog__body" @submit.prevent="submitDialog">
          <p class="ui-dialog__message">
            Escolha o motivo da pausa para registrar no painel{{ employee?.name ? ` de ${displayName(employee)}` : "" }}.
          </p>

          <OperationProductPicker
            label="Motivo da pausa"
            :options="pickerOptions"
            :selected-items="selectedItems"
            :multiple="false"
            trigger-label="Selecionar motivo"
            search-placeholder="Busque e selecione o motivo da pausa"
            empty-selected-label="Nenhum motivo selecionado"
            testid-prefix="operation-pause-reason"
            @update:selected-items="selectedItems = $event"
          />

          <div class="ui-dialog__actions">
            <button
              class="column-action column-action--secondary"
              type="button"
              data-testid="operation-pause-dialog-cancel"
              :disabled="pending"
              @click="closeDialog"
            >
              Cancelar
            </button>
            <button
              class="column-action column-action--primary"
              type="submit"
              data-testid="operation-pause-dialog-confirm"
              :disabled="pending || !selectedReason"
            >
              {{ pending ? "Salvando..." : "Pausar" }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.operation-pause-dialog {
  width: min(560px, calc(100vw - 32px));
}
</style>
