<script setup>
import { computed, watch } from "vue";
import CampaignWorkspace from "~/components/campaigns/CampaignWorkspace.vue";
import { storeToRefs } from "pinia";
import { canUseAllStoresScope } from "~/domain/utils/permissions";
import { useAuthStore } from "~/stores/auth";
import { useCampaignsStore } from "~/stores/campaigns";

definePageMeta({
  layout: "dashboard",
  workspaceId: "campanhas",
  supportsAllStoresScope: true
});

const auth = useAuthStore();
const campaignsStore = useCampaignsStore();
const { state, integratedHistory, integratedPending, integratedError } = storeToRefs(campaignsStore);
const { isAllStoresScope } = storeToRefs(auth);
const canSeeIntegrated = computed(() => canUseAllStoresScope(auth.accessibleStoreIds));
const integratedScope = computed(() => canSeeIntegrated.value && isAllStoresScope.value);
const storeOptions = computed(() => auth.storeContext || []);

watch(
  () => [integratedScope.value, auth.activeStoreId, auth.activeTenantId, auth.isAuthenticated],
  async () => {
    try {
      await auth.ensureSession();

      if (!auth.isAuthenticated) {
        campaignsStore.clearIntegratedHistory();
        return;
      }

      if (integratedScope.value) {
        await campaignsStore.ensureIntegratedHistory();
        return;
      }

      campaignsStore.clearIntegratedHistory();
    } catch {
      campaignsStore.clearIntegratedHistory();
    }
  },
  { immediate: true }
);
</script>

<template>
  <div class="workspace-host">
    <CampaignWorkspace
      :state="state"
      :integrated-scope="integratedScope"
      :integrated-history="integratedHistory"
      :integrated-pending="integratedPending"
      :integrated-error="integratedError"
      :stores="storeOptions"
    />
  </div>
</template>
