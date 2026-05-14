<script setup lang="ts">
import AdminPageHeader from '../../core/components/admin/AdminPageHeader.vue'
import { useTasksWorkspace } from '../composables/useTasksWorkspace'
import type { TaskItem, TaskPriority } from '../types/tasks'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'tasks',
  pageLabel: 'Tracking'
})

const tasksWorkspace = useTasksWorkspace()
const { trackedTaskIds, startTracking, pauseTracking, stopTracking, isRunning, isTracking, getElapsedMs, formatElapsed } = useTimeTracking()

const ORDER_STEP = 10
const COLUMN_COLOR_OPTIONS = ['indigo', 'slate', 'blue', 'amber', 'emerald', 'violet', 'rose']

function normalizeKey(value: unknown) {
  return String(value ?? '').normalize('NFD').replace(/[\u0300-\u036f]/g, '').toLowerCase().replace(/[^a-z0-9]+/g, '_').replace(/^_+|_+$/g, '')
}
function normalizeText(value: unknown, max = 240) { return String(value ?? '').replace(/\s+/g, ' ').trim().slice(0, max) }
function taskSort(a: TaskItem, b: TaskItem) { const d = Number(a.order || 0) - Number(b.order || 0); return d !== 0 ? d : a.createdAt.localeCompare(b.createdAt) }
function priorityLabel(value: TaskPriority) { return value === 'alta' ? 'Alta' : value === 'baixa' ? 'Baixa' : 'Media' }
function priorityColor(value: TaskPriority): 'error' | 'warning' | 'neutral' { return value === 'alta' ? 'error' : value === 'media' ? 'warning' : 'neutral' }
function columnColorClass(color: string) { return `tracking-board-column--${normalizeKey(color) || 'indigo'}` }
function dateLabel(value: unknown) {
  const iso = normalizeText(value, 24)
  if (!iso) return ''
  const d = new Date(iso.length === 10 ? `${iso}T00:00:00` : iso)
  if (Number.isNaN(d.getTime())) return iso
  return `${String(d.getDate()).padStart(2, '0')}/${String(d.getMonth() + 1).padStart(2, '0')}/${d.getFullYear()}`
}

interface TrackingProjectBoard {
  projectId: string
  projectName: string
  projectIcon: string
  columns: Array<{
    id: string
    label: string
    color: string
    tasks: TaskItem[]
  }>
}

const trackingBoards = computed((): TrackingProjectBoard[] => {
  const ids = new Set(trackedTaskIds.value)
  const boards: TrackingProjectBoard[] = []

  for (const project of tasksWorkspace.projects.value) {
    const tracked = tasksWorkspace.tasks.value
      .filter(t => t.projectId === project.id && ids.has(t.id))
    if (!tracked.length) continue

    const schemaColorMap = new Map(project.columns.map(c => [normalizeKey(c.label), c.color]))
    const seenStatuses = [...new Set(tracked.map(t => t.status))]
    const columns = seenStatuses.map((status, i) => ({
      id: `${project.id}-${normalizeKey(status) || 'empty'}`,
      label: status || 'Sem status',
      color: schemaColorMap.get(normalizeKey(status)) || COLUMN_COLOR_OPTIONS[i % COLUMN_COLOR_OPTIONS.length]!,
      tasks: tracked.filter(t => normalizeKey(t.status) === normalizeKey(status)).sort(taskSort)
    }))

    boards.push({
      projectId: project.id,
      projectName: project.name,
      projectIcon: project.icon || 'i-lucide-folder',
      columns
    })
  }

  return boards
})

const totalTracked = computed(() => trackedTaskIds.value.length)
</script>

<template>
  <div class="tracking-page">
    <AdminPageHeader title="Tracking" :description="`${totalTracked} task${totalTracked !== 1 ? 's' : ''} em andamento`" />

    <div class="tracking-page__body">
      <div v-if="trackingBoards.length === 0" class="tracking-page__empty">
        <UIcon name="i-lucide-timer-off" class="h-10 w-10 text-[rgb(var(--muted))]" />
        <p class="mt-3 text-sm font-medium text-[rgb(var(--text))]">Nenhuma task em andamento</p>
        <p class="mt-1 text-xs text-[rgb(var(--muted))]">Inicie o timer em um card na página de Tasks.</p>
      </div>

      <div v-else class="tracking-page__projects space-y-8">
        <section v-for="board in trackingBoards" :key="board.projectId" class="tracking-page__project">
          <div class="tracking-page__project-header flex items-center gap-2 mb-4">
            <UIcon :name="board.projectIcon" class="h-4 w-4 text-[rgb(var(--muted))]" />
            <h2 class="text-sm font-semibold text-[rgb(var(--text))]">{{ board.projectName }}</h2>
          </div>

          <div class="tracking-board">
            <div
              v-for="column in board.columns"
              :key="column.id"
              class="tracking-board-column"
              :class="columnColorClass(column.color)"
            >
              <header class="tracking-board-column__head">
                <span class="tracking-board-column__dot" aria-hidden="true" />
                <p class="tracking-board-column__title truncate text-sm font-semibold">{{ column.label }}</p>
                <UBadge color="neutral" variant="soft" size="xs">{{ column.tasks.length }}</UBadge>
              </header>

              <div class="tracking-board-column__body space-y-2 p-2">
                <article
                  v-for="task in column.tasks"
                  :key="task.id"
                  class="tracking-card"
                  :class="{
                    'tracking-card--paused': isTracking(task.id) && !isRunning(task.id),
                    'tracking-card--running': isRunning(task.id)
                  }"
                >
                  <div class="tracking-card__head">
                    <p class="tracking-card__title truncate text-sm font-semibold">{{ task.title }}</p>
                    <div class="tracking-card__controls flex items-center gap-0.5">
                      <span class="tracking-card__timer">{{ formatElapsed(getElapsedMs(task.id)) }}</span>
                      <UButton v-if="isRunning(task.id)" icon="i-lucide-pause" color="neutral" variant="ghost" size="xs" title="Pausar" @click="pauseTracking(task.id)" />
                      <UButton v-else icon="i-lucide-play" color="neutral" variant="ghost" size="xs" title="Iniciar / Retomar" @click="startTracking(task.id)" />
                      <UButton icon="i-lucide-square" color="neutral" variant="ghost" size="xs" title="Parar" @click="stopTracking(task.id)" />
                    </div>
                  </div>

                  <div v-if="task.responsible || task.involved?.length || task.type || task.priority" class="tracking-card__meta mt-1.5 flex flex-wrap items-center gap-1">
                    <span v-if="task.responsible" class="tracking-card__chip">{{ task.responsible }}</span>
                    <span v-for="p in task.involved" :key="p" class="tracking-card__chip">{{ p }}</span>
                    <UBadge v-if="task.type" color="neutral" variant="soft" size="xs">{{ task.type }}</UBadge>
                    <UBadge v-if="task.priority" :color="priorityColor(task.priority)" variant="soft" size="xs">{{ priorityLabel(task.priority) }}</UBadge>
                  </div>

                  <div v-if="task.dueDate" class="mt-1 flex items-center gap-1 text-xs text-[rgb(var(--muted))]">
                    <UIcon name="i-lucide-calendar-days" class="h-3 w-3" />
                    <span>{{ dateLabel(task.dueDate) }}</span>
                  </div>

                  <span v-if="isTracking(task.id) && !isRunning(task.id)" class="tracking-card__pause-dot" />
                </article>
              </div>
            </div>
          </div>
        </section>
      </div>
    </div>
  </div>
