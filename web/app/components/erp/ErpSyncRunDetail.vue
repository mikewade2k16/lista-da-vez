<script setup lang="ts">
const props = withDefaults(defineProps<{
  run?: Record<string, any> | null;
  lastImportedFile?: Record<string, any> | null;
}>(), {
  run: null,
  lastImportedFile: null
});

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
  <aside class="erp-sync-detail">
    <header class="erp-sync-detail__header">
      <div>
        <h3>Detalhe do run</h3>
        <p>{{ props.run?.id || 'Selecione um run para ver os detalhes.' }}</p>
      </div>
    </header>

    <div v-if="props.run" class="erp-sync-detail__grid">
      <article class="erp-sync-detail__card">
        <span>Tipo</span>
        <strong>{{ props.run.dataType || '-' }}</strong>
      </article>
      <article class="erp-sync-detail__card">
        <span>Status</span>
        <strong>{{ props.run.status || '-' }}</strong>
      </article>
      <article class="erp-sync-detail__card">
        <span>Arquivos</span>
        <strong>{{ formatNumber(props.run.filesImported) }}</strong>
        <small>{{ formatNumber(props.run.filesSkipped) }} pulados</small>
      </article>
      <article class="erp-sync-detail__card">
        <span>Linhas</span>
        <strong>{{ formatNumber(props.run.rowsImported) }}</strong>
        <small>{{ formatNumber(props.run.rowsRead) }} lidas</small>
      </article>
      <article class="erp-sync-detail__card">
        <span>Iniciado</span>
        <strong>{{ formatDateTime(props.run.startedAt) }}</strong>
      </article>
      <article class="erp-sync-detail__card">
        <span>Concluído</span>
        <strong>{{ formatDateTime(props.run.finishedAt) }}</strong>
      </article>
      <article class="erp-sync-detail__card erp-sync-detail__card--wide">
        <span>Último arquivo conhecido</span>
        <strong>{{ props.lastImportedFile?.sourceName || '-' }}</strong>
        <small>{{ formatDateTime(props.lastImportedFile?.importedAt) }}</small>
      </article>
      <article v-if="props.run.errorMessage" class="erp-sync-detail__card erp-sync-detail__card--wide">
        <span>Erro</span>
        <strong>{{ props.run.errorMessage }}</strong>
      </article>
    </div>
    <p v-else class="erp-sync-detail__empty">Nenhum run selecionado.</p>
  </aside>
</template>

<style scoped>
.erp-sync-detail {
  display: grid;
  gap: 0.75rem;
  padding: 1rem;
  border-radius: 1rem;
  border: 1px solid var(--line-soft);
  background: rgba(15, 23, 36, 0.86);
}

.erp-sync-detail__header h3,
.erp-sync-detail__header p,
.erp-sync-detail__empty {
  margin: 0;
}

.erp-sync-detail__header h3 {
  color: var(--text-main);
}

.erp-sync-detail__header p,
.erp-sync-detail__empty {
  color: var(--text-muted);
}

.erp-sync-detail__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(170px, 1fr));
  gap: 0.7rem;
}

.erp-sync-detail__card {
  display: grid;
  gap: 0.2rem;
  padding: 0.85rem 0.95rem;
  border-radius: 0.9rem;
  border: 1px solid var(--line-soft);
  background: rgba(17, 24, 39, 0.92);
}

.erp-sync-detail__card span,
.erp-sync-detail__card small {
  color: var(--text-muted);
}

.erp-sync-detail__card strong {
  color: var(--text-main);
}

.erp-sync-detail__card--wide {
  grid-column: 1 / -1;
}
</style>