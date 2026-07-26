<script setup lang="ts">
definePageMeta({
  layout: 'dashboard',
  workspaceId: 'feedback',
  alias: ['/operacao/feedback', '/suporte', '/operacao/suporte'],
})
</script>

<template>
  <div class="page-workspace">
    <FeedbackWorkspace />
  </div>
</template>
