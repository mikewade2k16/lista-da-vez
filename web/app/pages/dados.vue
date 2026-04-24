<script setup>
import { onMounted } from "vue";
import DataWorkspace from "~/components/data/DataWorkspace.vue";
import { storeToRefs } from "pinia";
import { useAnalyticsStore } from "~/stores/analytics";

definePageMeta({
  layout: "dashboard",
  workspaceId: "dados"
});

const analyticsStore = useAnalyticsStore();
const { data, pending, errorMessage } = storeToRefs(analyticsStore);

onMounted(() => {
  void analyticsStore.ensureData();
});
</script>

<template>
  <div class="workspace-host">
    <DataWorkspace :report="data" :pending="pending" :error-message="errorMessage" />
  </div>
</template>
