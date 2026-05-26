<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  formatCurrencyBRL,
  formatDurationMinutes,
  formatPercent,
} from '~/domain/utils/admin-metrics'
import { useConsultantDetailsDrawer } from '~/composables/useConsultantDetailsDrawer'
import ConsultantSimulator from './ConsultantSimulator.vue'

type Tab = 'overview' | 'history' | 'simulator'

interface DrawerStats {
  monthlyGoal: number
  soldValue: number
  remainingToGoal: number
  estimatedCommission: number
  commissionRate: number
  ticketAverage: number
  paScore: number
  conversionRate: number
  conversions: number
  nonConversions: number
  averageDurationMs: number
  queueJumpServices: number
  nonClientConversions: number
  avgTicketGoal?: number
  paGoal?: number
  conversionGoal?: number
  cancellationRate?: number
  monthEntries?: Array<{ finishedAt?: number; saleAmount?: number }>
}

interface DrawerConsultant {
  id: string
  name: string
  role?: string
  storeName?: string
  liveStatusCode?: string
  liveStatusLabel?: string
}

const props = defineProps<{
  consultant: DrawerConsultant | null
  stats: DrawerStats | null
  simulationAdditionalSales?: number
}>()

const emit = defineEmits<{
  (e: 'update:simulationAdditionalSales', value: number): void
}>()

const drawer = useConsultantDetailsDrawer()
const activeTab = ref<Tab>('overview')

const open = computed({
  get: () => drawer.isOpen.value,
  set: (next: boolean) => {
    if (!next) drawer.close()
  },
})

const drawerMode = computed(() => drawer.mode.value)

watch(
  () => [drawer.currentConsultantId.value, drawer.isOpen.value],
  ([, nextOpen]) => {
    if (nextOpen) {
      activeTab.value = drawer.initialTab.value
    }
  },
)

const goalPercent = computed(() => {
  if (!props.stats?.monthlyGoal) return 0
  return (props.stats.soldValue / props.stats.monthlyGoal) * 100
})

const conversionTotal = computed(() => {
  if (!props.stats) return 0
  return props.stats.conversions + props.stats.nonConversions
})

const sparklinePoints = computed(() => {
  const entries = props.stats?.monthEntries || []
  if (!entries.length) return [] as Array<{ day: string; value: number }>

  const map = new Map<string, number>()
  entries.forEach((entry) => {
    if (!entry.finishedAt) return
    const date = new Date(entry.finishedAt)
    const day = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
    const sum = map.get(day) || 0
    map.set(day, sum + Number(entry.saleAmount || 0))
  })

  const cutoff = Date.now() - 6 * 24 * 60 * 60 * 1000
  const result: Array<{ day: string; value: number }> = []
  for (let i = 0; i < 7; i += 1) {
    const dt = new Date(cutoff + i * 24 * 60 * 60 * 1000)
    const day = `${dt.getFullYear()}-${String(dt.getMonth() + 1).padStart(2, '0')}-${String(dt.getDate()).padStart(2, '0')}`
    result.push({ day, value: map.get(day) || 0 })
  }
  return result
})

const sparklinePath = computed(() => {
  const points = sparklinePoints.value
  if (!points.length) return ''
  const maxValue = Math.max(...points.map((p) => p.value), 1)
  const width = 320
  const height = 60
  const step = width / Math.max(points.length - 1, 1)

  return points
    .map((point, index) => {
      const x = Math.round(index * step)
      const y = Math.round(height - (point.value / maxValue) * height)
      return `${index === 0 ? 'M' : 'L'} ${x} ${y}`
    })
    .join(' ')
})

function handleSimulatorUpdate(value: number | string) {
  const numeric = Number(value || 0)
  emit('update:simulationAdditionalSales', Number.isFinite(numeric) ? numeric : 0)
}
</script>

