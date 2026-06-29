<script setup lang="ts">
import { computed, ref } from 'vue'

import { useCardapioStore } from '~/stores/cardapio'
import { useCardapioAnalytics } from '~/composables/useCardapioAnalytics'
import { ANALYTICS_DEFAULT_RANGE_DAYS, type AnalyticsRange } from '~/domain/cardapio/analytics'
import CardapioAnalyticsToolbar from '~/components/cardapio/analytics/CardapioAnalyticsToolbar.vue'
import CardapioAnalyticsKpis from '~/components/cardapio/analytics/CardapioAnalyticsKpis.vue'
import CardapioAnalyticsTrend from '~/components/cardapio/analytics/CardapioAnalyticsTrend.vue'
import CardapioAnalyticsHours from '~/components/cardapio/analytics/CardapioAnalyticsHours.vue'
import CardapioAnalyticsFunnel from '~/components/cardapio/analytics/CardapioAnalyticsFunnel.vue'
import CardapioAnalyticsTopProducts from '~/components/cardapio/analytics/CardapioAnalyticsTopProducts.vue'
import CardapioAnalyticsTraffic from '~/components/cardapio/analytics/CardapioAnalyticsTraffic.vue'
import CardapioAnalyticsDevices from '~/components/cardapio/analytics/CardapioAnalyticsDevices.vue'
import CardapioAnalyticsDwell from '~/components/cardapio/analytics/CardapioAnalyticsDwell.vue'
import CardapioAnalyticsPages from '~/components/cardapio/analytics/CardapioAnalyticsPages.vue'
import CardapioAnalyticsClicks from '~/components/cardapio/analytics/CardapioAnalyticsClicks.vue'

// Orquestrador fino da aba Relatorios (F4). NAO faz fetch nem logica: monta o
// periodo (default ultimos 7 dias), injeta useCardapioAnalytics (unica camada de
// fetch, herda withScope + X-Account-Id do store) e repassa data/loading/error a
// cada bloco. O restaurantId vem do store (o editor ja carregou o restaurante e
// fixou o scopeAccountId) — sem re-resolver escopo. Layout em grid de 12 colunas
// que colapsa para 1 coluna em <=880px (espelha o body do editor).

const store = useCardapioStore()

// Fonte autoritativa do escopo: o restaurante ativo do editor.
const restaurantId = computed(() => store.restaurantId)

