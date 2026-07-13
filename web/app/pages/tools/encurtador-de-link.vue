<script setup lang="ts">
import ToolsShortLinksWorkspace from '~/components/tools/ToolsShortLinksWorkspace.vue'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'tools',
  pageLabel: 'Encurtador de Link',
})
</script>

<template>
  <div class="page-workspace">
    <ToolsShortLinksWorkspace />
  </div>
</template>
