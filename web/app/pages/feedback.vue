<script setup lang="ts">
definePageMeta({
  layout: "dashboard",
  workspaceId: "feedback",
  alias: ["/operacao/feedback"],
});
</script>

<template>
  <div class="page-workspace">
    <FeedbackWorkspace />
  </div>
</template>
