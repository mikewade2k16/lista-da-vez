<script setup lang="ts">
import { computed } from 'vue'
import { useMetaAdsStore } from '~/stores/meta-ads'

const store = useMetaAdsStore()

const currency = computed(() => store.selectedAdAccount?.currency || 'BRL')

const numberFmt = new Intl.NumberFormat('pt-BR', { maximumFractionDigits: 0 })

function formatMoney(value: number): string {
  try {
    return new Intl.NumberFormat('pt-BR', {
      style: 'currency',
      currency: currency.value,
      maximumFractionDigits: 2,
    }).format(value)
  } catch {
    // Moeda invalida/desconhecida — cai para numero puro sem quebrar a tela.
    return numberFmt.format(value)
  }
}

function formatInt(value: number): string {
  return numberFmt.format(value)
}

function formatPercent(value: number): string {
  return `${new Intl.NumberFormat('pt-BR', { maximumFractionDigits: 2 }).format(value)}%`
}

interface Tile {
  key: string
  label: string
  value: string
}

const tiles = computed<Tile[]>(() => {
  const kpis = store.kpis
  if (!kpis) return []
  return [
    { key: 'spend', label: 'Investimento', value: formatMoney(kpis.spend) },
    { key: 'impressions', label: 'Impressoes', value: formatInt(kpis.impressions) },
    { key: 'clicks', label: 'Cliques', value: formatInt(kpis.clicks) },
    { key: 'ctr', label: 'CTR', value: formatPercent(kpis.ctr) },
    { key: 'cpc', label: 'CPC', value: formatMoney(kpis.cpc) },
    { key: 'conversions', label: 'Conversoes', value: formatInt(kpis.conversions) },
  ]
})

const showSkeleton = computed(() => store.pending && !store.kpis)
</script>

<template>
  <section class="ma-overview" aria-label="Indicadores principais">
    <div v-if="showSkeleton" class="ma-overview__grid">
      <div v-for="n in 6" :key="n" class="ma-overview__tile ma-overview__tile--skeleton">
        <span class="ma-overview__skeleton-line ma-overview__skeleton-line--label"></span>
        <span class="ma-overview__skeleton-line ma-overview__skeleton-line--value"></span>
      </div>
    </div>

    <div v-else-if="tiles.length" class="ma-overview__grid">
      <article v-for="tile in tiles" :key="tile.key" class="ma-overview__tile">
        <span class="ma-overview__label">{{ tile.label }}</span>
        <span class="ma-overview__value">{{ tile.value }}</span>
      </article>
    </div>

    <p v-else class="ma-overview__empty">
      Nenhum indicador ainda. Sincronize para puxar as metricas da Meta.
    </p>
  </section>
</template>

<style scoped>
.ma-overview__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 0.9rem;
}

.ma-overview__tile {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
  padding: 1.1rem 1.25rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.7);
  box-shadow: var(--shadow-card);
}

.ma-overview__label {
  font-size: 0.7rem;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.ma-overview__value {
  font-size: 1.5rem;
  font-weight: 700;
  letter-spacing: -0.01em;
  color: var(--text-main);
}

.ma-overview__empty {
  font-size: 0.88rem;
  color: var(--text-muted);
  padding: 1.1rem 1.25rem;
  border: 1px dashed var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface-2) / 0.4);
}

/* Skeleton de carregamento */
.ma-overview__tile--skeleton {
  gap: 0.7rem;
}

.ma-overview__skeleton-line {
  display: block;
  height: 0.7rem;
  border-radius: 999px;
  background: rgb(var(--surface-2));
  animation: ma-overview-pulse 1.2s ease-in-out infinite;
}

.ma-overview__skeleton-line--label {
  width: 55%;
}

.ma-overview__skeleton-line--value {
  width: 80%;
  height: 1.4rem;
}

@keyframes ma-overview-pulse {
  0%,
  100% {
    opacity: 0.5;
  }
  50% {
    opacity: 1;
  }
}
</style>
