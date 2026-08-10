<script setup lang="ts">
import type { PerformanceFeedbackHistoryItem } from '~/types/performance-feedback'

defineProps<{
  items: PerformanceFeedbackHistoryItem[]
}>()

const currency = new Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL' })
const number = new Intl.NumberFormat('pt-BR', { maximumFractionDigits: 2 })

const statusLabels: Record<string, string> = {
  draft: 'Rascunho',
  shared: 'Compartilhado',
  acknowledged: 'Concluído',
}
</script>

<template>
  <section class="pf-history">
    <header>
      <div>
        <span class="pf-section-eyebrow">Evolução</span>
        <h2>Histórico de ciclos</h2>
      </div>
      <p>Os snapshots preservam o resultado observado em cada reunião.</p>
    </header>

    <div v-if="!items.length" class="pf-history__empty">
      <UIcon name="i-lucide-history" />
      <span>O primeiro ciclo salvo aparecerá aqui para comparação.</span>
    </div>
    <div v-else class="pf-history__list">
      <article v-for="item in items" :key="`${item.period.month}-${item.period.week}`">
        <div class="pf-history__period">
          <strong>{{ item.period.label }}</strong>
          <span>{{ statusLabels[item.status] }}</span>
        </div>
        <dl>
          <div>
            <dt>Vendido</dt>
            <dd>{{ currency.format(item.metrics.soldValue) }}</dd>
          </div>
          <div>
            <dt>Ticket</dt>
            <dd>{{ currency.format(item.metrics.ticketAverage) }}</dd>
          </div>
          <div>
            <dt>P.A.</dt>
            <dd>{{ number.format(item.metrics.paScore) }}</dd>
          </div>
          <div>
            <dt>Conversão</dt>
            <dd>{{ number.format(item.metrics.conversionRate) }}%</dd>
          </div>
          <div>
            <dt>Transcrição</dt>
            <dd>
              {{
                item.metrics.transcriptionScore === undefined
                  ? '—'
                  : `${number.format(item.metrics.transcriptionScore)}/10`
              }}
            </dd>
          </div>
        </dl>
      </article>
    </div>
  </section>
</template>
