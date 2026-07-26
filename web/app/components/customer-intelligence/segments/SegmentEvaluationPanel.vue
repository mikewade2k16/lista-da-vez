<script setup lang="ts">
import type { SegmentEvaluationRun } from '~/domain/customer-data/segment-types'

defineProps<{
  run: SegmentEvaluationRun | null
  canEvaluate: boolean
  dirty: boolean
  busy: boolean
  hasDraft: boolean
}>()

const emit = defineEmits<{ preview: [] }>()
</script>

<template>
  <section class="segment-evaluation">
    <header>
      <div>
        <h3>Preview</h3>
        <p>Amostra bounded e mascarada; preview nao cria membership nem export.</p>
      </div>
      <button
        type="button"
        :disabled="!canEvaluate || !hasDraft || dirty || busy"
        @click="emit('preview')"
      >
        Avaliar draft
      </button>
    </header>
    <div v-if="run" class="segment-evaluation__result">
      <strong>{{ run.status }}</strong>
      <span>{{ run.countBucket || `${run.candidateCount ?? 0} candidatos` }}</span>
      <span>asOf {{ new Date(run.asOf).toLocaleString('pt-BR') }}</span>
      <span v-for="reason in run.reasonCodes ?? []" :key="reason">{{ reason }}</span>
    </div>
    <p v-else>Nenhuma avaliacao iniciada nesta sessao.</p>
  </section>
</template>

<style scoped>
.segment-evaluation {
  display: grid;
  gap: 0.75rem;
  padding: 1rem;
  border: 1px solid rgb(var(--border) / 0.8);
  border-radius: 0.9rem;
}

.segment-evaluation header,
.segment-evaluation__result {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.65rem;
  flex-wrap: wrap;
}

.segment-evaluation h3,
.segment-evaluation p {
  margin: 0;
}

.segment-evaluation p,
.segment-evaluation span {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}
</style>
