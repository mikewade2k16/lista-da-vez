<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  formatCurrencyBRL,
  formatDurationMinutes,
  formatPercent,
} from '~/domain/utils/admin-metrics'
import { useRankingDetailsDrawer } from '~/composables/useRankingDetailsDrawer'
import RankingScoreBreakdown from './RankingScoreBreakdown.vue'
import RankingTable from './RankingTable.vue'

type Tab = 'overview' | 'breakdown' | 'alerts'

interface DrawerRow {
  rowKey: string
  consultantId: string
  consultantName: string
  storeId?: string
  storeName?: string
  soldValue: number
  attendances: number
  conversions: number
  conversionRate: number
  ticketAverage: number
  paScore: number
  qualityScore: number
  avgDurationMs: number
  queueJumpServices: number
  score360: number
}

interface DrawerAlert {
  consultantName: string
  type: string
  value: number
  threshold: number
}

const props = defineProps<{
  row: DrawerRow | null
  alerts?: DrawerAlert[]
  maxSold: number
  maxPa: number
  legacyRows?: DrawerRow[]
}>()

const drawer = useRankingDetailsDrawer()
const activeTab = ref<Tab>('overview')
const showLegacy = ref(false)

const open = computed({
  get: () => drawer.isOpen.value,
  set: (next: boolean) => {
    if (!next) drawer.close()
  },
})

const drawerMode = computed(() => drawer.mode.value)

watch(
  () => drawer.currentRowKey.value,
  () => {
    activeTab.value = 'overview'
    showLegacy.value = false
  },
)

const filteredAlerts = computed(() => {
  if (!props.row || !props.alerts) return []
  return props.alerts.filter((alert) => alert.consultantName === props.row?.consultantName)
})

const alertCount = computed(() => filteredAlerts.value.length)

const ALERT_LABELS: Record<string, (alert: DrawerAlert) => string> = {
  conversion: (a) => `Conversão ${a.value.toFixed(1)}% — abaixo do mínimo de ${a.threshold}%`,
  queueJump: (a) => `Fora da vez ${a.value.toFixed(1)}% — acima do máximo de ${a.threshold}%`,
  pa: (a) => `P.A. ${a.value.toFixed(2)} — abaixo do mínimo de ${a.threshold}`,
  ticket: (a) =>
    `Ticket ${formatCurrencyBRL(a.value)} — abaixo do mínimo de ${formatCurrencyBRL(a.threshold)}`,
}
</script>

