<script setup lang="ts">
import { inject } from 'vue'
import { TASKS_PAGE_CONTEXT_KEY } from '../composables/useTasksPageContext'
import OmniSelectMenuInput from './inputs/OmniSelectMenuInput.vue'
import AppDatePicker from './AppDatePicker.vue'
import OmniEditor from './editor/TasksRichEditor.vue'

const ctx = inject(TASKS_PAGE_CONTEXT_KEY)!
const {
  taskEditorOpen,
  taskEditorMode,
  closeTaskEditor,
  setTaskEditorMode,
  modalModeOptions,
  taskDraft,
  isModalFieldVisible,
  statusOptions,
  responsibleOptions,
  involvedOptions,
  clientOptions,
  typeOptions,
  PRIORITY_OPTIONS,
  isTracking,
  isRunning,
  startTracking,
  pauseTracking,
  stopTracking,
  formatElapsed,
  getElapsedMs,
  dateLabel,
  currentUserName,
  peopleMentionLabels,
  clientMentionLabels,
  taskMentionLabels,
  startTaskEditorResize,
  projectSettingsOpen,
  viewerUserType,
  taskSaving,
  saveTask,
  deleteCurrentDraftTask,
} = ctx
</script>

<template>
  <USlideover v-model:open="taskEditorOpen"
    :ui="{ content: `tasks-page__task-overlay tasks-page__task-overlay--${taskEditorMode}` }"
    @update:open="(open: boolean) => { if (!open) closeTaskEditor() }">
    <template #header>
      <div class="tasks-page__task-modal-header flex w-full items-center justify-between gap-3">
        <div class="flex items-center gap-1">
          <UButton icon="i-lucide-chevrons-right" color="neutral" variant="ghost" size="xs" title="Fechar"
            @click="closeTaskEditor" />
          <UButton icon="i-lucide-expand" color="neutral" variant="ghost" size="xs" title="Pagina inteira"
            @click="setTaskEditorMode('fullscreen')" />
          <UPopover :content="{ side: 'bottom', align: 'start' }">
            <UButton icon="i-lucide-panel-right" color="neutral" variant="ghost" size="xs" title="Modo do modal" />
            <template #content>
              <div class="tasks-page__task-mode-menu w-56 space-y-1 p-1">
                <button v-for="option in modalModeOptions" :key="option.value" type="button"
                  class="tasks-page__task-mode-item" @click="setTaskEditorMode(option.value)">
                  <UIcon :name="option.icon" class="h-4 w-4" />
                  <span>{{ option.label }}</span>
                  <UIcon v-if="taskEditorMode === option.value" name="i-lucide-check" class="ml-auto h-4 w-4" />
                </button>
              </div>
            </template>
          </UPopover>
        </div>

        <div class="flex min-w-0 items-center justify-end gap-1">
          <UButton icon="i-lucide-lock-keyhole" label="Compartilhar" color="neutral" variant="ghost" size="xs" />
          <UButton icon="i-lucide-link" color="neutral" variant="ghost" size="xs" title="Copiar link" />
          <UButton icon="i-lucide-star" color="neutral" variant="ghost" size="xs" title="Favoritar" />
          <UButton icon="i-lucide-ellipsis" color="neutral" variant="ghost" size="xs" title="Mais opcoes" />
        </div>
      </div>
    </template>

    <template #body>
      <div class="tasks-page__task-editor">
        <button v-if="taskEditorMode === 'side'" class="tasks-page__task-resize-handle" type="button"
          aria-label="Redimensionar modal" @mousedown="startTaskEditorResize" />

        <div class="tasks-page__task-title-row">
          <UInput v-model="taskDraft.title" class="tasks-page__task-title-input" variant="none"
            placeholder="Nova task" autofocus @keydown.enter.prevent="saveTask" />
        </div>

        <div class="tasks-page__task-properties">
          <div v-if="isModalFieldVisible('status')" class="tasks-page__task-property-row">
            <span class="tasks-page__task-property-label">
              <UIcon name="i-lucide-loader-circle" />Status
            </span>
            <OmniSelectMenuInput v-model="taskDraft.status" class="tasks-page__task-property-control"
              :items="statusOptions" placeholder="Empty" :searchable="true" :full-content-width="true"
              item-display-mode="text" color="neutral" variant="none" :highlight="false" :badge-mode="true" clear
              option-edit-mode="color" />
          </div>

          <div v-if="isModalFieldVisible('responsible')" class="tasks-page__task-property-row">
            <span class="tasks-page__task-property-label">
              <UIcon name="i-lucide-user-round" />Responsavel
            </span>
            <OmniSelectMenuInput v-model="taskDraft.responsible" class="tasks-page__task-property-control"
              :items="responsibleOptions" placeholder="Empty" :creatable="{ when: 'always', position: 'bottom' }"
              :searchable="true" :full-content-width="true" item-display-mode="text" color="neutral" variant="none"
              :highlight="false" :badge-mode="true" clear option-edit-mode="full" />
          </div>

          <div v-if="isModalFieldVisible('involved')" class="tasks-page__task-property-row">
            <span class="tasks-page__task-property-label">
              <UIcon name="i-lucide-users-round" />Envolvidos
            </span>
            <OmniSelectMenuInput v-model="taskDraft.involved" class="tasks-page__task-property-control"
              :items="involvedOptions" placeholder="Empty" :multiple="true"
              :creatable="{ when: 'always', position: 'bottom' }" :searchable="true" :full-content-width="true"
              item-display-mode="text" color="neutral" variant="none" :highlight="false" :badge-mode="true" clear
              option-edit-mode="full" />
          </div>

          <div v-if="viewerUserType === 'admin' && isModalFieldVisible('clientId')"
            class="tasks-page__task-property-row">
            <span class="tasks-page__task-property-label">
              <UIcon name="i-lucide-circle-dot" />Cliente
            </span>
            <OmniSelectMenuInput v-model="taskDraft.clientId" class="tasks-page__task-property-control"
              :items="clientOptions" placeholder="Empty" :searchable="true" :full-content-width="true"
              item-display-mode="text" color="neutral" variant="none" :highlight="false" :badge-mode="true" clear
              option-edit-mode="color" />
          </div>

          <div v-if="isModalFieldVisible('dueDate')" class="tasks-page__task-property-row">
            <span class="tasks-page__task-property-label">
              <UIcon name="i-lucide-calendar-days" />Prazo
            </span>
            <AppDatePicker v-model="taskDraft.dueDate" placement="left">
              <template #default="{ labelStart, labelEnd }">
                <button class="tasks-page__task-date-btn" type="button">
                  <span v-if="labelStart" class="flex flex-col leading-tight">
                    <span>{{ labelStart }}</span>
                    <span v-if="labelEnd" class="tasks-page__task-date-btn--end">{{ labelEnd }}</span>
                  </span>
                  <span v-else class="tasks-page__task-date-btn--empty">Sem data</span>
                </button>
              </template>
            </AppDatePicker>
          </div>

          <div v-if="isModalFieldVisible('priority')" class="tasks-page__task-property-row">
            <span class="tasks-page__task-property-label">
              <UIcon name="i-lucide-badge-alert" />Prioridade
            </span>
            <OmniSelectMenuInput v-model="taskDraft.priority" class="tasks-page__task-property-control"
              :items="PRIORITY_OPTIONS" placeholder="Empty" :searchable="false" :full-content-width="true"
              item-display-mode="text" color="neutral" variant="none" :highlight="false" :badge-mode="true" clear
              option-edit-mode="color" />
          </div>

          <div v-if="taskDraft.id" class="tasks-page__task-property-row">
            <span class="tasks-page__task-property-label">
              <UIcon name="i-lucide-timer" />Tracking
            </span>
            <div class="tasks-page__task-tracking-controls flex items-center gap-1.5">
              <span v-if="isTracking(taskDraft.id)" class="tasks-page__task-tracking-timer">{{
                formatElapsed(getElapsedMs(taskDraft.id)) }}</span>
              <UButton v-if="!isTracking(taskDraft.id)" icon="i-lucide-play" color="neutral" variant="ghost" size="xs"
                title="Iniciar tracking" @click="startTracking(taskDraft.id)" />
              <UButton v-if="isRunning(taskDraft.id)" icon="i-lucide-pause" color="neutral" variant="ghost" size="xs"
                title="Pausar tracking" @click="pauseTracking(taskDraft.id)" />
              <UButton v-if="isTracking(taskDraft.id) && !isRunning(taskDraft.id)" icon="i-lucide-play"
                color="neutral" variant="ghost" size="xs" title="Retomar tracking"
                @click="startTracking(taskDraft.id)" />
              <UButton v-if="isTracking(taskDraft.id)" icon="i-lucide-square" color="neutral" variant="ghost"
                size="xs" title="Parar tracking" @click="stopTracking(taskDraft.id)" />
            </div>
          </div>

          <div v-if="isModalFieldVisible('type')" class="tasks-page__task-property-row">
            <span class="tasks-page__task-property-label">
              <UIcon name="i-lucide-hash" />Tipo
            </span>
            <OmniSelectMenuInput v-model="taskDraft.type" class="tasks-page__task-property-control"
              :items="typeOptions" placeholder="Empty" :creatable="{ when: 'always', position: 'bottom' }"
              :searchable="true" :full-content-width="true" item-display-mode="text" color="neutral" variant="none"
              :highlight="false" :badge-mode="true" clear option-edit-mode="full" />
          </div>

          <div v-if="isModalFieldVisible('createdAt') && taskDraft.createdAt" class="tasks-page__task-property-row">
            <span class="tasks-page__task-property-label">
              <UIcon name="i-lucide-clock-3" />Criada em
            </span>
            <span class="tasks-page__task-property-static">{{ dateLabel(taskDraft.createdAt) }}</span>
          </div>
        </div>

        <UButton icon="i-lucide-plus" label="Add a property" color="neutral" variant="ghost" size="sm"
          @click="projectSettingsOpen = true" />

        <div v-if="isModalFieldVisible('description')" class="tasks-page__task-description-bridge">
          <UTextarea v-model="taskDraft.description" variant="none" :rows="2" placeholder="Resumo curto..." />
        </div>

        <div class="tasks-page__task-comments">
          <p class="tasks-page__task-comments-title">Comments</p>
          <div class="tasks-page__task-comment-input">
            <UAvatar :text="currentUserName.slice(0, 1)" size="xs" />
            <UInput variant="none" placeholder="Add a comment..." />
          </div>
        </div>

        <OmniEditor v-model="taskDraft.contentHtml" class="tasks-page__task-rich-editor" :people="peopleMentionLabels"
          :clients="clientMentionLabels" :tasks="taskMentionLabels" content-type="html" min-height="320px"
          max-height="52vh" placeholder="Press '/' for commands, ':' for emojis, '@' to mention..." />

        <label v-if="isModalFieldVisible('archived')" class="tasks-page__task-archived">
          <span>Task arquivada</span>
          <USwitch v-model="taskDraft.archived" />
        </label>
      </div>
    </template>

    <template #footer>
      <div class="tasks-page__task-footer flex w-full items-center justify-between gap-2">
        <UButton icon="i-lucide-trash-2" label="Excluir" color="error" variant="ghost" :disabled="!taskDraft.id"
          @click="deleteCurrentDraftTask" />
        <div class="flex items-center gap-2">
          <UButton label="Cancelar" color="neutral" variant="ghost" @click="closeTaskEditor" />
          <UButton label="Salvar task" color="primary" :loading="taskSaving" @click="saveTask" />
        </div>
      </div>
    </template>
  </USlideover>
</template>