<template>
  <USlideover
    v-model:open="open"
    :overlay="true"
    :modal="true"
    :dismissible="true"
    :ui="{ content: `consultant-drawer consultant-drawer--${drawerMode}` }"
  >
    <template #header>
      <div class="consultant-drawer__header">
        <div class="consultant-drawer__header-identity">
          <span v-if="consultant" class="consultant-drawer__avatar" aria-hidden="true">
            {{ consultant.name.charAt(0).toUpperCase() }}
          </span>
          <div class="consultant-drawer__title-block">
            <strong class="consultant-drawer__title">
              {{ consultant?.name || 'Consultor' }}
            </strong>
            <span
              v-if="consultant?.storeName || consultant?.role"
              class="consultant-drawer__subtitle"
            >
              <template v-if="consultant?.storeName">{{ consultant.storeName }}</template>
              <template v-if="consultant?.storeName && consultant?.role">·</template>
              <template v-if="consultant?.role">{{ consultant.role }}</template>
            </span>
          </div>
          <span
            v-if="consultant?.liveStatusCode"
            class="consultant-status"
            :class="`consultant-status--${consultant.liveStatusCode}`"
          >
            {{ consultant.liveStatusLabel }}
          </span>
        </div>
        <div class="consultant-drawer__header-actions">
          <button
            type="button"
            class="consultant-drawer__icon-btn"
            :title="drawerMode === 'fullscreen' ? 'Sair de página inteira' : 'Página inteira'"
            data-testid="consultant-drawer-toggle-fullscreen"
            @click="drawer.toggleFullscreen()"
          >
            <UIcon
              :name="drawerMode === 'fullscreen' ? 'i-lucide-minimize-2' : 'i-lucide-expand'"
              class="h-4 w-4"
            />
          </button>
          <button
            type="button"
            class="consultant-drawer__icon-btn"
            title="Fechar"
            data-testid="consultant-drawer-close"
            @click="drawer.close()"
          >
            <UIcon name="i-lucide-x" class="h-4 w-4" />
          </button>
        </div>
      </div>
      <nav class="consultant-drawer__tabs" aria-label="Abas do detalhamento">
        <button
          type="button"
          class="consultant-drawer__tab"
          :class="{ 'consultant-drawer__tab--active': activeTab === 'overview' }"
          data-testid="consultant-drawer-tab-overview"
          @click="activeTab = 'overview'"
        >
          Visão geral
        </button>
        <button
          type="button"
          class="consultant-drawer__tab"
          :class="{ 'consultant-drawer__tab--active': activeTab === 'history' }"
          data-testid="consultant-drawer-tab-history"
          @click="activeTab = 'history'"
        >
          Histórico
        </button>
        <button
          type="button"
          class="consultant-drawer__tab"
          :class="{ 'consultant-drawer__tab--active': activeTab === 'simulator' }"
          data-testid="consultant-drawer-tab-simulator"
          @click="activeTab = 'simulator'"
        >
          Simulador
        </button>
      </nav>
    </template>

    <template #body>
      <div v-if="!stats" class="consultant-drawer__empty">
        Selecione um consultor para ver os detalhes.
      </div>

      <section
        v-else-if="activeTab === 'overview'"
        class="consultant-drawer__section"
        data-testid="consultant-drawer-overview"
      >
        <div class="consultant-drawer__metric-grid">
          <article class="consultant-drawer__metric">
            <span class="consultant-drawer__metric-label">Meta mensal</span>
            <strong class="consultant-drawer__metric-value">
              {{ formatCurrencyBRL(stats.monthlyGoal) }}
            </strong>
            <span class="consultant-drawer__metric-text">
              Faltam {{ formatCurrencyBRL(stats.remainingToGoal) }} ({{
                formatPercent(goalPercent)
              }}
              batido)
            </span>
          </article>
          <article class="consultant-drawer__metric">
            <span class="consultant-drawer__metric-label">Vendido no mês</span>
            <strong class="consultant-drawer__metric-value">
              {{ formatCurrencyBRL(stats.soldValue) }}
            </strong>
          </article>
          <article class="consultant-drawer__metric">
            <span class="consultant-drawer__metric-label">Comissão estimada</span>
            <strong class="consultant-drawer__metric-value">
              {{ formatCurrencyBRL(stats.estimatedCommission) }}
            </strong>
            <span class="consultant-drawer__metric-text">
              Taxa atual: {{ formatPercent(stats.commissionRate * 100) }}
            </span>
          </article>
        </div>

        <div class="consultant-drawer__metric-grid consultant-drawer__metric-grid--tight">
          <article class="consultant-drawer__metric consultant-drawer__metric--soft">
            <span class="consultant-drawer__metric-label">Atendimentos</span>
            <strong class="consultant-drawer__metric-value">{{ conversionTotal }}</strong>
          </article>
          <article class="consultant-drawer__metric consultant-drawer__metric--soft">
            <span class="consultant-drawer__metric-label">Conversões / Não convertidas</span>
            <strong class="consultant-drawer__metric-value">
              {{ stats.conversions }} / {{ stats.nonConversions }}
            </strong>
          </article>
          <article class="consultant-drawer__metric consultant-drawer__metric--soft">
            <span class="consultant-drawer__metric-label">Taxa de conversão</span>
            <strong class="consultant-drawer__metric-value">
              {{ formatPercent(stats.conversionRate) }}
            </strong>
            <span
              v-if="stats.conversionGoal"
              class="consultant-drawer__metric-text"
              :class="
                stats.conversionRate >= stats.conversionGoal
                  ? 'consultant-drawer__metric-text--hit'
                  : 'consultant-drawer__metric-text--miss'
              "
            >
              Meta: {{ formatPercent(stats.conversionGoal) }}
            </span>
          </article>
          <article class="consultant-drawer__metric consultant-drawer__metric--soft">
            <span class="consultant-drawer__metric-label">Ticket médio</span>
            <strong class="consultant-drawer__metric-value">
              {{ formatCurrencyBRL(stats.ticketAverage) }}
            </strong>
            <span
              v-if="stats.avgTicketGoal"
              class="consultant-drawer__metric-text"
              :class="
                stats.ticketAverage >= stats.avgTicketGoal
                  ? 'consultant-drawer__metric-text--hit'
                  : 'consultant-drawer__metric-text--miss'
              "
            >
              Meta: {{ formatCurrencyBRL(stats.avgTicketGoal) }}
            </span>
          </article>
          <article class="consultant-drawer__metric consultant-drawer__metric--soft">
            <span class="consultant-drawer__metric-label">P.A.</span>
            <strong class="consultant-drawer__metric-value">{{ stats.paScore.toFixed(2) }}</strong>
            <span
              v-if="stats.paGoal"
              class="consultant-drawer__metric-text"
              :class="
                stats.paScore >= stats.paGoal
                  ? 'consultant-drawer__metric-text--hit'
                  : 'consultant-drawer__metric-text--miss'
              "
            >
              Meta: {{ stats.paGoal.toFixed(2) }}
            </span>
          </article>
          <article class="consultant-drawer__metric consultant-drawer__metric--soft">
            <span class="consultant-drawer__metric-label">Tempo médio</span>
            <strong class="consultant-drawer__metric-value">
              {{ formatDurationMinutes(stats.averageDurationMs) }}
            </strong>
          </article>
          <article class="consultant-drawer__metric consultant-drawer__metric--soft">
            <span class="consultant-drawer__metric-label">Não-clientes convertidos</span>
            <strong class="consultant-drawer__metric-value">
              {{ stats.nonClientConversions }}
            </strong>
          </article>
          <article class="consultant-drawer__metric consultant-drawer__metric--soft">
            <span class="consultant-drawer__metric-label">Atendimentos fora da vez</span>
            <strong class="consultant-drawer__metric-value">{{ stats.queueJumpServices }}</strong>
          </article>
          <article
            v-if="typeof stats.cancellationRate === 'number'"
            class="consultant-drawer__metric consultant-drawer__metric--soft"
          >
            <span class="consultant-drawer__metric-label">Taxa de cancelamento</span>
            <strong class="consultant-drawer__metric-value">
              {{ formatPercent(stats.cancellationRate) }}
            </strong>
          </article>
        </div>
      </section>

      <section
        v-else-if="activeTab === 'history'"
        class="consultant-drawer__section"
        data-testid="consultant-drawer-history"
      >
        <h3 class="consultant-drawer__section-title">Vendas dos últimos 7 dias</h3>
        <div v-if="sparklinePoints.length" class="consultant-drawer__sparkline">
          <svg viewBox="0 0 320 60" preserveAspectRatio="none" aria-hidden="true">
            <path :d="sparklinePath" fill="none" stroke="rgb(var(--primary))" stroke-width="2" />
          </svg>
          <div class="consultant-drawer__sparkline-legend">
            <span v-for="point in sparklinePoints" :key="point.day">
              {{ formatCurrencyBRL(point.value) }}
            </span>
          </div>
        </div>
        <p v-else class="consultant-drawer__empty">Sem dados suficientes nos últimos 7 dias.</p>
      </section>

      <section
        v-else-if="activeTab === 'simulator'"
        class="consultant-drawer__section"
        data-testid="consultant-drawer-simulator"
      >
        <ConsultantSimulator
          :sold-value="stats.soldValue"
          :monthly-goal="stats.monthlyGoal"
          :commission-rate="stats.commissionRate"
          :simulation-additional-sales="Number(simulationAdditionalSales || 0)"
          @update:simulation-additional-sales="handleSimulatorUpdate"
        />
      </section>
    </template>
  </USlideover>
