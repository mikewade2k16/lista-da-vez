<script setup lang="ts">
import CustomerIntelligenceWorkspace from '~/components/customer-intelligence/CustomerIntelligenceWorkspace.vue'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'customer_intelligence',
  pageLabel: 'Inteligencia de clientes',
})
</script>

<template>
  <div class="page-workspace">
    <CustomerIntelligenceWorkspace />
  </div>
</template>
