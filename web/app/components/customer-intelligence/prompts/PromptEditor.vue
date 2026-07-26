<script setup lang="ts">
import type { PromptVariableDescriptor } from '~/domain/customer-intelligence/prompt-types'

defineProps<{
  modelValue: string
  variables: PromptVariableDescriptor[]
  disabled?: boolean
}>()
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

function variableToken(key: string): string {
  return `{{${key}}}`
}
</script>

<template>
  <section class="prompt-editor">
    <header>
      <div>
        <h3>Prompt do processo</h3>
        <p>Comanda estrategia e linguagem deste processo; invariantes continuam fora do texto.</p>
      </div>
      <span>{{ modelValue.length }} caracteres</span>
    </header>
    <textarea
      :value="modelValue"
      :disabled="disabled"
      spellcheck="true"
      placeholder="Defina objetivo, tom, criterios e formato para este processo especifico."
      @input="emit('update:modelValue', ($event.target as HTMLTextAreaElement).value)"
    ></textarea>
    <div class="prompt-editor__variables">
      <span v-for="variable in variables" :key="variable.key" :title="variable.classification">
        {{ variableToken(variable.key) }}
      </span>
    </div>
  </section>
</template>

<style scoped>
.prompt-editor {
  display: grid;
  gap: 0.7rem;
}

.prompt-editor header {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
}

.prompt-editor h3,
.prompt-editor p {
  margin: 0;
}

.prompt-editor p,
.prompt-editor header span {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

.prompt-editor textarea {
  width: 100%;
  min-height: 18rem;
  resize: vertical;
  padding: 0.85rem;
  border: 1px solid rgb(var(--border));
  border-radius: 0.8rem;
  background: rgb(var(--surface-2));
  color: rgb(var(--text));
  line-height: 1.5;
}

.prompt-editor__variables {
  display: flex;
  gap: 0.35rem;
  flex-wrap: wrap;
}

.prompt-editor__variables span {
  padding: 0.2rem 0.45rem;
  border-radius: 999px;
  background: rgb(var(--primary) / 0.1);
  color: rgb(var(--primary));
  font-family: monospace;
  font-size: 0.7rem;
}
</style>
