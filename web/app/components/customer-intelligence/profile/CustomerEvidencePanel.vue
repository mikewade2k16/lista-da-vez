<script setup lang="ts">
import type { IntelligenceFactView } from '~/domain/customer-intelligence/types'

const props = defineProps<{ facts: IntelligenceFactView[] }>()
const evidence = computed(() =>
  props.facts.flatMap((fact) =>
    (fact.evidenceRefs ?? []).map((item) => ({ ...item, factKey: fact.factKey })),
  ),
)
</script>

<template>
  <article class="ci-card">
    <h3>Evidencias minimizadas</h3>
    <p v-if="!evidence.length" class="muted">Nenhuma evidencia autorizada para exibicao.</p>
    <ul v-else>
      <li v-for="item in evidence" :key="`${item.factKey}-${item.id}`">
        <strong>{{ item.factKey }} · {{ item.sourceKey }}</strong>
        <span>
          {{ item.excerpt || (item.masked ? 'Conteudo mascarado' : 'Referencia sem trecho') }}
        </span>
        <small v-if="item.observedAt">
          {{ new Date(item.observedAt).toLocaleString('pt-BR') }}
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
  gap: 0.25rem;
  padding: 0.65rem;
  border-left: 3px solid rgb(var(--primary) / 0.35);
}

.ci-card span,
.ci-card small,
.muted {
  color: rgb(var(--muted));
  font-size: 0.78rem;
}
</style>
