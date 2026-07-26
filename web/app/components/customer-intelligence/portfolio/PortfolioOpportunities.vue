<script setup lang="ts">
import CustomerIntelligenceStatus from '~/components/customer-intelligence/CustomerIntelligenceStatus.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import type {
  PortfolioFilters,
  PortfolioOpportunityView,
} from '~/domain/customer-intelligence/portfolio-types'
import { usePortfolioOpportunities } from '~/composables/customer-intelligence/usePortfolioOpportunities'
import PortfolioOpportunityDrawer from './PortfolioOpportunityDrawer.vue'
import PortfolioPolicySummary from './PortfolioPolicySummary.vue'

const portfolio = usePortfolioOpportunities()
const filters = ref<PortfolioFilters>({})
const selected = ref<PortfolioOpportunityView | null>(null)
const drawerOpen = ref(false)

function setFilter(key: keyof PortfolioFilters, value: string): void {
  filters.value = { ...filters.value, [key]: value }
}

function open(opportunity: PortfolioOpportunityView): void {
  selected.value = opportunity
  drawerOpen.value = true
}

async function decide(input: {
  opportunity: PortfolioOpportunityView
  decision: 'approve' | 'reject'
  reasonCode: string
  reason: string
}): Promise<void> {
  const saved = await portfolio.decide(
    input.opportunity,
    input.decision,
    input.reasonCode,
    input.reason,
  )
  if (saved) drawerOpen.value = false
}
</script>

<template>
  <div class="portfolio-opportunities">
    <PortfolioPolicySummary :summary="portfolio.policySummary.value" />
    <div v-if="portfolio.descriptors.value.length" class="portfolio-filters">
      <AppSelectField
        v-for="descriptor in portfolio.descriptors.value"
        :key="descriptor.key"
        :model-value="String(filters[descriptor.key] ?? '')"
        :options="[{ value: '', label: 'Todos' }, ...descriptor.options]"
        :label="descriptor.label"
        compact
        @update:model-value="setFilter(descriptor.key, $event)"
      />
      <button type="button" :disabled="portfolio.loading.value" @click="portfolio.load(filters)">
        Aplicar
      </button>
    </div>

    <CustomerIntelligenceStatus
      v-if="portfolio.loading.value && !portfolio.items.value.length"
      title="Carregando oportunidades agregadas"
      loading
    />
    <CustomerIntelligenceStatus
      v-else-if="portfolio.error.value"
      title="Portfolio indisponivel"
      :error="portfolio.error.value"
      @retry="portfolio.load(filters)"
    />
    <CustomerIntelligenceStatus
      v-else-if="!portfolio.items.value.length"
      title="Sem oportunidades"
      empty
      empty-text="Nenhum agregado protegido foi retornado. Consultas suprimidas nao revelam contagens."
    />

    <div v-else class="portfolio-grid">
      <article v-for="opportunity in portfolio.items.value" :key="opportunity.id">
        <header>
          <small>{{ opportunity.status }} · {{ opportunity.purposeKey }}</small>
          <span>{{ opportunity.cohortSizeBucket || 'bucket protegido' }}</span>
        </header>
        <h3>{{ opportunity.title }}</h3>
        <p>{{ opportunity.summary }}</p>
        <footer>
          <span>{{ opportunity.targetClients.length }} clientes-alvo autorizados</span>
          <button type="button" @click="open(opportunity)">Revisar</button>
        </footer>
      </article>
    </div>
    <button
      v-if="portfolio.nextCursor.value"
      type="button"
      :disabled="portfolio.loading.value"
      @click="portfolio.load(filters, true)"
    >
      Carregar mais
    </button>
    <PortfolioOpportunityDrawer
      v-model:open="drawerOpen"
      :opportunity="selected"
      :can-manage="portfolio.access.canManagePortfolio.value"
      :busy="Boolean(portfolio.mutatingId.value)"
      :reasons="portfolio.decisionReasons.value"
      @decide="decide"
    />
  </div>
</template>

<style scoped>
.portfolio-opportunities,
.portfolio-grid {
  display: grid;
  gap: 1rem;
}

.portfolio-filters {
  display: grid;
  grid-template-columns: repeat(4, minmax(9rem, 1fr)) auto;
  align-items: end;
  gap: 0.65rem;
}

.portfolio-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.portfolio-grid article {
  display: grid;
  gap: 0.65rem;
  padding: 1rem;
  border: 1px solid rgb(var(--border) / 0.8);
  border-radius: 0.9rem;
}

.portfolio-grid header,
.portfolio-grid footer {
  display: flex;
  justify-content: space-between;
  gap: 0.65rem;
}

.portfolio-grid h3,
.portfolio-grid p {
  margin: 0;
}

.portfolio-grid small,
.portfolio-grid span,
.portfolio-grid p {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

@media (max-width: 900px) {
  .portfolio-filters,
  .portfolio-grid {
    grid-template-columns: 1fr;
  }
}
</style>
