<script setup lang="ts">
defineProps<{
  scopeLabel: string
  scopeHint: string
}>()

// Cabecalho do workspace ERP oculto por ora (redundante com o cabecalho da pagina).
// Markup preservado e vivo — reativar trocando showHeader para true.
const showHeader = false
</script>

<template>
  <header v-if="showHeader" class="erp-panel__header">
    <div>
      <h2 class="erp-panel__title">ERP FTP Workspace</h2>
      <p class="erp-panel__text">
        Produtos funcionais com raw exato do FTP, projecao rapida para busca e trilha de sync por
        lote no escopo ERP completo do sistema.
      </p>
    </div>
    <div class="erp-panel__store-selector">
      <div class="erp-panel__selectors-row">
        <div class="erp-panel__selector-group">
          <label class="erp-panel__store-label">Escopo ERP</label>
          <div class="erp-panel__store-select" role="status" aria-live="polite">
            {{ scopeLabel }}
          </div>
          <small class="erp-panel__store-hint">{{ scopeHint }}</small>
        </div>
      </div>
    </div>
  </header>
</template>
