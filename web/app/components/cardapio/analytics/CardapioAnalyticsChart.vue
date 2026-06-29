<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import type { ApexOptions } from 'apexcharts'

import { defineLazyComponent } from '~/utils/lazy-component'

// Wrapper generico de grafico do dashboard (F4). Espelha MetaAdsReportChart:
// ApexCharts via defineLazyComponent + <ClientOnly> (quebra no SSR e fica fora do
// bundle inicial) + leitura dos tokens do design system em RUNTIME (segue tema/
// dark mode); toolbar off; fundo transparente. Diferenca: aqui e REUTILIZAVEL —
// o tipo, as series e as categorias vem por prop (cada bloco monta as suas).
//
// As cores derivadas dos tokens (--primary/--success/--muted/--border) entram no
// base `colors`/`grid`/eixos; heatmap e donut herdam o --primary daqui sem precisar
// de cor propria. Quem so passa series/categories herda o tema sozinho.

type ChartType = 'line' | 'area' | 'bar' | 'heatmap' | 'donut'

const props = withDefaults(
  defineProps<{
    type: ChartType
    series: ApexOptions['series']
    categories?: Array<string | number>
    labels?: string[]
    height?: number
    options?: ApexOptions
  }>(),
  {
    categories: () => [],
    labels: () => [],
    height: 300,
  },
)

const ApexChart = defineLazyComponent(() => import('vue3-apexcharts'))

const primaryColor = ref('rgb(99 102 241)')
const successColor = ref('rgb(34 197 94)')
const mutedColor = ref('rgb(100 116 139)')
const lineColor = ref('rgb(226 232 240 / 0.6)')

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

// Base tema-aware comum a todos os tipos. O bloco sobrescreve/mescla via prop
// `options` (merge raso no topo + xaxis). Cores default = [primary, success].
const baseOptions = computed<ApexOptions>(() => ({
  chart: {
    type: props.type,
    fontFamily: 'inherit',
    toolbar: { show: false },
    zoom: { enabled: false },
    background: 'transparent',
  },
  colors: [primaryColor.value, successColor.value],
  dataLabels: { enabled: false },
  grid: { borderColor: lineColor.value, strokeDashArray: 4 },
  stroke: { curve: 'smooth', width: 2 },
  legend: { labels: { colors: mutedColor.value } },
  tooltip: { theme: 'dark', shared: true },
  ...(props.labels.length ? { labels: props.labels } : {}),
  xaxis: {
    categories: props.categories,
    labels: { style: { colors: mutedColor.value } },
    axisBorder: { color: lineColor.value },
    axisTicks: { color: lineColor.value },
  },
  yaxis: {
    labels: { style: { colors: mutedColor.value } },
  },
}))

const mergedOptions = computed<ApexOptions>(() => {
  const override = props.options ?? {}
  return {
    ...baseOptions.value,
    ...override,
    chart: { ...baseOptions.value.chart, ...(override.chart ?? {}) },
    xaxis: { ...baseOptions.value.xaxis, ...(override.xaxis ?? {}) },
  }
})

const heightPx = computed(() => String(props.height))
</script>

<template>
  <div class="cardapio-analytics-chart" :style="{ minHeight: `${height}px` }">
    <ClientOnly>
      <ApexChart :type="type" :height="heightPx" :options="mergedOptions" :series="series" />
      <template #fallback>
        <div
          class="cardapio-analytics-chart__placeholder"
          :style="{ height: `${height}px` }"
          aria-hidden="true"
        ></div>
      </template>
    </ClientOnly>
  </div>
</template>

<style scoped>
.cardapio-analytics-chart {
  width: 100%;
  min-width: 0;
}

.cardapio-analytics-chart__placeholder {
  border-radius: var(--radius-soft);
  background: rgb(var(--surface-2) / 0.5);
  animation: cardapio-analytics-chart-pulse 1.2s ease-in-out infinite;
}

@keyframes cardapio-analytics-chart-pulse {
  0%,
  100% {
    opacity: 0.5;
  }
  50% {
    opacity: 1;
  }
}
</style>
