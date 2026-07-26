<script setup lang="ts">
import type { PromptProcessView } from '~/domain/customer-intelligence/prompt-types'

defineProps<{
  view: PromptProcessView
  dirty: boolean
  saving: boolean
  canManage: boolean
  canPublish: boolean
}>()
const emit = defineEmits<{
  save: []
  discard: []
  validate: []
  test: []
  publish: []
  rollback: []
}>()
</script>

<template>
  <section class="prompt-actions">
    <div>
      <strong>Lifecycle controlado</strong>
      <span v-if="dirty">Alteracoes locais nao salvas</span>
      <span v-else>Draft sincronizado com a revisao do backend</span>
    </div>
    <div class="prompt-actions__buttons">
      <button v-if="dirty" type="button" :disabled="saving" @click="emit('discard')">
        Descartar
      </button>
      <button type="button" :disabled="!canManage || saving || !dirty" @click="emit('save')">
        Salvar draft
      </button>
      <button type="button" :disabled="!view.canEdit || saving || dirty" @click="emit('validate')">
        Validar
      </button>
      <button type="button" :disabled="!view.canTest || saving || dirty" @click="emit('test')">
        Testar
      </button>
      <button
        type="button"
        :disabled="!canPublish || !view.canPublish || saving || dirty"
        @click="emit('publish')"
      >
        Publicar
      </button>
      <button
        type="button"
        :disabled="!canPublish || !view.canRollback || saving || dirty"
        @click="emit('rollback')"
      >
        Rollback
      </button>
    </div>
  </section>
</template>

<style scoped>
.prompt-actions,
.prompt-actions__buttons {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.6rem;
  flex-wrap: wrap;
}

.prompt-actions {
  position: sticky;
  bottom: 0;
  padding: 0.75rem;
  border: 1px solid rgb(var(--border));
  border-radius: 0.8rem;
  background: rgb(var(--surface) / 0.97);
}

.prompt-actions div:first-child {
  display: grid;
  gap: 0.15rem;
}

.prompt-actions span {
  color: rgb(var(--muted));
  font-size: 0.7rem;
}

.prompt-actions button {
  min-height: 2.2rem;
  padding: 0 0.75rem;
  border: 1px solid rgb(var(--primary) / 0.28);
  border-radius: 999px;
  background: rgb(var(--primary) / 0.1);
  color: rgb(var(--primary));
  font-weight: 700;
}
</style>
