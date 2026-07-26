<script setup lang="ts">
import UserFeedbackWorkspace from '~/components/feedback/UserFeedbackWorkspace.vue'

definePageMeta({
  layout: 'dashboard',
  workspaceId: '',
  alias: ['/meus-chamados'],
})
</script>

<template>
  <div class="page-workspace">
    <UserFeedbackWorkspace />
  </div>
</template>
