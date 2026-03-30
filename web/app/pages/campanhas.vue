<script setup>
import CampaignWorkspace from "~/components/campaigns/CampaignWorkspace.vue";
import { storeToRefs } from "pinia";
import { useDashboardStore } from "~/stores/dashboard";

definePageMeta({
  layout: "dashboard",
  workspaceId: "campanhas"
});

const dashboard = useDashboardStore();
const { state } = storeToRefs(dashboard);
</script>

<template>
  <div class="workspace-host">
    <CampaignWorkspace :state="state" />
  </div>
</template>
