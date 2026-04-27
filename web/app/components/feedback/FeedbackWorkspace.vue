<script setup lang="ts">
import { onMounted, ref, computed } from "vue";
import { useFeedbackStore } from "~/stores/feedback";
import { useUiStore } from "~/stores/ui";
import AppEntityGrid from "~/components/ui/AppEntityGrid.vue";
import AppDetailDialog from "~/components/ui/AppDetailDialog.vue";
import AppSelectField from "~/components/ui/AppSelectField.vue";

const feedbackStore = useFeedbackStore();
const ui = useUiStore();

const selectedFeedback = ref(null);
const detailOpen = ref(false);
const editingNotes = ref("");
const editingStatus = ref("");
const selectedKindFilter = ref("");
const selectedStatusFilter = ref("");
const searchValue = ref("");
const saving = ref(false);

const kindOptions = [
  { value: "", label: "Todos" },
  { value: "suggestion", label: "Sugestão" },
  { value: "question", label: "Dúvida" },
  { value: "problem", label: "Problema" }
];

const statusOptions = [
  { value: "", label: "Todos" },
  { value: "open", label: "Aberto" },
  { value: "in_progress", label: "Em análise" },
  { value: "resolved", label: "Resolvido" },
  { value: "closed", label: "Fechado" }
];

const columns = [
  {
    id: "kind",
    label: "Tipo",
    width: "100px",
    align: "left"
  },
  {
    id: "subject",
    label: "Assunto",
    width: "300px",
    align: "left"
  },
  {
    id: "user_name",
    label: "Usuário",
    width: "150px",
    align: "left"
  },
  {
    id: "status",
    label: "Status",
    width: "120px",
    align: "center"
  },
  {
    id: "created_at",
    label: "Criado em",
    width: "150px",
    align: "left"
  }
];

const filteredFeedbacks = computed(() => {
  let result = feedbackStore.feedbacks;

  if (selectedKindFilter.value) {
    result = result.filter((f) => f.kind === selectedKindFilter.value);
  }

  if (selectedStatusFilter.value) {
    result = result.filter((f) => f.status === selectedStatusFilter.value);
  }

  if (searchValue.value) {
    const search = searchValue.value.toLowerCase();
    result = result.filter((f) =>
      f.subject.toLowerCase().includes(search) ||
      f.user_name.toLowerCase().includes(search) ||
      f.body.toLowerCase().includes(search)
    );
  }

  return result;
});

const kindLabel = (kind: string) => {
  const opt = kindOptions.find((o) => o.value === kind);
  return opt?.label || kind;
};

const statusLabel = (status: string) => {
  const opt = statusOptions.find((o) => o.value === status);
  return opt?.label || status;
};

const createdAtFormatted = (isoString: string) => {
  try {
    const date = new Date(isoString);
    return date.toLocaleDateString("pt-BR", {
      day: "2-digit",
      month: "2-digit",
      year: "numeric",
      hour: "2-digit",
      minute: "2-digit"
    });
  } catch {
    return isoString;
  }
};

function openDetail(feedback) {
  selectedFeedback.value = feedback;
  editingStatus.value = feedback.status;
  editingNotes.value = feedback.admin_note;
  detailOpen.value = true;
}

async function handleSaveDetail() {
  if (!selectedFeedback.value) return;

  const hasChanges =
    selectedFeedback.value.status !== editingStatus.value ||
    selectedFeedback.value.admin_note !== editingNotes.value;

  if (!hasChanges) {
    detailOpen.value = false;
    return;
  }

  saving.value = true;
  try {
    const result = await feedbackStore.updateFeedback(
      selectedFeedback.value.id,
      {
        status: editingStatus.value,
        admin_note: editingNotes.value
      }
    );

    if (result.ok) {
      ui.success("Feedback atualizado com sucesso!");
      detailOpen.value = false;
      await loadFeedbacks();
    } else {
      ui.error(result.message || "Erro ao atualizar feedback");
    }
  } finally {
    saving.value = false;
  }
}

