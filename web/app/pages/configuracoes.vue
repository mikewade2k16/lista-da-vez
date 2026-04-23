<script setup>
import SettingsWorkspace from "~/components/settings/SettingsWorkspace.vue";
import { storeToRefs } from "pinia";
import { useSettingsStore } from "~/stores/settings";

definePageMeta({
  layout: "dashboard",
  workspaceId: "configuracoes"
});

const settingsStore = useSettingsStore();
const { state } = storeToRefs(settingsStore);
</script>

<template>
  <div class="workspace-host">
    <SettingsWorkspace :state="state" />
  </div>
</template>
