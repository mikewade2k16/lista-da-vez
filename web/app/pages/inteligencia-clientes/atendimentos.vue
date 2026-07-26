<script setup lang="ts">
import IntelligenceRunsWorkspace from '~/components/customer-intelligence/runs/IntelligenceRunsWorkspace.vue'
import { useCustomerIntelligenceAccess } from '~/composables/customer-intelligence/useCustomerIntelligenceAccess'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'customer_intelligence',
  pageLabel: 'Runs de atendimento',
})

const access = useCustomerIntelligenceAccess()
</script>

<template>
  <div class="page-workspace">
    <CustomerIntelligencePageShell
      title="Atendimentos e runs"
      description="Leitura operacional sanitizada de processos, custo, latencia e falhas. Nenhum payload bruto e exposto."
    >
      <CustomerIntelligenceStatus
        v-if="!access.canViewRuns.value"
        title="Runs indisponiveis"
        :error="{
          kind: access.hasCustomerIntelligenceModule.value ? 'forbidden' : 'capability_off',
          message: '',
          reasonCode: access.hasCustomerIntelligenceModule.value
            ? 'customer_intelligence_runs_view_required'
            : 'customer_intelligence_module_disabled',
          statusCode: access.hasCustomerIntelligenceModule.value ? 403 : 0,
        }"
      />
      <IntelligenceRunsWorkspace v-else />
    </CustomerIntelligencePageShell>
  </div>
</template>
