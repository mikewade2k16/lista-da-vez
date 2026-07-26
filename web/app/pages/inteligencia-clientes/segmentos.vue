<script setup lang="ts">
import CustomerSegmentsWorkspace from '~/components/customer-intelligence/segments/CustomerSegmentsWorkspace.vue'
import { useCustomerIntelligenceAccess } from '~/composables/customer-intelligence/useCustomerIntelligenceAccess'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'customer_intelligence',
  pageLabel: 'Segmentos de clientes',
})

const access = useCustomerIntelligenceAccess()
</script>

<template>
  <div class="page-workspace">
    <CustomerIntelligencePageShell
      title="Segmentos CRM e marketing"
      description="Definicoes deterministicas, versionadas e avaliadas no Customer Data. Segmentacao funciona sem IA."
    >
      <CustomerIntelligenceStatus
        v-if="!access.canViewSegments.value"
        title="Segmentacao indisponivel"
        :error="{
          kind: access.hasCustomerDataModule.value ? 'forbidden' : 'capability_off',
          message: '',
          reasonCode: access.hasCustomerDataModule.value
            ? 'customer_data_segments_view_required'
            : 'customer_data_module_disabled',
          statusCode: access.hasCustomerDataModule.value ? 403 : 0,
        }"
      />
      <CustomerSegmentsWorkspace v-else />
    </CustomerIntelligencePageShell>
  </div>
</template>
