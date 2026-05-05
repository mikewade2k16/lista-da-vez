<script setup>
import { computed } from "vue";
import DemoWorkspacePage from "~/components/demo/DemoWorkspacePage.vue";
import { getDemoPage } from "~/utils/demo-pages";

definePageMeta({
  layout: "dashboard",
  workspaceId: "",
  pageLabel: "Tracking"
});

const page = computed(() => getDemoPage("tracking"));
</script>

<template>
  <div class="page-workspace">
    <DemoWorkspacePage :page="page" />
  </div>
</template>
