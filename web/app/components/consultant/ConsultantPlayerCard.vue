<script setup lang="ts">
import { computed } from 'vue'
import { formatCurrencyBRL, formatPercent } from '~/domain/utils/admin-metrics'
import { goalProgressTier } from '~/domain/utils/goal-progress-color'
import { useGamificationConfig } from '~/composables/useGamificationConfig'
import AppInfoPopover from '~/components/ui/AppInfoPopover.vue'
import InlineFieldGuard from '~/components/quick-edit/InlineFieldGuard.vue'
import { consultantMonthlyGoal } from '~/domain/quick-edit/fields/consultantMonthlyGoal'
import type { GoalQuickEditContext } from '~/domain/quick-edit/fields/goalContext'
import type { PlayerCardConsultant, PlayerCardStats } from './player-card-types'
import ConsultantBadges from './ConsultantBadges.vue'
import ConsultantStoreGoalBar from './ConsultantStoreGoalBar.vue'
import ConsultantPlayerCardMetrics from './ConsultantPlayerCardMetrics.vue'

const FORECAST_NOTE =
  'Previsão calculada com base nos dados recebidos até o dia anterior e nas metas cadastradas pela ' +
  'gerência. Os valores são apenas para acompanhamento e devem ser validados com o gerente responsável.'

const props = withDefaults(
  defineProps<{
    consultant: PlayerCardConsultant
    stats: PlayerCardStats
    mode?: 'full' | 'mini'
    rankingPosition?: number | null
    storeConversionAvg?: number | null
    showDetailsButton?: boolean
    storeGoalProgress?: number | null
    goalPayoutAmount?: number | null
    goalPayoutLabel?: string
    // Contexto do quick-edit de metas (motor plugável). null = sem avisos inline.
    goalContext?: GoalQuickEditContext | null
  }>(),
  {
    mode: 'full',
    rankingPosition: null,
    storeConversionAvg: null,
    showDetailsButton: true,
    storeGoalProgress: null,
    goalPayoutAmount: null,
    goalPayoutLabel: '',
    goalContext: null,
  },
)

const emit = defineEmits<{
  (e: 'open-details', consultantId: string): void
}>()

const { enabledBadges } = useGamificationConfig()

// Quick-edit de metas (motor plugável): só o aviso de meta INDIVIDUAL fica no card;
// os de ticket/PA da loja vivem no cabeçalho do grupo (uma vez por loja).
const hasGoalContext = computed(() => Boolean(props.goalContext))

const goalPercent = computed(() => {
  if (!props.stats.monthlyGoal) return 0
  return (props.stats.soldValue / props.stats.monthlyGoal) * 100
})

const hasGoal = computed(() => Number(props.stats.monthlyGoal || 0) > 0)
const gaugeTierClass = computed(
  () => `player-card__gauge-fill--${goalProgressTier(goalPercent.value, hasGoal.value)}`,
)

const showStoreBar = computed(() => typeof props.storeGoalProgress === 'number')
const showPayout = computed(() => typeof props.goalPayoutAmount === 'number')

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
  if (goalPercent.value >= 100) return 'Meta batida'
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
          <!--<span v-if="consultant.storeName || consultant.role" class="player-card__subtitle">
            <template v-if="consultant.storeName">{{ consultant.storeName }}</template>
            <template v-if="consultant.storeName && consultant.role">·</template>
            <template v-if="consultant.role">{{ consultant.role }}</template>
          </span>-->
        </div>
      </div>
      <div class="player-card__header-end">
        <!--<span :class="statusClass">{{ statusLabel }}</span>-->
        <!--<span v-if="stats.soldValueSource === 'erp'" class="player-card__source-badge">ERP teste</span>-->
      </div>
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
            :class="['player-card__gauge-fill', gaugeTierClass]"
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
        <span
          v-if="stats.soldValueSource === 'erp' && stats.erpOrders"
          class="player-card__gauge-orders"
        >
          {{ stats.erpOrders }} vendas ERP
        </span>
      </div>
      <div class="player-card__gauge-side">
        <div class="player-card__goal-copy">
          <!-- <span v-if="stats.soldValueSource === 'erp'" class="player-card__source-badge">ERP</span> -->
          <span class="player-card__sold-amount">{{ formatCurrencyBRL(stats.soldValue) }}</span>
          <div class="player-card__goal-line">
            <span class="player-card__sold-of-goal">
              de {{ formatCurrencyBRL(stats.monthlyGoal) }}
            </span>
            <span class="player-card__sold-caption">{{ goalProgressText }}</span>
          </div>
        </div>

        <div v-if="hasGoalContext && goalContext" class="player-card__goal-alerts">
          <InlineFieldGuard :descriptor="consultantMonthlyGoal" :context="goalContext" />
        </div>

        <div v-if="showStoreBar" class="player-card__store">
          <ConsultantStoreGoalBar :progress="storeGoalProgress" />
          <span v-if="showPayout" class="player-card__store-payout">
            Recebe
            <strong>{{ formatCurrencyBRL(goalPayoutAmount || 0) }}</strong>
            <AppInfoPopover
              :text="FORECAST_NOTE"
              label="Sobre a previsão"
              align="start"
              class="player-card__payout-info"
            />
            <template v-if="goalPayoutLabel">· {{ goalPayoutLabel }}</template>
          </span>
        </div>

        <ConsultantPlayerCardMetrics v-if="mode === 'full'" section="hero" :stats="stats" />
      </div>
    </div>

    <ConsultantPlayerCardMetrics section="detail" :stats="stats" :mode="mode" />

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
  border: 1px solid rgb(var(--primary) / 0.18);
  background: rgb(var(--surface) / 0.86);
  color: rgb(var(--text) / 0.92);
  box-shadow: var(--shadow-xs);
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
  border-color: rgb(var(--ring) / 0.42);
  transform: translateY(-1px);
  outline: none;
}

