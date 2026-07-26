<script setup lang="ts">
import PortfolioOpportunities from '~/components/customer-intelligence/portfolio/PortfolioOpportunities.vue'
import { useCustomerIntelligenceAccess } from '~/composables/customer-intelligence/useCustomerIntelligenceAccess'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'customer_intelligence',
  pageLabel: 'Portfolio de oportunidades',
})

const access = useCustomerIntelligenceAccess()
</script>

<template>
  <div class="page-workspace">
    <CustomerIntelligencePageShell
      title="Portfolio de oportunidades"
      description="Oportunidades agregadas entre clientes, com coorte, supressao e revisao humana. Nunca revela contributors ou PII."
      requires-client
      show-client-selector
    >
      <CustomerIntelligenceStatus
        v-if="!access.canViewPortfolio.value"
        title="Portfolio indisponivel"
        :error="{
          kind: access.hasCustomerIntelligenceModule.value ? 'forbidden' : 'capability_off',
          message: '',
          reasonCode: access.hasCustomerIntelligenceModule.value
            ? 'portfolio_cross_client_gates_required'
            : 'customer_intelligence_module_disabled',
          statusCode: access.hasCustomerIntelligenceModule.value ? 403 : 0,
        }"
      />
      <template v-else>
        <PortfolioOpportunities />
        <p class="portfolio-policy-gap">
          As politicas de recomendacao continuam server-side. O backend ainda nao expoe catalogo e
          versionamento dessas policies; por isso o painel nao cria valores ou endpoints
          substitutos.
        </p>
      </template>
    </CustomerIntelligencePageShell>
  </div>
</template>

<style scoped>
.portfolio-policy-gap {
  padding: 0.8rem;
  border: 1px dashed rgb(var(--warning) / 0.35);
  border-radius: 0.75rem;
  color: rgb(var(--muted));
  font-size: 0.72rem;
}
</style>
