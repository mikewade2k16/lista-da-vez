<script setup lang="ts">
import { computed } from 'vue'
import { useMetaAdsStore } from '~/stores/meta-ads'

const store = useMetaAdsStore()

const currency = computed(() => store.selectedAdAccount?.currency || 'BRL')

function formatBudget(value: number | null): string {
  if (value === null || value === undefined) return '—'
  try {
    return new Intl.NumberFormat('pt-BR', {
      style: 'currency',
      currency: currency.value,
      maximumFractionDigits: 2,
    }).format(value)
  } catch {
    return new Intl.NumberFormat('pt-BR', { maximumFractionDigits: 2 }).format(value)
  }
}

// Mapeia o status da Meta (ACTIVE/PAUSED/...) para um modificador de badge.
function statusModifier(status: string): string {
  const normalized = status.toUpperCase()
  if (normalized === 'ACTIVE') return 'ma-campaigns__badge--active'
  if (normalized === 'PAUSED') return 'ma-campaigns__badge--paused'
  return 'ma-campaigns__badge--neutral'
}

function statusLabel(status: string): string {
  const normalized = status.toUpperCase()
  if (normalized === 'ACTIVE') return 'Ativa'
  if (normalized === 'PAUSED') return 'Pausada'
  return status || '—'
}

const hasCampaigns = computed(() => store.campaigns.length > 0)
</script>

<template>
  <section class="ma-campaigns" aria-label="Campanhas">
    <header class="ma-campaigns__head">
      <h3 class="ma-campaigns__title">Campanhas</h3>
      <span v-if="hasCampaigns" class="ma-campaigns__count">{{ store.campaigns.length }}</span>
    </header>

    <div v-if="hasCampaigns" class="ma-campaigns__table-wrap">
      <table class="ma-campaigns__table">
        <thead>
          <tr>
            <th class="ma-campaigns__th">Nome</th>
            <th class="ma-campaigns__th">Objetivo</th>
            <th class="ma-campaigns__th">Status</th>
            <th class="ma-campaigns__th ma-campaigns__th--num">Orcamento diario</th>
            <th class="ma-campaigns__th ma-campaigns__th--num">Orcamento total</th>
            <!-- Coluna de acoes (editar/pausar) entra na fase Plataforma. -->
          </tr>
        </thead>
        <tbody>
          <tr v-for="campaign in store.campaigns" :key="campaign.id" class="ma-campaigns__row">
            <td class="ma-campaigns__td ma-campaigns__td--name">{{ campaign.name }}</td>
            <td class="ma-campaigns__td ma-campaigns__td--muted">
              {{ campaign.objective || '—' }}
            </td>
            <td class="ma-campaigns__td">
              <span class="ma-campaigns__badge" :class="statusModifier(campaign.status)">
                {{ statusLabel(campaign.status) }}
              </span>
            </td>
            <td class="ma-campaigns__td ma-campaigns__td--num">
              {{ formatBudget(campaign.dailyBudget) }}
            </td>
            <td class="ma-campaigns__td ma-campaigns__td--num">
              {{ formatBudget(campaign.lifetimeBudget) }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <p v-else class="ma-campaigns__empty">
      Nenhuma campanha no cache ainda. Sincronize para ver as campanhas da Meta.
    </p>
  </section>
</template>

<style scoped>
.ma-campaigns {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding: 1.4rem 1.5rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.7);
  box-shadow: var(--shadow-card);
}

.ma-campaigns__head {
  display: flex;
  align-items: center;
  gap: 0.6rem;
}

.ma-campaigns__title {
  font-size: 1.1rem;
  font-weight: 700;
}

.ma-campaigns__count {
  font-size: 0.72rem;
  font-weight: 700;
  padding: 0.05rem 0.5rem;
  border-radius: 999px;
  background: rgb(var(--primary) / 0.15);
  color: rgb(var(--primary));
}

.ma-campaigns__table-wrap {
  overflow-x: auto;
}

.ma-campaigns__table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.88rem;
}

.ma-campaigns__th {
  text-align: left;
  font-size: 0.68rem;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: var(--text-muted);
  padding: 0.55rem 0.85rem;
  border-bottom: 1px solid var(--line-soft);
  white-space: nowrap;
}

.ma-campaigns__th--num {
  text-align: right;
}

.ma-campaigns__row:hover {
  background: rgb(var(--surface-2) / 0.4);
}

.ma-campaigns__td {
  padding: 0.7rem 0.85rem;
  border-bottom: 1px solid var(--line-soft);
  color: var(--text-main);
  vertical-align: middle;
}

.ma-campaigns__td--name {
  font-weight: 600;
}

.ma-campaigns__td--muted {
  color: var(--text-muted);
}

.ma-campaigns__td--num {
  text-align: right;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.ma-campaigns__badge {
  display: inline-flex;
  align-items: center;
  font-size: 0.74rem;
  font-weight: 600;
  padding: 0.18rem 0.6rem;
  border-radius: 999px;
  border: 1px solid transparent;
}

.ma-campaigns__badge--active {
  color: rgb(var(--success));
  background: rgb(var(--success) / 0.12);
  border-color: rgb(var(--success) / 0.35);
}

.ma-campaigns__badge--paused {
  color: var(--text-muted);
  background: rgb(var(--surface-2) / 0.8);
  border-color: var(--line-soft);
}

.ma-campaigns__badge--neutral {
  color: rgb(var(--primary));
  background: rgb(var(--primary) / 0.12);
  border-color: rgb(var(--primary) / 0.3);
}

.ma-campaigns__empty {
  font-size: 0.88rem;
  color: var(--text-muted);
  padding: 1.1rem 1.25rem;
  border: 1px dashed var(--line-soft);
  border-radius: var(--radius-soft);
  background: rgb(var(--surface-2) / 0.4);
}
</style>
