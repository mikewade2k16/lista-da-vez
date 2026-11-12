<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import CardapioEditorWorkspace from '~/components/cardapio/CardapioEditorWorkspace.vue'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'cardapio_web',
  pageLabel: 'Presence',
})

const route = useRoute()
const restaurantId = computed(() => String(route.params.id || '').trim())
// account do restaurante (so platform_admin abrindo cardapio de outro cliente).
// Vem da listagem/criacao via query ?account= e sobrevive ao refresh da pagina.
const accountId = computed(() => String(route.query.account || '').trim())
</script>

<template>
  <div class="page-workspace cardapio-editor-page">
    <CardapioEditorWorkspace :restaurant-id="restaurantId" :account-id="accountId" />
  </div>
</template>

<style scoped>
/* Barra do estabelecimento colada no menu do app e na largura total: anula o
   padding do container global (.module-workspace-full: 8px 12px 0, layout
   dashboard) so nesta pagina. Fica no wrapper (overflow:hidden) e nao no editor,
   para nada ser cortado. */
.cardapio-editor-page {
  width: auto;
  margin: -8px -12px 0;
}
</style>
