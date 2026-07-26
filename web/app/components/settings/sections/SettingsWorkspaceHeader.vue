<script setup>
defineProps({
  runtimeSettingsNotice: {
    type: String,
    default: '',
  },
})
</script>

<template>
  <header class="admin-panel__header settings-workspace-header">
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
.settings-workspace-header {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  align-items: center;
  gap: 4px 12px;
}

.settings-workspace-header .admin-panel__title {
  font-size: 0.94rem;
  line-height: 1.2;
}

.settings-tenant-scope-banner {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  margin: 0;
  color: var(--text-muted);
  font-size: 0.72rem;
  line-height: 1.25;
}

.settings-tenant-scope-banner .material-icons-round {
  flex: 0 0 auto;
  font-size: 15px;
}

.settings-runtime-warning {
  grid-column: 1 / -1;
  display: flex;
  align-items: flex-start;
  gap: 6px;
  margin: 0;
  padding: 6px 8px;
  border-radius: 9px;
  border: 1px solid rgb(var(--primary) / 0.28);
  background: rgb(var(--primary) / 0.08);
  color: rgb(var(--primary));
  font-size: 0.7rem;
  line-height: 1.25;
}

.settings-runtime-warning .material-icons-round {
  font-size: 15px;
}

@media (max-width: 760px) {
  .settings-workspace-header {
    grid-template-columns: 1fr;
    gap: 3px;
  }

  .settings-runtime-warning {
    grid-column: auto;
  }
}
</style>
