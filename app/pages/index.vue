<script setup>
import { onMounted } from "vue";

onMounted(async () => {
  const { bootstrapLegacyApp } = await import("@legacy/main.js");
  await bootstrapLegacyApp();
});
</script>

<template>
  <div id="app"></div>
</template>
