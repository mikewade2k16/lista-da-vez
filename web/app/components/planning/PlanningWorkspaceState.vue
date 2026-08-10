<script setup lang="ts">
import { AlertCircle, Inbox, RefreshCw } from 'lucide-vue-next'

withDefaults(
  defineProps<{
    state: 'loading' | 'error' | 'empty'
    title?: string
    message?: string
    retryLabel?: string
  }>(),
  {
    title: '',
    message: '',
    retryLabel: 'Tentar novamente',
  },
)

defineEmits<{ retry: [] }>()
</script>

<template>
  <section
    class="planning-state omni-glass"
    :class="`is-${state}`"
    :role="state === 'error' ? 'alert' : 'status'"
    :aria-live="state === 'error' ? 'assertive' : 'polite'"
  >
    <template v-if="state === 'loading'">
      <CoreSkeleton variant="table-row" :count="4" />
      <span class="planning-state__copy">
        <strong>{{ title || 'Carregando planejamento…' }}</strong>
        <small>{{ message || 'Buscando os dados atualizados no banco.' }}</small>
      </span>
    </template>
    <template v-else>
      <span class="planning-state__icon">
        <AlertCircle v-if="state === 'error'" :size="22" />
        <Inbox v-else :size="22" />
      </span>
      <span class="planning-state__copy">
        <strong>{{ title }}</strong>
        <small>{{ message }}</small>
      </span>
      <button v-if="state === 'error'" type="button" @click="$emit('retry')">
        <RefreshCw :size="14" />
        {{ retryLabel }}
      </button>
    </template>
  </section>
</template>

<style scoped>
.planning-state {
  display: grid;
  place-items: center;
  gap: 0.65rem;
  min-height: 11rem;
  border: 1px solid rgb(var(--border) / 0.72);
  border-radius: var(--radius-card);
  padding: 1rem;
  background: rgb(var(--surface) / 0.84);
  color: var(--text-muted);
  text-align: center;
}

.planning-state.is-loading {
  display: block;
  text-align: left;
}

.planning-state.is-loading .planning-state__copy {
  margin-top: 0.8rem;
}

.planning-state.is-error {
  border-color: rgb(var(--danger) / 0.38);
}

.planning-state__icon {
  display: grid;
  place-items: center;
  width: 2.5rem;
  height: 2.5rem;
  border-radius: 0.75rem;
  background: rgb(var(--warning) / 0.12);
  color: rgb(var(--warning));
}

.is-error .planning-state__icon {
  background: rgb(var(--danger) / 0.12);
  color: rgb(var(--danger));
}

.planning-state__copy {
  display: grid;
  gap: 0.2rem;
}

.planning-state__copy strong {
  color: var(--text-main);
  font-size: 0.84rem;
}

.planning-state__copy small {
  font-size: 0.7rem;
}

.planning-state button {
  min-height: 2.2rem;
}
</style>
