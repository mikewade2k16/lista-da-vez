<script setup lang="ts">
import AttendanceTranscriptionsWorkspace from '~/components/operation/AttendanceTranscriptionsWorkspace.vue'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'transcricoes',
})
</script>

<template>
  <div class="page-workspace">
    <AttendanceTranscriptionsWorkspace />
  </div>
</template>
