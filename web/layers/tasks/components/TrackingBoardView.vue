<script setup lang="ts">
import { computed, inject } from 'vue'
import { TASKS_PAGE_CONTEXT_KEY } from '../composables/useTasksPageContext'
import type { TaskItem } from '../types/tasks'

// Campos configuraveis do card de tracking (nome e sempre mostrado).
defineProps<{
  fields: { time: boolean; client: boolean; responsible: boolean }
}>()

const ctx = inject(TASKS_PAGE_CONTEXT_KEY)!
const {
  boardColumns,
  columnColorClass,
  isTracking,
  isRunning,
  getElapsedMs,
  formatElapsed,
  clientLabel,
  openTaskEditor,
  startTracking,
  pauseTracking,
  stopTracking,
} = ctx

type BoardColumnView = (typeof boardColumns.value)[number]

// Em tracking so aparecem as tasks com timer ativo (play ou pause).
function trackedTasks(column: BoardColumnView): TaskItem[] {
  return column.tasks.filter((task) => isTracking(task.id))
}

function clientName(task: TaskItem) {
  return String(task.clientName || '').trim() || clientLabel(task.clientId)
}

// So renderiza colunas que tem pelo menos uma task em play/pause.
const trackedColumns = computed(() =>
  boardColumns.value.filter((column) => column.tasks.some((task) => isTracking(task.id))),
)

const hasAnyTracked = computed(() => trackedColumns.value.length > 0)
</script>

<template>
  <div class="tracking-board-wrap">
    <div v-if="!hasAnyTracked" class="tracking-board__empty">
      <UIcon name="i-lucide-timer-off" class="h-9 w-9 text-[rgb(var(--muted))]" />
      <p class="tracking-board__empty-title">Nenhuma task em andamento</p>
      <p class="tracking-board__empty-hint">
        De play no timer de uma task na pagina de Tasks para ela aparecer aqui.
      </p>
    </div>

    <div v-else class="tracking-board">
      <section
        v-for="column in trackedColumns"
        :key="column.id"
        class="tracking-board__column"
        :class="columnColorClass(column.color)"
      >
        <header class="tracking-board__column-head">
          <span class="tracking-board__column-dot" aria-hidden="true"></span>
          <p class="tracking-board__column-title">{{ column.label }}</p>
          <UBadge color="neutral" variant="soft" size="xs">
            {{ trackedTasks(column).length }}
          </UBadge>
        </header>

        <div class="tracking-board__column-body">
          <article
            v-for="task in trackedTasks(column)"
            :key="task.id"
            class="tracking-board__card"
            :class="{
              'tracking-board__card--running': isRunning(task.id),
              'tracking-board__card--paused': isTracking(task.id) && !isRunning(task.id),
            }"
            @click="openTaskEditor(task)"
          >
            <div class="tracking-board__card-head">
              <p class="tracking-board__card-title">{{ task.title }}</p>
              <div v-if="fields.time" class="tracking-board__card-controls" @click.stop>
                <span class="tracking-board__card-timer">
                  {{ formatElapsed(getElapsedMs(task.id)) }}
                </span>
                <UButton
                  v-if="isRunning(task.id)"
                  icon="i-lucide-pause"
                  color="neutral"
                  variant="ghost"
                  size="xs"
                  title="Pausar"
                  @click="pauseTracking(task.id)"
                />
                <UButton
                  v-else
                  icon="i-lucide-play"
                  color="neutral"
                  variant="ghost"
                  size="xs"
                  title="Retomar"
                  @click="startTracking(task.id)"
                />
                <UButton
                  icon="i-lucide-square"
                  color="neutral"
                  variant="ghost"
                  size="xs"
                  title="Parar"
                  @click="stopTracking(task.id)"
                />
              </div>
            </div>

            <div
              v-if="(fields.responsible && task.responsible) || (fields.client && clientName(task))"
              class="tracking-board__card-meta"
            >
              <span v-if="fields.responsible && task.responsible" class="tracking-board__chip">
                <UIcon name="i-lucide-user" class="h-3 w-3" />
                {{ task.responsible }}
              </span>
              <span v-if="fields.client && clientName(task)" class="tracking-board__chip">
                <UIcon name="i-lucide-circle-dot" class="h-3 w-3" />
                {{ clientName(task) }}
              </span>
            </div>

            <span
              v-if="isTracking(task.id) && !isRunning(task.id)"
              class="tracking-board__card-pause-dot"
              aria-hidden="true"
            ></span>
          </article>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.tracking-board-wrap {
  overflow-x: auto;
  padding-bottom: 0.5rem;
}