<template>
  <USlideover
    v-model:open="open"
    :overlay="true"
    :modal="true"
    :dismissible="true"
    :ui="{ content: `ranking-drawer ranking-drawer--${drawerMode}` }"
  >
    <template #header>
      <div class="ranking-drawer__header">
        <div class="ranking-drawer__title-block">
          <strong class="ranking-drawer__title">
            {{ row?.consultantName || 'Selecione um item' }}
          </strong>
          <span v-if="row?.storeName" class="ranking-drawer__subtitle">{{ row.storeName }}</span>
        </div>
        <div class="ranking-drawer__header-actions">
          <button
            type="button"
            class="ranking-drawer__icon-btn"
            :title="drawerMode === 'fullscreen' ? 'Sair de página inteira' : 'Página inteira'"
            data-testid="ranking-drawer-toggle-fullscreen"
            @click="drawer.toggleFullscreen()"
          >
            <UIcon
              :name="drawerMode === 'fullscreen' ? 'i-lucide-minimize-2' : 'i-lucide-expand'"
              class="h-4 w-4"
            />
          </button>
          <button
            type="button"
            class="ranking-drawer__icon-btn"
            title="Fechar"
            data-testid="ranking-drawer-close"
            @click="drawer.close()"
          >
            <UIcon name="i-lucide-x" class="h-4 w-4" />
          </button>
        </div>
      </div>
      <nav class="ranking-drawer__tabs" aria-label="Abas do detalhamento">
        <button
          type="button"
          class="ranking-drawer__tab"
          :class="{ 'ranking-drawer__tab--active': activeTab === 'overview' }"
          data-testid="ranking-drawer-tab-overview"
          @click="activeTab = 'overview'"
        >
          Visão geral
        </button>
        <button
          type="button"
          class="ranking-drawer__tab"
          :class="{ 'ranking-drawer__tab--active': activeTab === 'breakdown' }"
          data-testid="ranking-drawer-tab-breakdown"
          @click="activeTab = 'breakdown'"
        >
          Breakdown 360
        </button>
        <button
          type="button"
          class="ranking-drawer__tab"
          :class="{ 'ranking-drawer__tab--active': activeTab === 'alerts' }"
          data-testid="ranking-drawer-tab-alerts"
          @click="activeTab = 'alerts'"
        >
          Alertas
          <span v-if="alertCount" class="ranking-drawer__tab-badge">{{ alertCount }}</span>
        </button>
      </nav>
    </template>

    <template #body>
      <div v-if="!row" class="ranking-drawer__empty">
        Selecione um consultor no ranking para ver os detalhes.
      </div>

      <section
        v-else-if="activeTab === 'overview'"
        class="ranking-drawer__section"
        data-testid="ranking-drawer-overview"
      >
        <div class="ranking-drawer__grid">
          <article class="ranking-drawer__metric">
            <span class="ranking-drawer__metric-label">Score 360</span>
            <strong class="ranking-drawer__metric-value">{{ row.score360.toFixed(1) }}</strong>
          </article>
          <article class="ranking-drawer__metric">
            <span class="ranking-drawer__metric-label">Vendido</span>
            <strong class="ranking-drawer__metric-value">
              {{ formatCurrencyBRL(row.soldValue) }}
            </strong>
          </article>
          <article class="ranking-drawer__metric">
            <span class="ranking-drawer__metric-label">Atendimentos</span>
            <strong class="ranking-drawer__metric-value">{{ row.attendances }}</strong>
          </article>
          <article class="ranking-drawer__metric">
            <span class="ranking-drawer__metric-label">Conversões</span>
            <strong class="ranking-drawer__metric-value">
              {{ row.conversions }} / {{ row.attendances }}
            </strong>
            <span class="ranking-drawer__metric-text">
              {{ formatPercent(row.conversionRate) }}
            </span>
          </article>
          <article class="ranking-drawer__metric">
            <span class="ranking-drawer__metric-label">Ticket médio</span>
            <strong class="ranking-drawer__metric-value">
              {{ formatCurrencyBRL(row.ticketAverage) }}
            </strong>
          </article>
          <article class="ranking-drawer__metric">
            <span class="ranking-drawer__metric-label">P.A.</span>
            <strong class="ranking-drawer__metric-value">{{ row.paScore.toFixed(2) }}</strong>
          </article>
          <article class="ranking-drawer__metric">
            <span class="ranking-drawer__metric-label">Qualidade</span>
            <strong class="ranking-drawer__metric-value">
              {{ formatPercent(row.qualityScore) }}
            </strong>
          </article>
          <article class="ranking-drawer__metric">
            <span class="ranking-drawer__metric-label">Tempo médio</span>
            <strong class="ranking-drawer__metric-value">
              {{ formatDurationMinutes(row.avgDurationMs) }}
            </strong>
          </article>
          <article class="ranking-drawer__metric">
            <span class="ranking-drawer__metric-label">Fora da vez</span>
            <strong class="ranking-drawer__metric-value">{{ row.queueJumpServices }}</strong>
          </article>
        </div>

        <div v-if="legacyRows && legacyRows.length" class="ranking-drawer__legacy-toggle">
          <button
            type="button"
            class="ranking-drawer__legacy-btn"
            @click="showLegacy = !showLegacy"
          >
            {{ showLegacy ? 'Ocultar tabela completa' : 'Ver como tabela' }}
          </button>
        </div>
        <RankingTable
          v-if="showLegacy && legacyRows"
          title="Tabela completa"
          :rows="legacyRows"
          testid="ranking-drawer-legacy"
        />
      </section>

      <section
        v-else-if="activeTab === 'breakdown'"
        class="ranking-drawer__section"
        data-testid="ranking-drawer-breakdown"
      >
        <RankingScoreBreakdown :row="row" :max-sold="maxSold" :max-pa="maxPa" />
      </section>

      <section
        v-else-if="activeTab === 'alerts'"
        class="ranking-drawer__section"
        data-testid="ranking-drawer-alerts"
      >
        <p v-if="!filteredAlerts.length" class="ranking-drawer__empty">
          Nenhum alerta para este consultor no período.
        </p>
        <ul v-else class="ranking-drawer__alerts">
          <li v-for="(alert, idx) in filteredAlerts" :key="idx" class="ranking-drawer__alert">
            <span class="ranking-drawer__alert-type">{{ alert.type }}</span>
            <span class="ranking-drawer__alert-msg">
              {{ ALERT_LABELS[alert.type]?.(alert) || alert.type }}
            </span>
          </li>
        </ul>
      </section>
    </template>
  </USlideover>
</template>

<style>
.ranking-drawer {
  width: min(720px, 92vw) !important;
  max-width: 92vw !important;
  height: auto !important;
  max-height: calc(100vh - 2rem) !important;
  inset: auto !important;
  left: 50% !important;
  top: 50% !important;
  transform: translate(-50%, -50%) !important;
  border-radius: 1rem !important;
}

