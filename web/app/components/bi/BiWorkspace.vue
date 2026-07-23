<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Activity, BrainCircuit, KeyRound, RefreshCw, ShieldCheck, X } from 'lucide-vue-next'

import BiApiCatalog from '~/components/bi/BiApiCatalog.vue'
import BiDatasetTable from '~/components/bi/BiDatasetTable.vue'
import BiIntelligencePanel from '~/components/bi/BiIntelligencePanel.vue'
import BiManualConnection from '~/components/bi/BiManualConnection.vue'
import AdminPageHeader from '../../../layers/core/components/admin/AdminPageHeader.vue'
import SettingsTabs from '~/components/settings/SettingsTabs.vue'
import { useBiStore } from '~/stores/bi'
import { useUiStore } from '~/stores/ui'

const biStore = useBiStore()
const ui = useUiStore()
const {
  overview,
  loading,
  inventoryLoading,
  error,
  metrics,
  sources,
  insights,
  sections,
  tables,
  hasManualToken,
} = storeToRefs(biStore)

const activeTab = ref('entidades')
const diagnosticsOpen = ref(false)

const tabs = computed(() => [
  { id: 'entidades', label: 'Entidades', icon: 'database' },
  { id: 'visao', label: 'Visão', icon: 'dashboard' },
  ...tables.value.map((table) => ({
    id: table.key,
    label: table.label,
    icon:
      table.key === 'items'
        ? 'inventory_2'
        : table.key === 'inventario'
          ? 'inventory'
          : 'table_chart',
  })),
  { id: 'inteligencia', label: 'Inteligência', icon: 'psychology' },
])

const activeTable = computed(
  () => tables.value.find((table) => table.key === activeTab.value) || null,
)
const activeSource = computed(
  () => sources.value.find((source) => source.key === activeTab.value) || null,
)
const activeTableLoading = computed(
  () => loading.value || (activeTable.value?.key === 'inventario' && inventoryLoading.value),
)
const isDataView = computed(() => activeTab.value !== 'entidades')

const generatedAtLabel = computed(() => {
  const raw = overview.value?.generatedAt
  if (!raw) return 'Ainda não atualizado'

  return new Intl.DateTimeFormat('pt-BR', {
    dateStyle: 'short',
    timeStyle: 'short',
  }).format(new Date(raw))
})

async function refresh() {
  const response = await biStore.refreshOverview()
  if (!response.ok) {
    ui.error(response.message || 'Não foi possível carregar o BI.')
    return
  }

  const inventoryPending = Boolean(
    response.data?.sources?.find((source) => source.key === 'inventario')?.pending,
  )
  ui.success(
    inventoryPending
      ? 'Visão rápida atualizada. O inventário permanece sob demanda para evitar carga aberta.'
      : 'BI Pérola atualizado.',
  )
}

watch(
  tables,
  () => {
    if (['entidades', 'visao', 'inteligencia'].includes(activeTab.value)) return
    if (!activeTable.value) activeTab.value = 'entidades'
  },
  { deep: true },
)
</script>

