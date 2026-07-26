<script setup lang="ts">
import OmniEntityDrawer from '~/components/ui/OmniEntityDrawer.vue'
import type { RuntimeRunListItem } from '~/domain/customer-intelligence/runs-types'

defineProps<{ open: boolean; run: RuntimeRunListItem | null }>()
const emit = defineEmits<{ 'update:open': [value: boolean] }>()
</script>

<template>
  <OmniEntityDrawer
    :model-value="open"
    :title="run?.processKey || run?.pipelineKey || 'Run de inteligencia'"
    :subtitle="run ? `${run.status} · ${run.id}` : ''"
    @update:model-value="emit('update:open', $event)"
  >
    <dl v-if="run" class="run-detail">
      <div>
        <dt>Status</dt>
        <dd>{{ run.status }}</dd>
      </div>
      <div>
        <dt>Pipeline</dt>
        <dd>{{ run.pipelineKey || '—' }}</dd>
      </div>
      <div>
        <dt>Binding</dt>
        <dd>{{ run.promptBindingRef || '—' }}</dd>
      </div>
      <div>
        <dt>Versao de prompt</dt>
        <dd>{{ run.promptVersionRef || '—' }}</dd>
      </div>
      <div>
        <dt>Schema</dt>
        <dd>{{ run.schemaVersionRef || '—' }}</dd>
      </div>
      <div>
        <dt>Executor</dt>
        <dd>{{ run.executorType || '—' }}</dd>
      </div>
      <div>
        <dt>Provider/modelo</dt>
        <dd>{{ run.providerName || '—' }} / {{ run.modelName || '—' }}</dd>
      </div>
      <div>
        <dt>Tentativas</dt>
        <dd>{{ run.attempts }}</dd>
      </div>
      <div>
        <dt>Duracao/latencia</dt>
        <dd>{{ run.durationMs ?? '—' }} / {{ run.latencyMs ?? '—' }} ms</dd>
      </div>
      <div>
        <dt>Fontes</dt>
        <dd>{{ run.sourceCount ?? 0 }} · {{ run.sourceStatus || '—' }}</dd>
      </div>
      <div>
        <dt>Tools</dt>
        <dd>{{ run.toolCount ?? 0 }} · {{ run.toolStatus || '—' }}</dd>
      </div>
      <div>
        <dt>Reason/error</dt>
        <dd>{{ run.reasonCode || run.errorCode || '—' }}</dd>
      </div>
      <div>
        <dt>Correlacao</dt>
        <dd>{{ run.correlationRef || '—' }}</dd>
      </div>
    </dl>
    <p class="run-detail__notice">
      Inputs, outputs, mensagens, prompt compilado, credenciais e argumentos de tools nao fazem
      parte deste read model.
    </p>
  </OmniEntityDrawer>
</template>

<style scoped>
.run-detail {
  display: grid;
  gap: 0.65rem;
  margin: 0;
}

.run-detail div {
  display: grid;
  grid-template-columns: minmax(8rem, 0.4fr) 1fr;
  gap: 0.75rem;
  padding-bottom: 0.5rem;
  border-bottom: 1px solid rgb(var(--border) / 0.6);
}

.run-detail dt {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

.run-detail dd {
  margin: 0;
  overflow-wrap: anywhere;
}

.run-detail__notice {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}
</style>
