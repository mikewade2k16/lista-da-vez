<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { TaskChecklistItem } from '../types/tasks'
import {
  createTaskChecklistItemId,
  normalizeTaskChecklist,
  taskChecklistProgress,
  withTaskChecklistCompleted,
} from '../utils/task-checklist'
import { normalizeText } from '../utils/text'
import TaskChecklistItemMeta from './TaskChecklistItemMeta.vue'

const props = withDefaults(
  defineProps<{
    items: TaskChecklistItem[]
    disabled?: boolean
  }>(),
  {
    disabled: false,
  },
)

const emit = defineEmits<{
  'update:items': [items: TaskChecklistItem[]]
}>()

const newTitle = ref('')
const isOpen = ref(false)
const progress = computed(() => taskChecklistProgress(props.items))

watch(
  () => props.items.length,
  (itemCount) => {
    if (itemCount === 0) isOpen.value = false
  },
)

function addItem() {
  if (props.disabled || props.items.length >= 200) return
  const title = normalizeText(newTitle.value, 220)
  if (!title) return
  emit('update:items', [
    ...normalizeTaskChecklist(props.items),
    { id: createTaskChecklistItemId(), title, completed: false },
  ])
  newTitle.value = ''
}

function openItems() {
  if (props.disabled) return
  isOpen.value = true
}

function toggleItem(itemId: string, completed: boolean) {
  if (props.disabled) return
  emit(
    'update:items',
    normalizeTaskChecklist(props.items).map((item) =>
      item.id === itemId ? withTaskChecklistCompleted(item, completed) : item,
    ),
  )
}

function onToggleItem(event: Event, itemId: string) {
  toggleItem(itemId, (event.target as HTMLInputElement | null)?.checked === true)
}

function renameItem(itemId: string, value: unknown) {
  if (props.disabled) return
  const title = normalizeText(value, 220)
  if (!title) return
  emit(
    'update:items',
    normalizeTaskChecklist(props.items).map((item) =>
      item.id === itemId ? { ...item, title } : item,
    ),
  )
}

function onRenameItem(event: Event, itemId: string) {
  renameItem(itemId, (event.target as HTMLInputElement | null)?.value)
}

function removeItem(itemId: string) {
  if (props.disabled) return
  emit(
    'update:items',
    normalizeTaskChecklist(props.items).filter((item) => item.id !== itemId),
  )
}

function replaceItem(nextItem: TaskChecklistItem) {
  if (props.disabled) return
  emit(
    'update:items',
    normalizeTaskChecklist(props.items).map((item) => (item.id === nextItem.id ? nextItem : item)),
  )
}
</script>

