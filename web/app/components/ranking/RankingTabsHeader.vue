<script setup lang="ts">
export type RankingScope = 'consultants' | 'stores' | 'per-store'
export type RankingMetric =
  | 'score360'
  | 'soldValue'
  | 'conversionRate'
  | 'ticketAverage'
  | 'paScore'
  | 'qualityScore'

interface SortOption {
  key: RankingMetric
  label: string
}

defineProps<{
  scope: RankingScope
  metric: RankingMetric
  integratedScope?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:scope', value: RankingScope): void
  (e: 'update:metric', value: RankingMetric): void
}>()

const SORT_OPTIONS: SortOption[] = [
  { key: 'score360', label: 'Score 360' },
  { key: 'soldValue', label: 'Valor' },
  { key: 'conversionRate', label: 'Conversão' },
  { key: 'ticketAverage', label: 'Ticket' },
  { key: 'paScore', label: 'P.A.' },
  { key: 'qualityScore', label: 'Qualidade' },
]

function selectScope(next: RankingScope) {
  emit('update:scope', next)
}

function selectMetric(next: RankingMetric) {
  emit('update:metric', next)
}
</script>

<template>
  <header class="ranking-tabs-header" data-testid="ranking-tabs-header">
    <div class="ranking-tabs-header__scope" role="tablist" aria-label="Tipo de ranking">
      <button
        v-if="integratedScope"
        type="button"
        class="ranking-tabs-header__scope-btn"
        :class="{ 'ranking-tabs-header__scope-btn--active': scope === 'stores' }"
        role="tab"
        :aria-selected="scope === 'stores'"
        data-testid="ranking-scope-stores"
        @click="selectScope('stores')"
      >
        Lojas
      </button>
      <button
        type="button"
        class="ranking-tabs-header__scope-btn"
        :class="{ 'ranking-tabs-header__scope-btn--active': scope === 'consultants' }"
        role="tab"
        :aria-selected="scope === 'consultants'"
        data-testid="ranking-scope-consultants"
        @click="selectScope('consultants')"
      >
        Consultores
      </button>
      <button
        v-if="integratedScope"
        type="button"
        class="ranking-tabs-header__scope-btn"
        :class="{ 'ranking-tabs-header__scope-btn--active': scope === 'per-store' }"
        role="tab"
        :aria-selected="scope === 'per-store'"
        data-testid="ranking-scope-per-store"
        @click="selectScope('per-store')"
      >
        Por loja
      </button>
    </div>

    <div class="ranking-tabs-header__sort" role="tablist" aria-label="Critério de ordenação">
      <span class="ranking-tabs-header__sort-label">Sort:</span>
      <button
        v-for="option in SORT_OPTIONS"
        :key="option.key"
        type="button"
        class="ranking-tabs-header__sort-chip"
        :class="{ 'ranking-tabs-header__sort-chip--active': metric === option.key }"
        :data-testid="`ranking-sort-${option.key}`"
        @click="selectMetric(option.key)"
      >
        <span v-if="option.key === 'score360'" aria-hidden="true">★</span>
        {{ option.label }}
      </button>
    </div>
  </header>
</template>

<style scoped>
.ranking-tabs-header {
  display: grid;
  gap: 0.65rem;
  padding-bottom: 0.5rem;
}

.ranking-tabs-header__scope {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 0.25rem;
  padding: 0.25rem;
  border-radius: 999px;
  background: rgb(var(--surface-2) / 0.74);
  border: 1px solid rgb(var(--border) / 0.78);
  width: max-content;
  max-width: 100%;
}

.ranking-tabs-header__scope-btn {
  padding: 0.4rem 0.95rem;
  border-radius: 999px;
  border: none;
  background: transparent;
  color: rgb(var(--muted) / 0.92);
  font-size: 0.78rem;
  font-weight: 600;
  cursor: pointer;
}

.ranking-tabs-header__scope-btn--active {
  background: rgb(var(--primary) / 0.18);
  color: rgb(var(--primary));
}

.ranking-tabs-header__sort {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.35rem;
}

.ranking-tabs-header__sort-label {
  font-size: 0.72rem;
  color: rgb(var(--muted) / 0.88);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.ranking-tabs-header__sort-chip {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  padding: 0.3rem 0.75rem;
  border-radius: 999px;
  border: 1px solid rgb(var(--border) / 0.82);
  background: transparent;
  color: rgb(var(--muted) / 0.92);
  font-size: 0.74rem;
  font-weight: 600;
  cursor: pointer;
}

.ranking-tabs-header__sort-chip--active {
  background: rgb(var(--primary) / 0.18);
  border-color: rgb(var(--ring) / 0.42);
  color: rgb(var(--primary));
}
</style>
