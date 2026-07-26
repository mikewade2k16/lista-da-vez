<script setup lang="ts">
import type { IntelligenceFactView } from '~/domain/customer-intelligence/types'

defineProps<{ facts: IntelligenceFactView[] }>()

function displayValue(value: unknown): string {
  if (typeof value === 'string' || typeof value === 'number') return String(value)
  if (typeof value === 'boolean') return value ? 'Sim' : 'Nao'
  if (value === null || value === undefined) return '—'
  return 'Dado estruturado'
}
</script>

<template>
  <article class="ci-card">
    <h3>Fatos e claims</h3>
    <p v-if="!facts.length" class="muted">Nenhum fato autorizado foi retornado.</p>
    <ul v-else>
      <li v-for="fact in facts" :key="fact.id">
        <div>
          <strong>{{ fact.label || fact.factKey }}</strong>
          <span>{{ displayValue(fact.value) }}</span>
        </div>
        <small>
          {{ fact.state }}
          <template v-if="fact.confidence != null">
            · confianca {{ Math.round(fact.confidence * 100) }}%
          </template>
        </small>
      </li>
    </ul>
  </article>
</template>

<style scoped>
.ci-card {
  padding: 1rem;
  border: 1px solid rgb(var(--border) / 0.8);
  border-radius: 1rem;
  background: rgb(var(--surface) / 0.9);
}

.ci-card h3 {
  margin-top: 0;
}

.ci-card ul {
  display: grid;
  gap: 0.55rem;
  padding: 0;
  list-style: none;
}

.ci-card li {
  display: grid;
  gap: 0.2rem;
  padding: 0.65rem;
  border-radius: 0.7rem;
  background: rgb(var(--surface-2) / 0.65);
}

.ci-card li div {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
}

.ci-card small,
.muted {
  color: rgb(var(--muted));
}
</style>
