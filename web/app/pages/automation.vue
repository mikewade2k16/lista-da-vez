<script setup lang="ts">
import AutomationWorkspace from '~/components/automation/AutomationWorkspace.vue'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'automation',
  pageLabel: 'Automacao',
})
</script>

<template>
  <div class="page-workspace">
    <AutomationWorkspace />
  </div>
</template>
