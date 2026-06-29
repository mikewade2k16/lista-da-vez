<script setup lang="ts">
import { computed } from 'vue'

import CardapioAnalyticsCard from './CardapioAnalyticsCard.vue'
import {
  formatAnalyticsDuration,
  formatAnalyticsInt,
  type AnalyticsPages,
} from '~/domain/cardapio/analytics'

// Bloco Paginas mais vistas — mapeia `pages?limit=`. Tabela: caminho da pagina +
// visualizacoes + tempo medio (mm:ss). Complementa o Dwell (que e por dimensao):
// aqui o foco e o RANKING de paginas por views. NAO faz fetch.

const props = defineProps<{
  data: AnalyticsPages | null
  loading?: boolean
  error?: string
}>()

const emit = defineEmits<{ (e: 'retry'): void }>()

const items = computed(() => props.data?.items ?? [])
const isEmpty = computed(() => items.value.length === 0)
</script>

<template>
  <CardapioAnalyticsCard
    title="Paginas mais vistas"
    subtitle="Visualizacoes e tempo medio por pagina"
    :loading="loading"
    :error="error"
    :empty="!loading && !error && isEmpty"
    @retry="emit('retry')"
  >
    <div class="cardapio-analytics-pages__scroll">
      <table class="cardapio-analytics-pages__table">
        <thead>
          <tr>
            <th scope="col">Pagina</th>
            <th scope="col" class="cardapio-analytics-pages__num">Visualizacoes</th>
            <th scope="col" class="cardapio-analytics-pages__num">Tempo medio</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(item, index) in items" :key="`${item.pagePath}-${index}`">
            <td>
              <span class="cardapio-analytics-pages__path">{{ item.pagePath || '/' }}</span>
            </td>
            <td class="cardapio-analytics-pages__num cardapio-analytics-pages__num--strong">
              {{ formatAnalyticsInt(item.views) }}
            </td>
            <td class="cardapio-analytics-pages__num">
              {{ formatAnalyticsDuration(item.avgDwellSeconds) }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </CardapioAnalyticsCard>
</template>

<style scoped>
.cardapio-analytics-pages__scroll {
  overflow-x: auto;
}

.cardapio-analytics-pages__table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.85rem;
}

.cardapio-analytics-pages__table th {
  padding: 0.5rem 0.65rem;
  text-align: left;
  font-size: 0.7rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-muted);
  border-bottom: 1px solid var(--line-soft);
}

.cardapio-analytics-pages__table td {
  padding: 0.55rem 0.65rem;
  border-bottom: 1px solid var(--line-soft);
  color: var(--text-main);
}

.cardapio-analytics-pages__num {
  text-align: right;
  white-space: nowrap;
}

.cardapio-analytics-pages__num--strong {
  font-weight: 700;
}

.cardapio-analytics-pages__path {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.8rem;
}
</style>