</template>

<style>
.consultant-drawer {
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

.consultant-drawer--side {
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

.consultant-drawer--fullscreen {
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
  .consultant-drawer,
  .consultant-drawer--side {
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
.consultant-drawer__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.85rem 1rem 0.5rem 1rem;
}

.consultant-drawer__header-identity {
  display: flex;
  align-items: center;
  gap: 0.7rem;
  min-width: 0;
}

.consultant-drawer__avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2.5rem;
  height: 2.5rem;
  border-radius: 999px;
  background: rgb(var(--primary) / 0.18);
  color: rgb(var(--primary));
  font-weight: 700;
}

.consultant-drawer__title-block {
  display: grid;
  gap: 0.15rem;
  min-width: 0;
}

.consultant-drawer__title {
  font-size: 1rem;
  color: rgb(var(--text) / 0.96);
}

.consultant-drawer__subtitle {
  font-size: 0.75rem;
  color: rgb(var(--muted) / 0.92);
}

.consultant-drawer__header-actions {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
}

.consultant-drawer__icon-btn {
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

.consultant-drawer__icon-btn:hover {
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
}

.consultant-drawer__tabs {
  display: flex;
  gap: 0.25rem;
  padding: 0 1rem;
  border-bottom: 1px solid rgb(var(--border) / 0.8);
}

.consultant-drawer__tab {
  padding: 0.5rem 0.85rem;
  border: none;
  border-bottom: 2px solid transparent;
  background: transparent;
  color: rgb(var(--muted) / 0.92);
  font-size: 0.82rem;
  font-weight: 600;
  cursor: pointer;
}

.consultant-drawer__tab--active {
  color: rgb(var(--text) / 0.96);
  border-bottom-color: rgb(var(--primary));
}

.consultant-drawer__section {
  display: grid;
  gap: 1rem;
  padding: 1rem;
}

.consultant-drawer__section-title {
  margin: 0;
  font-size: 0.95rem;
  color: rgb(var(--text) / 0.96);
}

.consultant-drawer__metric-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.6rem;
}

.consultant-drawer__metric-grid--tight {
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.consultant-drawer__metric {
  display: grid;
  gap: 0.2rem;
  padding: 0.75rem;
  border-radius: 0.7rem;
  border: 1px solid rgb(var(--border) / 0.72);
  background: rgb(var(--surface-2) / 0.74);
}

.consultant-drawer__metric-label {
  font-size: 0.7rem;
  color: rgb(var(--muted) / 0.88);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.consultant-drawer__metric-value {
  font-size: 0.95rem;
  color: rgb(var(--text) / 0.96);
}

.consultant-drawer__metric-text {
  font-size: 0.72rem;
  color: rgb(var(--muted) / 0.92);
}

.consultant-drawer__metric-text--hit {
  color: rgb(var(--success));
}

.consultant-drawer__metric-text--miss {
  color: rgb(var(--danger));
}

.consultant-drawer__sparkline {
  display: grid;
  gap: 0.5rem;
  padding: 0.85rem;
  border-radius: 0.7rem;
  background: rgb(var(--surface-2) / 0.74);
  border: 1px solid rgb(var(--border) / 0.72);
}

.consultant-drawer__sparkline svg {
  width: 100%;
  height: 60px;
}

.consultant-drawer__sparkline-legend {
  display: grid;
  grid-template-columns: repeat(7, minmax(0, 1fr));
  gap: 0.25rem;
  font-size: 0.7rem;
  color: rgb(var(--muted) / 0.88);
  text-align: center;
}

.consultant-drawer__empty {
  padding: 2rem;
  text-align: center;
  color: rgb(var(--muted) / 0.92);
}

@media (max-width: 900px) {
  .consultant-drawer__metric-grid,
  .consultant-drawer__metric-grid--tight {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
