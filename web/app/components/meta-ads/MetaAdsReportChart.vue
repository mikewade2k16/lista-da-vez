<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import type { ApexOptions } from 'apexcharts'
import { useMetaAdsStore } from '~/stores/meta-ads'
import { defineLazyComponent } from '~/utils/lazy-component'

const store = useMetaAdsStore()

// ApexCharts quebra no SSR — carregado dinamicamente e renderizado so no cliente
// (<ClientOnly> no template). O import dinamico tambem mantem a lib fora do
// bundle inicial (principio de performance / bundle enxuto).
const ApexChart = defineLazyComponent(() => import('vue3-apexcharts'))

// Le os tokens do design system em runtime (client-only) para o grafico seguir
// o tema. Sem isso, o Apex usa cores proprias que ignoram dark mode/rebranding.
const primaryColor = ref('#6366f1')
const successColor = ref('#22c55e')
const mutedColor = ref('#64748b')
const lineColor = ref('rgba(226, 232, 240, 0.72)')

function readToken(name: string, wrap: (raw: string) => string, fallback: string): string {
  if (typeof window === 'undefined') return fallback
  const raw = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  return raw ? wrap(raw) : fallback
}

onMounted(() => {
  primaryColor.value = readToken('--primary', (raw) => `rgb(${raw})`, primaryColor.value)
  successColor.value = readToken('--success', (raw) => `rgb(${raw})`, successColor.value)
  mutedColor.value = readToken('--muted', (raw) => `rgb(${raw})`, mutedColor.value)
  lineColor.value = readToken('--border', (raw) => `rgb(${raw} / 0.6)`, lineColor.value)
})

const hasData = computed(() => store.insights.length > 0)

const categories = computed(() => store.insights.map((point) => point.date))

const series = computed(() => [
  { name: 'Investimento', type: 'area', data: store.insights.map((point) => point.spend) },
  { name: 'CTR (%)', type: 'line', data: store.insights.map((point) => point.ctr) },
])

const chartOptions = computed<ApexOptions>(() => ({
  chart: {
    type: 'line',
    fontFamily: 'inherit',
    toolbar: { show: false },
    zoom: { enabled: false },
    background: 'transparent',
  },
  colors: [primaryColor.value, successColor.value],
  stroke: { curve: 'smooth', width: [2, 2] },
  fill: {
    type: ['gradient', 'solid'],
    gradient: { opacityFrom: 0.35, opacityTo: 0.05, shadeIntensity: 0.4 },
  },
  dataLabels: { enabled: false },
  grid: { borderColor: lineColor.value, strokeDashArray: 4 },
  legend: { labels: { colors: mutedColor.value } },
  xaxis: {
    categories: categories.value,
    labels: { style: { colors: mutedColor.value } },
    axisBorder: { color: lineColor.value },
    axisTicks: { color: lineColor.value },
  },
  yaxis: [
    {
      seriesName: 'Investimento',
      labels: { style: { colors: mutedColor.value } },
    },
    {
      seriesName: 'CTR (%)',
      opposite: true,
      labels: { style: { colors: mutedColor.value } },
    },
  ],
  tooltip: { theme: 'dark', shared: true },
}))
</script>

<template>
  <section class="ma-chart" aria-label="Tendencia de investimento e CTR">
    <header class="ma-chart__head">
      <h3 class="ma-chart__title">Tendencia</h3>
      <p class="ma-chart__subtitle">Investimento e CTR ao longo do periodo</p>
    </header>

    <div class="ma-chart__body">
      <ClientOnly>
        <ApexChart
          v-if="hasData"
          type="line"
          height="320"
          :options="chartOptions"
          :series="series"
        />
        <p v-else class="ma-chart__empty">
          Sem metricas no periodo. Sincronize para ver a tendencia.
        </p>
        <template #fallback>
          <div class="ma-chart__placeholder" aria-hidden="true"></div>
        </template>
      </ClientOnly>
    </div>
  </section>
</template>

<style scoped>
.ma-chart {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding: 1.4rem 1.5rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.7);
  box-shadow: var(--shadow-card);
}

.ma-chart__title {
  font-size: 1.1rem;
  font-weight: 700;
}

.ma-chart__subtitle {
  font-size: 0.82rem;
  color: var(--text-muted);
  margin-top: 0.15rem;
}

.ma-chart__body {
  min-height: 320px;
}

.ma-chart__empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 320px;
  font-size: 0.88rem;
  color: var(--text-muted);
  border: 1px dashed var(--line-soft);
  border-radius: var(--radius-soft);
  background: rgb(var(--surface-2) / 0.4);
}

.ma-chart__placeholder {
  height: 320px;
  border-radius: var(--radius-soft);
  background: rgb(var(--surface-2) / 0.5);
  animation: ma-chart-pulse 1.2s ease-in-out infinite;
}

@keyframes ma-chart-pulse {
  0%,
  100% {
    opacity: 0.5;
  }
  50% {
    opacity: 1;
  }
}
</style>
