<script setup lang="ts">
import CustomerControlPlane from '~/components/customer-intelligence/settings/CustomerControlPlane.vue'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'customer_intelligence',
  pageLabel: 'Configuracoes de Customer Intelligence',
})
</script>

<template>
  <div class="page-workspace">
    <CustomerIntelligencePageShell
      title="Ativacao e comportamento"
      description="Painel-first para capabilities, modelos, credenciais, agentes, fontes e prompts, preservando guardrails de plataforma e writer unico."
    >
      <CustomerControlPlane />
    </CustomerIntelligencePageShell>
  </div>
</template>