.player-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.player-card__header-end {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  flex-shrink: 0;
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
  background: rgb(var(--primary) / 0.18);
  color: rgb(var(--primary));
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
  color: rgb(var(--text) / 0.96);
}

.player-card__subtitle {
  font-size: 0.72rem;
  color: rgb(var(--muted) / 0.92);
}

.consultant-status {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 1.6rem;
  padding: 0 0.55rem;
  border-radius: 999px;
  border: 1px solid rgb(var(--border) / 0.86);
  font-size: 0.68rem;
  font-weight: 700;
  white-space: nowrap;
}

.consultant-status--available {
  background: rgb(var(--success) / 0.14);
  color: rgb(var(--success));
}

.consultant-status--service {
  background: rgb(var(--primary) / 0.14);
  color: rgb(var(--primary));
}

.consultant-status--queue {
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
}

.consultant-status--paused,
.consultant-status--assignment {
  background: rgb(var(--danger) / 0.12);
  color: rgb(var(--danger));
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

.player-card__gauge {
  display: grid;
  justify-items: center;
  gap: 0.3rem;
}

.player-card__gauge-svg {
  width: 100%;
  height: auto;
  display: block;
}

.player-card__gauge-orders {
  font-size: 0.72rem;
  font-weight: 600;
  color: rgb(var(--muted) / 0.92);
  text-align: center;
}

.player-card__gauge-track {
  stroke: rgb(var(--border) / 0.68);
}

.player-card__gauge-fill {
  stroke: rgb(var(--primary));
  transition:
    stroke-dasharray 240ms ease,
    stroke 200ms ease;
}

.player-card__gauge-fill--none {
  stroke: rgb(var(--muted) / 0.5);
}

.player-card__gauge-fill--low {
  stroke: rgb(var(--danger));
}

.player-card__gauge-fill--mid {
  stroke: var(--accent-warning);
}

.player-card__gauge-fill--high,
.player-card__gauge-fill--hit {
  stroke: rgb(var(--success));
}

.player-card__store {
  display: grid;
  gap: 0.4rem;
  padding: 0.5rem 0.65rem;
  border-radius: 0.7rem;
  background: rgb(var(--surface-2) / 0.6);
  border: 1px solid rgb(var(--border) / 0.6);
}

.player-card__store-payout {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.3rem;
  font-size: 0.74rem;
  color: rgb(var(--muted) / 0.95);
}

/* Ícone "i" da previsão em amarelo (destaque), junto do valor a receber. */
.player-card__payout-info :deep(.app-info-popover__trigger) {
  color: var(--accent-warning);
  border-color: var(--accent-warning);
  background: rgb(var(--surface-2) / 0.6);
}

.player-card__payout-info :deep(.app-info-popover__trigger:hover),
.player-card__payout-info :deep(.app-info-popover__trigger[aria-expanded='true']) {
  color: var(--accent-warning);
  border-color: var(--accent-warning);
}

.player-card__store-payout strong {
  color: rgb(var(--success));
  font-weight: 800;
}

.player-card__gauge-value {
  fill: rgb(var(--text) / 0.96);
  font-size: 22px;
  font-weight: 700;
}

.player-card__gauge-caption {
  fill: rgb(var(--muted) / 0.92);
  font-size: 11px;
  font-weight: 500;
}

.player-card__gauge-side {
  display: grid;
  gap: 0.75rem;
}

.player-card__goal-copy {
  position: relative;
  display: grid;
  gap: 0.25rem;
  /* padding-right: 2.6rem; */
}

.player-card__goal-line {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0.3rem 0.45rem;
}

.player-card__goal-line .player-card__sold-caption::before {
  content: '·';
  margin-right: 0.45rem;
  color: rgb(var(--muted) / 0.6);
}

.player-card__goal-alerts {
  display: flex;
  flex-wrap: wrap;
  gap: 0.3rem;
}

.player-card__sold-amount {
  font-size: 1.15rem;
  font-weight: 700;
  color: rgb(var(--text) / 0.96);
}

.player-card__sold-of-goal,
.player-card__sold-caption {
  font-size: 0.78rem;
  color: rgb(var(--muted) / 0.92);
}

.player-card__source-badge {
  position: absolute;
  top: 0.42rem;
  right: 0.45rem;
  display: inline-flex;
  align-items: center;
  min-height: 1rem;
  padding: 0.1rem 0.32rem;
  border-radius: 999px;
  background: rgb(var(--primary) / 0.14);
  color: rgb(var(--primary));
  font-size: 0.58rem;
  font-weight: 800;
  line-height: 1;
}

.player-card__footer {
  display: flex;
  justify-content: flex-end;
}

.player-card__details-btn {
  padding: 0.45rem 0.95rem;
  border-radius: 0.6rem;
  border: 1px solid rgb(var(--ring) / 0.42);
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary));
  font-size: 0.78rem;
  font-weight: 600;
  cursor: pointer;
  transition: background 120ms ease;
}

.player-card__details-btn:hover {
  background: rgb(var(--primary) / 0.24);
}

@media (max-width: 720px) {
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
}
</style>
