<script setup lang="ts">
definePageMeta({
  layout: "dashboard",
  workspaceId: "roadmap",
  alias: ["/operacao/roadmap"],
});
</script>

<template>
  <div class="page-workspace">
    <RoadmapWorkspace />
  </div>
</template>
