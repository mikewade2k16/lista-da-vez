<script setup>
import ReportsWorkspace from "~/components/reports/ReportsWorkspace.vue";
import { storeToRefs } from "pinia";
import { useDashboardStore } from "~/stores/dashboard";

definePageMeta({
  layout: "dashboard",
  workspaceId: "relatorios"
});

const dashboard = useDashboardStore();
const { state } = storeToRefs(dashboard);
</script>

<template>
  <div class="workspace-host">
    <ReportsWorkspace :state="state" />
  </div>
</template>
