<script setup>
import { computed } from "vue";
import DemoWorkspacePage from "~/components/demo/DemoWorkspacePage.vue";
import { getDemoPage } from "~/utils/demo-pages";

definePageMeta({
  layout: "dashboard",
  workspaceId: "",
  pageLabel: "Omnichannel"
});

const page = computed(() => getDemoPage("omnichannel"));
</script>

<template>
  <div class="page-workspace">
    <DemoWorkspacePage :page="page" />
  </div>
</template>
