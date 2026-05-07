<script setup lang="ts">
const props = withDefaults(defineProps<{
  runs?: Array<Record<string, any>>;
  selectedRunId?: string;
}>(), {
  runs: () => [],
  selectedRunId: ""
});

const emit = defineEmits<{
  (e: "select", runId: string): void;
}>();

function formatDateTime(value?: string | null) {
  const normalized = String(value || "").trim();
  if (!normalized) {
    return "-";
  }
  const parsed = new Date(normalized);
  if (Number.isNaN(parsed.getTime())) {
    return normalized;
  }
  return parsed.toLocaleString("pt-BR", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit"
  });
}

function formatNumber(value?: number | null) {
  return Number(value || 0).toLocaleString("pt-BR");
}
</script>

<template>
  <section class="erp-sync-runs">
    <header class="erp-sync-runs__header">
      <h3>Últimos runs conhecidos</h3>
      <span>{{ props.runs.length }} item(ns)</span>
    </header>

    <div v-if="props.runs.length" class="erp-sync-runs__table">
      <button
        v-for="run in props.runs"
        :key="run.id"
        class="erp-sync-runs__row"
        :class="{ 'erp-sync-runs__row--active': run.id === props.selectedRunId }"
        type="button"
        @click="emit('select', run.id)"
      >
        <strong>{{ run.dataType || 'erp' }}</strong>
        <span>{{ run.status || 'desconhecido' }}</span>
        <span>{{ formatNumber(run.filesImported) }} arquivos</span>
        <span>{{ formatNumber(run.rowsImported) }} linhas</span>
        <span>{{ formatDateTime(run.finishedAt || run.startedAt) }}</span>
      </button>
    </div>
    <p v-else class="erp-sync-runs__empty">Nenhum run disponível no status atual do módulo.</p>
  </section>
</template>

<style scoped>
.erp-sync-runs {
  display: grid;
  gap: 0.75rem;
  padding: 1rem;
  border-radius: 1rem;
  border: 1px solid var(--line-soft);
  background: rgba(15, 23, 36, 0.86);
}

.erp-sync-runs__header {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.erp-sync-runs__header h3,
.erp-sync-runs__header span {
  margin: 0;
  color: var(--text-main);
}

.erp-sync-runs__table {
  display: grid;
  gap: 0.55rem;
}

.erp-sync-runs__row {
  display: grid;
  grid-template-columns: minmax(90px, 120px) minmax(120px, 1fr) minmax(110px, 130px) minmax(110px, 130px) minmax(160px, 190px);
  gap: 0.6rem;
  align-items: center;
  padding: 0.8rem 0.9rem;
  border-radius: 0.85rem;
  border: 1px solid var(--line-soft);
  background: rgba(17, 24, 39, 0.92);
  color: var(--text-muted);
  text-align: left;
}

.erp-sync-runs__row strong {
  color: var(--text-main);
  text-transform: capitalize;
}

.erp-sync-runs__row--active {
  border-color: rgba(98, 129, 255, 0.35);
  background: rgba(26, 37, 58, 0.96);
}

.erp-sync-runs__empty {
  margin: 0;
  color: var(--text-muted);
}

@media (max-width: 920px) {
  .erp-sync-runs__row {
    grid-template-columns: 1fr;
  }
}
</style>