<template>
  <section class="admin-panel bi-panel" data-testid="bi-panel">
    <header class="bi-panel__header">
      <AdminPageHeader
        eyebrow="BI"
        title="Business Intelligence"
        description="Consulte entidades e indicadores com dados carregados pela integração protegida no backend."
      />

      <div class="bi-panel__actions">
        <span class="bi-panel__auth" :data-manual="hasManualToken">
          <ShieldCheck :size="14" aria-hidden="true" />
          {{ hasManualToken ? 'Sessão manual ativa' : 'Autenticação automática' }}
        </span>

        <button
          class="bi-panel__button bi-panel__button--quiet"
          type="button"
          :aria-expanded="diagnosticsOpen"
          aria-controls="bi-connection-diagnostics"
          @click="diagnosticsOpen = !diagnosticsOpen"
        >
          <component :is="diagnosticsOpen ? X : KeyRound" :size="15" aria-hidden="true" />
          {{ diagnosticsOpen ? 'Fechar diagnóstico' : 'Diagnóstico de conexão' }}
        </button>

        <button
          v-if="isDataView"
          class="bi-panel__button bi-panel__button--primary"
          type="button"
          :disabled="loading"
          @click="refresh"
        >
          <RefreshCw :size="15" aria-hidden="true" />
          {{ loading ? 'Atualizando...' : 'Atualizar dados' }}
        </button>
      </div>
    </header>

    <div v-if="diagnosticsOpen" id="bi-connection-diagnostics">
      <BiManualConnection />
    </div>

    <p v-if="error && isDataView" class="bi-panel__error">{{ error }}</p>

    <SettingsTabs :tabs="tabs" :active-tab="activeTab" @update:active-tab="activeTab = $event" />

    <BiApiCatalog v-if="activeTab === 'entidades'" />

    <template v-else>
      <section v-if="metrics.length" class="bi-panel__metrics" aria-label="Resumo Pérola BI">
        <article
          v-for="metric in metrics"
          :key="metric.key"
          class="bi-panel__metric"
          :data-tone="metric.tone"
        >
          <span>{{ metric.label }}</span>
          <strong>{{ metric.value }}</strong>
          <small>{{ metric.detail }}</small>
        </article>
      </section>

      <div v-if="overview?.generatedAt" class="bi-panel__meta">
        <span>CNPJ {{ overview?.cnpjEmpresa || '-' }}</span>
        <span>Atualizado em {{ generatedAtLabel }}</span>
      </div>

      <section v-if="activeTab === 'visao'" class="bi-panel__overview">
        <div class="bi-panel__insights">
          <article
            v-for="insight in insights"
            :key="insight.title"
            class="bi-panel__insight"
            :data-tone="insight.tone"
          >
            <BrainCircuit :size="18" aria-hidden="true" />
            <div>
              <h3>{{ insight.title }}</h3>
              <p>{{ insight.body }}</p>
            </div>
          </article>
        </div>

        <aside class="bi-panel__sources">
          <h3>Fontes consultadas</h3>
          <article v-for="source in sources" :key="source.key" class="bi-panel__source">
            <Activity :size="16" aria-hidden="true" />
            <div>
              <strong>{{ source.label }}</strong>
              <span>
                {{ source.pending ? 'Em carga' : source.ok ? 'OK' : 'Falha' }} ·
                {{ source.fetched.toLocaleString('pt-BR') }}
                <template v-if="source.total > source.fetched">
                  de {{ source.total.toLocaleString('pt-BR') }}
                </template>
                registros · {{ source.durationMs }} ms
              </span>
              <small v-if="source.truncated && !source.error">
                Amostra rápida carregada para manter a tela ágil.
              </small>
              <small v-if="source.error">{{ source.error }}</small>
            </div>
          </article>
        </aside>
      </section>

      <section v-else-if="activeTab === 'inteligencia'" class="bi-panel__intelligence">
        <BiIntelligencePanel :sections="sections" :inventory-loading="inventoryLoading" />
      </section>

      <BiDatasetTable
        v-else-if="activeTable"
        :table="activeTable"
        :source="activeSource"
        :loading="activeTableLoading"
        @refresh="refresh"
      />
    </template>
  </section>
</template>

<style scoped>
.bi-panel {
  display: grid;
  gap: 16px;
  align-content: start;
}

.bi-panel__header,
.bi-panel__actions,
.bi-panel__meta,
.bi-panel__overview {
  display: flex;
  gap: 16px;
}

.bi-panel__header {
  align-items: flex-start;
  justify-content: space-between;
}

.bi-panel__actions {
  align-items: center;
  justify-content: flex-end;
  flex-wrap: wrap;
}

.bi-panel__auth,
.bi-panel__button {
  display: inline-flex;
  gap: 7px;
  align-items: center;
  justify-content: center;
  min-height: 38px;
  border-radius: var(--radius-sm);
}