<template>
  <section class="task-checklist" aria-label="Itens da tarefa">
    <UButton
      v-if="!items.length && !isOpen"
      class="task-checklist__empty-trigger"
      icon="i-lucide-plus"
      label="Adicionar itens"
      color="neutral"
      variant="ghost"
      size="sm"
      :disabled="disabled"
      @click="openItems"
    />

    <div v-else class="task-checklist__dropdown">
      <button
        id="task-checklist-trigger"
        class="task-checklist__toggle"
        type="button"
        :aria-expanded="isOpen"
        aria-controls="task-checklist-panel"
        @click="isOpen = !isOpen"
      >
        <span class="task-checklist__heading">
          <UIcon name="i-lucide-list-checks" />
          <span>Itens da tarefa</span>
        </span>
        <span class="task-checklist__summary">
          <span v-if="items.length" class="task-checklist__metric">
            {{ progress.completed }}/{{ progress.total }} · {{ progress.percent }}%
          </span>
          <UIcon :name="isOpen ? 'i-lucide-chevron-up' : 'i-lucide-chevron-down'" />
        </span>
      </button>

      <div v-if="isOpen" id="task-checklist-panel" class="task-checklist__panel">
        <div
          v-if="items.length"
          class="task-checklist__progress"
          role="progressbar"
          aria-label="Progresso dos itens da tarefa"
          :aria-valuenow="progress.percent"
          aria-valuemin="0"
          aria-valuemax="100"
        >
          <span :style="{ width: `${progress.percent}%` }"></span>
        </div>

        <div v-if="items.length" class="task-checklist__items">
          <div v-for="item in items" :key="item.id" class="task-checklist__item">
            <input
              :id="`task-checklist-${item.id}`"
              class="task-checklist__checkbox"
              type="checkbox"
              :checked="item.completed"
              :disabled="disabled"
              @change="onToggleItem($event, item.id)"
            />
            <input
              class="task-checklist__title"
              type="text"
              :value="item.title"
              maxlength="220"
              :disabled="disabled"
              :class="{ 'task-checklist__title--completed': item.completed }"
              :aria-label="`Titulo do item ${item.title}`"
              @change="onRenameItem($event, item.id)"
            />
            <TaskChecklistItemMeta :item="item" :disabled="disabled" @update:item="replaceItem" />
            <UButton
              icon="i-lucide-x"
              color="neutral"
              variant="ghost"
              size="xs"
              :disabled="disabled"
              :aria-label="`Remover ${item.title}`"
              @click="removeItem(item.id)"
            />
          </div>
        </div>

        <div class="task-checklist__add">
          <UInput
            v-model="newTitle"
            class="min-w-0 flex-1"
            placeholder="Adicionar item (ex.: título do filme)"
            maxlength="220"
            :disabled="disabled || items.length >= 200"
            @keydown.enter.prevent="addItem"
          />
          <UButton
            icon="i-lucide-plus"
            label="Adicionar"
            color="neutral"
            variant="soft"
            :disabled="disabled || !newTitle.trim() || items.length >= 200"
            @click="addItem"
          />
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.task-checklist {
  margin: 1rem 0 1.25rem;
}

.task-checklist__empty-trigger {
  width: fit-content;
  padding-inline: 0.25rem;
}

.task-checklist__dropdown {
  overflow: hidden;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-md);
  background: rgb(var(--surface-2) / 0.42);
}

.task-checklist__heading,
.task-checklist__summary,
.task-checklist__add,
.task-checklist__item {
  display: flex;
  align-items: center;
}

.task-checklist__toggle {
  display: flex;
  width: 100%;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.7rem 0.85rem;
  border: 0;
  background: transparent;
  cursor: pointer;
  text-align: left;
}

.task-checklist__toggle:hover {
  background: rgb(var(--surface));
}

.task-checklist__heading {
  gap: 0.45rem;
  color: rgb(var(--text));
  font-size: 0.9rem;
  font-weight: 700;
}

.task-checklist__metric {
  color: rgb(var(--primary));
  font-size: 0.8rem;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.task-checklist__summary {
  gap: 0.5rem;
  color: rgb(var(--muted));
}

.task-checklist__panel {
  display: grid;
  gap: 0.75rem;
  padding: 0.2rem 0.85rem 0.85rem;
  border-top: 1px solid rgb(var(--border));
}

.task-checklist__progress {
  height: 0.4rem;
  overflow: hidden;
  border-radius: 999px;
  background: rgb(var(--border));
}

.task-checklist__progress span {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: rgb(var(--primary));
  transition: width 180ms ease;
}

.task-checklist__items {
  display: grid;
  gap: 0.25rem;
}

.task-checklist__item {
  gap: 0.55rem;
  min-height: 2.35rem;
  padding: 0.2rem 0.25rem 0.2rem 0.45rem;
  border-radius: var(--radius-sm);
}

.task-checklist__item:hover {
  background: rgb(var(--surface));
}

.task-checklist__checkbox {
  width: 1rem;
  height: 1rem;
  accent-color: rgb(var(--primary));
}

.task-checklist__title {
  min-width: 0;
  flex: 1;
  border: 0;
  outline: 0;
  background: transparent;
  color: rgb(var(--text));
  font-size: 0.875rem;
}

.task-checklist__title--completed {
  color: rgb(var(--muted));
  text-decoration: line-through;
}

.task-checklist__add {
  gap: 0.5rem;
}

@media (max-width: 560px) {
  .task-checklist__item {
    align-items: flex-start;
    flex-wrap: wrap;
  }

  .task-checklist__title {
    flex-basis: calc(100% - 4.5rem);
  }

  .task-checklist__add {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
