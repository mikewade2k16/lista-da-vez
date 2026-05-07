<script setup lang="ts">
const props = withDefaults(defineProps<{
  storeCode?: string;
  lastRun?: Record<string, any> | null;
  lastImportedFile?: Record<string, any> | null;
  syncing?: boolean;
  canSync?: boolean;
}>(), {
  storeCode: "",
  lastRun: null,
  lastImportedFile: null,
  syncing: false,
  canSync: false
});

const emit = defineEmits<{
  (e: "sync"): void;
  (e: "backfill"): void;
  (e: "refresh"): void;
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
  <section class="erp-sync-status">
    <header class="erp-sync-status__header">
      <div>
        <h3 class="erp-sync-status__title">Sincronização CSV ERP</h3>
        <p class="erp-sync-status__text">
          Dispara a leitura dos CSVs brutos do escopo ERP completo do sistema e acompanha o último ciclo conhecido pelo módulo.
        </p>
      </div>

      <div class="erp-sync-status__actions">
        <button class="erp-sync-status__button erp-sync-status__button--ghost" type="button" :disabled="syncing" @click="emit('refresh')">
          Atualizar
        </button>
        <button class="erp-sync-status__button erp-sync-status__button--ghost" type="button" :disabled="!canSync || syncing" @click="emit('backfill')">
          {{ syncing ? 'Processando...' : 'Backfill retroativo' }}
        </button>
        <button class="erp-sync-status__button erp-sync-status__button--primary" type="button" :disabled="!canSync || syncing" @click="emit('sync')">
          {{ syncing ? 'Sincronizando...' : 'Rodar agora' }}
        </button>
      </div>
    </header>

    <div class="erp-sync-status__grid">
      <article class="erp-sync-status__card">
        <span class="erp-sync-status__label">Último status</span>
        <strong class="erp-sync-status__value">{{ lastRun?.status || 'sem execução' }}</strong>
        <small>{{ formatDateTime(lastRun?.finishedAt || lastRun?.startedAt) }}</small>
      </article>
      <article class="erp-sync-status__card">
        <span class="erp-sync-status__label">Arquivos</span>
        <strong class="erp-sync-status__value">{{ formatNumber(lastRun?.filesImported) }}</strong>
        <small>{{ formatNumber(lastRun?.filesSkipped) }} pulados</small>
      </article>
      <article class="erp-sync-status__card">
        <span class="erp-sync-status__label">Linhas importadas</span>
        <strong class="erp-sync-status__value">{{ formatNumber(lastRun?.rowsImported) }}</strong>
        <small>{{ formatNumber(lastRun?.rowsRead) }} lidas</small>
      </article>
      <article class="erp-sync-status__card">
        <span class="erp-sync-status__label">Último arquivo</span>
        <strong class="erp-sync-status__value erp-sync-status__value--small">{{ lastImportedFile?.sourceName || '-' }}</strong>
        <small>{{ formatDateTime(lastImportedFile?.importedAt) }}</small>
      </article>
    </div>
  </section>
</template>

<style scoped>
.erp-sync-status {
  display: grid;
  gap: 0.9rem;
}

.erp-sync-status__header {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
  flex-wrap: wrap;
}

.erp-sync-status__title {
  margin: 0;
  color: var(--text-main);
  font-size: 1.15rem;
}

.erp-sync-status__text {
  margin: 0.35rem 0 0;
  max-width: 48rem;
  color: var(--text-muted);
  line-height: 1.5;
}

.erp-sync-status__actions {
  display: flex;
  gap: 0.55rem;
  flex-wrap: wrap;
}

.erp-sync-status__button {
  min-height: 2.5rem;
  padding: 0 0.95rem;
  border-radius: 0.8rem;
  border: 1px solid var(--line-soft);
  background: rgba(17, 24, 39, 0.92);
  color: var(--text-main);
  font-weight: 600;
}

.erp-sync-status__button--primary {
  border-color: rgba(83, 198, 160, 0.32);
  background: linear-gradient(135deg, rgba(13, 102, 87, 0.92), rgba(14, 73, 67, 0.94));
}

.erp-sync-status__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 0.8rem;
}

.erp-sync-status__card {
  display: grid;
  gap: 0.25rem;
  padding: 0.95rem 1rem;
  border-radius: 1rem;
  border: 1px solid var(--line-soft);
  background: rgba(15, 23, 36, 0.86);
}

.erp-sync-status__label,
.erp-sync-status__card small {
  color: var(--text-muted);
}

.erp-sync-status__value {
  color: var(--text-main);
  font-size: 1.35rem;
  font-weight: 700;
}

.erp-sync-status__value--small {
  font-size: 0.95rem;
  line-height: 1.4;
  word-break: break-word;
}
@media (max-width: 720px) {
  .erp-sync-status__actions {
    width: 100%;
  }

  .erp-sync-status__button {
    flex: 1 1 12rem;
  }
}
</style>