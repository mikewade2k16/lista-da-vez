<script setup>
defineProps({
  runtimeSettingsNotice: {
    type: String,
    default: '',
  },
})
</script>

<template>
  <header class="admin-panel__header">
    <h2 class="admin-panel__title">Configuracoes</h2>
    <p class="admin-panel__subtitle settings-tenant-scope-banner">
      <span class="material-icons-round" aria-hidden="true">domain</span>
      Estas configuracoes valem para todas as lojas do tenant. Alteracoes feitas aqui afetam toda a
      operacao.
    </p>
    <p v-if="runtimeSettingsNotice" class="admin-panel__subtitle settings-runtime-warning">
      <span class="material-icons-round" aria-hidden="true">warning</span>
      {{ runtimeSettingsNotice }}
    </p>
  </header>
</template>

<style scoped>
.settings-runtime-warning {
  display: flex;
  align-items: flex-start;
  gap: 0.55rem;
  margin-top: 0.85rem;
  padding: 0.8rem 0.95rem;
  border-radius: 14px;
  border: 1px solid rgba(245, 158, 11, 0.28);
  background: rgba(245, 158, 11, 0.08);
  color: #fbbf24;
}

.settings-runtime-warning .material-icons-round {
  font-size: 1rem;
  margin-top: 0.1rem;
}
</style>
