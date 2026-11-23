<script setup lang="ts">
import CardapioAnalyticsChart from '~/components/cardapio/analytics/CardapioAnalyticsChart.vue'
import ObservabilityState from '~/components/admin/observability/ObservabilityState.vue'
import { useObsQuery } from '~/composables/useObservability'
import type { ObsOverview } from '~/types/observability'
import {
  errorRateBand,
  formatInt,
  formatMs,
  formatPercentValue,
  formatRate,
  latencyBand,
  scoreBand,
} from '~/utils/observability/format'

const props = defineProps<{ days: number }>()

const { data, loading, error, reload } = useObsQuery<ObsOverview>('/overview', () => ({
  days: props.days,
}))

const modules = computed(() =>
  [...(data.value?.modules || [])].sort((a, b) => (a.score ?? 0) - (b.score ?? 0)),
)
const trend = computed(() => data.value?.trend || [])
const isEmpty = computed(
  () => !data.value || (modules.value.length === 0 && !(data.value.live?.requests > 0)),
)

const trendModules = computed(() => {
  const names = new Set<string>()
  trend.value.forEach((point) => Object.keys(point.scores || {}).forEach((name) => names.add(name)))
  return Array.from(names).sort()
})
const trendSeries = computed(() =>
  trendModules.value.map((name) => ({
    name,
    data: trend.value.map((point) => point.scores?.[name] ?? null),
  })),
)
const trendCategories = computed(() =>
  trend.value.map((point) => {
    const date = new Date(point.startedAt)
    return Number.isNaN(date.getTime())
      ? point.runId
      : date.toLocaleDateString('pt-BR', { day: '2-digit', month: '2-digit' })
  }),
)
const averageCoverage = computed(() => {
  const values = modules.value.map((m) => m.coverage).filter((v): v is number => Number.isFinite(v))
  return values.length ? values.reduce((a, b) => a + b, 0) / values.length : null
})
</script>

<template>
  <ObservabilityState
    :loading="loading"
    :error="error"
    :empty="isEmpty"
    empty-title="Ainda nao ha dados de observabilidade."
    empty-hint="Rode o runner com `npm run obs:run` e use o painel por alguns minutos para gerar trafego medido."
    @retry="reload"
  >
    <div v-if="data" class="obs-stack">
      <div class="obs-cards">
        <article class="obs-card">
          <span class="obs-card__label">Instrumentacao</span>
          <strong class="obs-card__value">
            <span :class="['obs-band', `obs-band--${data.live.enabled ? 'good' : 'bad'}`]">
              {{ data.live.enabled ? 'Ligada' : 'Desligada' }}
            </span>
          </strong>
        </article>
        <article class="obs-card">
          <span class="obs-card__label">Requisicoes</span>
          <strong class="obs-card__value">{{ formatInt(data.live.requests) }}</strong>
        </article>
        <article class="obs-card">
          <span class="obs-card__label">Taxa de erro</span>
          <strong :class="['obs-card__value', `obs-text--${errorRateBand(data.live.errorRate)}`]">
            {{ formatRate(data.live.errorRate) }}
          </strong>
        </article>
        <article class="obs-card">
          <span class="obs-card__label">p95</span>
          <strong :class="['obs-card__value', `obs-text--${latencyBand(data.live.p95Ms)}`]">
            {{ formatMs(data.live.p95Ms) }}
          </strong>
        </article>
        <article class="obs-card">
          <span class="obs-card__label">Eventos descartados</span>
          <strong :class="['obs-card__value', data.live.droppedEvents > 0 ? 'obs-text--warn' : '']">
            {{ formatInt(data.live.droppedEvents) }}
          </strong>
        </article>
        <article class="obs-card">
          <span class="obs-card__label">Cobertura media</span>
          <strong class="obs-card__value">{{ formatPercentValue(averageCoverage) }}</strong>
        </article>
      </div>

      <section class="obs-section">
        <h3 class="obs-section__title">Nota por modulo</h3>
        <p v-if="modules.length === 0" class="obs-hint">
          Nenhuma nota ainda. As notas saem do runner: `npm run obs:run`.
        </p>
        <div v-else class="obs-tablewrap">
          <table class="obs-table">
            <thead>
              <tr>
                <th>Modulo</th>
                <th class="obs-num">Nota</th>
                <th class="obs-num">Cobertura</th>
                <th>Componentes</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in modules" :key="item.module">
                <td>{{ item.module }}</td>
                <td class="obs-num">
                  <span :class="['obs-band', `obs-band--${scoreBand(item.score)}`]">
                    {{ formatInt(item.score) }}
                  </span>
                </td>
                <td class="obs-num">{{ formatPercentValue(item.coverage) }}</td>
                <td class="obs-muted">
                  <span v-for="(value, name) in item.components || {}" :key="name" class="obs-chip">
                    {{ name }} {{ formatInt(value) }}
                  </span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="obs-section">
        <h3 class="obs-section__title">Tendencia das notas por execucao</h3>
        <p v-if="trend.length < 2" class="obs-hint">
          A tendencia aparece a partir de duas execucoes do runner.
        </p>
        <CardapioAnalyticsChart
          v-else
          type="line"
          :series="trendSeries"
          :categories="trendCategories"
          :height="280"
          :options="{ yaxis: { min: 0, max: 100 } }"
        />
      </section>
    </div>
  </ObservabilityState>
</template>
