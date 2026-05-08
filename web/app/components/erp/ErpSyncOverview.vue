<script setup lang="ts">
import { computed } from "vue";

const props = withDefaults(defineProps<{
  overview?: Record<string, any> | null;
  loading?: boolean;
}>(), {
  overview: null,
  loading: false
});

const entityLabels: Record<string, string> = {
  item: "Produtos",
  customer: "Clientes",
  employee: "Funcionários",
  order: "Pedidos",
  ordercanceled: "Cancelados"
};

const automaticLabel = computed(() => props.overview?.automatic?.enabled ? "Sim" : "Não");
const automaticDetail = computed(() => {
  if (!props.overview?.automatic?.enabled) {
    return "O agendamento automático está desligado neste ambiente.";
  }
  const hour = Number(props.overview?.automatic?.hourUtc ?? 0);
  const interval = String(props.overview?.automatic?.interval || "").trim();
  return `Rodando com janela ${interval || "configurada"} e referência ${hour.toString().padStart(2, "0")}:00 UTC.`;
});
const importStatusLabel = computed(() => props.overview?.fullyImported ? "Sim" : "Ainda não");
const nextSteps = computed(() => {
  const items: string[] = [];
  if (!props.overview?.automatic?.enabled) {
    items.push("Ligar o scheduler automático no ambiente operacional.");
  }
  const pending = Number(props.overview?.totals?.pendingFiles || 0);
  if (pending > 0) {
    items.push(`Importar os ${pending.toLocaleString("pt-BR")} CSVs que ainda faltam do FTP atual.`);
  }
  items.push("Fechar alertas operacionais e ações de reprocessamento/abort para suporte." );
  return items;
});

function formatNumber(value?: number | null) {
  return Number(value || 0).toLocaleString("pt-BR");
}

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

