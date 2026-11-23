<script setup lang="ts">
import CardapioAnalyticsChart from '~/components/cardapio/analytics/CardapioAnalyticsChart.vue'
import type { ObsSystem } from '~/types/observability'

// Gráficos de série curta (goroutines, heap, pool, conexões do Postgres).
const props = defineProps<{ samples: ObsSystem['api'] }>()

const MB = 1024 * 1024
const categories = computed(() =>
  props.samples.map((s) =>
    new Date(s.at).toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' }),
  ),
)

function round(value: number | undefined, factor = 1): number | null {
  return Number.isFinite(value) ? Math.round(((value as number) / factor) * 10) / 10 : null
}

const charts = computed(() => [
  {
    title: 'Goroutines',
    series: [
      { name: 'Goroutines', data: props.samples.map((s) => s.metrics.go?.goroutines ?? null) },
    ],
  },
  {
    title: 'Heap (MB)',
    series: [
      {
        name: 'Heap em uso',
        data: props.samples.map((s) => round(s.metrics.go?.heapAllocBytes, MB)),
      },
    ],
  },
  {
    title: 'Pool de conexões (% utilizado)',
    series: [
      {
        name: 'Utilização',
        data: props.samples.map((s) =>
          Number.isFinite(s.metrics.pool?.utilization)
            ? Math.round((s.metrics.pool!.utilization as number) * 1000) / 10
            : null,
        ),
      },
    ],
    options: { yaxis: { min: 0, max: 100 } },
  },
  {
    title: 'Conexões do Postgres',
    series: [
      { name: 'Conexões', data: props.samples.map((s) => s.metrics.pg?.connections ?? null) },
      { name: 'Ativas', data: props.samples.map((s) => s.metrics.pg?.active ?? null) },
    ],
  },
])
</script>

<template>
  <div class="obs-system__charts">
    <section v-for="chart in charts" :key="chart.title" class="obs-section">
      <h3 class="obs-section__title">{{ chart.title }}</h3>
      <CardapioAnalyticsChart
        type="line"
        :series="chart.series"
        :categories="categories"
        :height="200"
        :options="chart.options"
      />
    </section>
  </div>
</template>

<style>
.obs-system__charts {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(22rem, 1fr));
  gap: 1rem;
}
</style>
