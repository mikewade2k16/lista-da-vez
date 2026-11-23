<script setup lang="ts">
import ObservabilityWorkspace from '~/components/admin/observability/ObservabilityWorkspace.vue'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'observability_admin',
  pageLabel: 'Observabilidade',
})
</script>

<template>
  <ObservabilityWorkspace />
</template>
