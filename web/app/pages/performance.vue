<script setup lang="ts">
definePageMeta({
  layout: 'dashboard',
  workspaceId: 'performance',
  pageLabel: 'Performance',
})
</script>

<template>
  <div class="page-workspace">
    <PerformanceWorkspace />
  </div>
</template>