.bi-panel__auth {
  padding: 0 10px;
  color: var(--accent-success);
  font-size: 0.74rem;
  font-weight: 750;
  border: 1px solid color-mix(in srgb, var(--accent-success) 32%, var(--line-soft));
  background: color-mix(in srgb, var(--accent-success) 6%, var(--bg-panel));
}

.bi-panel__auth[data-manual='true'] {
  color: var(--accent-warning);
  border-color: color-mix(in srgb, var(--accent-warning) 35%, var(--line-soft));
  background: color-mix(in srgb, var(--accent-warning) 7%, var(--bg-panel));
}

.bi-panel__button {
  padding: 0 12px;
  color: var(--text-main);
  font-weight: 750;
  cursor: pointer;
  border: 1px solid var(--line-soft);
  background: var(--bg-panel);
}

.bi-panel__button--quiet {
  color: var(--text-muted);
  font-size: 0.76rem;
}

.bi-panel__button--primary {
  border-color: color-mix(in srgb, var(--accent-success) 38%, var(--line-soft));
  background: color-mix(in srgb, var(--accent-success) 12%, var(--bg-panel));
}

.bi-panel__button:disabled {
  opacity: 0.6;
  cursor: wait;
}

.bi-panel__error {
  margin: 0;
  padding: 12px;
  color: rgb(var(--danger));
  border: 1px solid color-mix(in srgb, rgb(var(--danger)) 30%, var(--line-soft));
  border-radius: var(--radius-sm);
  background: color-mix(in srgb, rgb(var(--danger)) 7%, var(--bg-panel));
}

.bi-panel__metrics {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(168px, 1fr));
  gap: 12px;
}

.bi-panel__metric,
.bi-panel__insight,
.bi-panel__source {
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-md);
  background: var(--bg-panel);
  box-shadow: var(--shadow-card);
}

.bi-panel__metric {
  display: grid;
  gap: 4px;
  padding: 14px;
}

.bi-panel__metric span,
.bi-panel__metric small,
.bi-panel__meta,
.bi-panel__source span,
.bi-panel__source small,
.bi-panel__insight p {
  color: var(--text-muted);
}

.bi-panel__metric span {
  font-size: 0.72rem;
  letter-spacing: 0.07em;
  text-transform: uppercase;
}

.bi-panel__metric strong {
  color: var(--text-main);
  font-size: 1.35rem;
  font-variant-numeric: tabular-nums;
}

.bi-panel__metric[data-tone='success'] strong {
  color: var(--accent-success);
}

.bi-panel__metric[data-tone='warning'] strong {
  color: var(--accent-warning);
}

.bi-panel__meta {
  flex-wrap: wrap;
  justify-content: space-between;
  font-size: 0.8rem;
}

.bi-panel__overview {
  align-items: stretch;
}

.bi-panel__insights {
  display: grid;
  flex: 1 1 0;
  gap: 12px;
}

.bi-panel__insight,
.bi-panel__source {
  display: flex;
  gap: 12px;
  padding: 14px;
}

.bi-panel__insight h3,
.bi-panel__sources h3 {
  margin: 0;
  color: var(--text-main);
  font-size: 0.98rem;
}

.bi-panel__insight p {
  margin: 5px 0 0;
  line-height: 1.55;
}

.bi-panel__sources {
  display: grid;
  flex: 0 1 384px;
  gap: 10px;
  align-content: start;
  padding: 15px;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-md);
  background: var(--bg-muted);
}

.bi-panel__source {
  box-shadow: none;
}

.bi-panel__source strong,
.bi-panel__source small {
  display: block;
}

.bi-panel__source strong {
  color: var(--text-main);
}

.bi-panel__source small {
  margin-top: 3px;
}

@media (max-width: 980px) {
  .bi-panel__header,
  .bi-panel__overview {
    flex-direction: column;
  }

  .bi-panel__actions {
    justify-content: flex-start;
  }
}

@media (max-width: 620px) {
  .bi-panel__actions > * {
    flex: 1 1 100%;
  }
}
</style>
