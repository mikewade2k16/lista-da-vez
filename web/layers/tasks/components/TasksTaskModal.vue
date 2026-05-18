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
  taskRelations,
  isModalFieldVisible,
  statusOptions,
  responsibleOptionsAvatar,
  involvedOptionsForResponsible,
  clientOptionsAvatar,
  typeOptions,
  PRIORITY_OPTIONS,
  isTracking,
  isRunning,
  startTracking,
  pauseTracking,
  stopTracking,
  formatElapsed,
  getElapsedMs,
  presenceParticipants,
  focusPresenceField,
  blurPresenceField,
  presenceFieldLabel,
  isPresenceFieldLocked,
  dateLabel,
  currentUserName,
  peopleMentionLabels,
  clientMentionLabels,
  taskMentionLabels,
  startTaskEditorResize,
  projectSettingsOpen,
  viewerUserType,
  taskSaving,
  taskVideoDrafts,
  flushTaskDraftAutosave,
  onTaskVideoInput,
  onTaskVideoDrop,
  removeTaskVideoDraft,
  deleteCurrentDraftTask,
} = ctx
</script>

<template>
  <USlideover v-model:open="taskEditorOpen"
    :overlay="false" :modal="false" :dismissible="false"
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

        <div class="flex min-w-0 items-center justify-end gap-2">
          <div v-if="presenceParticipants.length" class="tasks-page__presence-stack" :title="`${presenceParticipants.length} pessoa(s) nesta task`">
            <UAvatar
              v-for="participant in presenceParticipants.slice(0, 4)"
              :key="participant.userId"
              :src="participant.avatarPath || undefined"
              :text="participant.avatarText"
              size="xs"
              class="tasks-page__presence-avatar"
            />
            <span v-if="presenceParticipants.length > 4" class="tasks-page__presence-more">
              +{{ presenceParticipants.length - 4 }}
            </span>
          </div>
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

        <div class="tasks-page__task-title-row" @focusin="focusPresenceField('title')" @focusout="blurPresenceField('title', $event)">
          <span v-if="presenceFieldLabel('title')" class="tasks-page__presence-field tasks-page__presence-field--title">
            {{ presenceFieldLabel('title') }}
          </span>
          <UInput v-model="taskDraft.title" class="tasks-page__task-title-input" variant="none"
            :disabled="isPresenceFieldLocked('title')"
            placeholder="Nova task" autofocus @keydown.enter.prevent="flushTaskDraftAutosave" />
        </div>

        <div class="tasks-page__task-properties">
          <div v-if="isModalFieldVisible('status')" class="tasks-page__task-property-row"
            @focusin="focusPresenceField('status')"
            @click.capture="focusPresenceField('status')">
            <span class="tasks-page__task-property-label">
              <span class="tasks-page__task-property-label-main"><UIcon name="i-lucide-loader-circle" />Status</span>
              <span v-if="presenceFieldLabel('status')" class="tasks-page__presence-field">{{ presenceFieldLabel('status') }}</span>
            </span>
            <OmniSelectMenuInput v-model="taskDraft.status" class="tasks-page__task-property-control"
              :items="statusOptions" placeholder="Empty" :searchable="true" :full-content-width="true"
              item-display-mode="text" color="neutral" variant="none" :highlight="false" :badge-mode="true" clear
              :disabled="isPresenceFieldLocked('status')"
              option-edit-mode="color" @update:open="(open: boolean) => open ? focusPresenceField('status') : blurPresenceField('status')" />
          </div>

          <div v-if="isModalFieldVisible('responsible')" class="tasks-page__task-property-row"
            @focusin="focusPresenceField('responsible')"
            @click.capture="focusPresenceField('responsible')">
            <span class="tasks-page__task-property-label">
              <span class="tasks-page__task-property-label-main"><UIcon name="i-lucide-user-round" />Responsavel</span>
              <span v-if="presenceFieldLabel('responsible')" class="tasks-page__presence-field">{{ presenceFieldLabel('responsible') }}</span>
            </span>
            <OmniSelectMenuInput v-model="taskDraft.responsible" class="tasks-page__task-property-control"
              :items="responsibleOptionsAvatar" placeholder="Empty" :searchable="true" :full-content-width="true"
              item-display-mode="rich" :show-avatar="true" color="neutral" variant="none" :highlight="false"
              :badge-mode="true" badge-style="entity" clear option-edit-mode="color"
              :disabled="isPresenceFieldLocked('responsible')"
              @update:open="(open: boolean) => open ? focusPresenceField('responsible') : blurPresenceField('responsible')" />
          </div>

          <div v-if="isModalFieldVisible('involved')" class="tasks-page__task-property-row"
            @focusin="focusPresenceField('involved')"
            @click.capture="focusPresenceField('involved')">
            <span class="tasks-page__task-property-label">
              <span class="tasks-page__task-property-label-main"><UIcon name="i-lucide-users-round" />Envolvidos</span>
              <span v-if="presenceFieldLabel('involved')" class="tasks-page__presence-field">{{ presenceFieldLabel('involved') }}</span>
            </span>
            <OmniSelectMenuInput v-model="taskDraft.involved" class="tasks-page__task-property-control"
              :items="involvedOptionsForResponsible(taskDraft.responsible)" placeholder="Empty" :multiple="true"
              :searchable="true" :full-content-width="true" item-display-mode="rich" :show-avatar="true"
              color="neutral" variant="none" :highlight="false" :badge-mode="true" badge-style="entity" clear
              :disabled="isPresenceFieldLocked('involved')"
              option-edit-mode="color" @update:open="(open: boolean) => open ? focusPresenceField('involved') : blurPresenceField('involved')" />
          </div>

          <div v-if="viewerUserType === 'admin' && isModalFieldVisible('clientId')"
            class="tasks-page__task-property-row"
            @focusin="focusPresenceField('clientId')"
            @click.capture="focusPresenceField('clientId')">
            <span class="tasks-page__task-property-label">
              <span class="tasks-page__task-property-label-main"><UIcon name="i-lucide-circle-dot" />Cliente</span>
              <span v-if="presenceFieldLabel('clientId')" class="tasks-page__presence-field">{{ presenceFieldLabel('clientId') }}</span>
            </span>
            <OmniSelectMenuInput v-model="taskDraft.clientId" class="tasks-page__task-property-control"
              :items="clientOptionsAvatar" placeholder="Empty" :searchable="true" :full-content-width="true"
              item-display-mode="rich" :show-avatar="true" color="neutral" variant="none" :highlight="false"
              :badge-mode="true" badge-style="entity" clear option-edit-mode="color"
              :disabled="isPresenceFieldLocked('clientId')"
              @update:open="(open: boolean) => open ? focusPresenceField('clientId') : blurPresenceField('clientId')" />
          </div>

          <div v-if="isModalFieldVisible('dueDate')" class="tasks-page__task-property-row"
            @focusin="focusPresenceField('dueDate')"
            @click.capture="focusPresenceField('dueDate')">
            <span class="tasks-page__task-property-label">
              <span class="tasks-page__task-property-label-main"><UIcon name="i-lucide-calendar-days" />Prazo</span>
              <span v-if="presenceFieldLabel('dueDate')" class="tasks-page__presence-field">{{ presenceFieldLabel('dueDate') }}</span>
            </span>
            <AppDatePicker v-model="taskDraft.dueDate" v-model:end-date="taskDraft.dueEndDate" placement="left"
              @update:open="(open: boolean) => open ? focusPresenceField('dueDate') : blurPresenceField('dueDate')">
              <template #default="{ labelStart, labelEnd }">
                <button class="tasks-page__task-date-btn" type="button" :disabled="isPresenceFieldLocked('dueDate')">
                  <span v-if="labelStart" class="flex flex-col leading-tight">
                    <span>{{ labelStart }}</span>
                    <span v-if="labelEnd" class="tasks-page__task-date-btn--end">{{ labelEnd }}</span>
                  </span>
                  <span v-else class="tasks-page__task-date-btn--empty">Sem data</span>
                </button>
              </template>
            </AppDatePicker>
          </div>

          <div v-if="isModalFieldVisible('priority')" class="tasks-page__task-property-row"
            @focusin="focusPresenceField('priority')"
            @click.capture="focusPresenceField('priority')">
            <span class="tasks-page__task-property-label">
              <span class="tasks-page__task-property-label-main"><UIcon name="i-lucide-badge-alert" />Prioridade</span>
              <span v-if="presenceFieldLabel('priority')" class="tasks-page__presence-field">{{ presenceFieldLabel('priority') }}</span>
            </span>
            <OmniSelectMenuInput v-model="taskDraft.priority" class="tasks-page__task-property-control"
              :items="PRIORITY_OPTIONS" placeholder="Empty" :searchable="false" :full-content-width="true"
              item-display-mode="text" color="neutral" variant="none" :highlight="false" :badge-mode="true" clear
              :disabled="isPresenceFieldLocked('priority')"
              option-edit-mode="color" @update:open="(open: boolean) => open ? focusPresenceField('priority') : blurPresenceField('priority')" />
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

          <div v-if="isModalFieldVisible('type')" class="tasks-page__task-property-row"
            @focusin="focusPresenceField('type')"
            @click.capture="focusPresenceField('type')">
            <span class="tasks-page__task-property-label">
              <span class="tasks-page__task-property-label-main"><UIcon name="i-lucide-hash" />Tipo</span>
              <span v-if="presenceFieldLabel('type')" class="tasks-page__presence-field">{{ presenceFieldLabel('type') }}</span>
            </span>
            <OmniSelectMenuInput v-model="taskDraft.type" class="tasks-page__task-property-control"
              :items="typeOptions" placeholder="Empty" :creatable="{ when: 'always', position: 'bottom' }"
              :searchable="true" :full-content-width="true" item-display-mode="text" color="neutral" variant="none"
              :highlight="false" :badge-mode="true" clear option-edit-mode="full"
              :disabled="isPresenceFieldLocked('type')"
              @update:open="(open: boolean) => open ? focusPresenceField('type') : blurPresenceField('type')" />
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

        <div class="tasks-page__task-video-upload" @focusin="focusPresenceField('videos')"
          @focusout="blurPresenceField('videos', $event)" @dragenter="focusPresenceField('videos')">
          <div class="tasks-page__task-video-head">
            <span class="tasks-page__task-video-title">
              <UIcon name="i-lucide-video" />Videos
              <span v-if="presenceFieldLabel('videos')" class="tasks-page__presence-field tasks-page__presence-field--inline">
                {{ presenceFieldLabel('videos') }}
              </span>
            </span>
            <label class="tasks-page__task-video-action">
              <UIcon name="i-lucide-upload" />
              <span>Adicionar video</span>
              <input class="sr-only" type="file" accept="video/*" multiple @change="onTaskVideoInput" />
            </label>
          </div>
          <label class="tasks-page__task-video-drop" @dragover.prevent @drop.prevent="onTaskVideoDrop">
            <UIcon name="i-lucide-film" />
            <span>Solte arquivos de video aqui</span>
            <small>MP4, MOV, WebM</small>
            <input class="sr-only" type="file" accept="video/*" multiple @change="onTaskVideoInput" />
          </label>
          <div v-if="taskVideoDrafts.length" class="tasks-page__task-video-list">
            <div v-for="file in taskVideoDrafts" :key="file.id" class="tasks-page__task-video-item">
              <video v-if="file.url" :src="file.url" controls preload="metadata" />
              <div class="tasks-page__task-video-meta min-w-0">
                <p class="truncate">{{ file.name }}</p>
                <span>{{ file.sizeLabel }}</span>
              </div>
              <UButton icon="i-lucide-x" color="neutral" variant="ghost" size="xs" title="Remover video"
                @click="removeTaskVideoDraft(file.id)" />
            </div>
          </div>
        </div>

        <div v-if="taskRelations.relations.value.length || taskRelations.status.value === 'loading'"
          class="tasks-page__task-relations">
          <p class="tasks-page__task-relations-title">
            <UIcon name="i-lucide-link-2" />
            <span>Vinculos</span>
            <span v-if="taskRelations.status.value === 'loading'" class="tasks-page__task-relations-loading">
              <UIcon name="i-lucide-loader-circle" class="animate-spin" />
            </span>
          </p>
          <ul v-if="taskRelations.relations.value.length" class="tasks-page__task-relations-list">
            <li v-for="relation in taskRelations.relations.value" :key="relation.id"
              class="tasks-page__task-relations-item">
              <UIcon
                :name="relation.module === 'crm' ? 'i-lucide-user-round'
                  : relation.module === 'erp' ? 'i-lucide-package'
                  : relation.module === 'operations' ? 'i-lucide-clipboard-list'
                  : 'i-lucide-link'"
                class="tasks-page__task-relations-icon" />
              <span class="tasks-page__task-relations-label">{{ relation.labelCache || relation.resourceId }}</span>
              <span class="tasks-page__task-relations-type">{{ relation.resourceType }}</span>
              <UBadge v-if="typeof relation.metadataCache.status === 'string' && relation.metadataCache.status"
                :color="relation.metadataCache.status === 'unknown' ? 'neutral'
                  : relation.metadataCache.status === 'active' ? 'success'
                  : 'warning'"
                variant="soft" size="xs">{{ relation.metadataCache.status }}</UBadge>
              <UButton v-if="typeof relation.metadataCache.url === 'string' && relation.metadataCache.url"
                :to="relation.metadataCache.url" target="_blank" external icon="i-lucide-external-link"
                color="neutral" variant="ghost" size="xs" title="Abrir recurso" />
            </li>
          </ul>
          <p v-else-if="taskRelations.errorMessage.value" class="tasks-page__task-relations-error">
            {{ taskRelations.errorMessage.value }}
          </p>
        </div>

        <div class="tasks-page__task-comments">
          <p class="tasks-page__task-comments-title">Comments</p>
          <div class="tasks-page__task-comment-input">
            <UAvatar :text="currentUserName.slice(0, 1)" size="xs" />
            <UInput variant="none" placeholder="Add a comment..." />
          </div>
        </div>

        <div class="tasks-page__task-rich-editor-wrap" @focusin="focusPresenceField('description')"
          @focusout="blurPresenceField('description', $event)">
          <span v-if="presenceFieldLabel('description')" class="tasks-page__presence-field tasks-page__presence-field--editor">
            {{ presenceFieldLabel('description') }}
          </span>
          <OmniEditor v-model="taskDraft.contentHtml" class="tasks-page__task-rich-editor" :people="peopleMentionLabels"
            :clients="clientMentionLabels" :tasks="taskMentionLabels" content-type="html" min-height="320px"
            max-height="52vh" placeholder="Press '/' for commands, ':' for emojis, '@' to mention..." />
        </div>

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
          <span class="tasks-page__task-autosave-status">
            <UIcon :name="taskSaving ? 'i-lucide-loader-circle' : 'i-lucide-check'" :class="{ 'animate-spin': taskSaving }" />
            {{ taskSaving ? 'Salvando...' : 'Salvo automatico' }}
          </span>
          <UButton label="Fechar" color="neutral" variant="ghost" @click="closeTaskEditor" />
        </div>
      </div>
    </template>
  </USlideover>
</template>
