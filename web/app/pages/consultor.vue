<script setup>
import ConsultantWorkspace from "~/components/consultant/ConsultantWorkspace.vue";
import { storeToRefs } from "pinia";
import { useDashboardStore } from "~/stores/dashboard";

definePageMeta({
  layout: "dashboard",
  workspaceId: "consultor"
});

const dashboard = useDashboardStore();
const { state } = storeToRefs(dashboard);
</script>

<template>
  <div class="workspace-host">
    <ConsultantWorkspace :state="state" />
  </div>
</template>
