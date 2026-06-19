<script setup lang="ts">
import CardapioListWorkspace from '~/components/cardapio/CardapioListWorkspace.vue'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'cardapio_web',
  pageLabel: 'Presence',
})
</script>

<template>
  <div class="page-workspace">
    <CardapioListWorkspace />
  </div>
</template>
