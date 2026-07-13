<script setup lang="ts">
import ToolsQrCodeWorkspace from '~/components/tools/ToolsQrCodeWorkspace.vue'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'tools',
  pageLabel: 'QR Code',
})
</script>

<template>
  <div class="page-workspace">
    <ToolsQrCodeWorkspace />
  </div>
</template>
