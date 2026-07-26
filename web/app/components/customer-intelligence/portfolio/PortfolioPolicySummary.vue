<script setup lang="ts">
import type { PortfolioOpportunitiesPage } from '~/domain/customer-intelligence/portfolio-types'

defineProps<{ summary: PortfolioOpportunitiesPage['policySummary'] }>()
</script>

<template>
  <section class="portfolio-policy">
    <div>
      <span>Finalidade</span>
      <strong>{{ summary.purposeKey || '—' }}</strong>
    </div>
    <div>
      <span>Policy</span>
      <strong>{{ summary.policyVersionRef || '—' }}</strong>
    </div>
    <div>
      <span>Piso de coorte</span>
      <strong>{{ summary.minimumCohortLabel || 'protegido' }}</strong>
    </div>
    <div>
      <span>Supressao</span>
      <strong>{{ summary.suppressionMode || '—' }}</strong>
    </div>
    <div>
      <span>Freshness</span>
      <strong>{{ summary.freshnessPolicy || '—' }}</strong>
    </div>
  </section>
</template>

<style scoped>
.portfolio-policy {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 0.65rem;
  padding: 0.85rem;
  border: 1px solid rgb(var(--border) / 0.8);
  border-radius: 0.85rem;
}

.portfolio-policy div {
  display: grid;
  gap: 0.2rem;
}

.portfolio-policy span {
  color: rgb(var(--muted));
  font-size: 0.68rem;
}

.portfolio-policy strong {
  overflow-wrap: anywhere;
  font-size: 0.76rem;
}

@media (max-width: 900px) {
  .portfolio-policy {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
