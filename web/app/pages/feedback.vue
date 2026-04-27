<script setup lang="ts">
definePageMeta({
  layout: "dashboard",
  workspaceId: "feedback"
});
</script>

<template>
  <div class="feedback-page">
    <FeedbackWorkspace />
  </div>
</template>

<style scoped>
.feedback-page {
  width: 100%;
  height: 100%;
}
</style>
