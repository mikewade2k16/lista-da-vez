<script setup lang="ts">
definePageMeta({
  layout: 'dashboard',
  workspaceId: 'comunicados',
})

await navigateTo(
  {
    path: '/campanhas',
    query: { secao: 'comunicados' },
  },
  { redirectCode: 301 },
)
</script>

<template>
  <div></div>
</template>