.ranking-drawer--side {
  inset: 0 auto 0 auto !important;
  right: 0 !important;
  left: auto !important;
  top: 0 !important;
  transform: none !important;
  width: min(560px, 100vw) !important;
  height: 100vh !important;
  max-height: 100vh !important;
  border-radius: 0 !important;
}

.ranking-drawer--fullscreen {
  inset: 0 !important;
  width: 100vw !important;
  max-width: 100vw !important;
  height: 100vh !important;
  max-height: 100vh !important;
  left: 0 !important;
  top: 0 !important;
  transform: none !important;
  border-radius: 0 !important;
}

@media (max-width: 720px) {
  .ranking-drawer,
  .ranking-drawer--side {
    width: 100vw !important;
    max-width: 100vw !important;
    height: 100vh !important;
    max-height: 100vh !important;
    left: 0 !important;
    top: 0 !important;
    transform: none !important;
    border-radius: 0 !important;
  }
}
</style>

<style scoped>
.ranking-drawer__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.85rem 1rem 0.5rem 1rem;
}

.ranking-drawer__title-block {
  display: grid;
  gap: 0.15rem;
  min-width: 0;
}

.ranking-drawer__title {
  font-size: 1rem;
  color: rgb(var(--text) / 0.96);
}

.ranking-drawer__subtitle {
  font-size: 0.75rem;
  color: rgb(var(--muted) / 0.92);
}

.ranking-drawer__header-actions {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
}

.ranking-drawer__icon-btn {
  width: 2rem;
  height: 2rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 0.5rem;
  border: 1px solid transparent;
  background: transparent;
  color: rgb(var(--muted) / 0.92);
  cursor: pointer;
}

.ranking-drawer__icon-btn:hover {
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
}

.ranking-drawer__tabs {
  display: flex;
  gap: 0.25rem;
  padding: 0 1rem;
  border-bottom: 1px solid rgb(var(--border) / 0.8);
}

.ranking-drawer__tab {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.5rem 0.85rem;
  border: none;
  border-bottom: 2px solid transparent;
  background: transparent;
  color: rgb(var(--muted) / 0.92);
  font-size: 0.82rem;
  font-weight: 600;
  cursor: pointer;
}

.ranking-drawer__tab--active {
  color: rgb(var(--text) / 0.96);
  border-bottom-color: rgb(var(--primary));
}

.ranking-drawer__tab-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 1.2rem;
  padding: 0 0.35rem;
  border-radius: 999px;
  background: rgb(var(--danger) / 0.14);
  color: rgb(var(--danger));
  font-size: 0.68rem;
  font-weight: 700;
}

.ranking-drawer__section {
  display: grid;
  gap: 1rem;
  padding: 1rem;
}

.ranking-drawer__grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.6rem;
}

.ranking-drawer__metric {
  display: grid;
  gap: 0.2rem;
  padding: 0.75rem;
  border-radius: 0.7rem;
  border: 1px solid rgb(var(--border) / 0.72);
  background: rgb(var(--surface-2) / 0.74);
}

.ranking-drawer__metric-label {
  font-size: 0.7rem;
  color: rgb(var(--muted) / 0.88);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.ranking-drawer__metric-value {
  font-size: 0.95rem;
  color: rgb(var(--text) / 0.96);
}

.ranking-drawer__metric-text {
  font-size: 0.72rem;
  color: rgb(var(--muted) / 0.92);
}

.ranking-drawer__legacy-toggle {
  display: flex;
  justify-content: flex-end;
}

.ranking-drawer__legacy-btn {
  padding: 0.4rem 0.85rem;
  border-radius: 0.5rem;
  border: 1px solid rgb(var(--ring) / 0.32);
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
  font-size: 0.75rem;
  font-weight: 600;
  cursor: pointer;
}

.ranking-drawer__legacy-btn:hover {
  background: rgb(var(--primary) / 0.22);
}

.ranking-drawer__alerts {
  display: grid;
  gap: 0.5rem;
  margin: 0;
  padding: 0;
  list-style: none;
}

.ranking-drawer__alert {
  display: grid;
  gap: 0.2rem;
  padding: 0.6rem 0.8rem;
  border-radius: 0.6rem;
  border: 1px solid rgb(var(--danger) / 0.32);
  background: rgb(var(--danger) / 0.08);
}

.ranking-drawer__alert-type {
  font-size: 0.7rem;
  text-transform: uppercase;
  color: rgb(var(--danger));
  font-weight: 700;
}

.ranking-drawer__alert-msg {
  font-size: 0.82rem;
  color: rgb(var(--text) / 0.92);
}

.ranking-drawer__empty {
  padding: 2rem;
  text-align: center;
  color: rgb(var(--muted) / 0.92);
}

@media (max-width: 900px) {
  .ranking-drawer__grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
