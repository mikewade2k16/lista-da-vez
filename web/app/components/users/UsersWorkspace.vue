<script setup>
import UsersAccessManager from "~/components/users/UsersAccessManager.vue";
</script>

<template>
  <section class="admin-panel" data-testid="users-panel">
    <header class="admin-panel__header">
      <h2 class="admin-panel__title">Usuarios e acessos</h2>
      <p class="admin-panel__text">Gestao de usuarios com filtros, colunas configuraveis, edicao inline e acoes rapidas por icone.</p>
    </header>

    <UsersAccessManager />
  </section>
</template>