</template>

<style scoped>
.tracking-page {
  display: flex;
  flex-direction: column;
  height: 100%;
}
.tracking-page__body {
  flex: 1;
  overflow-y: auto;
  padding: 1.5rem;
}
.tracking-page__empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 5rem 1rem;
  text-align: center;
}
.tracking-board {
  display: grid;
  grid-auto-flow: column;
  grid-auto-columns: minmax(280px, 1fr);
  gap: 1rem;
  align-items: start;
  overflow-x: auto;
  padding-bottom: 0.5rem;
}
.tracking-board-column {
  border-top-width: 3px;
  border-radius: var(--radius-sm);
  background: rgb(var(--surface));
  border: 1px solid rgb(var(--border));
}
.tracking-board-column--indigo { border-top-color: rgb(129 140 248); }
.tracking-board-column--slate  { border-top-color: rgb(148 163 184); }
.tracking-board-column--blue   { border-top-color: rgb(59 130 246); }
.tracking-board-column--amber  { border-top-color: rgb(245 158 11); }
.tracking-board-column--emerald{ border-top-color: rgb(16 185 129); }
.tracking-board-column--violet { border-top-color: rgb(139 92 246); }
.tracking-board-column--rose   { border-top-color: rgb(244 63 94); }
.tracking-board-column__head {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 0.75rem;
  border-bottom: 1px solid rgb(var(--border));
}
.tracking-board-column__dot {
  width: 0.55rem;
  height: 0.55rem;
  border-radius: 50%;
  background: currentColor;
  flex-shrink: 0;
}
.tracking-board-column--indigo .tracking-board-column__dot { color: rgb(129 140 248); }
.tracking-board-column--slate  .tracking-board-column__dot { color: rgb(148 163 184); }
.tracking-board-column--blue   .tracking-board-column__dot { color: rgb(59 130 246); }
.tracking-board-column--amber  .tracking-board-column__dot { color: rgb(245 158 11); }
.tracking-board-column--emerald .tracking-board-column__dot { color: rgb(16 185 129); }
.tracking-board-column--violet .tracking-board-column__dot { color: rgb(139 92 246); }
.tracking-board-column--rose   .tracking-board-column__dot { color: rgb(244 63 94); }
.tracking-card {
  position: relative;
  border-radius: var(--radius-sm);
  border: 1px solid rgb(var(--border));
  background: rgb(var(--surface-2));
  padding: 0.6rem 0.75rem;
  box-shadow: var(--shadow-xs);
  transition: border-color 0.16s ease;
}
.tracking-card--paused { border-color: rgb(234 179 8 / 0.7); }
.tracking-card--running { border-color: rgb(34 197 94 / 0.7); }
.tracking-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}
.tracking-card__title { flex: 1; min-width: 0; }
.tracking-card__controls { display: flex; align-items: center; gap: 0.15rem; flex-shrink: 0; }
.tracking-card__controls :deep(svg) { width: 0.7rem; height: 0.7rem; }
.tracking-card__timer {
  font-size: 0.8rem;
  font-variant-numeric: tabular-nums;
  font-weight: 600;
  color: rgb(var(--color-primary-500));
  min-width: 3.2rem;
  text-align: right;
}
.tracking-card__chip {
  display: inline-flex;
  align-items: center;
  padding: 0.1rem 0.45rem;
  border-radius: 999px;
  font-size: 0.7rem;
  background: rgb(var(--color-primary-500) / 0.15);
  color: rgb(var(--color-primary-400));
}
.tracking-card__pause-dot {
  position: absolute;
  bottom: 0.4rem;
  right: 0.4rem;
  width: 0.45rem;
  height: 0.45rem;
  border-radius: 50%;
  background: rgb(234 179 8);
}
</style>
