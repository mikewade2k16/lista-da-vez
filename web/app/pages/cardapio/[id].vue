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
</script>

<template>
  <div class="page-workspace">
    <CardapioEditorWorkspace :restaurant-id="restaurantId" />
  </div>
</template>
