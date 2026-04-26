<script setup>
import { computed, watch } from "vue";
import ConsultantWorkspace from "~/components/consultant/ConsultantWorkspace.vue";
import { storeToRefs } from "pinia";
import { canUseAllStoresScope } from "~/domain/utils/permissions";
import { useAuthStore } from "~/stores/auth";
import { useConsultantsStore } from "~/stores/consultants";

definePageMeta({
  layout: "dashboard",
  workspaceId: "consultor",
  supportsAllStoresScope: true
});

const auth = useAuthStore();
const consultantsStore = useConsultantsStore();
const {
  state,
  integratedRoster,
  integratedRanking,
  integratedOverview,
  integratedPending,
  integratedError
} = storeToRefs(consultantsStore);
const { isAllStoresScope } = storeToRefs(auth);
const canSeeIntegrated = computed(() => canUseAllStoresScope(auth.accessibleStoreIds));
const integratedScope = computed(() => canSeeIntegrated.value && isAllStoresScope.value);

watch(
  () => [integratedScope.value, auth.activeStoreId, auth.activeTenantId, auth.isAuthenticated],
  async () => {
    try {
      await auth.ensureSession();

      if (!auth.isAuthenticated) {
        consultantsStore.clearIntegratedView();
        return;
      }

      if (integratedScope.value) {
        await consultantsStore.ensureIntegratedView();
        return;
      }

      consultantsStore.clearIntegratedView();
      await consultantsStore.refreshActiveStore();
    } catch {
      consultantsStore.clearIntegratedView();
    }
  },
  { immediate: true }
);
</script>

<template>
  <div class="workspace-host">
    <ConsultantWorkspace
      :state="state"
      :integrated-scope="integratedScope"
      :integrated-roster="integratedRoster"
      :integrated-ranking="integratedRanking"
      :integrated-overview="integratedOverview"
      :integrated-pending="integratedPending"
      :integrated-error="integratedError"
    />
  </div>
</template>
