<script setup lang="ts">
import type { IntelligenceSummaryView } from '~/domain/customer-intelligence/types'

const props = defineProps<{ summaries: IntelligenceSummaryView[] }>()
const summary = computed(
  () =>
    props.summaries.find((item) => item.summaryType === 'relationship_profile') ??
    props.summaries[0] ??
    null,
)
</script>

<template>
  <article class="ci-card">
    <header>
      <h3>Resumo inteligente</h3>
      <span v-if="summary?.asOf">
        Atualizado em {{ new Date(summary.asOf).toLocaleString('pt-BR') }}
      </span>
    </header>
    <p v-if="summary">{{ summary.text }}</p>
    <p v-else class="ci-card__muted">Nenhuma sintese publicada para este relacionamento.</p>
    <small v-if="summary?.promptVersionRef">
      Gerado por IA · prompt {{ summary.promptVersionRef }}
    </small>
  </article>
</template>

<style scoped>
.ci-card {
  display: grid;
  gap: 0.65rem;
  padding: 1rem;
  border: 1px solid rgb(var(--border) / 0.8);
  border-radius: 1rem;
  background: rgb(var(--surface) / 0.9);
}

.ci-card header {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.ci-card h3,
.ci-card p {
  margin: 0;
}

.ci-card span,
.ci-card small,
.ci-card__muted {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}
</style>
