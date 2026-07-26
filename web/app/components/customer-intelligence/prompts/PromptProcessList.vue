<script setup lang="ts">
import type { PromptProcessDefinition } from '~/domain/customer-intelligence/prompt-types'

defineProps<{
  processes: PromptProcessDefinition[]
  selectedProcessKey: string
}>()
const emit = defineEmits<{ select: [processKey: string] }>()
</script>

<template>
  <aside class="prompt-processes">
    <button
      v-for="process in processes"
      :key="process.processKey"
      type="button"
      :class="{ 'is-active': process.processKey === selectedProcessKey }"
      @click="emit('select', process.processKey)"
    >
      <strong>{{ process.name || process.processKey }}</strong>
      <span>{{ process.processKey }}</span>
      <small>{{ process.status }} · {{ process.owner }}</small>
    </button>
  </aside>
</template>

<style scoped>
.prompt-processes {
  display: grid;
  gap: 0.4rem;
  align-content: start;
}

.prompt-processes button {
  display: grid;
  gap: 0.2rem;
  padding: 0.7rem;
  border: 1px solid rgb(var(--border) / 0.75);
  border-radius: 0.75rem;
  background: rgb(var(--surface-2) / 0.55);
  color: rgb(var(--text));
  text-align: left;
  cursor: pointer;
}

.prompt-processes button.is-active {
  border-color: rgb(var(--primary) / 0.45);
  background: rgb(var(--primary) / 0.1);
}

.prompt-processes span,
.prompt-processes small {
  color: rgb(var(--muted));
  font-size: 0.68rem;
}
</style>
