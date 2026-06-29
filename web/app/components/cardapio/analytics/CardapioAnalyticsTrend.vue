<script setup lang="ts">
import { computed } from 'vue'
import type { ApexOptions } from 'apexcharts'

import CardapioAnalyticsCard from './CardapioAnalyticsCard.vue'
import CardapioAnalyticsChart from './CardapioAnalyticsChart.vue'
import type { AnalyticsTimeseries } from '~/domain/cardapio/analytics'

// Bloco Tendencia — mapeia `timeseries?granularity=day`. Area de Visitas
// (pageviews) + linha de Pedidos no mesmo eixo de datas. NAO faz fetch: recebe a
// serie densa por dia (data/loading/error) e emite retry.

const props = defineProps<{
  data: AnalyticsTimeseries | null
  loading?: boolean
  error?: string
}>()

const emit = defineEmits<{ (e: 'retry'): void }>()

const points = computed(() => props.data?.points ?? [])
const isEmpty = computed(() =>
  points.value.every((point) => point.pageviews === 0 && point.orders === 0),
)

// Categorias = datas curtas (dd/mm) a partir do bucket YYYY-MM-DD.
const categories = computed(() =>
  points.value.map((point) => {
    const bucket = String(point.bucket ?? '')
    if (bucket.length < 10) {
      return bucket
    }
    return `${bucket.slice(8, 10)}/${bucket.slice(5, 7)}`
  }),
)

const series = computed<ApexOptions['series']>(() => [
  { name: 'Visitas', type: 'area', data: points.value.map((point) => point.pageviews) },
  { name: 'Pedidos', type: 'line', data: points.value.map((point) => point.orders) },
])

const chartOptions = computed<ApexOptions>(() => ({
  stroke: { curve: 'smooth', width: [2, 2] },
  fill: {
    type: ['gradient', 'solid'],
    gradient: { opacityFrom: 0.35, opacityTo: 0.05, shadeIntensity: 0.4 },
  },
}))
</script>

<template>
  <CardapioAnalyticsCard
    title="Tendencia"
    subtitle="Visitas e pedidos por dia"
    :loading="loading"
    :error="error"
    :empty="!loading && !error && isEmpty"
    @retry="emit('retry')"
  >
    <CardapioAnalyticsChart
      type="area"
      :series="series"
      :categories="categories"
      :options="chartOptions"
      :height="320"
    />
  </CardapioAnalyticsCard>
</template>