function formatSize(value?: number | null) {
  const bytes = Number(value || 0);
  if (!bytes) {
    return "-";
  }
  if (bytes < 1024) {
    return `${bytes} B`;
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`;
  }
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function entityLabel(dataType?: string) {
  return entityLabels[String(dataType || "").trim()] || String(dataType || "-");
}
</script>

<template>
  <section class="erp-sync-overview">
    <header class="erp-sync-overview__header">
      <div>
        <h3 class="erp-sync-overview__title">Resumo operacional do ERP</h3>
        <p class="erp-sync-overview__text">
          Esta área responde se o FTP já foi coberto, se a rotina está automática e o que ainda falta para encerrar a operação no escopo completo do sistema.
        </p>
      </div>

      <a
        v-if="overview?.agentDocUrl"
        class="erp-sync-overview__agent-link"
        :href="overview.agentDocUrl"
        target="_blank"
        rel="noreferrer"
      >
        Abrir AGENT técnico
      </a>
    </header>

    <div v-if="loading && !overview" class="erp-sync-overview__empty">Carregando overview do ERP...</div>

    <template v-else-if="overview">
      <div class="erp-sync-overview__hero-grid">
        <article class="erp-sync-overview__hero-card">
          <span>Puxamos tudo do FTP?</span>
          <strong>{{ importStatusLabel }}</strong>
          <small>{{ formatNumber(overview.totals?.importedFiles) }} de {{ formatNumber(overview.totals?.totalFiles) }} CSVs do lote atual (raiz do FTP) já estão no banco.</small>
        </article>

        <article class="erp-sync-overview__hero-card">
          <span>Automatizado agora?</span>
          <strong>{{ automaticLabel }}</strong>
          <small>{{ automaticDetail }}</small>
        </article>

        <article class="erp-sync-overview__hero-card">
          <span>CSVs faltando</span>
          <strong>{{ formatNumber(overview.totals?.pendingFiles) }}</strong>
          <small>Origem atual: {{ overview.sourceKind }} em {{ overview.sourcePath || '-' }}</small>
        </article>

        <article class="erp-sync-overview__hero-card">
          <span>Pesquisa no painel</span>
          <strong>Disponível</strong>
          <small>Sim: além do lote atual do FTP, registros históricos já carregados também aparecem nas abas do ERP.</small>
        </article>
      </div>

      <div class="erp-sync-overview__grid">
        <section class="erp-sync-overview__panel">
          <header class="erp-sync-overview__panel-header">
            <h4>As 5 entidades do ERP</h4>
            <span>Sistema completo</span>
          </header>

          <p class="erp-sync-overview__table-note">
            Este resumo não segue a subloja operacional do topo. “Linhas no banco” mostra tudo que já foi carregado antes, inclusive cargas legadas por markdown/CSV. “CSV no FTP” mostra só o conjunto remoto atual que ainda precisamos cobrir.
          </p>

          <div class="erp-sync-overview__entities-table">
            <div class="erp-sync-overview__entities-head">
              <span>Entidade</span>
              <span>CSV no FTP</span>
              <span>Já puxados</span>
              <span>Faltando</span>
              <span>Linhas no banco</span>
              <span>Pesquisáveis</span>
            </div>

            <div v-for="entity in overview.entities || []" :key="entity.dataType" class="erp-sync-overview__entities-row">
              <strong>{{ entityLabel(entity.dataType) }}</strong>
              <span>{{ formatNumber(entity.remoteFilesTotal) }}</span>
              <span>{{ formatNumber(entity.importedFiles) }}</span>
              <span>{{ formatNumber(entity.pendingFiles) }}</span>
              <span>{{ formatNumber(entity.rowsInBank) }}</span>
              <span>
                {{ formatNumber(entity.searchableRows) }}
                <small v-if="entity.dataType === 'item'">catálogo atual</small>
              </span>
            </div>
          </div>
        </section>

        <section class="erp-sync-overview__panel">
          <header class="erp-sync-overview__panel-header">
            <h4>O que falta para terminar</h4>
            <span>{{ nextSteps.length }} ponto(s)</span>
          </header>

          <ul class="erp-sync-overview__next-steps">
            <li v-for="item in nextSteps" :key="item">{{ item }}</li>
          </ul>

          <div class="erp-sync-overview__technical-note">
            <strong>Referência técnica:</strong>
            <span>{{ overview.agentDocPath || '-' }}</span>
          </div>
        </section>
      </div>

      <section class="erp-sync-overview__panel">
        <header class="erp-sync-overview__panel-header">
          <h4>CSVs que ainda faltam entrar no banco</h4>
          <span>{{ formatNumber(overview.missingFiles?.length) }} arquivo(s)</span>
        </header>

        <div v-if="overview.missingFiles?.length" class="erp-sync-overview__missing-list">
          <article v-for="file in overview.missingFiles" :key="file.sourceName" class="erp-sync-overview__missing-item">
            <div>
              <strong>{{ file.sourceName }}</strong>
              <small>{{ entityLabel(file.dataType) }} • referência {{ formatDateTime(file.dataReference) }}</small>
            </div>
            <div class="erp-sync-overview__missing-meta">
              <span>{{ formatSize(file.sizeBytes) }}</span>
              <span>{{ file.status }}</span>
              <span>mod {{ formatDateTime(file.modTime) }}</span>
            </div>
          </article>
        </div>
        <p v-else class="erp-sync-overview__empty">Nenhum CSV pendente: o FTP atual já está totalmente coberto no banco.</p>
      </section>
    </template>

    <p v-else class="erp-sync-overview__empty">Overview indisponível para o escopo ERP do sistema.</p>
  </section>
</template>

<style scoped>
.erp-sync-overview {
  display: grid;
  gap: 1rem;
}

.erp-sync-overview__header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 1rem;
  flex-wrap: wrap;
}

.erp-sync-overview__title {
  margin: 0;
  color: var(--text-main);
  font-size: 1.15rem;
}

.erp-sync-overview__text,
.erp-sync-overview__empty {
  margin: 0.35rem 0 0;
  color: var(--text-muted);
  line-height: 1.5;
}

.erp-sync-overview__agent-link {
  display: inline-flex;
  align-items: center;
  min-height: 2.5rem;
  padding: 0 0.95rem;
  border-radius: 0.8rem;
  border: 1px solid rgba(98, 129, 255, 0.25);
  background: rgba(17, 24, 39, 0.92);
  color: var(--text-main);
  text-decoration: none;
  font-weight: 600;
}

.erp-sync-overview__hero-grid,
.erp-sync-overview__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 0.85rem;
}

.erp-sync-overview__hero-card,
.erp-sync-overview__panel {
  display: grid;
  gap: 0.35rem;
  padding: 1rem;
  border-radius: 1rem;
  border: 1px solid var(--line-soft);
  background: rgba(15, 23, 36, 0.86);
}

.erp-sync-overview__hero-card span,
.erp-sync-overview__hero-card small,
.erp-sync-overview__technical-note span,
.erp-sync-overview__panel-header span {
  color: var(--text-muted);
}

.erp-sync-overview__hero-card strong {
  color: var(--text-main);
  font-size: 1.55rem;
}

.erp-sync-overview__panel-header {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.erp-sync-overview__panel-header h4,
.erp-sync-overview__technical-note strong {
  margin: 0;
  color: var(--text-main);
}

.erp-sync-overview__table-note {
  margin: 0;
  color: var(--text-muted);
  line-height: 1.5;
}

.erp-sync-overview__entities-table {
  display: grid;
  gap: 0.55rem;
}

.erp-sync-overview__entities-head,
.erp-sync-overview__entities-row {
  display: grid;
  grid-template-columns: minmax(120px, 1.3fr) repeat(5, minmax(90px, 1fr));
  gap: 0.55rem;
  align-items: center;
}

.erp-sync-overview__entities-head {
  color: var(--text-muted);
  font-size: 0.84rem;
}

.erp-sync-overview__entities-row {
  padding: 0.8rem 0.85rem;
  border-radius: 0.85rem;
  border: 1px solid var(--line-soft);
  background: rgba(17, 24, 39, 0.92);
  color: var(--text-main);
}

.erp-sync-overview__entities-row strong,
.erp-sync-overview__missing-item strong {
  color: var(--text-main);
}

.erp-sync-overview__entities-row small,
.erp-sync-overview__missing-item small {
  display: block;
  color: var(--text-muted);
}

.erp-sync-overview__next-steps {
  margin: 0;
  padding-left: 1.1rem;
  color: var(--text-main);
}

.erp-sync-overview__next-steps li + li {
  margin-top: 0.45rem;
}

.erp-sync-overview__technical-note {
  display: grid;
  gap: 0.2rem;
  padding-top: 0.65rem;
  border-top: 1px solid var(--line-soft);
}

.erp-sync-overview__missing-list {
  display: grid;
  gap: 0.55rem;
  max-height: 28rem;
  overflow: auto;
}

.erp-sync-overview__missing-item {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
  align-items: flex-start;
  padding: 0.85rem 0.9rem;
  border-radius: 0.85rem;
  border: 1px solid var(--line-soft);
  background: rgba(17, 24, 39, 0.92);
}

.erp-sync-overview__missing-meta {
  display: grid;
  gap: 0.15rem;
  min-width: 10rem;
  color: var(--text-muted);
  text-align: right;
}

@media (max-width: 920px) {
  .erp-sync-overview__entities-head,
  .erp-sync-overview__entities-row,
  .erp-sync-overview__missing-item {
    grid-template-columns: 1fr;
  }

  .erp-sync-overview__missing-item {
    display: grid;
  }

  .erp-sync-overview__missing-meta {
    min-width: 0;
    text-align: left;
  }
}
</style>