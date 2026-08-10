<script setup lang="ts">
import PlanningWorkspace from '~/components/planning/PlanningWorkspace.vue'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'planejamento',
})
</script>

<template>
  <div class="page-workspace"><PlanningWorkspace section="goals" /></div>
</template>
