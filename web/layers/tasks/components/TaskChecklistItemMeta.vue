<script setup lang="ts">
import { computed, ref } from 'vue'
import type { TaskChecklistItem, TaskChecklistItemStatus } from '../types/tasks'
import {
  TASK_CHECKLIST_STATUS_OPTIONS,
  taskChecklistToday,
  taskChecklistYesterday,
  withTaskChecklistStatus,
  withTaskChecklistStatusDate,
} from '../utils/task-checklist'

const props = withDefaults(
  defineProps<{
    item: TaskChecklistItem
    disabled?: boolean
  }>(),
  {
    disabled: false,
  },
)

const emit = defineEmits<{
  'update:item': [item: TaskChecklistItem]
}>()

const isOpen = ref(false)
const statusLabel = computed(
  () =>
    TASK_CHECKLIST_STATUS_OPTIONS.find((option) => option.value === props.item.status)?.label || '',
)
const dateLabel = computed(() => {
  if (!props.item.statusDate) return ''
  if (props.item.statusDate === taskChecklistToday()) return 'hoje'
  if (props.item.statusDate === taskChecklistYesterday()) return 'ontem'

  const [year, month, day] = props.item.statusDate.split('-').map(Number)
  const parsed = new Date(year || 0, (month || 1) - 1, day || 1)
  return parsed.toLocaleDateString('pt-BR', { day: '2-digit', month: 'short' }).replace('.', '')
})

function setStatus(status: TaskChecklistItemStatus) {
  if (props.disabled) return
  emit('update:item', withTaskChecklistStatus(props.item, status))
}

function setStatusDate(value: string) {
  if (props.disabled) return
  emit('update:item', withTaskChecklistStatusDate(props.item, value))
}

function onStatusDateChange(event: Event) {
  setStatusDate((event.target as HTMLInputElement | null)?.value || '')
}

function clearStatus() {
  if (props.disabled) return
  emit('update:item', withTaskChecklistStatus(props.item, null))
  isOpen.value = false
}
</script>

<template>
  <UPopover v-model:open="isOpen" :content="{ side: 'bottom', align: 'end' }">
    <button
      class="task-checklist-meta__trigger"
      :class="{ 'task-checklist-meta__trigger--set': item.status }"
      type="button"
      :disabled="disabled"
      :aria-label="
        statusLabel
          ? `Status ${statusLabel}${dateLabel ? `, ${dateLabel}` : ''}`
          : 'Adicionar status'
      "
    >
      <UIcon :name="item.status ? 'i-lucide-circle-dot' : 'i-lucide-plus'" />
      <span>{{ statusLabel || 'Status' }}</span>
      <span v-if="dateLabel" class="task-checklist-meta__date">· {{ dateLabel }}</span>
    </button>

    <template #content>
      <div class="task-checklist-meta__popover">
        <p class="task-checklist-meta__label">Status</p>
        <div class="task-checklist-meta__statuses">
          <button
            v-for="option in TASK_CHECKLIST_STATUS_OPTIONS"
            :key="option.value"
            class="task-checklist-meta__option"
            :class="{ 'task-checklist-meta__option--active': item.status === option.value }"
            type="button"
            @click="setStatus(option.value)"
          >
            <span class="task-checklist-meta__option-dot"></span>
            {{ option.label }}
          </button>
        </div>

        <template v-if="item.status">
          <div class="task-checklist-meta__divider"></div>
          <p class="task-checklist-meta__label">Data do status</p>
          <div class="task-checklist-meta__quick-dates">
            <button type="button" @click="setStatusDate(taskChecklistToday())">Hoje</button>
            <button type="button" @click="setStatusDate(taskChecklistYesterday())">Ontem</button>
            <button type="button" @click="setStatusDate('')">Sem data</button>
          </div>
          <input
            class="task-checklist-meta__date-input"
            type="date"
            :value="item.statusDate || ''"
            aria-label="Escolher data do status"
            @change="onStatusDateChange"
          />
          <button class="task-checklist-meta__clear" type="button" @click="clearStatus">
            Limpar status
          </button>
        </template>
      </div>
    </template>
  </UPopover>
</template>

<style scoped>
.task-checklist-meta__trigger {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  gap: 0.25rem;
  min-height: 1.75rem;
  padding: 0.15rem 0.45rem;
  border: 1px solid transparent;
  border-radius: 999px;
  background: transparent;
  color: rgb(var(--muted));
  cursor: pointer;
  font-size: 0.72rem;
  white-space: nowrap;
}

.task-checklist-meta__trigger:hover:not(:disabled),
.task-checklist-meta__trigger--set {
  border-color: rgb(var(--border));
  background: rgb(var(--surface));
  color: rgb(var(--text));
}

.task-checklist-meta__trigger--set {
  color: rgb(var(--primary));
  font-weight: 700;
}

.task-checklist-meta__trigger:disabled {
  cursor: default;
  opacity: 0.55;
}

.task-checklist-meta__date {
  color: rgb(var(--muted));
  font-weight: 600;
}

.task-checklist-meta__popover {
  display: grid;
  width: 15rem;
  gap: 0.4rem;
  padding: 0.55rem;
}

.task-checklist-meta__label {
  margin: 0;
  color: rgb(var(--muted));
  font-size: 0.68rem;
  font-weight: 800;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.task-checklist-meta__statuses {
  display: grid;
  gap: 0.15rem;
}

.task-checklist-meta__option,
.task-checklist-meta__quick-dates button,
.task-checklist-meta__clear {
  border: 0;
  border-radius: var(--radius-sm);
  background: transparent;
  color: rgb(var(--text));
  cursor: pointer;
  font-size: 0.8rem;
}

.task-checklist-meta__option {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  min-height: 1.9rem;
  padding: 0.3rem 0.45rem;
  text-align: left;
}

.task-checklist-meta__option:hover,
.task-checklist-meta__option--active,
.task-checklist-meta__quick-dates button:hover {
  background: rgb(var(--surface));
}

.task-checklist-meta__option--active {
  color: rgb(var(--primary));
  font-weight: 700;
}

.task-checklist-meta__option-dot {
  width: 0.45rem;
  height: 0.45rem;
  border-radius: 999px;
  background: currentColor;
}

.task-checklist-meta__divider {
  height: 1px;
  margin: 0.15rem -0.55rem;
  background: rgb(var(--border));
}

.task-checklist-meta__quick-dates {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.3rem;
}

.task-checklist-meta__quick-dates button {
  min-height: 1.85rem;
  border: 1px solid rgb(var(--border));
}

.task-checklist-meta__date-input {
  width: 100%;
  min-height: 2rem;
  padding: 0 0.45rem;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  outline: 0;
  background: rgb(var(--surface));
  color: rgb(var(--text));
  color-scheme: light dark;
  font-size: 0.8rem;
}

.task-checklist-meta__date-input:focus {
  border-color: rgb(var(--ring));
}

.task-checklist-meta__clear {
  min-height: 1.85rem;
  padding: 0.3rem 0.45rem;
  color: rgb(var(--muted));
  text-align: left;
}

.task-checklist-meta__clear:hover {
  background: rgb(var(--surface));
  color: rgb(var(--text));
}
</style>
