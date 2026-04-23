<script setup>
import { onMounted } from "vue";
import RankingWorkspace from "~/components/ranking/RankingWorkspace.vue";
import { storeToRefs } from "pinia";
import { useAnalyticsStore } from "~/stores/analytics";

definePageMeta({
  layout: "dashboard",
  workspaceId: "ranking"
});

const analyticsStore = useAnalyticsStore();
const { ranking, pending, errorMessage } = storeToRefs(analyticsStore);

onMounted(() => {
  void analyticsStore.ensureRanking();
});
</script>

<template>
  <div class="workspace-host">
    <RankingWorkspace :report="ranking" :pending="pending" :error-message="errorMessage" />
  </div>
</template>
