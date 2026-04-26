<script setup>
import { ref } from "vue";

import SettingsTabs from "~/components/settings/SettingsTabs.vue";
import UsersAccessManager from "~/components/users/UsersAccessManager.vue";
import UsersRoleMatrixManager from "~/components/users/UsersRoleMatrixManager.vue";

const activeTab = ref("usuarios");
const tabs = [
  { id: "usuarios", label: "Usuarios", icon: "group" },
  { id: "perfis", label: "Perfis e padroes", icon: "fact_check" }
];
</script>

<template>
  <section class="admin-panel" data-testid="users-panel">
    <header class="admin-panel__header">
      <h2 class="admin-panel__title">Usuarios e acessos</h2>
      <p class="admin-panel__text">Gerencie contas, overrides individuais e o padrao de visibilidade do painel por tipo de usuario.</p>
    </header>

    <SettingsTabs :tabs="tabs" :active-tab="activeTab" @update:active-tab="activeTab = $event" />

    <UsersAccessManager v-if="activeTab === 'usuarios'" />
    <UsersRoleMatrixManager v-else />
  </section>
</template>
