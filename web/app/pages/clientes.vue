<script setup>
import { onMounted } from 'vue'

import TenantsWorkspace from '~/components/tenants/TenantsWorkspace.vue'
import { useTenantsStore } from '~/stores/tenants'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'clientes',
  alias: ['/operacao/clientes'],
  // Path canonico passou a ser /operacao/clientes (decisao 2026-05-29:
  // essa tela e do modulo Fila/queue e fica dentro de /operacao).
  // O alias mantem /clientes funcionando para URLs antigas.
  // Admin global de accounts (multi-tenant) fica em /manage/clientes-web.
})

const tenantsStore = useTenantsStore()

onMounted(() => {
  void tenantsStore.ensureLoaded()
})
</script>

<template>
  <div class="page-workspace">
    <TenantsWorkspace />
  </div>
</template>
