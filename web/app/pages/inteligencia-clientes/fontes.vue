<script setup lang="ts">
import IntelligenceRetentionPolicies from '~/components/customer-intelligence/sources/IntelligenceRetentionPolicies.vue'
import IntelligenceSourcesCatalog from '~/components/customer-intelligence/sources/IntelligenceSourcesCatalog.vue'
import { useCustomerIntelligenceAccess } from '~/composables/customer-intelligence/useCustomerIntelligenceAccess'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'customer_intelligence',
  pageLabel: 'Fontes de inteligencia',
})

const access = useCustomerIntelligenceAccess()
</script>

<template>
  <div class="page-workspace">
    <CustomerIntelligencePageShell
      title="Fontes de dados"
      description="Catalogo registrado, health e configuracao tipada. URLs, SQL e credenciais nunca sao campos livres."
    >
      <CustomerIntelligenceStatus
        v-if="!access.canViewSources.value"
        title="Acesso a fontes indisponivel"
        :error="{
          kind: access.hasCustomerIntelligenceModule.value ? 'forbidden' : 'capability_off',
          message: '',
          reasonCode: access.hasCustomerIntelligenceModule.value
            ? 'customer_intelligence_sources_view_required'
            : 'customer_intelligence_module_disabled',
          statusCode: access.hasCustomerIntelligenceModule.value ? 403 : 0,
        }"
      />
      <div v-else class="intelligence-sources-workspace">
        <IntelligenceRetentionPolicies />
        <IntelligenceSourcesCatalog />
      </div>
    </CustomerIntelligencePageShell>
  </div>
</template>

<style scoped>
.intelligence-sources-workspace {
  display: grid;
  gap: 1rem;
  min-width: 0;
}
</style>
