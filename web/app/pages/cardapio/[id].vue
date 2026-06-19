<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import CardapioEditorWorkspace from '~/components/cardapio/CardapioEditorWorkspace.vue'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'cardapio_web',
  pageLabel: 'Cardapio Online',
})

const route = useRoute()
const restaurantId = computed(() => String(route.params.id || '').trim())
// account do restaurante (so platform_admin abrindo cardapio de outro cliente).
// Vem da listagem/criacao via query ?account= e sobrevive ao refresh da pagina.
const accountId = computed(() => String(route.query.account || '').trim())
</script>

<template>
  <div class="page-workspace">
    <CardapioEditorWorkspace :restaurant-id="restaurantId" :account-id="accountId" />
  </div>
</template>
