<script setup lang="ts">
import { computed } from 'vue'
import {
  formatCurrencyBRL,
  formatDurationMinutes,
  formatPercent,
} from '~/domain/utils/admin-metrics'
import { useGamificationConfig } from '~/composables/useGamificationConfig'
import ConsultantBadges from './ConsultantBadges.vue'

type LiveStatusCode = 'available' | 'service' | 'queue' | 'paused' | 'assignment'

interface PlayerCardStats {
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
  averageDurationMs?: number
  nonClientConversions: number
  queueJumpServices: number
  avgTicketGoal?: number
  paGoal?: number
  conversionGoal?: number
  cancellationRate?: number
}

interface PlayerCardConsultant {
  id: string
  name: string
  role?: string
  storeName?: string
  liveStatusCode?: LiveStatusCode
  liveStatusLabel?: string
}

const props = withDefaults(
  defineProps<{
    consultant: PlayerCardConsultant
    stats: PlayerCardStats
    mode?: 'full' | 'mini'
    rankingPosition?: number | null
    storeConversionAvg?: number | null
    showDetailsButton?: boolean
  }>(),
  {
    mode: 'full',
    rankingPosition: null,
    storeConversionAvg: null,
    showDetailsButton: true,
  },
)

const emit = defineEmits<{
  (e: 'open-details', consultantId: string): void
}>()

const { enabledBadges } = useGamificationConfig()

const goalPercent = computed(() => {
  if (!props.stats.monthlyGoal) return 0
  return (props.stats.soldValue / props.stats.monthlyGoal) * 100
})

const clampedGoalPercent = computed(() => Math.min(100, Math.max(0, goalPercent.value)))

const gaugeStroke = computed(() => {
  const radius = 56
  const circumference = 2 * Math.PI * radius
  return {
    circumference,
    dasharray: `${(clampedGoalPercent.value / 100) * circumference} ${circumference}`,
  }
})

const statusClass = computed(() => {
  const code = props.consultant.liveStatusCode || 'available'
  return `consultant-status consultant-status--${code}`
})

const statusLabel = computed(() => props.consultant.liveStatusLabel || 'Disponível')

const goalProgressText = computed(() => {
  if (!props.stats.monthlyGoal) return 'Sem meta cadastrada'
  if (goalPercent.value >= 100) return 'Meta batida 🎉'
  return `Faltam ${formatCurrencyBRL(props.stats.remainingToGoal)}`
})

function handleClick() {
  if (props.mode === 'mini') {
    emit('open-details', props.consultant.id)
  }
}

function handleDetailsClick() {
  emit('open-details', props.consultant.id)
}
</script>

