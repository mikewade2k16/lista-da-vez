<script setup lang="ts">
import IntelligenceAuditWorkspace from '~/components/customer-intelligence/audit/IntelligenceAuditWorkspace.vue'
import { useCustomerIntelligenceAccess } from '~/composables/customer-intelligence/useCustomerIntelligenceAccess'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'customer_intelligence',
  pageLabel: 'Auditoria de inteligencia',
})

const access = useCustomerIntelligenceAccess()
</script>

<template>
  <div class="page-workspace">
    <CustomerIntelligencePageShell
      title="Auditoria e proveniencia"
      description="Eventos, hashes, diffs allowlisted e observacoes minimizadas. Superficie estritamente read-only."
    >
      <CustomerIntelligenceStatus
        v-if="!access.canViewAudit.value"
        title="Auditoria indisponivel"
        :error="{
          kind: access.hasCustomerIntelligenceModule.value ? 'forbidden' : 'capability_off',
          message: '',
          reasonCode: access.hasCustomerIntelligenceModule.value
            ? 'customer_intelligence_audit_view_required'
            : 'customer_intelligence_module_disabled',
          statusCode: access.hasCustomerIntelligenceModule.value ? 403 : 0,
        }"
      />
      <IntelligenceAuditWorkspace v-else />
    </CustomerIntelligencePageShell>
  </div>
</template>
