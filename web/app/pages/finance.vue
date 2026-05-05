<script setup>
import { computed } from "vue";
import DemoWorkspacePage from "~/components/demo/DemoWorkspacePage.vue";
import { getDemoPage } from "~/utils/demo-pages";

definePageMeta({
  layout: "dashboard",
  workspaceId: "",
  pageLabel: "Finance"
});

const page = computed(() => getDemoPage("finance"));
</script>

<template>
  <div class="page-workspace">
    <DemoWorkspacePage :page="page" />
  </div>
</template>