<template>
  <article
    class="player-card"
    :class="[`player-card--${mode}`]"
    :data-consultant-id="consultant.id"
    :data-testid="`player-card-${consultant.id}`"
    :tabindex="mode === 'mini' ? 0 : -1"
    :role="mode === 'mini' ? 'button' : 'article'"
    @click="handleClick"
    @keydown.enter="handleClick"
    @keydown.space.prevent="handleClick"
  >
    <header class="player-card__header">
      <div class="player-card__identity">
        <span class="player-card__avatar" aria-hidden="true">
          {{ consultant.name.charAt(0).toUpperCase() }}
        </span>
        <div class="player-card__name-block">
          <strong class="player-card__name">{{ consultant.name }}</strong>
          <span v-if="consultant.storeName || consultant.role" class="player-card__subtitle">
            <template v-if="consultant.storeName">{{ consultant.storeName }}</template>
            <template v-if="consultant.storeName && consultant.role">·</template>
            <template v-if="consultant.role">{{ consultant.role }}</template>
          </span>
        </div>
      </div>
      <span :class="statusClass">{{ statusLabel }}</span>
    </header>

    <div class="player-card__gauge-block">
      <div class="player-card__gauge">
        <svg viewBox="0 0 140 140" class="player-card__gauge-svg" aria-hidden="true">
          <circle
            cx="70"
            cy="70"
            r="56"
            class="player-card__gauge-track"
            fill="none"
            stroke-width="14"
          />
          <circle
            cx="70"
            cy="70"
            r="56"
            class="player-card__gauge-fill"
            fill="none"
            stroke-width="14"
            stroke-linecap="round"
            :stroke-dasharray="gaugeStroke.dasharray"
            transform="rotate(-90 70 70)"
          />
          <text x="70" y="68" text-anchor="middle" class="player-card__gauge-value">
            {{ formatPercent(goalPercent) }}
          </text>
          <text x="70" y="86" text-anchor="middle" class="player-card__gauge-caption">da meta</text>
        </svg>
      </div>
      <div class="player-card__gauge-side">
        <div class="player-card__goal-copy">
          <span class="player-card__sold-amount">{{ formatCurrencyBRL(stats.soldValue) }}</span>
          <span class="player-card__sold-of-goal">
            de {{ formatCurrencyBRL(stats.monthlyGoal) }}
          </span>
          <span class="player-card__sold-caption">{{ goalProgressText }}</span>
        </div>

        <div v-if="mode === 'full'" class="player-card__hero-metrics">
          <div class="player-card__hero-metric">
            <span class="player-card__kpi-icon" aria-hidden="true">🎯</span>
            <span class="player-card__kpi-label">Ticket</span>
            <strong class="player-card__kpi-value">
              {{ formatCurrencyBRL(stats.ticketAverage) }}
            </strong>
            <span
              v-if="stats.avgTicketGoal"
              class="player-card__metric-note"
              :class="
                stats.ticketAverage >= stats.avgTicketGoal
                  ? 'player-card__metric-note--hit'
                  : 'player-card__metric-note--miss'
              "
            >
              Meta: {{ formatCurrencyBRL(stats.avgTicketGoal) }}
            </span>
          </div>
          <div class="player-card__hero-metric">
            <span class="player-card__kpi-icon" aria-hidden="true">📦</span>
            <span class="player-card__kpi-label">P.A.</span>
            <strong class="player-card__kpi-value">{{ stats.paScore.toFixed(2) }}</strong>
            <span
              v-if="stats.paGoal"
              class="player-card__metric-note"
              :class="
                stats.paScore >= stats.paGoal
                  ? 'player-card__metric-note--hit'
                  : 'player-card__metric-note--miss'
              "
            >
              Meta: {{ stats.paGoal.toFixed(2) }}
            </span>
          </div>
          <div class="player-card__hero-metric">
            <span class="player-card__kpi-icon" aria-hidden="true">⚡</span>
            <span class="player-card__kpi-label">Conversão</span>
            <strong class="player-card__kpi-value">
              {{ formatPercent(stats.conversionRate) }}
            </strong>
            <span
              v-if="stats.conversionGoal"
              class="player-card__metric-note"
              :class="
                stats.conversionRate >= stats.conversionGoal
                  ? 'player-card__metric-note--hit'
                  : 'player-card__metric-note--miss'
              "
            >
              Meta: {{ formatPercent(stats.conversionGoal) }}
            </span>
          </div>
          <div class="player-card__hero-metric">
            <span class="player-card__kpi-icon" aria-hidden="true">⏱</span>
            <span class="player-card__kpi-label">Tempo médio</span>
            <strong class="player-card__kpi-value">
              {{ formatDurationMinutes(stats.averageDurationMs || 0) }}
            </strong>
          </div>
        </div>
      </div>
    </div>

    <div v-if="mode === 'full'" class="player-card__detail-grid">
      <div class="player-card__kpi">
        <span class="player-card__kpi-icon" aria-hidden="true">💸</span>
        <span class="player-card__kpi-label">Comissão estimada</span>
        <strong class="player-card__kpi-value">
          {{ formatCurrencyBRL(stats.estimatedCommission) }}
        </strong>
        <span class="player-card__metric-note">
          Taxa atual: {{ formatPercent(stats.commissionRate * 100) }}
        </span>
      </div>
      <div class="player-card__kpi">
        <span class="player-card__kpi-icon" aria-hidden="true">👥</span>
        <span class="player-card__kpi-label">Atendimentos</span>
        <strong class="player-card__kpi-value">
          {{ stats.conversions + stats.nonConversions }}
        </strong>
      </div>
      <div class="player-card__kpi">
        <span class="player-card__kpi-icon" aria-hidden="true">🔄</span>
        <span class="player-card__kpi-label">Conversões / Não convertidas</span>
        <strong class="player-card__kpi-value">
          {{ stats.conversions }} / {{ stats.nonConversions }}
        </strong>
      </div>
      <div class="player-card__kpi">
        <span class="player-card__kpi-icon" aria-hidden="true">🆕</span>
        <span class="player-card__kpi-label">Não-clientes convertidos</span>
        <strong class="player-card__kpi-value">{{ stats.nonClientConversions }}</strong>
      </div>
      <div class="player-card__kpi">
        <span class="player-card__kpi-icon" aria-hidden="true">↪</span>
        <span class="player-card__kpi-label">Fora da vez</span>
        <strong class="player-card__kpi-value">{{ stats.queueJumpServices }}</strong>
      </div>
      <div v-if="typeof stats.cancellationRate === 'number'" class="player-card__kpi">
        <span class="player-card__kpi-icon" aria-hidden="true">⛔</span>
        <span class="player-card__kpi-label">Taxa de cancelamento</span>
        <strong class="player-card__kpi-value">{{ formatPercent(stats.cancellationRate) }}</strong>
      </div>
    </div>

    <div v-else class="player-card__kpis">
      <div class="player-card__kpi">
        <span class="player-card__kpi-icon" aria-hidden="true">⏱</span>
        <span class="player-card__kpi-label">Tempo</span>
        <strong class="player-card__kpi-value">
          {{ formatDurationMinutes(stats.averageDurationMs || 0) }}
        </strong>
      </div>
      <div class="player-card__kpi">
        <span class="player-card__kpi-icon" aria-hidden="true">⚡</span>
        <span class="player-card__kpi-label">Conversão</span>
        <strong class="player-card__kpi-value">{{ formatPercent(stats.conversionRate) }}</strong>
      </div>
      <div class="player-card__kpi">
        <span class="player-card__kpi-icon" aria-hidden="true">🎯</span>
        <span class="player-card__kpi-label">Ticket</span>
        <strong class="player-card__kpi-value">{{ formatCurrencyBRL(stats.ticketAverage) }}</strong>
      </div>
      <div class="player-card__kpi">
        <span class="player-card__kpi-icon" aria-hidden="true">📦</span>
        <span class="player-card__kpi-label">P.A.</span>
        <strong class="player-card__kpi-value">{{ stats.paScore.toFixed(2) }}</strong>
      </div>
    </div>

    <ConsultantBadges
      v-if="mode === 'full'"
      :stats="stats"
      :badges="enabledBadges"
      :ranking-position="rankingPosition"
      :store-conversion-avg="storeConversionAvg"
    />

    <footer v-if="mode === 'full' && showDetailsButton" class="player-card__footer">
      <button
        type="button"
        class="player-card__details-btn"
        data-testid="player-card-open-details"
        @click="handleDetailsClick"
      >
        Ver detalhes
      </button>
    </footer>
  </article>
