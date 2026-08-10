<script setup lang="ts">
import { computed, inject, ref, onMounted, onBeforeUnmount, watch } from 'vue'
import { TASKS_PAGE_CONTEXT_KEY } from '../composables/useTasksPageContext'
import CoreSkeleton from '../../core/components/CoreSkeleton.vue'
import OmniSelectMenuInput from './inputs/OmniSelectMenuInput.vue'
import OmniLazySelectMenuInput from './inputs/OmniLazySelectMenuInput.vue'
import AppDatePicker from './AppDatePicker.vue'
import { getApiBase } from '~/utils/api-client'
import type { TaskCalendarMediaItem, TaskVideoItem } from '../types/tasks'
import { taskChecklistProgress } from '../utils/task-checklist'

const ctx = inject(TASKS_PAGE_CONTEXT_KEY)!
const {
  activeProject,
  activeBoardLoading,
  boardColumns,
  columnColorClass,
  dropTarget,
  dragKind,
  markDropTarget,
  onDropColumnHeader,
  onDropColumn,
  boardGroupBy,
  onColumnDragStart,
  onColumnDragEnd,
  boardView,
  toggleAggregation,
  hideColumn,
  deleteTasksInColumn,
  beginCreateTaskInColumn,
  columnDraft,
  prepareColumnDraft,
  saveColumnSettings,
  deleteColumn,
  boardSchemaColumns,
  COLUMN_COLOR_OPTIONS,
  isTracking,
  isRunning,
  onDragStart,
  onDragEnd,
  openTaskEditor,
  onDropCard,
  activeInlineTaskId,
  onCardFocusOut,
  updateTaskInline,
  taskCardTitleValue,
  updateTaskCardTitle,
  normalizeText,
  clampText,
  startTracking,
  pauseTracking,
  stopTracking,
  toggleArchive,
  deleteTask,
  isCardFieldVisible,
  statusOptions,
  responsibleOptionsAvatar,
  involvedOptionsForResponsible,
  clientOptionsAvatar,
  clientLabel,
  typeOptions,
  PRIORITY_OPTIONS,
  toPriority,
  dateLabel,
  focusTaskCardPresence,
  blurTaskCardPresence,
  boardPresenceUsersForTask,
  boardPresenceSummary,
  isBoardPresenceFieldLocked,
  creatingCards,
  onDraftCardFocusOut,
  commitDraftCard,
  cancelDraftCard,
  isDraftFieldVisible,
  draftFieldOpen,
  setDraftFieldOpen,
  draftAvailableFields,
  addDraftField,
} = ctx

const checklistProgressByTask = computed(() => {
  const progress = new Map<string, ReturnType<typeof taskChecklistProgress>>()
  boardColumns.value.forEach((column) => {
    column.tasks.forEach((task) => progress.set(task.id, taskChecklistProgress(task.checklist)))
  })
  return progress
})

// Render progressivo: cada coluna pinta os primeiros INITIAL_RENDER cards na hora (above-the-fold)
// e o resto entra em lotes via requestIdleCallback, depois do primeiro paint. Num board com
// centenas de tasks (cada card monta varios selects pesados), isso destrava a primeira pintura sem
// montar tudo de uma vez. Reseta ao trocar de board.
const INITIAL_RENDER = 15
const RENDER_BATCH = 25
const renderLimit = ref(INITIAL_RENDER)
type BoardColumnView = (typeof boardColumns.value)[number]
let rampHandle: ReturnType<typeof setTimeout> | number | null = null
let rampIsIdle = false

function maxColumnTaskCount() {
  return boardColumns.value.reduce((max, column) => Math.max(max, column.tasks.length), 0)
}

function cancelRamp() {
  if (rampHandle == null) return
  if (rampIsIdle && typeof cancelIdleCallback !== 'undefined') {
    cancelIdleCallback(rampHandle as number)
  } else {
    clearTimeout(rampHandle as ReturnType<typeof setTimeout>)
  }
  rampHandle = null
}

function scheduleRamp() {
  if (rampHandle != null) return
  const step = () => {
    rampHandle = null
    if (renderLimit.value < maxColumnTaskCount()) {
      renderLimit.value += RENDER_BATCH
      scheduleRamp()
    }
  }
  if (typeof requestIdleCallback !== 'undefined') {
    rampIsIdle = true
    rampHandle = requestIdleCallback(step, { timeout: 200 })
  } else {
    rampIsIdle = false
    rampHandle = setTimeout(step, 32)
  }
}

