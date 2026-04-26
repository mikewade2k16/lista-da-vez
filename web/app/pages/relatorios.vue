<script setup>
import { computed, onMounted, watch } from "vue";
import ReportsWorkspace from "~/components/reports/ReportsWorkspace.vue";
import { storeToRefs } from "pinia";
import { canUseAllStoresScope } from "~/domain/utils/permissions";
import { useAuthStore } from "~/stores/auth";
import { useReportsStore } from "~/stores/reports";

definePageMeta({
  layout: "dashboard",
  workspaceId: "relatorios",
  supportsAllStoresScope: true
});

const auth = useAuthStore();
const reportsStore = useReportsStore();
const { state } = storeToRefs(reportsStore);
const canSeeIntegrated = computed(() => canUseAllStoresScope(auth.accessibleStoreIds));
const { isAllStoresScope } = storeToRefs(auth);
const integratedScope = computed(() => canSeeIntegrated.value && isAllStoresScope.value);

onMounted(() => {
  reportsStore.setIntegratedScope(integratedScope.value);
  void reportsStore.ensureLoaded();
});

watch(integratedScope, (value) => {
  reportsStore.setIntegratedScope(value);
  void reportsStore.ensureLoaded();
});
</script>

<template>
  <div class="workspace-host">
    <ReportsWorkspace :state="state" />
  </div>
</template>
