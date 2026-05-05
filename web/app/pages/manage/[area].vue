<script setup>
import { computed } from "vue";
import DemoWorkspacePage from "~/components/demo/DemoWorkspacePage.vue";
import { getDemoPage } from "~/utils/demo-pages";

definePageMeta({
  layout: "dashboard",
  workspaceId: "",
  pageLabel: "Manage"
});

const route = useRoute();
const page = computed(() => getDemoPage(`manage/${route.params.area}`));
</script>

<template>
  <div class="page-workspace">
    <DemoWorkspacePage :page="page" />
  </div>
</template>
