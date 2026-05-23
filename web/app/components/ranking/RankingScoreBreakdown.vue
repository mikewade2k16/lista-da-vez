<script setup lang="ts">
import { computed } from 'vue'
import { useGamificationConfig, scoreBreakdown } from '~/composables/useGamificationConfig'

interface BreakdownRow {
  conversionRate: number
  soldValue: number
  qualityScore: number
  paScore: number
  queueJumpServices: number
  attendances: number
}

const props = defineProps<{
  row: BreakdownRow
  maxSold: number
  maxPa: number
}>()

const { scoreWeights } = useGamificationConfig()

const breakdown = computed(() =>
  scoreBreakdown(props.row, {
    maxSold: props.maxSold,
    maxPa: props.maxPa,
    weights: scoreWeights.value,
  }),
)

const totalScore = computed(() =>
  breakdown.value.reduce((sum, item) => sum + item.contribution, 0),
)

const totalWeight = computed(() =>
  breakdown.value.reduce((sum, item) => sum + item.weight, 0),
)

const segments = computed(() =>
  breakdown.value.map((item) => ({
    ...item,
    sharePct: totalWeight.value > 0 ? (item.weight / totalWeight.value) * 100 : 0,
    contributionPct:
      item.weight > 0 ? (item.contribution / item.weight) * 100 : 0,
  })),
)
</script>

<template>
  <section class="score-breakdown" data-testid="score-breakdown">
    <header class="score-breakdown__header">
      <h3 class="score-breakdown__title">Score 360</h3>
      <strong class="score-breakdown__total">{{ totalScore.toFixed(1) }}</strong>
    </header>

    <div class="score-breakdown__bar" role="img" aria-label="Composição do Score 360">
      <span
        v-for="segment in segments"
        :key="segment.key"
        class="score-breakdown__segment"
        :class="`score-breakdown__segment--${segment.key}`"
        :style="{ width: `${segment.sharePct}%` }"
        :title="`${segment.label} — peso ${segment.weight}%, contribuição ${segment.contribution.toFixed(2)} pts`"
      ></span>
    </div>

    <ul class="score-breakdown__legend">
      <li v-for="segment in segments" :key="segment.key" class="score-breakdown__legend-item">
        <span
          class="score-breakdown__legend-dot"
          :class="`score-breakdown__legend-dot--${segment.key}`"
        ></span>
        <span class="score-breakdown__legend-label">{{ segment.label }}</span>
        <span class="score-breakdown__legend-weight">{{ segment.weight }}%</span>
        <span class="score-breakdown__legend-contribution">
          {{ segment.contribution.toFixed(2) }} pts ({{ segment.contributionPct.toFixed(0) }}% do peso)
        </span>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.score-breakdown {
  display: grid;
  gap: 0.75rem;
}

.score-breakdown__header {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
}

.score-breakdown__title {
  margin: 0;
  font-size: 0.95rem;
  color: rgba(248, 250, 252, 0.96);
}

.score-breakdown__total {
  font-size: 1.4rem;
  color: rgba(248, 250, 252, 0.96);
}

.score-breakdown__bar {
  display: flex;
  width: 100%;
  height: 0.85rem;
  border-radius: 999px;
  overflow: hidden;
  background: rgba(148, 163, 184, 0.18);
}

.score-breakdown__segment {
  display: block;
  height: 100%;
}

.score-breakdown__segment--conversion,
.score-breakdown__legend-dot--conversion {
  background: #6366f1;
}

.score-breakdown__segment--soldValue,
.score-breakdown__legend-dot--soldValue {
  background: #22c55e;
}

.score-breakdown__segment--quality,
.score-breakdown__legend-dot--quality {
  background: #f59e0b;
}

.score-breakdown__segment--pa,
.score-breakdown__legend-dot--pa {
  background: #ec4899;
}

.score-breakdown__segment--queueDiscipline,
.score-breakdown__legend-dot--queueDiscipline {
  background: #14b8a6;
}

.score-breakdown__legend {
  display: grid;
  gap: 0.4rem;
  margin: 0;
  padding: 0;
  list-style: none;
}

.score-breakdown__legend-item {
  display: grid;
  grid-template-columns: 0.65rem minmax(0, 1fr) auto auto;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.78rem;
  color: rgba(226, 232, 240, 0.88);
}

.score-breakdown__legend-dot {
  width: 0.65rem;
  height: 0.65rem;
  border-radius: 999px;
}

.score-breakdown__legend-weight {
  font-weight: 600;
  color: rgba(148, 163, 184, 0.92);
}

.score-breakdown__legend-contribution {
  color: rgba(148, 163, 184, 0.88);
  font-size: 0.72rem;
}
</style>
