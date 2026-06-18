<script setup lang="ts">
// Tile de KPI do card de consultor. Layout em 2 linhas:
//   linha 1 = ícone + rótulo (+ tag ERP quando vier de venda do ERP)
//   linha 2 = valor (slot) + meta lado a lado
// Para empilhar a meta embaixo do valor, basta `flex-direction: column` no __body.
withDefaults(
  defineProps<{
    icon: string
    label: string
    erp?: boolean
    note?: string | null
    valueClass?: string
    noteClass?: string
  }>(),
  { erp: false, note: null, valueClass: '', noteClass: '' },
)
</script>

<template>
  <div class="card-metrics__tile">
    <div class="card-metrics__head">
      <span class="card-metrics__icon" aria-hidden="true">{{ icon }}</span>
      <span class="card-metrics__label">{{ label }}</span>
      <span v-if="erp" class="card-metrics__erp">ERP</span>
    </div>
    <div class="card-metrics__body">
      <strong class="card-metrics__value" :class="valueClass"><slot></slot></strong>
      <span v-if="note" class="card-metrics__note" :class="noteClass">{{ note }}</span>
    </div>
  </div>
</template>

<style scoped>
.card-metrics__tile {
  display: grid;
  gap: 0.3rem;
  padding: 0.55rem 0.65rem;
  border-radius: 0.7rem;
  background: rgb(var(--surface-2) / 0.76);
  border: 1px solid rgb(var(--border) / 0.72);
}

/* Linha 1: ícone + rótulo juntos; tag ERP empurrada para a direita. */
.card-metrics__head {
  display: flex;
  align-items: center;
  gap: 0.35rem;
}

.card-metrics__icon {
  font-size: 0.95rem;
  line-height: 1;
}

.card-metrics__label {
  font-size: 0.68rem;
  color: rgb(var(--muted) / 0.88);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.card-metrics__erp {
  margin-left: auto;
  display: inline-flex;
  align-items: center;
  min-height: 1rem;
  padding: 0.1rem 0.32rem;
  border-radius: 999px;
  background: rgb(var(--primary) / 0.14);
  color: rgb(var(--primary));
  font-size: 0.58rem;
  font-weight: 800;
  line-height: 1;
}

/* Linha 2: valor e meta lado a lado (column = volta a meta para baixo). */
.card-metrics__body {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0.2rem 0.5rem;
}

.card-metrics__value {
  font-size: 0.92rem;
  color: rgb(var(--text) / 0.96);
}

.card-metrics__value--hit {
  color: rgb(var(--success));
}

.card-metrics__value--miss {
  color: rgb(var(--danger));
}

.card-metrics__note {
  font-size: 0.7rem;
  color: rgb(var(--muted) / 0.92);
}

.card-metrics__note--hit {
  color: rgb(var(--success));
}

.card-metrics__note--miss {
  color: rgb(var(--danger));
}
</style>
