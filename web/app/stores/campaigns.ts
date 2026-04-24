import { computed } from "vue";
import { defineStore, storeToRefs } from "pinia";
import { useAppRuntimeStore } from "~/stores/app-runtime";

export const useCampaignsStore = defineStore("campaigns", () => {
  const runtime = useAppRuntimeStore();
  const { state } = storeToRefs(runtime);

  const campaigns = computed(() => state.value.campaigns || []);

  return {
    state,
    campaigns,
    ensure: runtime.ensure,
    createCampaign(payload) {
      return runtime.run("createCampaign", payload);
    },
    updateCampaign(campaignId, patch) {
      return runtime.run("updateCampaign", campaignId, patch);
    },
    removeCampaign(campaignId) {
      return runtime.run("removeCampaign", campaignId);
    }
  };
});
