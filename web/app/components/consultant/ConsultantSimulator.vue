<script setup lang="ts">
import { computed } from 'vue'
import { formatCurrencyBRL, formatPercent } from '~/domain/utils/admin-metrics'

const props = withDefaults(
  defineProps<{
    soldValue: number
    monthlyGoal: number
    commissionRate: number
    simulationAdditionalSales: number
    // Taxa efetiva (%) do payout pré-calculado no back. Quando presente, o
    // recebimento estimado usa (venda+adicional)*ratePercent/100. A lógica de faixa
    // (trava/penalidade) é do back; aqui é só uma ESTIMATIVA linear na faixa atual.
    payoutRatePercent?: number | null
  }>(),
  {
    payoutRatePercent: null,
  },
)

const emit = defineEmits<{
  (e: 'update:simulationAdditionalSales', value: number): void
}>()

const projectedSales = computed(() => props.soldValue + props.simulationAdditionalSales)
const projectedGoalPercent = computed(() =>
  props.monthlyGoal ? (projectedSales.value / props.monthlyGoal) * 100 : 0,
)
const projectedCommission = computed(() => projectedSales.value * props.commissionRate)

const hasPayoutRate = computed(() => typeof props.payoutRatePercent === 'number')
const projectedPayout = computed(() =>
  hasPayoutRate.value ? (projectedSales.value * (props.payoutRatePercent || 0)) / 100 : 0,
)

function handleInput(event: Event) {
  const target = event.target as HTMLInputElement | null
  emit('update:simulationAdditionalSales', Number(target?.value || 0))
}
</script>

<template>
  <section class="simulator" data-testid="consultant-simulator">
    <h3 class="simulator__title">Simulador de fechamento</h3>
    <label class="simulator__field">
      <span>Venda adicional para simular (R$)</span>
      <input
        class="simulator__input"
        type="number"
        min="0"
        step="100"
        data-testid="consultant-simulator-input"
        :value="simulationAdditionalSales"
        @input="handleInput"
      />
    </label>
    <div class="metric-grid metric-grid--tight">
      <article class="metric-card metric-card--soft">
        <span class="metric-card__label">Vendido projetado</span>
        <strong class="metric-card__value">{{ formatCurrencyBRL(projectedSales) }}</strong>
        <span class="metric-card__text">{{ formatPercent(projectedGoalPercent) }} da meta.</span>
      </article>
      <article class="metric-card metric-card--soft">
        <span class="metric-card__label">Comissao projetada</span>
        <strong class="metric-card__value">{{ formatCurrencyBRL(projectedCommission) }}</strong>
        <span class="metric-card__text">Com base na taxa atual.</span>
      </article>
      <article v-if="hasPayoutRate" class="metric-card metric-card--soft">
        <span class="metric-card__label">Recebimento estimado</span>
        <strong class="metric-card__value">{{ formatCurrencyBRL(projectedPayout) }}</strong>
        <span class="metric-card__text">
          Estimativa: {{ formatPercent(payoutRatePercent || 0) }} da venda na faixa atual.
        </span>
      </article>
    </div>
  </section>
</template>
