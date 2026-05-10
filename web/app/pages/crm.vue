<script setup lang="ts">
import { watch } from "vue";

import { useAuthStore } from "~/stores/auth";
import { useCrmStore } from "~/stores/crm";

definePageMeta({
  layout: "dashboard",
  workspaceId: "crm"
});

const auth = useAuthStore();
const crmStore = useCrmStore();

watch(
  () => [auth.activeTenantId, auth.isAuthenticated],
  async () => {
    try {
      await auth.ensureSession();

      if (!auth.isAuthenticated) {
        crmStore.clearState();
        return;
      }

      await crmStore.ensureLoaded();
    } catch {
      crmStore.clearState();
    }
  },
  { immediate: true }
);
</script>

<template>
  <div class="page-workspace">
    <CrmWorkspace />
  </div>
</template>