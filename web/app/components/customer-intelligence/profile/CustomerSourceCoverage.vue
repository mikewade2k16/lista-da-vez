<script setup lang="ts">
import type { CustomerSourceLink } from '~/domain/customer-data/profile-types'

defineProps<{ sources: CustomerSourceLink[] }>()
</script>

<template>
  <article class="ci-card">
    <h3>Cobertura de fontes</h3>
    <p v-if="!sources.length" class="muted">Sem links de fonte autorizados.</p>
    <ul v-else>
      <li v-for="source in sources" :key="source.sourceKey">
        <strong>{{ source.sourceKey }}</strong>
        <span>{{ source.status }}</span>
        <small>{{ source.freshness || source.reasonCode || 'Freshness nao informada' }}</small>
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
  gap: 0.5rem;
  padding: 0;
  list-style: none;
}

.ci-card li {
  display: grid;
  grid-template-columns: minmax(8rem, 1fr) auto minmax(8rem, 1fr);
  gap: 0.75rem;
}

.ci-card span,
.ci-card small,
.muted {
  color: rgb(var(--muted));
  font-size: 0.75rem;
}
</style>
