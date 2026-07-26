<script setup lang="ts">
import type { PromptEvaluationView } from '~/domain/customer-intelligence/prompt-types'

defineProps<{ evaluations: PromptEvaluationView[] }>()
</script>

<template>
  <section class="prompt-evals">
    <h3>Evals</h3>
    <p v-if="!evaluations.length">Sem avaliacao registrada.</p>
    <ul v-else>
      <li v-for="evaluation in evaluations" :key="evaluation.id">
        <strong>{{ evaluation.status }}</strong>
        <span>qualidade {{ evaluation.qualityScore ?? '—' }}</span>
        <span>seguranca {{ evaluation.safetyScore ?? '—' }}</span>
        <span>schema {{ evaluation.schemaScore ?? '—' }}</span>
        <small v-if="evaluation.violations?.length">
          {{ evaluation.violations.join(', ') }}
        </small>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.prompt-evals ul {
  display: grid;
  gap: 0.4rem;
  padding: 0;
  list-style: none;
}

.prompt-evals li {
  display: flex;
  gap: 0.6rem;
  flex-wrap: wrap;
  padding: 0.55rem;
  border-radius: 0.65rem;
  background: rgb(var(--surface-2) / 0.6);
}

.prompt-evals p,
.prompt-evals span,
.prompt-evals small {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}
</style>
