<script setup>
import { onMounted } from "vue";
import ReportsWorkspace from "~/components/reports/ReportsWorkspace.vue";
import { storeToRefs } from "pinia";
import { useReportsStore } from "~/stores/reports";

definePageMeta({
  layout: "dashboard",
  workspaceId: "relatorios"
});

const reportsStore = useReportsStore();
const { state } = storeToRefs(reportsStore);

onMounted(() => {
  void reportsStore.ensureLoaded();
});
</script>

<template>
  <div class="workspace-host">
    <ReportsWorkspace :state="state" />
  </div>
</template>
