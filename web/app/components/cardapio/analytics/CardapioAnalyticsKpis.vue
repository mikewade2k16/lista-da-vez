<script setup lang="ts">
import { computed } from 'vue'

import CoreErrorState from '../../../../layers/core/components/CoreErrorState.vue'
import { formatCurrency } from '~/domain/cardapio/types'
import {
  formatAnalyticsDuration,
  formatAnalyticsInt,
  formatAnalyticsRate,
  type AnalyticsOverview,
} from '~/domain/cardapio/analytics'

// Bloco KPIs — mapeia o endpoint `overview`. Linha de tiles (skeleton no loading,
// vazio honesto sem dado). NAO faz fetch: recebe overview/loading/error por prop e
// emite retry. Espelha o MetaAdsOverviewCard (grid auto-fit + skeleton pulsante).

const props = defineProps<{
  data: AnalyticsOverview | null
  loading?: boolean
  error?: string
}>()

const emit = defineEmits<{ (e: 'retry'): void }>()

interface Tile {
  key: string
  label: string
  value: string
  hint?: string
}

const tiles = computed<Tile[]>(() => {
  const overview = props.data
  if (!overview) {
    return []
  }
  return [
    { key: 'pageviews', label: 'Visitas', value: formatAnalyticsInt(overview.pageviews) },
    { key: 'sessions', label: 'Sessoes', value: formatAnalyticsInt(overview.sessions) },
    { key: 'devices', label: 'Visitantes', value: formatAnalyticsInt(overview.uniqueDevices) },
    { key: 'orders', label: 'Pedidos', value: formatAnalyticsInt(overview.orders) },
    { key: 'conversion', label: 'Conversao', value: formatAnalyticsRate(overview.conversionRate) },
    { key: 'ticket', label: 'Ticket medio', value: formatCurrency(overview.avgTicketCents) },
    {
      key: 'session-time',
      label: 'Tempo medio sessao',
      value: formatAnalyticsDuration(overview.avgSessionSeconds),
    },
    {
      key: 'new-returning',
      label: 'Novos vs recorrentes',
      value: `${formatAnalyticsInt(overview.newSessions)} / ${formatAnalyticsInt(
        overview.returningSessions,
      )}`,
      hint: 'novos / recorrentes',
    },
    {
      key: 'abandonment',
      label: 'Abandono de sacola',
      value: formatAnalyticsRate(overview.cartAbandonmentRate),
    },
  ]
})

const showSkeleton = computed(() => props.loading && !props.data)
const showEmpty = computed(() => !props.loading && !props.error && tiles.value.length === 0)
</script>

<template>
  <section class="cardapio-analytics-kpis" aria-label="Indicadores principais">
    <CoreErrorState
      v-if="error"
      compact
      :message="error"
      retry-label="Tentar de novo"
      @retry="emit('retry')"
    />

    <div v-else-if="showSkeleton" class="cardapio-analytics-kpis__grid">
      <div
        v-for="n in 9"
        :key="n"
        class="cardapio-analytics-kpis__tile cardapio-analytics-kpis__tile--skeleton"
      >
        <span
          class="cardapio-analytics-kpis__skeleton cardapio-analytics-kpis__skeleton--label"
        ></span>
        <span
          class="cardapio-analytics-kpis__skeleton cardapio-analytics-kpis__skeleton--value"
        ></span>
      </div>
    </div>

    <p v-else-if="showEmpty" class="cardapio-analytics-kpis__empty">Sem dados neste periodo.</p>

    <div v-else class="cardapio-analytics-kpis__grid">
      <article v-for="tile in tiles" :key="tile.key" class="cardapio-analytics-kpis__tile">
        <span class="cardapio-analytics-kpis__label">{{ tile.label }}</span>
        <span class="cardapio-analytics-kpis__value">{{ tile.value }}</span>
        <span v-if="tile.hint" class="cardapio-analytics-kpis__hint">{{ tile.hint }}</span>
      </article>
    </div>
  </section>
</template>

<style scoped>
.cardapio-analytics-kpis__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 0.9rem;
}

.cardapio-analytics-kpis__tile {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  padding: 1.05rem 1.2rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.7);
  box-shadow: var(--shadow-card);
}

.cardapio-analytics-kpis__label {
  font-size: 0.68rem;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.cardapio-analytics-kpis__value {
  font-size: 1.42rem;
  font-weight: 700;
  letter-spacing: -0.01em;
  color: var(--text-main);
}

.cardapio-analytics-kpis__hint {
  font-size: 0.7rem;
  color: var(--text-muted);
}

.cardapio-analytics-kpis__empty {
  font-size: 0.86rem;
  color: var(--text-muted);
  padding: 1.05rem 1.2rem;
  border: 1px dashed var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface-2) / 0.4);
}

.cardapio-analytics-kpis__tile--skeleton {
  gap: 0.65rem;
}

.cardapio-analytics-kpis__skeleton {
  display: block;
  height: 0.7rem;
  border-radius: 999px;
  background: rgb(var(--surface-2));
  animation: cardapio-analytics-kpis-pulse 1.2s ease-in-out infinite;
}

.cardapio-analytics-kpis__skeleton--label {
  width: 55%;
}

.cardapio-analytics-kpis__skeleton--value {
  width: 80%;
  height: 1.35rem;
}

@keyframes cardapio-analytics-kpis-pulse {
  0%,
  100% {
    opacity: 0.5;
  }
  50% {
    opacity: 1;
  }
}
</style>