async function loadFeedbacks() {
  const result = await feedbackStore.fetchFeedbacks({
    kind: selectedKindFilter.value,
    status: selectedStatusFilter.value
  });

  if (!result.ok) {
    ui.error(result.message || "Erro ao carregar feedbacks");
  }
}

onMounted(() => {
  loadFeedbacks();
});
</script>

<template>
  <div class="feedback-workspace">
    <AppEntityGrid
      :columns="columns"
      :rows="filteredFeedbacks"
      :row-key="(f) => f.id"
      :search-value="searchValue"
      :loading="feedbackStore.loading"
      empty-title="Nenhum feedback"
      empty-text="Não há feedbacks para exibir"
      search-placeholder="Buscar por assunto, usuário ou conteúdo..."
      storage-key="feedback-grid-columns"
      @update:search-value="searchValue = $event"
    >
      <template #toolbar-filters>
        <div class="feedback-workspace__filters">
          <div class="feedback-workspace__filter-item">
            <label class="feedback-workspace__filter-label">Tipo:</label>
            <AppSelectField
              v-model="selectedKindFilter"
              :options="kindOptions"
              compact
              @change="loadFeedbacks"
            />
          </div>
          <div class="feedback-workspace__filter-item">
            <label class="feedback-workspace__filter-label">Status:</label>
            <AppSelectField
              v-model="selectedStatusFilter"
              :options="statusOptions"
              compact
              @change="loadFeedbacks"
            />
          </div>
        </div>
      </template>

      <template #cell-kind="{ row }">
        <span class="feedback-workspace__kind-badge" :class="`feedback-workspace__kind-badge--${row.kind}`">
          {{ kindLabel(row.kind) }}
        </span>
      </template>

      <template #cell-status="{ row }">
        <span class="feedback-workspace__status-badge" :class="`feedback-workspace__status-badge--${row.status}`">
          {{ statusLabel(row.status) }}
        </span>
      </template>

      <template #cell-created_at="{ row }">
        <span class="feedback-workspace__text-muted">
          {{ createdAtFormatted(row.created_at) }}
        </span>
      </template>

      <template #cell-actions="{ row }">
        <button
          class="feedback-workspace__action-btn"
          @click="openDetail(row)"
          title="Abrir detalhes"
        >
          Detalhes
        </button>
      </template>
    </AppEntityGrid>

    <AppDetailDialog
      v-model="detailOpen"
      title="Detalhes do Feedback"
      :sections="[
        {
          id: 'info',
          title: 'Informações',
          description: 'Dados do feedback enviado',
          fields: [
            { label: 'Tipo', value: kindLabel(selectedFeedback?.kind) },
            { label: 'Assunto', value: selectedFeedback?.subject },
            { label: 'Usuário', value: selectedFeedback?.user_name },
            { label: 'Criado em', value: createdAtFormatted(selectedFeedback?.created_at) }
          ]
        }
      ]"
    >
      <div class="feedback-workspace__detail-body">
        <div class="feedback-workspace__detail-section">
          <h3 class="feedback-workspace__detail-title">Mensagem</h3>
          <p class="feedback-workspace__detail-message">{{ selectedFeedback?.body }}</p>
        </div>

        <div class="feedback-workspace__detail-section">
          <label class="feedback-workspace__detail-label">Status</label>
          <AppSelectField
            v-model="editingStatus"
            :options="statusOptions"
          />
        </div>

        <div class="feedback-workspace__detail-section">
          <label class="feedback-workspace__detail-label">Nota Interna</label>
          <textarea
            v-model="editingNotes"
            class="feedback-workspace__detail-textarea"
            placeholder="Adicione notas internas sobre este feedback"
            rows="4"
          ></textarea>
        </div>

        <div class="feedback-workspace__detail-actions">
          <button
            type="button"
            class="feedback-workspace__detail-btn feedback-workspace__detail-btn--secondary"
            @click="detailOpen = false"
            :disabled="saving"
          >
            Cancelar
          </button>
          <button
            type="button"
            class="feedback-workspace__detail-btn feedback-workspace__detail-btn--primary"
            @click="handleSaveDetail"
            :disabled="saving"
          >
            {{ saving ? "Salvando..." : "Salvar" }}
          </button>
        </div>
      </div>
    </AppDetailDialog>
  </div>