</template>

<style scoped>
.player-card {
  display: grid;
  gap: 1rem;
  padding: 1.25rem;
  border-radius: 1rem;
  border: 1px solid rgba(125, 146, 255, 0.18);
  background: rgba(13, 19, 36, 0.78);
  color: rgba(226, 232, 240, 0.92);
}

.player-card--mini {
  padding: 0.95rem;
  gap: 0.75rem;
  cursor: pointer;
  transition:
    border-color 120ms ease,
    transform 120ms ease;
}

.player-card--mini:hover,
.player-card--mini:focus-visible {
  border-color: rgba(125, 146, 255, 0.42);
  transform: translateY(-1px);
  outline: none;
}

.player-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.player-card__identity {
  display: flex;
  align-items: center;
  gap: 0.7rem;
  min-width: 0;
}

.player-card__avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2.25rem;
  height: 2.25rem;
  border-radius: 999px;
  background: rgba(99, 102, 241, 0.22);
  color: #c7d2fe;
  font-weight: 700;
  font-size: 0.95rem;
}

.player-card__name-block {
  display: grid;
  gap: 0.1rem;
  min-width: 0;
}

.player-card__name {
  font-size: 0.95rem;
  line-height: 1.2;
  color: rgba(248, 250, 252, 0.96);
}

.player-card__subtitle {
  font-size: 0.72rem;
  color: rgba(148, 163, 184, 0.92);
}

.consultant-status {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 1.6rem;
  padding: 0 0.55rem;
  border-radius: 999px;
  border: 1px solid rgba(148, 163, 184, 0.22);
  font-size: 0.68rem;
  font-weight: 700;
  white-space: nowrap;
}