function visibleColumnTasks(column: BoardColumnView) {
  return renderLimit.value >= column.tasks.length
    ? column.tasks
    : column.tasks.slice(0, renderLimit.value)
}

// --- Primeira midia no card -----------------------------------------------------
// O card do board mostra SO a primeira midia da task (a ordem vem do espelho do
// calendario — calendarMedia — e, sem ele, dos videos proprios). Resolvida uma vez
// por task num Map (o board renderiza centenas de cards; nada de recomputar por hover).
interface BoardCardMedia {
  image: string
  video: string
  name: string
  count: number
}

const runtimeConfig = useRuntimeConfig()

function taskMediaSrc(path: unknown): string {
  const normalizedPath = String(path || '').trim()
  if (!normalizedPath) return ''
  try {
    return new URL(normalizedPath, getApiBase(runtimeConfig)).toString()
  } catch {
    return normalizedPath
  }
}

function firstTaskMedia(task: {
  calendarMedia?: TaskCalendarMediaItem[]
  videos?: TaskVideoItem[]
}): BoardCardMedia | null {
  const count = (task.calendarMedia?.length || 0) + (task.videos?.length || 0)
  const cal = task.calendarMedia?.[0]
  if (cal) {
    if (cal.type === 'image')
      return { image: taskMediaSrc(cal.url), video: '', name: cal.name, count }
    if (cal.posterUrl)
      return { image: taskMediaSrc(cal.posterUrl), video: '', name: cal.name, count }
    return { image: '', video: taskMediaSrc(cal.url), name: cal.name, count }
  }
  const video = task.videos?.[0]
  if (video) return { image: '', video: taskMediaSrc(video.url), name: video.name, count }
  return null
}

const firstMediaByTask = computed(() => {
  const map = new Map<string, BoardCardMedia>()
  for (const column of boardColumns.value) {
    for (const task of column.tasks) {
      const media = firstTaskMedia(task)
      if (media) map.set(task.id, media)
    }
  }
  return map
})

function hiddenCountFor(column: BoardColumnView) {
  return Math.max(0, column.tasks.length - renderLimit.value)
}

onMounted(scheduleRamp)
onBeforeUnmount(cancelRamp)
// Ao trocar de board, volta ao cap inicial para a primeira pintura do novo board ser rapida.
watch(
  () => activeProject.value?.id,
  () => {
    cancelRamp()
    renderLimit.value = INITIAL_RENDER
    scheduleRamp()
  },
)
</script>