</template>

<style scoped>
.feedback-workspace {
  width: 100%;
}

.feedback-workspace__filters {
  display: flex;
  gap: 1rem;
  align-items: center;
  flex-wrap: wrap;
}

.feedback-workspace__filter-item {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.feedback-workspace__filter-label {
  font-size: 0.875rem;
  font-weight: 500;
  color: #6b7280;
}

.feedback-workspace__kind-badge,
.feedback-workspace__status-badge {
  display: inline-block;
  padding: 0.375rem 0.75rem;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
}

.feedback-workspace__kind-badge--suggestion {
  background-color: #dbeafe;
  color: #0c4a6e;
}

.feedback-workspace__kind-badge--question {
  background-color: #fce7f3;
  color: #831843;
}

.feedback-workspace__kind-badge--problem {
  background-color: #fee2e2;
  color: #7f1d1d;
}

.feedback-workspace__status-badge--open {
  background-color: #dbeafe;
  color: #0c4a6e;
}

.feedback-workspace__status-badge--in_progress {
  background-color: #fef3c7;
  color: #78350f;
}

.feedback-workspace__status-badge--resolved {
  background-color: #dcfce7;
  color: #15803d;
}

.feedback-workspace__status-badge--closed {
  background-color: #f3f4f6;
  color: #374151;
}

.feedback-workspace__text-muted {
  color: #6b7280;
  font-size: 0.875rem;
}

.feedback-workspace__action-btn {
  padding: 0.375rem 0.75rem;
  background-color: #3b82f6;
  color: white;
  border: none;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 500;
  cursor: pointer;
  transition: background-color 0.2s;
}

.feedback-workspace__action-btn:hover {
  background-color: #2563eb;
}

.feedback-workspace__detail-body {
  padding: 1.5rem 0;
}

.feedback-workspace__detail-section {
  margin-bottom: 1.5rem;
}

.feedback-workspace__detail-title {
  font-size: 1rem;
  font-weight: 600;
  color: #1f2937;
  margin: 0 0 0.5rem 0;
}

.feedback-workspace__detail-message {
  color: #374151;
  line-height: 1.6;
  white-space: pre-wrap;
  word-wrap: break-word;
  margin: 0;
}

.feedback-workspace__detail-label {
  display: block;
  font-size: 0.875rem;
  font-weight: 500;
  color: #374151;
  margin-bottom: 0.5rem;
}

.feedback-workspace__detail-textarea {
  width: 100%;
  padding: 0.625rem;
  border: 1px solid #d1d5db;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.875rem;
  color: #111827;
  resize: vertical;
}

.feedback-workspace__detail-textarea:focus {
  outline: none;
  border-color: #3b82f6;
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}

.feedback-workspace__detail-actions {
  display: flex;
  gap: 1rem;
  justify-content: flex-end;
  margin-top: 2rem;
  padding-top: 1.5rem;
  border-top: 1px solid #e5e7eb;
}

.feedback-workspace__detail-btn {
  padding: 0.625rem 1rem;
  border: none;
  border-radius: 4px;
  font-size: 0.875rem;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
}

.feedback-workspace__detail-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.feedback-workspace__detail-btn--primary {
  background-color: #3b82f6;
  color: white;
}

.feedback-workspace__detail-btn--primary:hover:not(:disabled) {
  background-color: #2563eb;
}

.feedback-workspace__detail-btn--secondary {
  background-color: #e5e7eb;
  color: #374151;
}

.feedback-workspace__detail-btn--secondary:hover:not(:disabled) {
  background-color: #d1d5db;
}
</style>
