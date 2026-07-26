<script setup lang="ts">
import type { PromptPipelineView } from '~/domain/customer-intelligence/prompt-types'

defineProps<{ pipelines: PromptPipelineView[] }>()
</script>

<template>
  <section class="prompt-pipelines">
    <h3>Pipelines estruturados</h3>
    <p>
      Ordem, branches e hard caps vem do catalogo Go. Cada etapa continua apontando para seu prompt.
    </p>
    <ul>
      <li v-for="pipeline in pipelines" :key="pipeline.pipelineKey">
        <strong>{{ pipeline.name || pipeline.pipelineKey }}</strong>
        <span>{{ pipeline.status }} · v{{ pipeline.activeVersion ?? '—' }}</span>
        <small>{{ pipeline.processKeys.join(' → ') }}</small>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.prompt-pipelines p,
.prompt-pipelines span,
.prompt-pipelines small {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

.prompt-pipelines ul {
  display: grid;
  gap: 0.4rem;
  padding: 0;
  list-style: none;
}

.prompt-pipelines li {
  display: grid;
  gap: 0.2rem;
  padding: 0.6rem;
  border-radius: 0.65rem;
  background: rgb(var(--surface-2) / 0.6);
}
</style>