<template>
  <div class="tasks-page__board-wrap overflow-x-auto">
    <div class="tasks-page__board min-w-[1200px] gap-3">
      <section
        v-for="(column, columnIndex) in boardColumns"
        :key="column.id"
        :class="[
          'tasks-page__board-column rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))]',
          columnColorClass(column.color),
          {
            'tasks-page__board-column--drop':
              dropTarget.columnId === column.id && dropTarget.index === -1,
          },
        ]"
        @dragover.prevent
        @dragenter.prevent="markDropTarget(column.id)"
        @drop.prevent="
          dragKind === 'column' ? onDropColumnHeader(column, columnIndex) : onDropColumn(column)
        "
      >
        <header
          class="tasks-page__board-column-head flex items-center justify-between border-b border-[rgb(var(--border))] px-3 py-2"
        >
          <div class="tasks-page__board-column-title-wrap flex min-w-0 items-center gap-2">
            <UButton
              v-if="boardGroupBy === 'status'"
              class="tasks-page__board-column-handle"
              icon="i-lucide-grip-vertical"
              color="neutral"
              variant="ghost"
              size="xs"
              title="Mover coluna"
              draggable="true"
              @dragstart.stop="onColumnDragStart(column, $event)"
              @dragend="onColumnDragEnd"
            />
            <span class="tasks-page__board-column-color" aria-hidden="true"></span>
            <p class="tasks-page__board-column-title truncate text-sm font-semibold">
              {{ column.status }}
            </p>
            <UBadge
              v-if="boardView.showAggregation !== false"
              color="neutral"
              variant="soft"
              size="xs"
            >
              {{ column.tasks.length }}
            </UBadge>
          </div>
          <div class="tasks-page__board-column-actions flex items-center gap-1" @click.stop>
            <UButton
              icon="i-lucide-plus"
              color="primary"
              variant="ghost"
              size="xs"
              title="Criar task nesta coluna"
              @click="beginCreateTaskInColumn(column)"
            />
            <UPopover :content="{ side: 'bottom', align: 'end' }">
              <UButton
                icon="i-lucide-ellipsis"
                color="neutral"
                variant="ghost"
                size="xs"
                title="Acoes da coluna"
              />
              <template #content>
                <div class="tasks-page__column-menu w-56 space-y-1 p-1">
                  <UPopover :content="{ side: 'right', align: 'start' }">
                    <button
                      class="tasks-page__column-menu-item"
                      type="button"
                      :disabled="!column.editable"
                      @click="prepareColumnDraft(column)"
                    >
                      <UIcon name="i-lucide-list-restart" class="h-4 w-4" />
                      <span>Editar grupo</span>
                    </button>
                    <template #content>
                      <div class="tasks-page__column-editor-popover w-72 space-y-3 p-3" @click.stop>
                        <div class="space-y-1">
                          <p
                            class="tasks-page__settings-label text-[11px] font-semibold uppercase tracking-wide text-[rgb(var(--muted))]"
                          >
                            Nome
                          </p>
                          <UInput
                            v-model="columnDraft.label"
                            placeholder="Nome da coluna"
                            @keydown.enter.prevent="saveColumnSettings"
                          />
                        </div>
                        <div class="space-y-1">
                          <p
                            class="tasks-page__settings-label text-[11px] font-semibold uppercase tracking-wide text-[rgb(var(--muted))]"
                          >
                            Cor
                          </p>
                          <OmniSelectMenuInput
                            v-model="columnDraft.color"
                            :items="COLUMN_COLOR_OPTIONS"
                            placeholder="Cor"
                            :searchable="false"
                            :full-content-width="true"
                            item-display-mode="text"
                            color="neutral"
                            variant="none"
                            :highlight="false"
                            :badge-mode="true"
                            option-edit-mode="color"
                          />
                        </div>
                        <div
                          class="flex items-center justify-between gap-2 border-t border-[rgb(var(--border))] pt-2"
                        >
                          <UButton
                            icon="i-lucide-trash-2"
                            label="Excluir"
                            color="error"
                            variant="ghost"
                            size="xs"
                            :disabled="boardSchemaColumns.length <= 1"
                            @click="deleteColumn"
                          />
                          <UButton
                            label="Salvar"
                            color="primary"
                            size="xs"
                            @click="saveColumnSettings"
                          />
                        </div>
                      </div>
                    </template>
                  </UPopover>
                  <button
                    class="tasks-page__column-menu-item"
                    type="button"
                    @click="toggleAggregation"
                  >
                    <UIcon
                      :name="
                        boardView.showAggregation === false ? 'i-lucide-eye' : 'i-lucide-eye-off'
                      "
                      class="h-4 w-4"
                    />
                    <span>
                      {{
                        boardView.showAggregation === false
                          ? 'Mostrar contagem'
                          : 'Ocultar contagem'
                      }}
                    </span>
                  </button>
                  <button
                    class="tasks-page__column-menu-item"
                    type="button"
                    @click="hideColumn(column.id)"
                  >
                    <UIcon name="i-lucide-eye-off" class="h-4 w-4" />
                    <span>Ocultar grupo</span>
                  </button>
                  <button
                    class="tasks-page__column-menu-item tasks-page__column-menu-item--danger"
                    type="button"
                    @click="deleteTasksInColumn(column)"
                  >
                    <UIcon name="i-lucide-trash-2" class="h-4 w-4" />
                    <span>Excluir cards do grupo</span>
                  </button>
                </div>
              </template>
            </UPopover>
          </div>
        </header>

        <div class="tasks-page__board-column-body gap-2 p-2">
          <!-- Skeleton enquanto as tasks do board ativo nao chegaram (boot ou troca de
               board), para nao mostrar "Sem tasks" antes de saber se ha dados. -->
          <div
            v-if="activeBoardLoading"
            class="tasks-page__board-column-skeleton grid gap-2"
            aria-hidden="true"
          >
            <CoreSkeleton variant="card" :count="3" />
          </div>

          <article
            v-for="(task, index) in visibleColumnTasks(column)"
            :key="task.id"
            class="tasks-page__board-card cursor-pointer rounded-[var(--radius-sm)] border border-[rgb(var(--border))] bg-[rgb(var(--surface))] p-3 transition-colors hover:border-primary"
            draggable="true"
            :class="{
              'tasks-page__board-card--drop-before':
                dropTarget.columnId === column.id && dropTarget.index === index,
              'tasks-page__board-card--paused': isTracking(task.id) && !isRunning(task.id),
              'tasks-page__board-card--running': isRunning(task.id),
            }"
            @dragstart.stop="onDragStart(task, $event)"
            @dragend="onDragEnd"
            @dragover.prevent="markDropTarget(column.id, index)"
            @dragenter.prevent="markDropTarget(column.id, index)"
            @drop.stop.prevent="onDropCard(column, index)"
            @click="openTaskEditor(task)"
            @focusin="activeInlineTaskId = task.id"
            @focusout="onCardFocusOut($event, task)"
          >
            <div class="tasks-page__board-card-head mb-2 flex items-start justify-between gap-2">
              <button
                class="tasks-page__board-card-handle"
                type="button"
                title="Mover task"
                draggable="true"
                @click.stop
                @dragstart.stop="onDragStart(task, $event)"
              >
                <UIcon name="i-lucide-grip-vertical" />
              </button>
              <UInput
                :model-value="taskCardTitleValue(task)"
                class="tasks-page__board-card-title-input min-w-0 flex-1"
                :data-task-title-input="task.id"
                size="xs"
                variant="none"
                :disabled="isBoardPresenceFieldLocked(task.id, 'title')"
                @click.stop
                @focusin.stop="focusTaskCardPresence(task.id, 'title')"
                @focusout="blurTaskCardPresence(task.id, 'title', $event)"
                @update:model-value="updateTaskCardTitle(task, $event)"
              />
              <div class="tasks-page__board-card-track flex items-center gap-0.5" @click.stop>
                <UButton
                  v-if="!isTracking(task.id)"
                  icon="i-lucide-play"
                  color="primary"
                  variant="soft"
                  size="xs"
                  title="Iniciar tracking"
                  @click="startTracking(task.id)"
                />
                <UButton
                  v-if="isRunning(task.id)"
                  icon="i-lucide-pause"
                  color="warning"
                  variant="soft"
                  size="xs"
                  title="Pausar tracking"
                  @click="pauseTracking(task.id)"
                />
                <UButton
                  v-if="isTracking(task.id) && !isRunning(task.id)"
                  icon="i-lucide-play"
                  color="primary"
                  variant="soft"
                  size="xs"
                  title="Retomar tracking"
                  @click="startTracking(task.id)"
                />
                <UButton
                  v-if="isTracking(task.id)"
                  icon="i-lucide-square"
                  color="neutral"
                  variant="ghost"
                  size="xs"
                  title="Parar tracking"
                  @click="stopTracking(task.id)"
                />
              </div>
              <div class="tasks-page__board-card-actions flex items-center gap-0.5" @click.stop>
                <UButton
                  color="neutral"
                  variant="ghost"
                  size="xs"
                  :title="task.archived ? 'Desarquivar' : 'Arquivar'"
                  @click="toggleArchive(task)"
                >
                  <template #leading>
                    <span
                      class="iconify i-lucide:archive shrink-0 size-3"
                      aria-hidden="true"
                    ></span>
                  </template>
                </UButton>
                <UButton
                  color="neutral"
                  variant="ghost"
                  size="xs"
                  title="Excluir"
                  @click="deleteTask(task)"
                >
                  <template #leading>
                    <span
                      class="iconify i-lucide:trash-2 shrink-0 size-3"
                      aria-hidden="true"
                    ></span>
                  </template>
                </UButton>
              </div>
            </div>

            <div
              v-if="boardPresenceUsersForTask(task.id).length"
              class="tasks-page__board-card-presence"
              @click.stop
            >
              <div class="tasks-page__presence-stack" :title="boardPresenceSummary(task.id)">
                <UAvatar
                  v-for="participant in boardPresenceUsersForTask(task.id).slice(0, 3)"
                  :key="participant.userId"
                  :src="participant.avatarPath || undefined"
                  :text="participant.avatarText"
                  size="xs"
                  class="tasks-page__presence-avatar"
                />
                <span
                  v-if="boardPresenceUsersForTask(task.id).length > 3"
                  class="tasks-page__presence-more"
                >
                  +{{ boardPresenceUsersForTask(task.id).length - 3 }}
                </span>
              </div>
              <span>{{ boardPresenceSummary(task.id) }}</span>
            </div>

            <!-- Primeira midia da task (ordem = espelho do calendario; +N sinaliza o resto). -->
            <figure
              v-if="firstMediaByTask.get(task.id)"
              class="tasks-page__board-card-media"
              :title="firstMediaByTask.get(task.id)!.name"
            >
              <img
                v-if="firstMediaByTask.get(task.id)!.image"
                :src="firstMediaByTask.get(task.id)!.image"
                :alt="firstMediaByTask.get(task.id)!.name"
                loading="lazy"
              />
              <video
                v-else
                :src="firstMediaByTask.get(task.id)!.video"
                preload="metadata"
                muted
              ></video>
              <span
                v-if="firstMediaByTask.get(task.id)!.count > 1"
                class="tasks-page__board-card-media-count"
              >
                +{{ firstMediaByTask.get(task.id)!.count - 1 }}
              </span>
            </figure>

            <p
              v-if="task.description && boardView.visibleFieldKeys.includes('description')"
              class="tasks-page__board-card-description line-clamp-2 text-xs text-[rgb(var(--muted))]"
            >
              {{ task.description }}
            </p>

            <div
              v-if="task.checklist?.length"
              class="mt-2 grid gap-1"
              :aria-label="'Progresso: ' + checklistProgressByTask.get(task.id)!.percent + '%'"
            >
              <div
                class="flex items-center justify-between gap-2 text-[11px] font-medium text-[rgb(var(--muted))]"
              >
                <span class="inline-flex items-center gap-1">
                  <UIcon name="i-lucide-list-checks" class="h-3.5 w-3.5" />
                  {{ checklistProgressByTask.get(task.id)!.completed }}/{{
                    checklistProgressByTask.get(task.id)!.total
                  }}
                </span>
                <span>{{ checklistProgressByTask.get(task.id)!.percent }}%</span>
              </div>
              <div class="h-1.5 overflow-hidden rounded-full bg-[rgb(var(--border))]">
                <span
                  class="block h-full rounded-full bg-[rgb(var(--primary))] transition-[width] duration-200"
                  :style="{ width: checklistProgressByTask.get(task.id)!.percent + '%' }"
                ></span>
              </div>
            </div>

            <div
              class="tasks-page__board-card-inline mt-2 flex flex-col items-start gap-1"
              @click.stop
            >
              <OmniLazySelectMenuInput
                v-if="isCardFieldVisible(task, 'status')"
                :model-value="task.status"
                :items="statusOptions"
                placeholder="Status"
                :searchable="true"
                :full-content-width="true"
                item-display-mode="text"
                color="neutral"
                variant="none"
                :highlight="false"
                :badge-mode="true"
                trailing-icon=""
                option-edit-mode="color"
                :disabled="isBoardPresenceFieldLocked(task.id, 'status')"
                @update:open="
                  (open: boolean) =>
                    open
                      ? focusTaskCardPresence(task.id, 'status')
                      : blurTaskCardPresence(task.id, 'status')
                "
                @update:model-value="
                  updateTaskInline(task, { status: normalizeText($event, 120) || task.status })
                "
              />
              <OmniLazySelectMenuInput
                v-if="isCardFieldVisible(task, 'responsible')"
                class="tasks-page__board-card-people"
                :model-value="task.responsible"
                :items="responsibleOptionsAvatar"
                placeholder="Responsavel"
                :searchable="true"
                :full-content-width="true"
                item-display-mode="rich"
                :show-avatar="true"
                color="neutral"
                variant="none"
                :highlight="false"
                :badge-mode="true"
                badge-style="entity"
                trailing-icon=""
                clear
                option-edit-mode="color"
                :disabled="isBoardPresenceFieldLocked(task.id, 'responsible')"
                @update:open="
                  (open: boolean) =>
                    open
                      ? focusTaskCardPresence(task.id, 'responsible')
                      : blurTaskCardPresence(task.id, 'responsible')
                "
                @update:model-value="
                  updateTaskInline(task, { responsible: normalizeText($event, 120) })
                "
              />
              <OmniLazySelectMenuInput
                v-if="isCardFieldVisible(task, 'involved')"
                class="tasks-page__board-card-people"
                :model-value="task.involved"
                :items="involvedOptionsForResponsible(task.responsible)"
                placeholder="Envolvidos"
                :multiple="true"
                :searchable="true"
                :full-content-width="true"
                item-display-mode="rich"
                :show-avatar="true"
                color="neutral"
                variant="none"
                :highlight="false"
                :badge-mode="true"
                badge-style="entity"
                trailing-icon=""
                clear
                option-edit-mode="color"
                :disabled="isBoardPresenceFieldLocked(task.id, 'involved')"
                @update:open="
                  (open: boolean) =>
                    open
                      ? focusTaskCardPresence(task.id, 'involved')
                      : blurTaskCardPresence(task.id, 'involved')
                "
                @update:model-value="
                  updateTaskInline(task, {
                    involved: Array.isArray($event)
                      ? $event.map((item: string) => normalizeText(item, 120)).filter(Boolean)
                      : [],
                  })
                "
              />
              <OmniLazySelectMenuInput
                v-if="isCardFieldVisible(task, 'client')"
                class="tasks-page__board-card-people"
                :model-value="task.clientId || null"
                :items="clientOptionsAvatar"
                placeholder="Cliente"
                :searchable="true"
                :full-content-width="true"
                item-display-mode="rich"
                :show-avatar="true"
                color="neutral"
                variant="none"
                :highlight="false"
                :badge-mode="true"
                badge-style="entity"
                trailing-icon=""
                option-edit-mode="color"
                :disabled="isBoardPresenceFieldLocked(task.id, 'clientId')"
                @update:open="
                  (open: boolean) =>
                    open
                      ? focusTaskCardPresence(task.id, 'clientId')
                      : blurTaskCardPresence(task.id, 'clientId')
                "
                @update:model-value="
                  updateTaskInline(task, {
                    clientId: String($event ?? '') || task.clientId,
                    clientName: clientLabel(String($event ?? '') || task.clientId),
                  })
                "
              />
              <OmniLazySelectMenuInput
                v-if="isCardFieldVisible(task, 'type')"
                :model-value="task.type"
                :items="typeOptions"
                placeholder="Tipo"
                :creatable="{ when: 'always', position: 'bottom' }"
                :searchable="true"
                :full-content-width="true"
                item-display-mode="text"
                color="neutral"
                variant="none"
                :highlight="false"
                :badge-mode="true"
                trailing-icon=""
                clear
                option-edit-mode="full"
                :disabled="isBoardPresenceFieldLocked(task.id, 'type')"
                @update:open="
                  (open: boolean) =>
                    open
                      ? focusTaskCardPresence(task.id, 'type')
                      : blurTaskCardPresence(task.id, 'type')
                "
                @update:model-value="updateTaskInline(task, { type: normalizeText($event, 120) })"
              />
              <OmniLazySelectMenuInput
                v-if="isCardFieldVisible(task, 'priority')"
                :model-value="task.priority"
                :items="PRIORITY_OPTIONS"
                placeholder="Prioridade"
                :searchable="false"
                :full-content-width="true"
                item-display-mode="text"
                color="neutral"
                variant="none"
                :highlight="false"
                :badge-mode="true"
                trailing-icon=""
                option-edit-mode="color"
                :disabled="isBoardPresenceFieldLocked(task.id, 'priority')"
                @update:open="
                  (open: boolean) =>
                    open
                      ? focusTaskCardPresence(task.id, 'priority')
                      : blurTaskCardPresence(task.id, 'priority')
                "
                @update:model-value="updateTaskInline(task, { priority: toPriority($event) })"
              />
            </div>

            <span
              v-if="isTracking(task.id) && !isRunning(task.id)"
              class="tasks-page__board-card-pause-dot"
            ></span>

            <AppDatePicker
              v-if="isCardFieldVisible(task, 'dueDate')"
              :model-value="task.dueDate"
              :end-date="task.dueEndDate"
              placement="bottom"
              @update:open="
                (open: boolean) =>
                  open
                    ? focusTaskCardPresence(task.id, 'dueDate')
                    : blurTaskCardPresence(task.id, 'dueDate')
              "
              @update:model-value="updateTaskInline(task, { dueDate: $event })"
              @update:end-date="updateTaskInline(task, { dueEndDate: $event })"
            >
              <template #default="{ labelStart, labelEnd }">
                <button
                  class="tasks-page__board-card-duedate mt-2 flex items-center gap-1.5 cursor-pointer"
                  type="button"
                  :disabled="isBoardPresenceFieldLocked(task.id, 'dueDate')"
                  @click.stop
                >
                  <UIcon
                    name="i-lucide-calendar-days"
                    class="h-3.5 w-3.5 text-[rgb(var(--muted))] shrink-0"
                  />
                  <span v-if="labelStart" class="flex flex-col leading-tight">
                    <span class="text-[rgb(var(--text))]">{{ labelStart }}</span>
                    <span v-if="labelEnd" class="text-[rgb(var(--muted))]">{{ labelEnd }}</span>
                  </span>
                  <span v-else class="text-[rgb(var(--muted))]">Sem data</span>
                </button>
              </template>
            </AppDatePicker>

            <div
              v-if="isCardFieldVisible(task, 'createdAt')"
              class="tasks-page__board-card-date mt-1 flex items-center gap-1 text-xs text-[rgb(var(--muted))]"
            >
              <UIcon name="i-lucide-clock-3" class="h-3.5 w-3.5" />
              <span>{{ dateLabel(task.createdAt) }}</span>
            </div>
          </article>

          <div
            v-if="hiddenCountFor(column) > 0"
            class="tasks-page__board-card-more flex items-center justify-center gap-1.5 py-2 text-xs text-[rgb(var(--muted))]"
          >
            <UIcon name="i-lucide-loader-circle" class="h-3.5 w-3.5 animate-spin" />
            <span>Carregando mais {{ hiddenCountFor(column) }}…</span>
          </div>

          <article
            v-if="creatingCards[column.id]"
            class="tasks-page__board-card tasks-page__board-card--draft rounded-[var(--radius-sm)] border border-primary bg-[rgb(var(--surface))] p-3"
            @focusout="onDraftCardFocusOut($event, column)"
          >
            <UInput
              v-model="creatingCards[column.id].title"
              :data-draft-card="column.id"
              class="tasks-page__board-card-title-input"
              size="xs"
              variant="none"
              placeholder="Digite um nome..."
              @keydown.enter.prevent="commitDraftCard(column, true, true)"
              @keydown.esc.prevent="cancelDraftCard(column.id)"
            />

            <div
              v-if="
                isDraftFieldVisible(column.id, 'responsible') ||
                isDraftFieldVisible(column.id, 'involved') ||
                isDraftFieldVisible(column.id, 'clientId') ||
                isDraftFieldVisible(column.id, 'type') ||
                isDraftFieldVisible(column.id, 'priority') ||
                isDraftFieldVisible(column.id, 'dueDate')
              "
              class="tasks-page__board-card-inline mt-2 flex flex-col items-start gap-1"
              @click.stop
            >
              <OmniSelectMenuInput
                v-if="isDraftFieldVisible(column.id, 'responsible')"
                v-model="creatingCards[column.id].responsible"
                class="tasks-page__board-card-people"
                :open="draftFieldOpen[column.id]?.responsible"
                :items="responsibleOptionsAvatar"
                placeholder="Responsavel"
                :searchable="true"
                :full-content-width="true"
                item-display-mode="rich"
                :show-avatar="true"
                color="neutral"
                variant="none"
                :highlight="false"
                :badge-mode="true"
                badge-style="entity"
                trailing-icon=""
                clear
                option-edit-mode="color"
                @update:open="setDraftFieldOpen(column.id, 'responsible', $event)"
              />
              <OmniSelectMenuInput
                v-if="isDraftFieldVisible(column.id, 'involved')"
                v-model="creatingCards[column.id].involved"
                class="tasks-page__board-card-people"
                :open="draftFieldOpen[column.id]?.involved"
                :items="involvedOptionsForResponsible(creatingCards[column.id].responsible)"
                placeholder="Envolvidos"
                :multiple="true"
                :searchable="true"
                :full-content-width="true"
                item-display-mode="rich"
                :show-avatar="true"
                color="neutral"
                variant="none"
                :highlight="false"
                :badge-mode="true"
                badge-style="entity"
                trailing-icon=""
                clear
                option-edit-mode="color"
                @update:open="setDraftFieldOpen(column.id, 'involved', $event)"
              />
              <OmniSelectMenuInput
                v-if="isDraftFieldVisible(column.id, 'clientId')"
                v-model="creatingCards[column.id].clientId"
                class="tasks-page__board-card-people"
                :open="draftFieldOpen[column.id]?.clientId"
                :items="clientOptionsAvatar"
                placeholder="Cliente"
                :searchable="true"
                :full-content-width="true"
                item-display-mode="rich"
                :show-avatar="true"
                color="neutral"
                variant="none"
                :highlight="false"
                :badge-mode="true"
                badge-style="entity"
                trailing-icon=""
                option-edit-mode="color"
                @update:model-value="
                  creatingCards[column.id].clientName = clientLabel(
                    String($event ?? '') || creatingCards[column.id].clientId,
                  )
                "
                @update:open="setDraftFieldOpen(column.id, 'clientId', $event)"
              />
              <OmniSelectMenuInput
                v-if="isDraftFieldVisible(column.id, 'type')"
                v-model="creatingCards[column.id].type"
                :open="draftFieldOpen[column.id]?.type"
                :items="typeOptions"
                placeholder="Tipo"
                :creatable="{ when: 'always', position: 'bottom' }"
                :searchable="true"
                :full-content-width="true"
                item-display-mode="text"
                color="neutral"
                variant="none"
                :highlight="false"
                :badge-mode="true"
                trailing-icon=""
                clear
                option-edit-mode="full"
                @update:open="setDraftFieldOpen(column.id, 'type', $event)"
              />
              <OmniSelectMenuInput
                v-if="isDraftFieldVisible(column.id, 'priority')"
                v-model="creatingCards[column.id].priority"
                :open="draftFieldOpen[column.id]?.priority"
                :items="PRIORITY_OPTIONS"
                placeholder="Prioridade"
                :searchable="false"
                :full-content-width="true"
                item-display-mode="text"
                color="neutral"
                variant="none"
                :highlight="false"
                :badge-mode="true"
                trailing-icon=""
                option-edit-mode="color"
                @update:open="setDraftFieldOpen(column.id, 'priority', $event)"
              />
              <AppDatePicker
                v-if="isDraftFieldVisible(column.id, 'dueDate')"
                :model-value="creatingCards[column.id].dueDate"
                :end-date="creatingCards[column.id].dueEndDate"
                :open="draftFieldOpen[column.id]?.dueDate"
                placement="bottom"
                @update:model-value="creatingCards[column.id].dueDate = $event"
                @update:end-date="creatingCards[column.id].dueEndDate = $event"
                @update:open="setDraftFieldOpen(column.id, 'dueDate', $event)"
              >
                <template #default="{ labelStart, labelEnd }">
                  <button
                    class="tasks-page__board-card-duedate flex items-center gap-1.5 cursor-pointer"
                    type="button"
                    @click.stop
                  >
                    <UIcon
                      name="i-lucide-calendar-days"
                      class="h-3.5 w-3.5 text-[rgb(var(--muted))] shrink-0"
                    />
                    <span v-if="labelStart" class="flex flex-col leading-tight">
                      <span>{{ labelStart }}</span>
                      <span v-if="labelEnd" class="text-[rgb(var(--muted))]">{{ labelEnd }}</span>
                    </span>
                    <span v-else class="text-[rgb(var(--muted))]">Sem data</span>
                  </button>
                </template>
              </AppDatePicker>
            </div>

            <div
              v-if="draftAvailableFields(column.id).length"
              class="tasks-page__draft-add-list mt-1.5 flex flex-col items-start"
            >
              <button
                v-for="field in draftAvailableFields(column.id)"
                :key="field.key"
                type="button"
                class="tasks-page__draft-add-row"
                @mousedown.prevent
                @click="addDraftField(column.id, field.key)"
              >
                <UIcon :name="field.icon" class="tasks-page__draft-add-row-icon" />
                <span>Adicionar {{ field.label }}</span>
              </button>
            </div>
          </article>

          <UAlert
            v-if="!activeBoardLoading && column.tasks.length === 0 && !creatingCards[column.id]"
            class="tasks-page__board-empty"
            color="neutral"
            variant="soft"
            icon="i-lucide-inbox"
            title="Sem tasks"
            description="Crie ou arraste cards para esta coluna."
          />

          <button
            v-if="!activeBoardLoading"
            class="tasks-page__board-add-card"
            type="button"
            @click="beginCreateTaskInColumn(column)"
          >
            <UIcon name="i-lucide-plus" class="h-4 w-4" />
            <span></span>
          </button>
        </div>
      </section>
    </div>
  </div>
</template>
