<script setup lang="ts">
import { computed } from 'vue'
import { formatCurrencyBRL, formatPercent } from '~/domain/utils/admin-metrics'

type MetricKey =
  | 'score360'
  | 'soldValue'
  | 'conversionRate'
  | 'ticketAverage'
  | 'paScore'
  | 'qualityScore'

const props = defineProps<{
  rowKey: string
  position: number
  name: string
  subtitle?: string
  metric: MetricKey
  metricValue: number
  goal?: number | null
  progressPercent?: number | null
  variation?: number | null
}>()

const emit = defineEmits<{
  (e: 'select', rowKey: string): void
}>()

const formattedMetricValue = computed(() => {
  switch (props.metric) {
    case 'soldValue':
    case 'ticketAverage':
      return formatCurrencyBRL(props.metricValue)
    case 'conversionRate':
    case 'qualityScore':
      return formatPercent(props.metricValue)
    case 'paScore':
      return props.metricValue.toFixed(2)
    default:
      return props.metricValue.toFixed(1)
  }
})

const clampedProgress = computed(() => {
  const value = props.progressPercent ?? 0
  return Math.min(100, Math.max(0, value))
})

const showProgress = computed(
  () => typeof props.goal === 'number' && props.goal > 0 && typeof props.progressPercent === 'number',
)

const variationLabel = computed(() => {
  if (typeof props.variation !== 'number' || props.variation === 0) return null
  const arrow = props.variation > 0 ? '↑' : '↓'
  return `${arrow} ${Math.abs(props.variation).toFixed(1)}%`
})

const variationClass = computed(() => {
  if (typeof props.variation !== 'number' || props.variation === 0) return ''
  return props.variation > 0 ? 'leaderboard-card__variation--up' : 'leaderboard-card__variation--down'
})

function handleClick() {
  emit('select', props.rowKey)
}
</script>

<template>
  <button
    type="button"
    class="leaderboard-card"
    :data-testid="`leaderboard-card-${rowKey}`"
    @click="handleClick"
  >
    <span class="leaderboard-card__position">{{ position }}</span>
    <div class="leaderboard-card__main">
      <div class="leaderboard-card__name-row">
        <strong class="leaderboard-card__name">{{ name }}</strong>
        <span v-if="subtitle" class="leaderboard-card__subtitle">{{ subtitle }}</span>
      </div>
      <div v-if="showProgress" class="leaderboard-card__progress">
        <div class="leaderboard-card__progress-bar">
          <span
            class="leaderboard-card__progress-fill"
            :style="{ width: `${clampedProgress}%` }"
          ></span>
        </div>
        <span class="leaderboard-card__progress-text">
          Meta {{ formatPercent(progressPercent ?? 0) }}
        </span>
      </div>
    </div>
    <div class="leaderboard-card__metric-block">
      <strong class="leaderboard-card__metric-value">{{ formattedMetricValue }}</strong>
      <span v-if="variationLabel" class="leaderboard-card__variation" :class="variationClass">
        {{ variationLabel }}
      </span>
    </div>
  </button>
</template>

<style scoped>
.leaderboard-card {
  display: grid;
  grid-template-columns: 2rem minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.85rem;
  padding: 0.7rem 0.95rem;
  border: 1px solid rgba(125, 146, 255, 0.16);
  border-radius: 0.85rem;
  background: rgba(13, 19, 36, 0.72);
  color: rgba(226, 232, 240, 0.92);
  text-align: left;
  cursor: pointer;
  transition: border-color 120ms ease, transform 120ms ease;
}

.leaderboard-card:hover,
.leaderboard-card:focus-visible {
  border-color: rgba(125, 146, 255, 0.42);
  transform: translateY(-1px);
  outline: none;
}

.leaderboard-card__position {
  font-size: 1.1rem;
  font-weight: 700;
  color: rgba(148, 163, 184, 0.88);
  text-align: center;
}

.leaderboard-card__main {
  display: grid;
  gap: 0.3rem;
  min-width: 0;
}

.leaderboard-card__name-row {
  display: flex;
  align-items: baseline;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.leaderboard-card__name {
  font-size: 0.92rem;
  color: rgba(248, 250, 252, 0.96);
}

.leaderboard-card__subtitle {
  font-size: 0.7rem;
  color: rgba(148, 163, 184, 0.92);
}

.leaderboard-card__progress {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.5rem;
}

.leaderboard-card__progress-bar {
  position: relative;
  height: 0.35rem;
  background: rgba(148, 163, 184, 0.18);
  border-radius: 999px;
  overflow: hidden;
}

.leaderboard-card__progress-fill {
  position: absolute;
  top: 0;
  left: 0;
  bottom: 0;
  background: #6366f1;
}

.leaderboard-card__progress-text {
  font-size: 0.68rem;
  color: rgba(148, 163, 184, 0.88);
  white-space: nowrap;
}

.leaderboard-card__metric-block {
  display: grid;
  justify-items: end;
  gap: 0.2rem;
}

.leaderboard-card__metric-value {
  font-size: 1.05rem;
  color: rgba(248, 250, 252, 0.96);
}

.leaderboard-card__variation {
  font-size: 0.68rem;
  font-weight: 700;
  padding: 0.1rem 0.4rem;
  border-radius: 999px;
}

.leaderboard-card__variation--up {
  background: rgba(34, 197, 94, 0.14);
  color: #86efac;
}

.leaderboard-card__variation--down {
  background: rgba(248, 113, 113, 0.14);
  color: #fca5a5;
}
</style>
