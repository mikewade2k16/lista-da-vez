<script setup>
import { onMounted } from "vue";

import TenantsWorkspace from "~/components/tenants/TenantsWorkspace.vue";
import { useTenantsStore } from "~/stores/tenants";

definePageMeta({
  layout: "dashboard",
  workspaceId: "clientes"
});

const tenantsStore = useTenantsStore();

onMounted(() => {
  void tenantsStore.ensureLoaded();
});
</script>

<template>
  <div class="page-workspace">
    <TenantsWorkspace />
  </div>
</template>