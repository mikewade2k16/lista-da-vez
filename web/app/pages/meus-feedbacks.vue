<script setup lang="ts">
import UserFeedbackWorkspace from "~/components/feedback/UserFeedbackWorkspace.vue";

definePageMeta({
  layout: "dashboard",
  workspaceId: ""
});
</script>

<template>
  <div class="page-workspace">
    <UserFeedbackWorkspace />
  </div>
</template>
