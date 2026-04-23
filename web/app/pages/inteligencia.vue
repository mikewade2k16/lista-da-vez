<script setup>
import { onMounted } from "vue";
import IntelligenceWorkspace from "~/components/intelligence/IntelligenceWorkspace.vue";
import { storeToRefs } from "pinia";
import { useAnalyticsStore } from "~/stores/analytics";

definePageMeta({
  layout: "dashboard",
  workspaceId: "inteligencia"
});

const analyticsStore = useAnalyticsStore();
const { intelligence, pending, errorMessage } = storeToRefs(analyticsStore);

onMounted(() => {
  void analyticsStore.ensureIntelligence();
});
</script>

<template>
  <div class="workspace-host">
    <IntelligenceWorkspace :report="intelligence" :pending="pending" :error-message="errorMessage" />
  </div>
</template>
