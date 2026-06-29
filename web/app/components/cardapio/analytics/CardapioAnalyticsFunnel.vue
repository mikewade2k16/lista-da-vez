<script setup lang="ts">
import { computed } from 'vue'

import CardapioAnalyticsCard from './CardapioAnalyticsCard.vue'
import {
  formatAnalyticsInt,
  formatAnalyticsRate,
  type AnalyticsFunnel,
  type AnalyticsFunnelStep,
} from '~/domain/cardapio/analytics'

// Bloco Funil — mapeia `funnel`. Barras decrescentes (CSS puro, sem lib de funil)
// com % vs o topo e a queda entre etapas. NAO faz fetch: recebe steps por prop.
// rateFromStart = largura da barra (vs primeira etapa); rateFromPrev = retencao
// vs etapa anterior (a queda = 1 - rateFromPrev).

const props = defineProps<{
  data: AnalyticsFunnel | null
  loading?: boolean
  error?: string
}>()

const emit = defineEmits<{ (e: 'retry'): void }>()

const steps = computed<AnalyticsFunnelStep[]>(() => props.data?.steps ?? [])
const isEmpty = computed(
  () => steps.value.length === 0 || steps.value.every((s) => s.sessions === 0),
)

// Largura proporcional (minimo 4% para a barra nunca sumir quando ha algum dado).
function barWidth(step: AnalyticsFunnelStep): string {
  const fraction = Math.max(0, Math.min(1, step.rateFromStart))
  return `${Math.max(fraction * 100, fraction > 0 ? 4 : 0)}%`
}

// Queda vs etapa anterior. A primeira etapa nao tem queda (index 0).
function dropLabel(step: AnalyticsFunnelStep, index: number): string {
  if (index === 0) {
    return ''
  }
  const drop = Math.max(0, 1 - step.rateFromPrev)
  return `-${formatAnalyticsRate(drop)}`
}
</script>

<template>
  <CardapioAnalyticsCard
    title="Funil de conversao"
    subtitle="Da visita ao pedido, por sessao"
    :loading="loading"
    :error="error"
    :empty="!loading && !error && isEmpty"
    @retry="emit('retry')"
  >
    <ol class="cardapio-analytics-funnel">
      <li v-for="(step, index) in steps" :key="step.key" class="cardapio-analytics-funnel__step">
        <div class="cardapio-analytics-funnel__row">
          <span class="cardapio-analytics-funnel__label">{{ step.label }}</span>
          <span class="cardapio-analytics-funnel__count">
            {{ formatAnalyticsInt(step.sessions) }}
            <span class="cardapio-analytics-funnel__pct">
              ({{ formatAnalyticsRate(step.rateFromStart) }})
            </span>
          </span>
        </div>
        <div class="cardapio-analytics-funnel__track">
          <div class="cardapio-analytics-funnel__bar" :style="{ width: barWidth(step) }"></div>
        </div>
        <span v-if="index > 0" class="cardapio-analytics-funnel__drop">
          {{ dropLabel(step, index) }} vs etapa anterior
        </span>
      </li>
    </ol>
  </CardapioAnalyticsCard>
</template>

<style scoped>
.cardapio-analytics-funnel {
  display: flex;
  flex-direction: column;
  gap: 0.85rem;
  list-style: none;
  margin: 0;
  padding: 0;
}

.cardapio-analytics-funnel__step {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
}

.cardapio-analytics-funnel__row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.75rem;
}

.cardapio-analytics-funnel__label {
  font-size: 0.86rem;
  font-weight: 600;
  color: var(--text-main);
}

.cardapio-analytics-funnel__count {
  font-size: 0.86rem;
  font-weight: 700;
  color: var(--text-main);
  white-space: nowrap;
}

.cardapio-analytics-funnel__pct {
  font-weight: 500;
  color: var(--text-muted);
}

.cardapio-analytics-funnel__track {
  height: 0.85rem;
  border-radius: 999px;
  background: rgb(var(--surface-2) / 0.7);
  overflow: hidden;
}

.cardapio-analytics-funnel__bar {
  height: 100%;
  border-radius: 999px;
  background: rgb(var(--primary));
  transition: width 0.3s ease;
}

.cardapio-analytics-funnel__drop {
  font-size: 0.74rem;
  color: var(--text-muted);
}
</style>