// Periodo default = ultimos N dias (decisao F4: 7). YYYY-MM-DD local.
function toIso(date: Date): string {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function defaultRange(): AnalyticsRange {
  const to = new Date()
  const from = new Date()
  from.setDate(from.getDate() - (ANALYTICS_DEFAULT_RANGE_DAYS - 1))
  return { from: toIso(from), to: toIso(to) }
}

const range = ref<AnalyticsRange>(defaultRange())

const analytics = useCardapioAnalytics(restaurantId, range)

// Estado agregado para o botao Atualizar do toolbar (qualquer bloco carregando).
const anyLoading = computed(
  () =>
    analytics.overview.pending.value ||
    analytics.trend.pending.value ||
    analytics.hours.pending.value ||
    analytics.funnel.pending.value ||
    analytics.topProducts.pending.value ||
    analytics.sources.pending.value ||
    analytics.devices.pending.value ||
    analytics.pages.pending.value ||
    analytics.dwell.pending.value ||
    analytics.clicks.pending.value,
)

function onRangeUpdate(next: AnalyticsRange) {
  range.value = next
}

function onRefresh() {
  void analytics.refresh()
}
</script>

<template>
  <section class="cardapio-analytics" aria-label="Relatorios">
    <header class="cardapio-analytics__header">
      <div class="cardapio-analytics__heading">
        <h2 class="cardapio-analytics__title">Relatorios</h2>
        <p class="cardapio-analytics__subtitle">
          Acessos, paginas, produtos e conversao do site publico no periodo.
        </p>
      </div>
      <CardapioAnalyticsToolbar
        :range="range"
        :loading="anyLoading"
        @update:range="onRangeUpdate"
        @refresh="onRefresh"
      />
    </header>

    <!-- KPIs: linha cheia (proprio shell com skeleton/erro/vazio). -->
    <CardapioAnalyticsKpis
      :data="analytics.overview.data.value"
      :loading="analytics.overview.pending.value"
      :error="analytics.overview.error.value"
      @retry="analytics.refreshOverview"
    />

    <div class="cardapio-analytics__grid">
      <div class="cardapio-analytics__cell cardapio-analytics__cell--wide">
        <CardapioAnalyticsTrend
          :data="analytics.trend.data.value"
          :loading="analytics.trend.pending.value"
          :error="analytics.trend.error.value"
          @retry="analytics.refreshTrend"
        />
      </div>

      <div class="cardapio-analytics__cell">
        <CardapioAnalyticsFunnel
          :data="analytics.funnel.data.value"
          :loading="analytics.funnel.pending.value"
          :error="analytics.funnel.error.value"
          @retry="analytics.refreshFunnel"
        />
      </div>

      <div class="cardapio-analytics__cell cardapio-analytics__cell--wide">
        <CardapioAnalyticsHours
          v-model:granularity="analytics.hoursGranularity.value"
          :data="analytics.hours.data.value"
          :loading="analytics.hours.pending.value"
          :error="analytics.hours.error.value"
          @retry="analytics.refreshHours"
        />
      </div>

      <div class="cardapio-analytics__cell">
        <CardapioAnalyticsTraffic
          v-model:dimension="analytics.sourcesDimension.value"
          :data="analytics.sources.data.value"
          :loading="analytics.sources.pending.value"
          :error="analytics.sources.error.value"
          @retry="analytics.refreshSources"
        />
      </div>

      <div class="cardapio-analytics__cell cardapio-analytics__cell--wide">
        <CardapioAnalyticsTopProducts
          v-model:metric="analytics.topProductsMetric.value"
          :data="analytics.topProducts.data.value"
          :loading="analytics.topProducts.pending.value"
          :error="analytics.topProducts.error.value"
          @retry="analytics.refreshTopProducts"
        />
      </div>

      <div class="cardapio-analytics__cell">
        <CardapioAnalyticsDwell
          v-model:dimension="analytics.dwellDimension.value"
          :data="analytics.dwell.data.value"
          :loading="analytics.dwell.pending.value"
          :error="analytics.dwell.error.value"
          @retry="analytics.refreshDwell"
        />
      </div>

      <div class="cardapio-analytics__cell">
        <CardapioAnalyticsPages
          :data="analytics.pages.data.value"
          :loading="analytics.pages.pending.value"
          :error="analytics.pages.error.value"
          @retry="analytics.refreshPages"
        />
      </div>

      <div class="cardapio-analytics__cell">
        <CardapioAnalyticsClicks
          :data="analytics.clicks.data.value"
          :loading="analytics.clicks.pending.value"
          :error="analytics.clicks.error.value"
          @retry="analytics.refreshClicks"
        />
      </div>

      <div class="cardapio-analytics__cell cardapio-analytics__cell--wide">
        <CardapioAnalyticsDevices
          :data="analytics.devices.data.value"
          :loading="analytics.devices.pending.value"
          :error="analytics.devices.error.value"
          @retry="analytics.refreshDevices"
        />
      </div>
    </div>
  </section>
</template>

<style scoped>
.cardapio-analytics {
  display: flex;
  flex-direction: column;
  gap: 1.1rem;
  min-width: 0;
}

.cardapio-analytics__header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 1rem;
  flex-wrap: wrap;
}

.cardapio-analytics__title {
  font-size: 1.25rem;
  font-weight: 700;
  color: var(--text-main);
}

.cardapio-analytics__subtitle {
  margin-top: 0.2rem;
  font-size: 0.85rem;
  color: var(--text-muted);
}

/* Grid de 12 colunas: cada cell ocupa 6 (metade); --wide ocupa 12 (linha cheia).
   Colapsa para 1 coluna em <=880px (espelha o body do editor). */
.cardapio-analytics__grid {
  display: grid;
  grid-template-columns: repeat(12, minmax(0, 1fr));
  gap: 1.1rem;
  align-items: start;
}

.cardapio-analytics__cell {
  grid-column: span 6;
  min-width: 0;
}

.cardapio-analytics__cell--wide {
  grid-column: span 12;
}

@media (max-width: 880px) {
  .cardapio-analytics__grid {
    grid-template-columns: 1fr;
  }

  .cardapio-analytics__cell,
  .cardapio-analytics__cell--wide {
    grid-column: 1 / -1;
  }
}
</style>
