<script setup>
import CampaignWorkspace from "~/components/campaigns/CampaignWorkspace.vue";
import { storeToRefs } from "pinia";
import { useCampaignsStore } from "~/stores/campaigns";

definePageMeta({
  layout: "dashboard",
  workspaceId: "campanhas"
});

const campaignsStore = useCampaignsStore();
const { state } = storeToRefs(campaignsStore);
</script>

<template>
  <div class="workspace-host">
    <CampaignWorkspace :state="state" />
  </div>
</template>
