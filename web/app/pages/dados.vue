<script setup>
import { computed, watch } from "vue";
import DataWorkspace from "~/components/data/DataWorkspace.vue";
import { storeToRefs } from "pinia";
import { canUseAllStoresScope } from "~/domain/utils/permissions";
import { useAuthStore } from "~/stores/auth";
import { useAnalyticsStore } from "~/stores/analytics";

definePageMeta({
  layout: "dashboard",
  workspaceId: "dados",
  supportsAllStoresScope: true
});

const auth = useAuthStore();
const analyticsStore = useAnalyticsStore();
const { data, pending, errorMessage } = storeToRefs(analyticsStore);
const canSeeIntegrated = computed(() => canUseAllStoresScope(auth.accessibleStoreIds));
const { isAllStoresScope } = storeToRefs(auth);
const integratedScope = computed(() => canSeeIntegrated.value && isAllStoresScope.value);

watch(
  () => [integratedScope.value, auth.activeStoreId, auth.activeTenantId],
  () => {
    analyticsStore.setIntegratedScope(integratedScope.value);
    void analyticsStore.fetchData();
  },
  { immediate: true }
);
</script>

<template>
  <div class="page-workspace">
    <DataWorkspace
      :report="data"
      :pending="pending"
      :error-message="errorMessage"
      :integrated-scope="integratedScope"
    />
  </div>
</template>
