<script setup>
import IntelligenceWorkspace from "~/components/intelligence/IntelligenceWorkspace.vue";
import { storeToRefs } from "pinia";
import { useDashboardStore } from "~/stores/dashboard";

definePageMeta({
  layout: "dashboard",
  workspaceId: "inteligencia"
});

const dashboard = useDashboardStore();
const { state } = storeToRefs(dashboard);
</script>

<template>
  <div class="workspace-host">
    <IntelligenceWorkspace :state="state" />
  </div>
</template>
