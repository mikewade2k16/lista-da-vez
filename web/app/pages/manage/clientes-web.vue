<script setup lang="ts">
import ClientsAdminWorkspace from '~/components/tenants/ClientsAdminWorkspace.vue'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'clientes_web',
  pageLabel: 'Clientes Web',
})
</script>

<template>
  <div class="page-workspace">
    <ClientsAdminWorkspace />
  </div>
</template>
