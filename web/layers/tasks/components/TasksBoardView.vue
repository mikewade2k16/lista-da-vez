<script setup lang="ts">
import { inject } from 'vue'
import { TASKS_PAGE_CONTEXT_KEY } from '../composables/useTasksPageContext'
import OmniSelectMenuInput from './inputs/OmniSelectMenuInput.vue'
import AppDatePicker from './AppDatePicker.vue'

const ctx = inject(TASKS_PAGE_CONTEXT_KEY)!
const {
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
  activeInlineTaskId,
  onCardFocusOut,
  updateTaskInline,
  normalizeText,
  startTracking,
  pauseTracking,
  stopTracking,
  toggleArchive,
  deleteTask,
  isCardFieldVisible,
  statusOptions,
  responsibleOptionsAvatar,
  involvedOptionsAvatar,
  clientOptionsAvatar,
  clientLabel,
  toNumberId,
  typeOptions,
  PRIORITY_OPTIONS,
  toPriority,
  formatElapsed,
  getElapsedMs,
  dateLabel,
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
</script>

<template>
  <div class="tasks-page__board-wrap overflow-x-auto">
    <div class="tasks-page__board min-w-[1200px] gap-3">
      <section v-for="(column, columnIndex) in boardColumns" :key="column.id"
        :class="['tasks-page__board-column rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))]', columnColorClass(column.color), { 'tasks-page__board-column--drop': dropTarget.columnId === column.id && dropTarget.index === -1 }]"
        @dragover.prevent @dragenter.prevent="markDropTarget(column.id)"
        @drop.prevent="dragKind === 'column' ? onDropColumnHeader(column, columnIndex) : onDropColumn(column)">
        <header
          class="tasks-page__board-column-head flex items-center justify-between border-b border-[rgb(var(--border))] px-3 py-2">
          <div class="tasks-page__board-column-title-wrap flex min-w-0 items-center gap-2">
            <UButton v-if="boardGroupBy === 'status'" class="tasks-page__board-column-handle"
              icon="i-lucide-grip-vertical" color="neutral" variant="ghost" size="xs" title="Mover coluna"
              draggable="true" @dragstart.stop="onColumnDragStart(column, $event)" @dragend="onColumnDragEnd" />
            <span class="tasks-page__board-column-color" aria-hidden="true" />
            <p class="tasks-page__board-column-title truncate text-sm font-semibold">{{ column.status }}</p>
            <UBadge v-if="boardView.showAggregation !== false" color="neutral" variant="soft" size="xs">{{
              column.tasks.length }}</UBadge>
          </div>
          <div class="tasks-page__board-column-actions flex items-center gap-1" @click.stop>
            <UButton icon="i-lucide-plus" color="primary" variant="ghost" size="xs" title="Criar task nesta coluna"
              @click="beginCreateTaskInColumn(column)" />
            <UPopover :content="{ side: 'bottom', align: 'end' }">
              <UButton icon="i-lucide-ellipsis" color="neutral" variant="ghost" size="xs" title="Acoes da coluna" />
              <template #content>
                <div class="tasks-page__column-menu w-56 space-y-1 p-1">
                  <UPopover :content="{ side: 'right', align: 'start' }">
                    <button class="tasks-page__column-menu-item" type="button" :disabled="!column.editable"
                      @click="prepareColumnDraft(column)">
                      <UIcon name="i-lucide-list-restart" class="h-4 w-4" />
                      <span>Editar grupo</span>
                    </button>
                    <template #content>
                      <div class="tasks-page__column-editor-popover w-72 space-y-3 p-3" @click.stop>
                        <div class="space-y-1">
                          <p
                            class="tasks-page__settings-label text-[11px] font-semibold uppercase tracking-wide text-[rgb(var(--muted))]">
                            Nome</p>
                          <UInput v-model="columnDraft.label" placeholder="Nome da coluna"
                            @keydown.enter.prevent="saveColumnSettings" />
                        </div>
                        <div class="space-y-1">
                          <p
                            class="tasks-page__settings-label text-[11px] font-semibold uppercase tracking-wide text-[rgb(var(--muted))]">
                            Cor</p>
                          <OmniSelectMenuInput v-model="columnDraft.color" :items="COLUMN_COLOR_OPTIONS"
                            placeholder="Cor" :searchable="false" :full-content-width="true"
                            item-display-mode="text" color="neutral" variant="none" :highlight="false"
                            :badge-mode="true" option-edit-mode="color" />
                        </div>
                        <div
                          class="flex items-center justify-between gap-2 border-t border-[rgb(var(--border))] pt-2">
                          <UButton icon="i-lucide-trash-2" label="Excluir" color="error" variant="ghost" size="xs"
                            :disabled="boardSchemaColumns.length <= 1" @click="deleteColumn" />
                          <UButton label="Salvar" color="primary" size="xs" @click="saveColumnSettings" />
                        </div>
                      </div>
                    </template>
                  </UPopover>
                  <button class="tasks-page__column-menu-item" type="button" @click="toggleAggregation">
                    <UIcon :name="boardView.showAggregation === false ? 'i-lucide-eye' : 'i-lucide-eye-off'"
                      class="h-4 w-4" />
                    <span>{{ boardView.showAggregation === false ? 'Mostrar contagem' : 'Ocultar contagem' }}</span>
                  </button>
                  <button class="tasks-page__column-menu-item" type="button" @click="hideColumn(column.id)">
                    <UIcon name="i-lucide-eye-off" class="h-4 w-4" />
                    <span>Ocultar grupo</span>
                  </button>
                  <button class="tasks-page__column-menu-item tasks-page__column-menu-item--danger" type="button"
                    @click="deleteTasksInColumn(column)">
                    <UIcon name="i-lucide-trash-2" class="h-4 w-4" />
                    <span>Excluir cards do grupo</span>
                  </button>
                </div>
              </template>
            </UPopover>
          </div>
        </header>

        <div class="tasks-page__board-column-body space-y-2 p-2">
          <article v-for="(task, index) in column.tasks" :key="task.id"
            class="tasks-page__board-card cursor-pointer rounded-[var(--radius-sm)] border border-[rgb(var(--border))] bg-[rgb(var(--surface))] p-3 transition-colors hover:border-primary"
            draggable="true" :class="{
              'tasks-page__board-card--drop-before': dropTarget.columnId === column.id && dropTarget.index === index,
              'tasks-page__board-card--paused': isTracking(task.id) && !isRunning(task.id),
              'tasks-page__board-card--running': isRunning(task.id)
            }" @dragstart.stop="onDragStart(task, $event)" @dragend="onDragEnd"
            @dragover.prevent="markDropTarget(column.id, index)"
            @dragenter.prevent="markDropTarget(column.id, index)" @drop.stop.prevent="onDropCard(column, index)"
            @click="openTaskEditor(task)" @focusin="activeInlineTaskId = task.id"
            @focusout="onCardFocusOut($event, task)">
            <div class="tasks-page__board-card-head mb-2 flex items-start justify-between gap-2">
              <UInput :model-value="task.title" class="tasks-page__board-card-title-input min-w-0 flex-1"
                :data-task-title-input="task.id" size="xs" variant="none" @click.stop
                @update:model-value="updateTaskInline(task, { title: normalizeText($event, 220) || task.title })" />
              <div class="tasks-page__board-card-actions flex items-center gap-0.5" @click.stop>
                <UButton v-if="!isTracking(task.id)" color="neutral" variant="ghost" size="xs"
                  title="Iniciar tracking" @click="startTracking(task.id)">
                  <template #leading>
                    <span class="iconify i-lucide:play shrink-0 size-3" aria-hidden="true"></span>
                  </template>
                </UButton>
                <UButton v-if="isRunning(task.id)" color="neutral" variant="ghost" size="xs" title="Pausar tracking"
                  @click="pauseTracking(task.id)">
                  <template #leading>
                    <span class="iconify i-lucide:pause shrink-0 size-3" aria-hidden="true"></span>
                  </template>
                </UButton>
                <UButton v-if="isTracking(task.id) && !isRunning(task.id)" color="neutral" variant="ghost" size="xs"
                  title="Retomar tracking" @click="startTracking(task.id)">
                  <template #leading>
                    <span class="iconify i-lucide:play shrink-0 size-3" aria-hidden="true"></span>
                  </template>
                </UButton>
                <UButton v-if="isTracking(task.id)" color="neutral" variant="ghost" size="xs" title="Parar tracking"
                  @click="stopTracking(task.id)">
                  <template #leading>
                    <span class="iconify i-lucide:square shrink-0 size-3" aria-hidden="true"></span>
                  </template>
                </UButton>
                <UButton color="neutral" variant="ghost" size="xs"
                  :title="task.archived ? 'Desarquivar' : 'Arquivar'" @click="toggleArchive(task)">
                  <template #leading>
                    <span class="iconify i-lucide:archive shrink-0 size-3" aria-hidden="true"></span>
                  </template>
                </UButton>
                <UButton color="neutral" variant="ghost" size="xs" title="Excluir" @click="deleteTask(task)">
                  <template #leading>
                    <span class="iconify i-lucide:trash-2 shrink-0 size-3" aria-hidden="true"></span>
                  </template>
                </UButton>
              </div>
            </div>

            <p v-if="task.description && boardView.visibleFieldKeys.includes('description')"
              class="tasks-page__board-card-description line-clamp-2 text-xs text-[rgb(var(--muted))]">{{
                task.description }}
            </p>

            <div class="tasks-page__board-card-inline mt-2 flex flex-col items-start gap-1" @click.stop>
              <OmniSelectMenuInput v-if="isCardFieldVisible(task, 'status') && task.status"
                :model-value="task.status" :items="statusOptions" placeholder="Status" :searchable="true"
                :full-content-width="true" item-display-mode="text" color="neutral" variant="none"
                :highlight="false" :badge-mode="true" trailing-icon="" option-edit-mode="color"
                @update:model-value="updateTaskInline(task, { status: normalizeText($event, 120) || task.status })" />
              <OmniSelectMenuInput v-if="isCardFieldVisible(task, 'responsible') && task.responsible"
                class="tasks-page__board-card-people" :model-value="task.responsible"
                :items="responsibleOptionsAvatar" placeholder="Responsavel"
                :creatable="{ when: 'always', position: 'bottom' }" :searchable="true" :full-content-width="true"
                item-display-mode="rich" :show-avatar="true" color="neutral" variant="none" :highlight="false"
                :badge-mode="true" trailing-icon="" clear option-edit-mode="full"
                @update:model-value="updateTaskInline(task, { responsible: normalizeText($event, 120) })" />
              <OmniSelectMenuInput v-if="isCardFieldVisible(task, 'involved') && task.involved?.length"
                class="tasks-page__board-card-people" :model-value="task.involved" :items="involvedOptionsAvatar"
                placeholder="Envolvidos" :multiple="true" :creatable="{ when: 'always', position: 'bottom' }"
                :searchable="true" :full-content-width="true" item-display-mode="rich" :show-avatar="true"
                color="neutral" variant="none" :highlight="false" :badge-mode="true" trailing-icon="" clear
                option-edit-mode="full"
                @update:model-value="updateTaskInline(task, { involved: Array.isArray($event) ? $event.map((item: string) => normalizeText(item, 120)).filter(Boolean) : [] })" />
              <OmniSelectMenuInput v-if="isCardFieldVisible(task, 'client') && task.clientId"
                class="tasks-page__board-card-people" :model-value="task.clientId" :items="clientOptionsAvatar"
                placeholder="Cliente" :searchable="true" :full-content-width="true" item-display-mode="rich"
                :show-avatar="true" color="neutral" variant="none" :highlight="false" :badge-mode="true"
                trailing-icon="" option-edit-mode="color"
                @update:model-value="updateTaskInline(task, { clientId: toNumberId($event) || task.clientId, clientName: clientLabel(toNumberId($event) || task.clientId) })" />
              <OmniSelectMenuInput v-if="isCardFieldVisible(task, 'type') && task.type" :model-value="task.type"
                :items="typeOptions" placeholder="Tipo" :creatable="{ when: 'always', position: 'bottom' }"
                :searchable="true" :full-content-width="true" item-display-mode="text" color="neutral"
                variant="none" :highlight="false" :badge-mode="true" trailing-icon="" clear option-edit-mode="full"
                @update:model-value="updateTaskInline(task, { type: normalizeText($event, 120) })" />
              <OmniSelectMenuInput v-if="isCardFieldVisible(task, 'priority') && task.priority"
                :model-value="task.priority" :items="PRIORITY_OPTIONS" placeholder="Prioridade" :searchable="false"
                :full-content-width="true" item-display-mode="text" color="neutral" variant="none"
                :highlight="false" :badge-mode="true" trailing-icon="" option-edit-mode="color"
                @update:model-value="updateTaskInline(task, { priority: toPriority($event) })" />
            </div>

            <span v-if="isTracking(task.id) && !isRunning(task.id)" class="tasks-page__board-card-pause-dot" />

            <AppDatePicker v-if="isCardFieldVisible(task, 'dueDate')" :model-value="task.dueDate" placement="bottom"
              @update:model-value="updateTaskInline(task, { dueDate: $event })">
              <template #default="{ labelStart, labelEnd }">
                <button class="tasks-page__board-card-duedate mt-2 flex items-center gap-1.5 cursor-pointer"
                  type="button" @click.stop>
                  <UIcon name="i-lucide-calendar-days" class="h-3.5 w-3.5 text-[rgb(var(--muted))] shrink-0" />
                  <span v-if="labelStart" class="flex flex-col leading-tight">
                    <span class="text-[rgb(var(--text))]">{{ labelStart }}</span>
                    <span v-if="labelEnd" class="text-[rgb(var(--muted))]">{{ labelEnd }}</span>
                  </span>
                  <span v-else class="text-[rgb(var(--muted))]">Sem data</span>
                </button>
              </template>
            </AppDatePicker>

            <div v-if="isTracking(task.id)" class="tasks-page__board-card-timer mt-1" @click.stop>
              <UIcon name="i-lucide-timer" class="h-3 w-3 shrink-0" />
              <span>{{ formatElapsed(getElapsedMs(task.id)) }}</span>
            </div>

            <div v-if="isCardFieldVisible(task, 'createdAt')"
              class="tasks-page__board-card-date mt-1 flex items-center gap-1 text-xs text-[rgb(var(--muted))]">
              <UIcon name="i-lucide-clock-3" class="h-3.5 w-3.5" />
              <span>{{ dateLabel(task.createdAt) }}</span>
            </div>
          </article>

          <article v-if="creatingCards[column.id]"
            class="tasks-page__board-card tasks-page__board-card--draft rounded-[var(--radius-sm)] border border-primary bg-[rgb(var(--surface))] p-3"
            @focusout="onDraftCardFocusOut($event, column)">
            <UInput v-model="creatingCards[column.id].title" :data-draft-card="column.id"
              class="tasks-page__board-card-title-input" size="xs" variant="none" placeholder="Digite um nome..."
              @keydown.enter.prevent="commitDraftCard(column, true, true)"
              @keydown.esc.prevent="cancelDraftCard(column.id)" />

            <div
              v-if="isDraftFieldVisible(column.id, 'responsible') || isDraftFieldVisible(column.id, 'involved') || isDraftFieldVisible(column.id, 'clientId') || isDraftFieldVisible(column.id, 'type') || isDraftFieldVisible(column.id, 'priority') || isDraftFieldVisible(column.id, 'dueDate')"
              class="tasks-page__board-card-inline mt-2 flex flex-col items-start gap-1" @click.stop>
              <OmniSelectMenuInput v-if="isDraftFieldVisible(column.id, 'responsible')"
                class="tasks-page__board-card-people" v-model="creatingCards[column.id].responsible"
                :open="draftFieldOpen[column.id]?.responsible" :items="responsibleOptionsAvatar"
                placeholder="Responsavel" :creatable="{ when: 'always', position: 'bottom' }" :searchable="true"
                :full-content-width="true" item-display-mode="rich" :show-avatar="true" color="neutral"
                variant="none" :highlight="false" :badge-mode="true" trailing-icon="" clear option-edit-mode="full"
                @update:open="setDraftFieldOpen(column.id, 'responsible', $event)" />
              <OmniSelectMenuInput v-if="isDraftFieldVisible(column.id, 'involved')"
                class="tasks-page__board-card-people" v-model="creatingCards[column.id].involved"
                :open="draftFieldOpen[column.id]?.involved" :items="involvedOptionsAvatar" placeholder="Envolvidos"
                :multiple="true" :creatable="{ when: 'always', position: 'bottom' }" :searchable="true"
                :full-content-width="true" item-display-mode="rich" :show-avatar="true" color="neutral"
                variant="none" :highlight="false" :badge-mode="true" trailing-icon="" clear option-edit-mode="full"
                @update:open="setDraftFieldOpen(column.id, 'involved', $event)" />
              <OmniSelectMenuInput v-if="isDraftFieldVisible(column.id, 'clientId')"
                class="tasks-page__board-card-people" v-model="creatingCards[column.id].clientId"
                :open="draftFieldOpen[column.id]?.clientId" :items="clientOptionsAvatar" placeholder="Cliente"
                :searchable="true" :full-content-width="true" item-display-mode="rich" :show-avatar="true"
                color="neutral" variant="none" :highlight="false" :badge-mode="true" trailing-icon=""
                option-edit-mode="color"
                @update:model-value="creatingCards[column.id].clientName = clientLabel(toNumberId($event) || creatingCards[column.id].clientId)"
                @update:open="setDraftFieldOpen(column.id, 'clientId', $event)" />
              <OmniSelectMenuInput v-if="isDraftFieldVisible(column.id, 'type')"
                v-model="creatingCards[column.id].type" :open="draftFieldOpen[column.id]?.type" :items="typeOptions"
                placeholder="Tipo" :creatable="{ when: 'always', position: 'bottom' }" :searchable="true"
                :full-content-width="true" item-display-mode="text" color="neutral" variant="none"
                :highlight="false" :badge-mode="true" trailing-icon="" clear option-edit-mode="full"
                @update:open="setDraftFieldOpen(column.id, 'type', $event)" />
              <OmniSelectMenuInput v-if="isDraftFieldVisible(column.id, 'priority')"
                v-model="creatingCards[column.id].priority" :open="draftFieldOpen[column.id]?.priority"
                :items="PRIORITY_OPTIONS" placeholder="Prioridade" :searchable="false" :full-content-width="true"
                item-display-mode="text" color="neutral" variant="none" :highlight="false" :badge-mode="true"
                trailing-icon="" option-edit-mode="color"
                @update:open="setDraftFieldOpen(column.id, 'priority', $event)" />
              <AppDatePicker v-if="isDraftFieldVisible(column.id, 'dueDate')"
                :model-value="creatingCards[column.id].dueDate" :open="draftFieldOpen[column.id]?.dueDate"
                placement="bottom" @update:model-value="creatingCards[column.id].dueDate = $event"
                @update:open="setDraftFieldOpen(column.id, 'dueDate', $event)">
                <template #default="{ labelStart, labelEnd }">
                  <button class="tasks-page__board-card-duedate flex items-center gap-1.5 cursor-pointer"
                    type="button" @click.stop>
                    <UIcon name="i-lucide-calendar-days" class="h-3.5 w-3.5 text-[rgb(var(--muted))] shrink-0" />
                    <span v-if="labelStart" class="flex flex-col leading-tight">
                      <span>{{ labelStart }}</span>
                      <span v-if="labelEnd" class="text-[rgb(var(--muted))]">{{ labelEnd }}</span>
                    </span>
                    <span v-else class="text-[rgb(var(--muted))]">Sem data</span>
                  </button>
                </template>
              </AppDatePicker>
            </div>

            <div v-if="draftAvailableFields(column.id).length"
              class="tasks-page__draft-add-list mt-1.5 flex flex-col items-start">
              <button v-for="field in draftAvailableFields(column.id)" :key="field.key" type="button"
                class="tasks-page__draft-add-row" @mousedown.prevent @click="addDraftField(column.id, field.key)">
                <UIcon :name="field.icon" class="tasks-page__draft-add-row-icon" />
                <span>Adicionar {{ field.label }}</span>
              </button>
            </div>
          </article>

          <UAlert v-if="column.tasks.length === 0 && !creatingCards[column.id]" class="tasks-page__board-empty"
            color="neutral" variant="soft" icon="i-lucide-inbox" title="Sem tasks"
            description="Crie ou arraste cards para esta coluna." />

          <button class="tasks-page__board-add-card" type="button" @click="beginCreateTaskInColumn(column)">
            <UIcon name="i-lucide-plus" class="h-4 w-4" />
            <span></span>
          </button>
        </div>
      </section>
    </div>
  </div>
</template>
