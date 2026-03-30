<script setup>
import MultiStoreWorkspace from "~/components/multistore/MultiStoreWorkspace.vue";
import { storeToRefs } from "pinia";
import { useDashboardStore } from "~/stores/dashboard";

definePageMeta({
  layout: "dashboard",
  workspaceId: "multiloja"
});

const dashboard = useDashboardStore();
const { state } = storeToRefs(dashboard);
</script>

<template>
  <div class="workspace-host">
    <MultiStoreWorkspace :state="state" />
  </div>
</template>
