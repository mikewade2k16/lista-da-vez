<script setup lang="ts">
import { computed } from 'vue'

import CardapioAnalyticsCard from './CardapioAnalyticsCard.vue'
import { formatAnalyticsInt, type AnalyticsClicks } from '~/domain/cardapio/analytics'

// Bloco Cliques — mapeia `clicks`. Tabela "quais botoes foram clicados":
// botao (label, com name/kind de apoio) + contagem. NAO faz fetch.

const props = defineProps<{
  data: AnalyticsClicks | null
  loading?: boolean
  error?: string
}>()

const emit = defineEmits<{ (e: 'retry'): void }>()

const items = computed(() => props.data?.items ?? [])
const isEmpty = computed(() => items.value.length === 0)

// label e o texto do botao; cai no name quando o label vier vazio.
function buttonLabel(label: string, name: string): string {
  return label || name || '(sem rotulo)'
}
</script>

<template>
  <CardapioAnalyticsCard
    title="Cliques em botoes"
    subtitle="Quais botoes foram clicados"
    :loading="loading"
    :error="error"
    :empty="!loading && !error && isEmpty"
    @retry="emit('retry')"
  >
    <div class="cardapio-analytics-clicks__scroll">
      <table class="cardapio-analytics-clicks__table">
        <thead>
          <tr>
            <th scope="col">Botao</th>
            <th scope="col">Tipo</th>
            <th scope="col" class="cardapio-analytics-clicks__num">Cliques</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(item, index) in items" :key="`${item.name}-${item.kind}-${index}`">
            <td>
              <span class="cardapio-analytics-clicks__label">
                {{ buttonLabel(item.label, item.name) }}
              </span>
              <span v-if="item.name && item.label" class="cardapio-analytics-clicks__name">
                {{ item.name }}
              </span>
            </td>
            <td>
              <span v-if="item.kind" class="cardapio-analytics-clicks__kind">{{ item.kind }}</span>
              <span v-else class="cardapio-analytics-clicks__muted">-</span>
            </td>
            <td class="cardapio-analytics-clicks__num">{{ formatAnalyticsInt(item.count) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </CardapioAnalyticsCard>
</template>

<style scoped>
.cardapio-analytics-clicks__scroll {
  overflow-x: auto;
}

.cardapio-analytics-clicks__table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.85rem;
}

.cardapio-analytics-clicks__table th {
  padding: 0.5rem 0.65rem;
  text-align: left;
  font-size: 0.7rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-muted);
  border-bottom: 1px solid var(--line-soft);
}

.cardapio-analytics-clicks__table td {
  padding: 0.55rem 0.65rem;
  border-bottom: 1px solid var(--line-soft);
  color: var(--text-main);
  vertical-align: top;
}

.cardapio-analytics-clicks__num {
  text-align: right;
  white-space: nowrap;
  font-weight: 700;
}

.cardapio-analytics-clicks__label {
  display: block;
  font-weight: 600;
}

.cardapio-analytics-clicks__name {
  display: block;
  font-size: 0.72rem;
  color: var(--text-muted);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}

.cardapio-analytics-clicks__kind {
  display: inline-block;
  padding: 0.1rem 0.5rem;
  border-radius: 999px;
  background: rgb(var(--primary) / 0.14);
  color: rgb(var(--primary));
  font-size: 0.74rem;
  font-weight: 600;
}

.cardapio-analytics-clicks__muted {
  color: var(--text-muted);
}
</style>