.tracking-board {
  display: grid;
  grid-auto-flow: column;
  grid-auto-columns: minmax(280px, 1fr);
  gap: 0.75rem;
  align-items: start;
  min-width: min-content;
}

.tracking-board__empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 4.5rem 1rem;
  text-align: center;
}

.tracking-board__empty-title {
  margin-top: 0.75rem;
  color: rgb(var(--text));
  font-size: 0.9rem;
  font-weight: 600;
}

.tracking-board__empty-hint {
  margin-top: 0.25rem;
  color: rgb(var(--muted));
  font-size: 0.8rem;
}

.tracking-board__column {
  border-top-width: 3px;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface));
  min-height: 200px;
}

.tracking-board__column--indigo {
  border-top-color: rgb(129 140 248);
}
.tracking-board__column--slate {
  border-top-color: rgb(148 163 184);
}
.tracking-board__column--blue {
  border-top-color: rgb(59 130 246);
}
.tracking-board__column--amber {
  border-top-color: rgb(245 158 11);
}
.tracking-board__column--emerald {
  border-top-color: rgb(16 185 129);
}
.tracking-board__column--violet {
  border-top-color: rgb(139 92 246);
}
.tracking-board__column--rose {
  border-top-color: rgb(244 63 94);
}

.tracking-board__column-head {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 0.75rem;
  border-bottom: 1px solid rgb(var(--border));
}

.tracking-board__column-dot {
  width: 0.55rem;
  height: 0.55rem;
  border-radius: 50%;
  background: currentColor;
  flex-shrink: 0;
  color: rgb(var(--primary));
}
.tracking-board__column--slate .tracking-board__column-dot {
  color: rgb(148 163 184);
}
.tracking-board__column--blue .tracking-board__column-dot {
  color: rgb(59 130 246);
}
.tracking-board__column--amber .tracking-board__column-dot {
  color: rgb(245 158 11);
}
.tracking-board__column--emerald .tracking-board__column-dot {
  color: rgb(16 185 129);
}
.tracking-board__column--violet .tracking-board__column-dot {
  color: rgb(139 92 246);
}
.tracking-board__column--rose .tracking-board__column-dot {
  color: rgb(244 63 94);
}

.tracking-board__column-title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: rgb(var(--text));
  font-size: 0.85rem;
  font-weight: 600;
}

.tracking-board__column-body {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  padding: 0.5rem;
}

.tracking-board__column-empty {
  padding: 0.5rem 0.25rem;
  color: rgb(var(--muted));
  font-size: 0.78rem;
}

.tracking-board__card {
  position: relative;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2));
  padding: 0.6rem 0.7rem;
  box-shadow: var(--shadow-xs);
  cursor: pointer;
  transition: border-color 0.16s ease;
}

.tracking-board__card:hover {
  border-color: rgb(var(--primary) / 0.55);
}

.tracking-board__card--running {
  border-color: rgb(34 197 94 / 0.7);
}

.tracking-board__card--paused {
  border-color: rgb(234 179 8 / 0.7);
}

.tracking-board__card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.tracking-board__card-title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: rgb(var(--text));
  font-size: 0.85rem;
  font-weight: 600;
}

.tracking-board__card-controls {
  display: flex;
  align-items: center;
  gap: 0.15rem;
  flex-shrink: 0;
}

.tracking-board__card-controls :deep(svg) {
  width: 0.7rem;
  height: 0.7rem;
}

.tracking-board__card-timer {
  font-size: 0.8rem;
  font-variant-numeric: tabular-nums;
  font-weight: 600;
  color: rgb(var(--color-primary-500));
  min-width: 3rem;
  text-align: right;
}

.tracking-board__card-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.3rem;
  margin-top: 0.4rem;
}

.tracking-board__chip {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  padding: 0.1rem 0.45rem;
  border-radius: 999px;
  font-size: 0.72rem;
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
}

.tracking-board__card-pause-dot {
  position: absolute;
  bottom: 0.4rem;
  right: 0.4rem;
  width: 0.45rem;
  height: 0.45rem;
  border-radius: 50%;
  background: rgb(234 179 8);
}
</style>
