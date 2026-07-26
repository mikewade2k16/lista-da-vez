<script setup lang="ts">
import PromptStudio from '~/components/customer-intelligence/prompts/PromptStudio.vue'
import { useCustomerIntelligenceAccess } from '~/composables/customer-intelligence/useCustomerIntelligenceAccess'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'customer_intelligence',
  pageLabel: 'Prompt Studio',
})

const access = useCustomerIntelligenceAccess()
</script>

<template>
  <div class="page-workspace">
    <CustomerIntelligencePageShell
      title="Prompt Studio"
      description="Um prompt por processo, com schema, policy, evals, publicacao, canary e rollback versionados."
    >
      <CustomerIntelligenceStatus
        v-if="!access.canViewPrompts.value"
        title="Prompt Studio sem acesso"
        :error="{
          kind: access.hasCustomerIntelligenceModule.value ? 'forbidden' : 'capability_off',
          message: '',
          reasonCode: access.hasCustomerIntelligenceModule.value
            ? 'customer_intelligence_prompts_view_required'
            : 'customer_intelligence_module_disabled',
          statusCode: access.hasCustomerIntelligenceModule.value ? 403 : 0,
        }"
      />
      <PromptStudio v-else />
    </CustomerIntelligencePageShell>
  </div>
</template>
