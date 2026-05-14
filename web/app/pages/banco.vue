<script setup lang="ts">
definePageMeta({
  layout: "dashboard",
  workspaceId: "banco",
  alias: ["/operacao/banco"],
});
</script>

<template>
  <div class="page-workspace">
    <BancoWorkspace />
  </div>
</template>
