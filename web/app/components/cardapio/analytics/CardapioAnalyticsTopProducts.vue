<script setup lang="ts">
import { computed } from 'vue'

import CardapioAnalyticsCard from './CardapioAnalyticsCard.vue'
import {
  formatAnalyticsInt,
  formatAnalyticsRate,
  type AnalyticsTopProducts,
  type AnalyticsTopProductsMetric,
} from '~/domain/cardapio/analytics'

// Bloco Top produtos — mapeia `top-products?metric=`. Tabela: Produto (name/slug),
// Vistos, Pedidos, Conversao e a coluna da metrica selecionada. O seletor de
// metrica e controlado pelo pai (v-model:metric) para o composable re-buscar.

const props = defineProps<{
  data: AnalyticsTopProducts | null
  metric: AnalyticsTopProductsMetric
  loading?: boolean
  error?: string
}>()

const emit = defineEmits<{
  (e: 'retry'): void
  (e: 'update:metric', value: AnalyticsTopProductsMetric): void
}>()

const METRIC_OPTIONS: Array<{ value: AnalyticsTopProductsMetric; label: string }> = [
  { value: 'orders', label: 'Pedidos' },
  { value: 'viewed', label: 'Vistos' },
  { value: 'clicked', label: 'Clicados' },
  { value: 'add_to_cart', label: 'Add ao carrinho' },
]

const items = computed(() => props.data?.items ?? [])
const isEmpty = computed(() => items.value.length === 0)

const metricLabel = computed(
  () => METRIC_OPTIONS.find((option) => option.value === props.metric)?.label ?? 'Metrica',
)

function onMetricChange(event: Event) {
  emit('update:metric', (event.target as HTMLSelectElement).value as AnalyticsTopProductsMetric)
}
</script>

<template>
  <CardapioAnalyticsCard
    title="Top produtos"
    subtitle="Vistos, pedidos e conversao por produto"
    :loading="loading"
    :error="error"
    :empty="!loading && !error && isEmpty"
    @retry="emit('retry')"
  >
    <template #actions>
      <label class="cardapio-analytics-top__metric">
        <span class="cardapio-analytics-top__metric-label">Ordenar por</span>
        <select class="cardapio-analytics-top__select" :value="metric" @change="onMetricChange">
          <option v-for="option in METRIC_OPTIONS" :key="option.value" :value="option.value">
            {{ option.label }}
          </option>
        </select>
      </label>
    </template>

    <div class="cardapio-analytics-top__scroll">
      <table class="cardapio-analytics-top__table">
        <thead>
          <tr>
            <th scope="col">Produto</th>
            <th scope="col" class="cardapio-analytics-top__num">{{ metricLabel }}</th>
            <th scope="col" class="cardapio-analytics-top__num">Vistos</th>
            <th scope="col" class="cardapio-analytics-top__num">Pedidos</th>
            <th scope="col" class="cardapio-analytics-top__num">Conversao</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in items" :key="item.productSlug">
            <td>
              <span class="cardapio-analytics-top__name">{{ item.name || item.productSlug }}</span>
              <span class="cardapio-analytics-top__slug">{{ item.productSlug }}</span>
            </td>
            <td class="cardapio-analytics-top__num cardapio-analytics-top__num--strong">
              {{ formatAnalyticsInt(item.count) }}
            </td>
            <td class="cardapio-analytics-top__num">{{ formatAnalyticsInt(item.viewed) }}</td>
            <td class="cardapio-analytics-top__num">{{ formatAnalyticsInt(item.orders) }}</td>
            <td class="cardapio-analytics-top__num">
              {{ formatAnalyticsRate(item.conversionRate) }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </CardapioAnalyticsCard>
</template>

<style scoped>
.cardapio-analytics-top__metric {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
}

.cardapio-analytics-top__metric-label {
  font-size: 0.74rem;
  font-weight: 600;
  color: var(--text-muted);
}

.cardapio-analytics-top__select {
  min-height: 2rem;
  padding: 0 0.5rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface) / 0.9);
  color: var(--text-main);
  font-size: 0.82rem;
  cursor: pointer;
}

.cardapio-analytics-top__scroll {
  overflow-x: auto;
}

.cardapio-analytics-top__table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.85rem;
}

.cardapio-analytics-top__table th {
  padding: 0.5rem 0.65rem;
  text-align: left;
  font-size: 0.7rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-muted);
  border-bottom: 1px solid var(--line-soft);
}

.cardapio-analytics-top__table td {
  padding: 0.55rem 0.65rem;
  border-bottom: 1px solid var(--line-soft);
  color: var(--text-main);
  vertical-align: top;
}

.cardapio-analytics-top__num {
  text-align: right;
  white-space: nowrap;
}

.cardapio-analytics-top__num--strong {
  font-weight: 700;
}

.cardapio-analytics-top__name {
  display: block;
  font-weight: 600;
}

.cardapio-analytics-top__slug {
  display: block;
  font-size: 0.72rem;
  color: var(--text-muted);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}
</style>
