<script setup lang="ts">
import { ref, watch } from 'vue'
import type { TaskChecklistItem } from '../types/tasks'

const props = defineProps<{
  open: boolean
  fileNames: string[]
  items: TaskChecklistItem[]
}>()

const emit = defineEmits<{
  'update:open': [open: boolean]
  confirm: [checklistItemId: string]
}>()

const selectedItemId = ref('')

watch(
  () => props.open,
  (open) => {
    if (open) selectedItemId.value = ''
  },
)

function close(): void {
  emit('update:open', false)
}

function confirm(): void {
  emit('confirm', selectedItemId.value)
}
</script>

<template>
  <UModal :open="open" @update:open="!$event && close()">
    <template #content>
      <UCard class="task-video-link-dialog">
        <template #header>
          <div class="task-video-link-dialog__header">
            <UIcon name="i-lucide-list-checks" />
            <div>
              <h3>Este vídeo pertence a algum item?</h3>
              <p>Escolha um item da tarefa ou deixe o vídeo vinculado à tarefa inteira.</p>
            </div>
          </div>
        </template>

        <div class="task-video-link-dialog__files">
          <span v-for="fileName in fileNames" :key="fileName">{{ fileName }}</span>
        </div>

        <div class="task-video-link-dialog__options" role="radiogroup" aria-label="Item da tarefa">
          <label :class="{ 'is-selected': selectedItemId === '' }">
            <input v-model="selectedItemId" type="radio" value="" />
            <span>
              <strong>Nenhum item</strong>
              <small>Vincular à tarefa inteira</small>
            </span>
          </label>
          <label
            v-for="item in items"
            :key="item.id"
            :class="{ 'is-selected': selectedItemId === item.id }"
          >
            <input v-model="selectedItemId" type="radio" :value="item.id" />
            <span>
              <strong>{{ item.title }}</strong>
              <small>{{ item.completed ? 'Item concluído' : 'Item pendente' }}</small>
            </span>
          </label>
        </div>

        <template #footer>
          <div class="task-video-link-dialog__actions">
            <UButton label="Cancelar" color="neutral" variant="ghost" @click="close" />
            <UButton label="Continuar upload" icon="i-lucide-upload" @click="confirm" />
          </div>
        </template>
      </UCard>
    </template>
  </UModal>
</template>

<style scoped>
.task-video-link-dialog {
  width: min(32rem, calc(100vw - 2rem));
}

.task-video-link-dialog__header,
.task-video-link-dialog__actions,
.task-video-link-dialog__options label {
  display: flex;
  align-items: center;
}

.task-video-link-dialog__header {
  align-items: flex-start;
  gap: 0.7rem;
}

.task-video-link-dialog__header > svg,
.task-video-link-dialog__header > .iconify {
  width: 1.2rem;
  height: 1.2rem;
  margin-top: 0.1rem;
  color: rgb(var(--primary));
}

.task-video-link-dialog h3,
.task-video-link-dialog p {
  margin: 0;
}

.task-video-link-dialog h3 {
  color: rgb(var(--text));
  font-size: 0.95rem;
}

.task-video-link-dialog p,
.task-video-link-dialog small {
  color: rgb(var(--muted));
  font-size: 0.76rem;
}

.task-video-link-dialog__files,
.task-video-link-dialog__options,
.task-video-link-dialog__options label > span {
  display: grid;
}

.task-video-link-dialog__files {
  gap: 0.25rem;
  margin-bottom: 0.75rem;
  color: rgb(var(--muted));
  font-size: 0.76rem;
}

.task-video-link-dialog__options {
  gap: 0.45rem;
  max-height: 18rem;
  overflow-y: auto;
}

.task-video-link-dialog__options label {
  gap: 0.65rem;
  padding: 0.65rem 0.75rem;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2));
  cursor: pointer;
}

.task-video-link-dialog__options label.is-selected {
  border-color: rgb(var(--primary));
  background: rgb(var(--primary) / 0.08);
}

.task-video-link-dialog__options label > span {
  gap: 0.1rem;
}

.task-video-link-dialog__options strong {
  color: rgb(var(--text));
  font-size: 0.84rem;
}

.task-video-link-dialog__actions {
  justify-content: flex-end;
  gap: 0.5rem;
}
</style>
