<script setup lang="ts">
import { ref } from "vue";
import SettingsTabs from "~/components/settings/SettingsTabs.vue";
import BancoSettingsSchema from "~/components/banco/BancoSettingsSchema.vue";

const tabs = [
  { id: "settings", label: "Settings", icon: "tune" }
];

const activeTab = ref("settings");
</script>

<template>
  <div class="banco-workspace">
    <div class="banco-workspace__header">
      <h2 class="banco-workspace__title">Banco de Dados</h2>
      <p class="banco-workspace__text">Estrutura de tabelas, campos, tipos e status de migracao do banco PostgreSQL.</p>
    </div>

    <SettingsTabs :tabs="tabs" :active-tab="activeTab" @update:active-tab="activeTab = $event" />

    <BancoSettingsSchema v-if="activeTab === 'settings'" />
  </div>
</template>

<style scoped>
.banco-workspace {
  display: grid;
  align-content: start;
  gap: 1.2rem;
  padding: 1.2rem;
  overflow-y: auto;
  flex: 1;
  min-height: 0;
}

.banco-workspace__header {
  display: grid;
  gap: 0.35rem;
}

.banco-workspace__title {
  margin: 0;
  font-size: 1.35rem;
  color: var(--text-main);
}

.banco-workspace__text {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.85rem;
}
</style>