.consultant-status--available {
  background: rgba(34, 197, 94, 0.14);
  color: #86efac;
}

.consultant-status--service {
  background: rgba(59, 130, 246, 0.14);
  color: #93c5fd;
}

.consultant-status--queue {
  background: rgba(250, 204, 21, 0.14);
  color: #fde68a;
}

.consultant-status--paused,
.consultant-status--assignment {
  background: rgba(244, 114, 182, 0.14);
  color: #f9a8d4;
}

.player-card__gauge-block {
  display: grid;
  grid-template-columns: minmax(0, 9rem) minmax(0, 1fr);
  align-items: center;
  gap: 1rem;
}

.player-card--mini .player-card__gauge-block {
  grid-template-columns: minmax(0, 6.5rem) minmax(0, 1fr);
}

.player-card__gauge-svg {
  width: 100%;
  height: auto;
  display: block;
}

.player-card__gauge-track {
  stroke: rgba(148, 163, 184, 0.16);
}

.player-card__gauge-fill {
  stroke: #6366f1;
  transition: stroke-dasharray 240ms ease;
}

.player-card__gauge-value {
  fill: rgba(248, 250, 252, 0.96);
  font-size: 22px;
  font-weight: 700;
}

.player-card__gauge-caption {
  fill: rgba(148, 163, 184, 0.92);
  font-size: 11px;
  font-weight: 500;
}

.player-card__gauge-side {
  display: grid;
  gap: 0.75rem;
}

.player-card__goal-copy {
  display: grid;
  gap: 0.25rem;
}

.player-card__sold-amount {
  font-size: 1.15rem;
  font-weight: 700;
  color: rgba(248, 250, 252, 0.96);
}

.player-card__sold-of-goal,
.player-card__sold-caption {
  font-size: 0.78rem;
  color: rgba(148, 163, 184, 0.92);
}

.player-card__hero-metrics {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(9rem, 1fr));
  gap: 0.5rem;
}

.player-card__hero-metric,
.player-card__kpi {
  display: grid;
  gap: 0.15rem;
  padding: 0.55rem 0.65rem;
  border-radius: 0.7rem;
  background: rgba(15, 23, 42, 0.55);
  border: 1px solid rgba(148, 163, 184, 0.12);
}

.player-card__detail-grid {
  display: grid;
  grid-auto-flow: column;
  grid-auto-columns: minmax(11rem, 1fr);
  gap: 0.5rem;
  overflow-x: auto;
  padding-bottom: 0.1rem;
}

.player-card__kpis {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0.5rem;
}

.player-card--mini .player-card__kpis {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.player-card__kpi-icon {
  font-size: 0.95rem;
  line-height: 1;
}

.player-card__kpi-label {
  font-size: 0.68rem;
  color: rgba(148, 163, 184, 0.88);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.player-card__kpi-value {
  font-size: 0.92rem;
  color: rgba(248, 250, 252, 0.96);
}

.player-card__metric-note {
  font-size: 0.7rem;
  color: rgba(148, 163, 184, 0.92);
}

.player-card__metric-note--hit {
  color: #86efac;
}

.player-card__metric-note--miss {
  color: #fca5a5;
}

.player-card__footer {
  display: flex;
  justify-content: flex-end;
}

.player-card__details-btn {
  padding: 0.45rem 0.95rem;
  border-radius: 0.6rem;
  border: 1px solid rgba(125, 146, 255, 0.42);
  background: rgba(99, 102, 241, 0.16);
  color: #c7d2fe;
  font-size: 0.78rem;
  font-weight: 600;
  cursor: pointer;
  transition: background 120ms ease;
}

.player-card__details-btn:hover {
  background: rgba(99, 102, 241, 0.28);
}

@media (max-width: 720px) {
  .player-card__hero-metrics,
  .player-card__kpis {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .player-card__gauge-block {
    grid-template-columns: minmax(0, 7rem) minmax(0, 1fr);
  }
}

@media (max-width: 560px) {
  .player-card__gauge-block {
    grid-template-columns: 1fr;
  }

  .player-card__gauge {
    max-width: 8rem;
  }

  .player-card__hero-metrics {
    grid-template-columns: 1fr;
  }
}
</style>
