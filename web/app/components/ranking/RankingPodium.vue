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

interface PodiumRow {
  key: string
  name: string
  subtitle?: string
  metricValue: number
  score360?: number
  soldValue?: number
}

const props = defineProps<{
  rows: PodiumRow[]
  metric: MetricKey
}>()

const emit = defineEmits<{
  (e: 'select', rowKey: string): void
}>()

const ordered = computed(() => {
  const rows = props.rows.slice(0, 3)
  return {
    first: rows[0] || null,
    second: rows[1] || null,
    third: rows[2] || null,
  }
})

function formatValue(value: number): string {
  switch (props.metric) {
    case 'soldValue':
      return formatCurrencyBRL(value)
    case 'ticketAverage':
      return formatCurrencyBRL(value)
    case 'conversionRate':
    case 'qualityScore':
      return formatPercent(value)
    case 'paScore':
      return value.toFixed(2)
    default:
      return value.toFixed(1)
  }
}

function handleSelect(rowKey: string) {
  emit('select', rowKey)
}
</script>

<template>
  <div class="ranking-podium" data-testid="ranking-podium">
    <button
      v-if="ordered.second"
      type="button"
      class="ranking-podium__slot ranking-podium__slot--second"
      :data-testid="`ranking-podium-second`"
      @click="handleSelect(ordered.second.key)"
    >
      <span class="ranking-podium__medal" aria-hidden="true">🥈</span>
      <span class="ranking-podium__name">{{ ordered.second.name }}</span>
      <span v-if="ordered.second.subtitle" class="ranking-podium__subtitle">
        {{ ordered.second.subtitle }}
      </span>
      <strong class="ranking-podium__value">{{ formatValue(ordered.second.metricValue) }}</strong>
      <div class="ranking-podium__pedestal ranking-podium__pedestal--silver">2</div>
    </button>
    <div v-else class="ranking-podium__slot ranking-podium__slot--placeholder"></div>

    <button
      v-if="ordered.first"
      type="button"
      class="ranking-podium__slot ranking-podium__slot--first"
      :data-testid="`ranking-podium-first`"
      @click="handleSelect(ordered.first.key)"
    >
      <span class="ranking-podium__medal" aria-hidden="true">🥇</span>
      <span class="ranking-podium__name">{{ ordered.first.name }}</span>
      <span v-if="ordered.first.subtitle" class="ranking-podium__subtitle">
        {{ ordered.first.subtitle }}
      </span>
      <strong class="ranking-podium__value">{{ formatValue(ordered.first.metricValue) }}</strong>
      <div class="ranking-podium__pedestal ranking-podium__pedestal--gold">1</div>
    </button>
    <div v-else class="ranking-podium__slot ranking-podium__slot--placeholder"></div>

    <button
      v-if="ordered.third"
      type="button"
      class="ranking-podium__slot ranking-podium__slot--third"
      :data-testid="`ranking-podium-third`"
      @click="handleSelect(ordered.third.key)"
    >
      <span class="ranking-podium__medal" aria-hidden="true">🥉</span>
      <span class="ranking-podium__name">{{ ordered.third.name }}</span>
      <span v-if="ordered.third.subtitle" class="ranking-podium__subtitle">
        {{ ordered.third.subtitle }}
      </span>
      <strong class="ranking-podium__value">{{ formatValue(ordered.third.metricValue) }}</strong>
      <div class="ranking-podium__pedestal ranking-podium__pedestal--bronze">3</div>
    </button>
    <div v-else class="ranking-podium__slot ranking-podium__slot--placeholder"></div>
  </div>
</template>

<style scoped>
.ranking-podium {
  display: grid;
  grid-template-columns: 1fr 1.2fr 1fr;
  align-items: end;
  gap: 0.75rem;
  padding: 1.5rem 1rem 0.5rem 1rem;
}

.ranking-podium__slot {
  display: grid;
  gap: 0.35rem;
  padding: 0.85rem 0.75rem 0;
  border: none;
  background: transparent;
  text-align: center;
  cursor: pointer;
  color: rgba(226, 232, 240, 0.92);
}

.ranking-podium__slot--placeholder {
  cursor: default;
}

.ranking-podium__medal {
  font-size: 2rem;
  line-height: 1;
}

.ranking-podium__name {
  font-size: 0.85rem;
  font-weight: 700;
  color: rgba(248, 250, 252, 0.96);
  word-break: break-word;
}

.ranking-podium__subtitle {
  font-size: 0.7rem;
  color: rgba(148, 163, 184, 0.92);
}

.ranking-podium__value {
  margin-top: 0.25rem;
  font-size: 1.05rem;
  color: rgba(248, 250, 252, 0.96);
}

.ranking-podium__pedestal {
  margin-top: 0.5rem;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 1.6rem;
  font-weight: 800;
  border-radius: 0.5rem 0.5rem 0 0;
  color: rgba(15, 23, 42, 0.85);
}

.ranking-podium__pedestal--gold {
  background: linear-gradient(180deg, #facc15, #ca8a04);
  height: 7rem;
}

.ranking-podium__pedestal--silver {
  background: linear-gradient(180deg, #cbd5e1, #94a3b8);
  height: 5.5rem;
}

.ranking-podium__pedestal--bronze {
  background: linear-gradient(180deg, #fb923c, #c2410c);
  height: 4.5rem;
}

.ranking-podium__slot:hover .ranking-podium__pedestal,
.ranking-podium__slot:focus-visible .ranking-podium__pedestal {
  filter: brightness(1.08);
  outline: none;
}

@media (max-width: 640px) {
  .ranking-podium {
    grid-template-columns: 1fr;
    padding: 0.5rem 0;
  }

  .ranking-podium__pedestal {
    height: auto !important;
    padding: 0.45rem 0.85rem;
    margin-top: 0.35rem;
    border-radius: 0.4rem;
  }
}
</style>
