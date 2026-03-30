<script setup>
import SettingsWorkspace from "~/components/settings/SettingsWorkspace.vue";
import { storeToRefs } from "pinia";
import { useDashboardStore } from "~/stores/dashboard";

definePageMeta({
  layout: "dashboard",
  workspaceId: "configuracoes"
});

const dashboard = useDashboardStore();
const { state } = storeToRefs(dashboard);
</script>

<template>
  <div class="workspace-host">
    <SettingsWorkspace :state="state" />
  </div>
</template>
