<script setup lang="ts">
// SPEC-F6: a pagina de config virou um DRAWER lateral em /calendario. Esta rota
// antiga (/calendario/config) e mantida como redirect para nao quebrar links
// existentes — cai no calendario com o drawer aberto na aba Responsaveis.
definePageMeta({
  layout: 'dashboard',
  // Mesmo criterio da /calendario: tela global, sem gate de workspace.
  workspaceId: '',
  // Redireciona ANTES de renderizar (middleware de rota inline).
  middleware: [() => navigateTo('/calendario?config=responsaveis', { replace: true })],
})
</script>

<template>
  <div class="calendar-config-redirect" aria-hidden="true"></div>
</template>
