<script setup lang="ts">
definePageMeta({
  layout: "dashboard",
  workspaceId: "erp",
  alias: ["/operacao/erp"],
});
</script>

<template>
  <div class="page-workspace">
    <ErpWorkspace />
  </div>
</template>