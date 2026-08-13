<script setup lang="ts">
import { computed, inject, unref } from 'vue'
import OmniMediaGrid, { type OmniMediaGridItem } from '~/components/ui/OmniMediaGrid.vue'
import { orderMediaItemsByIds } from '~/components/ui/media-grid/utils'
import { getApiBase } from '~/utils/api-client'
import { TASKS_PAGE_CONTEXT_KEY } from '../composables/useTasksPageContext'
import OmniLazySelectMenuInput from './inputs/OmniLazySelectMenuInput.vue'
import AppDatePicker from './AppDatePicker.vue'
import OmniEditor from '../../../app/components/omni/OmniEditor.vue'
import OmniEntityDrawer from '../../../app/components/ui/OmniEntityDrawer.vue'
import TaskChecklistField from './TaskChecklistField.vue'
import TaskVideoUploadDialog from './TaskVideoUploadDialog.vue'

const ctx = inject(TASKS_PAGE_CONTEXT_KEY)!
const {
  taskEditorOpen,
  taskEditorMode,
  taskEditorWidth,
  closeTaskEditor,
  taskDraft,
  taskDraftTitleValue,
  updateTaskDraftTitle,
  taskDraftContentValue,
  updateTaskDraftContent,
  taskDraftStatusValue,
  updateTaskDraftStatus,
  taskDraftResponsibleValue,
  updateTaskDraftResponsible,
  taskDraftInvolvedValue,
  updateTaskDraftInvolved,
  taskDraftChecklistValue,
  updateTaskDraftChecklist,
  taskDraftClientIdValue,
  updateTaskDraftClientId,
  taskDraftDueDateValue,
  taskDraftDueEndDateValue,
  updateTaskDraftDueDate,
  updateTaskDraftDueEndDate,
  taskDraftPriorityValue,
  updateTaskDraftPriority,
  taskDraftTypeValue,
  updateTaskDraftType,
  taskRelations,
  taskComments,
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
  presenceParticipants,
  focusPresenceField,
  blurPresenceField,
  presenceFieldLabel,
  isPresenceFieldLocked,
  dateLabel,
  dateLabelLong,
  currentUserName,
  peopleMentionLabels,
  clientMentionLabels,
  taskMentionLabels,
  projectSettingsOpen,
  viewerUserType,
  taskSaving,
  taskVideoDrafts,
  taskVideoSaving,
  taskVideoError,
  taskImageMaxBytes,
  taskVideoMaxBytes,
  taskVideoUploads,
  taskVideoItemDialogOpen,
  taskVideoPendingFileNames,
  taskVideoChecklistItems,
  flushTaskDraftAutosave,
  onTaskVideoFiles,
  cancelTaskVideoUpload,
  confirmTaskVideoUpload,
  updateTaskVideoChecklistItem,
  removeTaskVideoDraft,
  reorderTaskVideoDrafts,
  taskCalendarMedia,
  taskMediaOrder,
  deleteCurrentDraftTask,
} = ctx

const presenceViewingParticipants = computed(() =>
  (Array.isArray(unref(presenceParticipants)) ? unref(presenceParticipants) : []).filter(
    (participant) => !participant.fieldKey,
  ),
)

const runtimeConfig = useRuntimeConfig()
const taskVideoChecklistOptions = computed(() =>
  unref(taskVideoChecklistItems).map((item) => ({ label: item.title, value: item.id })),
)

function taskVideoProgressLabel(phase: 'uploading' | 'processing' | 'linking'): string {
  switch (phase) {
    case 'processing':
      return 'Arquivo recebido · preparando entrega segura'
    case 'linking':
      return 'Upload concluído · vinculando à tarefa'
    default:
      return 'Preview disponível · enviando arquivo'
  }
}

function taskVideoSrc(path: unknown) {
  const normalizedPath = String(path || '').trim()
  if (!normalizedPath) return ''
  try {
    return new URL(normalizedPath, getApiBase(runtimeConfig)).toString()
  } catch {
    return normalizedPath
  }
}

const taskMediaGridItems = computed<OmniMediaGridItem[]>(() => {
  const ownMedia = unref(taskVideoDrafts).map((file) => ({
    id: file.id,
    name: file.name,
    url: taskVideoSrc(file.url),
    type: file.contentType.toLowerCase().startsWith('image/')
      ? ('image' as const)
      : ('video' as const),
    sizeLabel: file.sizeLabel,
    removable: true,
    reorderable: true,
  }))
  const ownUrls = new Set(ownMedia.map((item) => item.url))
  const inheritedMedia = unref(taskCalendarMedia)
    .map((media) => ({
      id: `calendar:${media.id}`,
      name: media.name,
      url: taskVideoSrc(media.url),
      type: media.type,
      posterUrl: media.posterUrl ? taskVideoSrc(media.posterUrl) : undefined,
      sizeLabel: formatFileSize(media.sizeBytes),
      removable: false,
      reorderable: true,
      badgeLabel: 'Calendário',
      badgeTone: 'primary' as const,
    }))
    .filter((media) => !ownUrls.has(media.url))
  const orderedMedia = orderMediaItemsByIds([...ownMedia, ...inheritedMedia], unref(taskMediaOrder))
  return [
    ...unref(taskVideoUploads).map((upload) => ({
      id: upload.key,
      name: upload.name,
      url: upload.previewUrl,
      type: upload.type,
      sizeLabel: `${upload.percent}%`,
      pending: true,
      progress: upload.percent,
      status: taskVideoProgressLabel(upload.phase),
      removable: false,
      reorderable: false,
    })),
    ...orderedMedia,
  ]
})

function taskVideoDraftById(id: string) {
  return unref(taskVideoDrafts).find((file) => file.id === id)
}

const presenceViewingLabel = computed(() => {
  if (!presenceViewingParticipants.value.length) return ''
  if (presenceViewingParticipants.value.length === 1)
    return `${presenceViewingParticipants.value[0]!.displayName} visualizando`
  return `${presenceViewingParticipants.value[0]!.displayName} +${presenceViewingParticipants.value.length - 1} visualizando`
})

// Ponte para o modal-template (OmniEntityDrawer): ele e o core de todo modal; aqui
// so injetamos o conteudo da task nos slots. Modo e largura sao do shell (resize +
// toggle de fullscreen vivem la), espelhados de volta no contexto da pagina.
function onEditorOpenChange(open: boolean) {
  if (!open) closeTaskEditor()
}
function onEditorMode(mode: 'side' | 'center' | 'fullscreen') {
  taskEditorMode.value = mode
}
function onEditorWidth(width: number) {
  taskEditorWidth.value = width
}
</script>

<template>
  <OmniEntityDrawer
    :model-value="taskEditorOpen"
    :mode="taskEditorMode"
    :width="taskEditorWidth"
    @update:model-value="onEditorOpenChange"
    @update:mode="onEditorMode"
    @update:width="onEditorWidth"
  >
    <template #header-extra>
      <div
        v-if="presenceParticipants.length"
        class="tasks-page__presence-stack"
        :title="`${presenceParticipants.length} pessoa(s) nesta task`"
      >
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
      <span
        v-if="presenceViewingLabel"
        class="tasks-page__presence-field tasks-page__presence-field--inline"
      >
        {{ presenceViewingLabel }}
      </span>
      <UButton
        icon="i-lucide-lock-keyhole"
        label="Compartilhar"
        color="neutral"
        variant="ghost"
        size="xs"
      />
      <UButton icon="i-lucide-link" color="neutral" variant="ghost" size="xs" title="Copiar link" />
      <UButton icon="i-lucide-star" color="neutral" variant="ghost" size="xs" title="Favoritar" />
      <UButton
        icon="i-lucide-ellipsis"
        color="neutral"
        variant="ghost"
        size="xs"
        title="Mais opcoes"
      />
    </template>

    <div class="tasks-page__task-editor">
      <div
        class="tasks-page__task-title-row"
        @focusin="focusPresenceField('title')"
        @focusout="blurPresenceField('title', $event)"
      >
        <span
          v-if="presenceFieldLabel('title')"
          class="tasks-page__presence-field tasks-page__presence-field--title"
        >
          {{ presenceFieldLabel('title') }}
        </span>
        <UInput
          :model-value="taskDraftTitleValue()"
          class="tasks-page__task-title-input"
          variant="none"
          :disabled="isPresenceFieldLocked('title')"
          placeholder="Nova task"
          @update:model-value="updateTaskDraftTitle"
          @keydown.enter.prevent="flushTaskDraftAutosave"
        />
      </div>

      <div class="tasks-page__task-properties">
        <div
          v-if="isModalFieldVisible('status')"
          class="tasks-page__task-property-row"
          @focusin="focusPresenceField('status')"
          @click.capture="focusPresenceField('status')"
        >
          <span class="tasks-page__task-property-label">
            <span class="tasks-page__task-property-label-main">
              <UIcon name="i-lucide-loader-circle" />
              Status
            </span>
            <span v-if="presenceFieldLabel('status')" class="tasks-page__presence-field">
              {{ presenceFieldLabel('status') }}
            </span>
          </span>
          <OmniLazySelectMenuInput
            :model-value="taskDraftStatusValue()"
            class="tasks-page__task-property-control"
            :items="statusOptions"
            placeholder="Nao definido"
            :searchable="true"
            :full-content-width="true"
            item-display-mode="text"
            color="neutral"
            variant="none"
            :highlight="false"
            :badge-mode="true"
            :disabled="isPresenceFieldLocked('status')"
            option-edit-mode="color"
            @update:model-value="updateTaskDraftStatus"
            @update:open="
              (open: boolean) => (open ? focusPresenceField('status') : blurPresenceField('status'))
            "
          />
        </div>

        <div
          v-if="isModalFieldVisible('responsible')"
          class="tasks-page__task-property-row"
          @focusin="focusPresenceField('responsible')"
          @click.capture="focusPresenceField('responsible')"
        >
          <span class="tasks-page__task-property-label">
            <span class="tasks-page__task-property-label-main">
              <UIcon name="i-lucide-user-round" />
              Responsavel
            </span>
            <span v-if="presenceFieldLabel('responsible')" class="tasks-page__presence-field">
              {{ presenceFieldLabel('responsible') }}
            </span>
          </span>
          <OmniLazySelectMenuInput
            :model-value="taskDraftResponsibleValue()"
            class="tasks-page__task-property-control"
            :items="responsibleOptionsAvatar"
            placeholder="Nao definido"
            :searchable="true"
            :full-content-width="true"
            item-display-mode="rich"
            :show-avatar="true"
            color="neutral"
            variant="none"
            :highlight="false"
            :badge-mode="true"
            badge-style="entity"
            clear
            option-edit-mode="color"
            :disabled="isPresenceFieldLocked('responsible')"
            @update:model-value="updateTaskDraftResponsible"
            @update:open="
              (open: boolean) =>
                open ? focusPresenceField('responsible') : blurPresenceField('responsible')
            "
          />
        </div>

        <div
          v-if="isModalFieldVisible('involved')"
          class="tasks-page__task-property-row"
          @focusin="focusPresenceField('involved')"
          @click.capture="focusPresenceField('involved')"
        >
          <span class="tasks-page__task-property-label">
            <span class="tasks-page__task-property-label-main">
              <UIcon name="i-lucide-users-round" />
              Envolvidos
            </span>
            <span v-if="presenceFieldLabel('involved')" class="tasks-page__presence-field">
              {{ presenceFieldLabel('involved') }}
            </span>
          </span>
          <OmniLazySelectMenuInput
            :model-value="taskDraftInvolvedValue()"
            class="tasks-page__task-property-control"
            :items="involvedOptionsForResponsible(taskDraftResponsibleValue())"
            placeholder="Nao definido"
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
            clear
            :disabled="isPresenceFieldLocked('involved')"
            option-edit-mode="color"
            @update:model-value="updateTaskDraftInvolved"
            @update:open="
              (open: boolean) =>
                open ? focusPresenceField('involved') : blurPresenceField('involved')
            "
          />
        </div>

        <div
          v-if="viewerUserType === 'admin' && isModalFieldVisible('clientId')"
          class="tasks-page__task-property-row"
          @focusin="focusPresenceField('clientId')"
          @click.capture="focusPresenceField('clientId')"
        >
          <span class="tasks-page__task-property-label">
            <span class="tasks-page__task-property-label-main">
              <UIcon name="i-lucide-circle-dot" />
              Cliente
            </span>
            <span v-if="presenceFieldLabel('clientId')" class="tasks-page__presence-field">
              {{ presenceFieldLabel('clientId') }}
            </span>
          </span>
          <OmniLazySelectMenuInput
            :model-value="taskDraftClientIdValue()"
            class="tasks-page__task-property-control"
            :items="clientOptionsAvatar"
            placeholder="Nao definido"
            :searchable="true"
            :full-content-width="true"
            item-display-mode="rich"
            :show-avatar="true"
            color="neutral"
            variant="none"
            :highlight="false"
            :badge-mode="true"
            badge-style="entity"
            clear
            option-edit-mode="color"
            :disabled="isPresenceFieldLocked('clientId')"
            @update:model-value="updateTaskDraftClientId"
            @update:open="
              (open: boolean) =>
                open ? focusPresenceField('clientId') : blurPresenceField('clientId')
            "
          />
        </div>

        <div
          v-if="isModalFieldVisible('dueDate')"
          class="tasks-page__task-property-row"
          @focusin="focusPresenceField('dueDate')"
          @click.capture="focusPresenceField('dueDate')"
        >
          <span class="tasks-page__task-property-label">
            <span class="tasks-page__task-property-label-main">
              <UIcon name="i-lucide-calendar-days" />
              Prazo
            </span>
            <span v-if="presenceFieldLabel('dueDate')" class="tasks-page__presence-field">
              {{ presenceFieldLabel('dueDate') }}
            </span>
          </span>
          <AppDatePicker
            :model-value="taskDraftDueDateValue()"
            :end-date="taskDraftDueEndDateValue()"
            placement="left"
            @update:model-value="updateTaskDraftDueDate"
            @update:end-date="updateTaskDraftDueEndDate"
            @update:open="
              (open: boolean) =>
                open ? focusPresenceField('dueDate') : blurPresenceField('dueDate')
            "
          >
            <template #default="{ labelStart, labelEnd }">
              <button
                class="tasks-page__task-date-btn"
                type="button"
                :disabled="isPresenceFieldLocked('dueDate')"
              >
                <span v-if="labelStart" class="flex flex-col leading-tight">
                  <span>{{ labelStart }}</span>
                  <span v-if="labelEnd" class="tasks-page__task-date-btn--end">
                    {{ labelEnd }}
                  </span>
                </span>
                <span v-else class="tasks-page__task-date-btn--empty">Sem data</span>
              </button>
            </template>
          </AppDatePicker>
        </div>

        <div
          v-if="isModalFieldVisible('priority')"
          class="tasks-page__task-property-row"
          @focusin="focusPresenceField('priority')"
          @click.capture="focusPresenceField('priority')"
        >
          <span class="tasks-page__task-property-label">
            <span class="tasks-page__task-property-label-main">
              <UIcon name="i-lucide-badge-alert" />
              Prioridade
            </span>
            <span v-if="presenceFieldLabel('priority')" class="tasks-page__presence-field">
              {{ presenceFieldLabel('priority') }}
            </span>
          </span>
          <OmniLazySelectMenuInput
            :model-value="taskDraftPriorityValue()"
            class="tasks-page__task-property-control"
            :items="PRIORITY_OPTIONS"
            placeholder="Nao definido"
            :searchable="false"
            :full-content-width="true"
            item-display-mode="text"
            color="neutral"
            variant="none"
            :highlight="false"
            :badge-mode="true"
            clear
            :disabled="isPresenceFieldLocked('priority')"
            option-edit-mode="color"
            @update:model-value="updateTaskDraftPriority"
            @update:open="
              (open: boolean) =>
                open ? focusPresenceField('priority') : blurPresenceField('priority')
            "
          />
        </div>

        <div v-if="taskDraft.id" class="tasks-page__task-property-row">
          <span class="tasks-page__task-property-label">
            <UIcon name="i-lucide-timer" />
            Tracking
          </span>
          <div class="tasks-page__task-tracking-controls flex items-center gap-1.5">
            <UButton
              v-if="!isTracking(taskDraft.id)"
              icon="i-lucide-play"
              color="neutral"
              variant="ghost"
              size="xs"
              title="Iniciar cronometro"
              @click="startTracking(taskDraft.id)"
            />
            <UButton
              v-if="isRunning(taskDraft.id)"
              icon="i-lucide-pause"
              color="neutral"
              variant="ghost"
              size="xs"
              title="Pausar cronometro"
              @click="pauseTracking(taskDraft.id)"
            />
            <UButton
              v-if="isTracking(taskDraft.id) && !isRunning(taskDraft.id)"
              icon="i-lucide-play"
              color="neutral"
              variant="ghost"
              size="xs"
              title="Retomar cronometro"
              @click="startTracking(taskDraft.id)"
            />
            <UButton
              v-if="isTracking(taskDraft.id)"
              icon="i-lucide-square"
              color="neutral"
              variant="ghost"
              size="xs"
              title="Parar cronometro"
              @click="stopTracking(taskDraft.id)"
            />
          </div>
        </div>

        <div
          v-if="isModalFieldVisible('type')"
          class="tasks-page__task-property-row"
          @focusin="focusPresenceField('type')"
          @click.capture="focusPresenceField('type')"
        >
          <span class="tasks-page__task-property-label">
            <span class="tasks-page__task-property-label-main">
              <UIcon name="i-lucide-hash" />
              Tipo
            </span>
            <span v-if="presenceFieldLabel('type')" class="tasks-page__presence-field">
              {{ presenceFieldLabel('type') }}
            </span>
          </span>
          <OmniLazySelectMenuInput
            :model-value="taskDraftTypeValue()"
            class="tasks-page__task-property-control"
            :items="typeOptions"
            placeholder="Nao definido"
            :creatable="{ when: 'always', position: 'bottom' }"
            :searchable="true"
            :full-content-width="true"
            item-display-mode="text"
            color="neutral"
            variant="none"
            :highlight="false"
            :badge-mode="true"
            clear
            option-edit-mode="full"
            :disabled="isPresenceFieldLocked('type')"
            @update:model-value="updateTaskDraftType"
            @update:open="
              (open: boolean) => (open ? focusPresenceField('type') : blurPresenceField('type'))
            "
          />
        </div>

        <div
          v-if="isModalFieldVisible('createdAt') && taskDraft.createdAt"
          class="tasks-page__task-property-row"
        >
          <span class="tasks-page__task-property-label">
            <UIcon name="i-lucide-clock-3" />
            Criada em
          </span>
          <span class="tasks-page__task-property-static">
            {{ dateLabel(taskDraft.createdAt) }}
          </span>
        </div>
      </div>

      <UButton
        icon="i-lucide-plus"
        label="Adicionar campo"
        color="neutral"
        variant="ghost"
        size="sm"
        @click="projectSettingsOpen = true"
      />

      <div
        @focusin="focusPresenceField('checklist')"
        @focusout="blurPresenceField('checklist', $event)"
      >
        <span
          v-if="presenceFieldLabel('checklist')"
          class="tasks-page__presence-field tasks-page__presence-field--inline"
        >
          {{ presenceFieldLabel('checklist') }}
        </span>
        <TaskChecklistField
          :items="taskDraftChecklistValue()"
          :disabled="isPresenceFieldLocked('checklist')"
          @update:items="updateTaskDraftChecklist"
        />
      </div>

      <div
        class="tasks-page__task-video-upload"
        @focusin="focusPresenceField('videos')"
        @focusout="blurPresenceField('videos', $event)"
        @dragenter="focusPresenceField('videos')"
      >
        <div class="tasks-page__task-video-head">
          <span class="tasks-page__task-video-title">
            <UIcon name="i-lucide-images" />
            Mídia
            <span
              v-if="presenceFieldLabel('videos')"
              class="tasks-page__presence-field tasks-page__presence-field--inline"
            >
              {{ presenceFieldLabel('videos') }}
            </span>
          </span>
        </div>
        <OmniMediaGrid
          :items="taskMediaGridItems"
          :hint="`Imagens até ${formatFileSize(taskImageMaxBytes)} · vídeos até ${formatFileSize(taskVideoMaxBytes)}`"
          :error="taskVideoError"
          accept="image/*,video/*"
          upload-label="Adicionar mídia"
          :reorderable="!taskVideoSaving && taskVideoUploads.length === 0"
          @files="onTaskVideoFiles"
          @remove="removeTaskVideoDraft"
          @reorder="reorderTaskVideoDrafts"
        >
          <template #item-footer="{ item }">
            <OmniLazySelectMenuInput
              v-if="taskVideoChecklistOptions.length && taskVideoDraftById(item.id)"
              class="tasks-page__task-video-checklist"
              :model-value="taskVideoDraftById(item.id)?.checklistItemId || null"
              :items="taskVideoChecklistOptions"
              placeholder="Tarefa inteira"
              :searchable="taskVideoChecklistOptions.length > 6"
              :clear="true"
              :disabled="taskVideoSaving"
              @update:model-value="
                (value: unknown) =>
                  updateTaskVideoChecklistItem(item.id, value ? String(value) : '')
              "
            />
          </template>
        </OmniMediaGrid>
      </div>

      <div
        v-if="taskRelations.relations.value.length || taskRelations.status.value === 'loading'"
        class="tasks-page__task-relations"
      >
        <p class="tasks-page__task-relations-title">
          <UIcon name="i-lucide-link-2" />
          <span>Vinculos</span>
          <span
            v-if="taskRelations.status.value === 'loading'"
            class="tasks-page__task-relations-loading"
          >
            <UIcon name="i-lucide-loader-circle" class="animate-spin" />
          </span>
        </p>
        <ul v-if="taskRelations.relations.value.length" class="tasks-page__task-relations-list">
          <li
            v-for="relation in taskRelations.relations.value"
            :key="relation.id"
            class="tasks-page__task-relations-item"
          >
            <UIcon
              :name="
                relation.module === 'crm'
                  ? 'i-lucide-user-round'
                  : relation.module === 'erp'
                    ? 'i-lucide-package'
                    : relation.module === 'operations'
                      ? 'i-lucide-clipboard-list'
                      : 'i-lucide-link'
              "
              class="tasks-page__task-relations-icon"
            />
            <span class="tasks-page__task-relations-label">
              {{ relation.labelCache || relation.resourceId }}
            </span>
            <span class="tasks-page__task-relations-type">{{ relation.resourceType }}</span>
            <UBadge
              v-if="
                typeof relation.metadataCache.status === 'string' && relation.metadataCache.status
              "
              :color="
                relation.metadataCache.status === 'unknown'
                  ? 'neutral'
                  : relation.metadataCache.status === 'active'
                    ? 'success'
                    : 'warning'
              "
              variant="soft"
              size="xs"
            >
              {{ relation.metadataCache.status }}
            </UBadge>
            <UButton
              v-if="typeof relation.metadataCache.url === 'string' && relation.metadataCache.url"
              :to="relation.metadataCache.url"
              target="_blank"
              external
              icon="i-lucide-external-link"
              color="neutral"
              variant="ghost"
              size="xs"
              title="Abrir recurso"
            />
          </li>
        </ul>
        <p v-else-if="taskRelations.errorMessage.value" class="tasks-page__task-relations-error">
          {{ taskRelations.errorMessage.value }}
        </p>
      </div>

      <div class="tasks-page__task-comments">
        <p class="tasks-page__task-comments-title">Comentarios</p>
        <div v-if="taskComments.comments.length" class="mb-3 space-y-2">
          <article
            v-for="comment in taskComments.comments"
            :key="comment.id"
            class="rounded-[var(--radius-sm)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] px-3 py-2"
          >
            <div
              class="mb-1 flex items-center justify-between gap-2 text-[11px] text-[rgb(var(--muted))]"
            >
              <strong class="truncate text-[rgb(var(--text))]">{{ comment.authorLabel }}</strong>
              <span>{{ dateLabelLong(comment.createdAt) }}</span>
            </div>
            <div
              class="prose prose-sm max-w-none text-sm text-[rgb(var(--text))]"
              v-html="comment.bodyHtml"
            ></div>
          </article>
        </div>
        <p
          v-else-if="taskComments.status === 'ready'"
          class="mb-3 text-xs text-[rgb(var(--muted))]"
        >
          Nenhum comentario ainda.
        </p>
        <p v-if="taskComments.errorMessage" class="mb-3 text-xs text-[rgb(var(--error))]">
          {{ taskComments.errorMessage }}
        </p>
        <div
          class="tasks-page__task-comment-input"
          @focusin="focusPresenceField('comments')"
          @focusout="blurPresenceField('comments', $event)"
          @keydown.enter.capture.prevent="taskComments.submitComment()"
        >
          <UAvatar :text="currentUserName.slice(0, 1)" size="xs" />
          <div class="min-w-0 flex-1">
            <span
              v-if="presenceFieldLabel('comments')"
              class="tasks-page__presence-field mb-1 inline-flex"
            >
              {{ presenceFieldLabel('comments') }}
            </span>
            <p
              v-if="taskComments.remoteDraft"
              class="mb-2 truncate text-xs text-[rgb(var(--ui-text-muted))]"
            >
              {{ taskComments.remoteDraft }}
            </p>
            <UInput
              :model-value="taskComments.draft"
              variant="none"
              placeholder="Add a comment..."
              @update:model-value="taskComments.setDraft($event)"
            />
          </div>
          <UButton
            icon="i-lucide-send"
            color="primary"
            variant="ghost"
            size="xs"
            :disabled="!taskComments.canSubmit"
            title="Enviar comentario"
            @click="taskComments.submitComment()"
          />
        </div>
      </div>

      <div
        class="tasks-page__task-rich-editor-wrap"
        @focusin="focusPresenceField('description')"
        @focusout="blurPresenceField('description', $event)"
      >
        <span
          v-if="presenceFieldLabel('description')"
          class="tasks-page__presence-field tasks-page__presence-field--editor"
        >
          {{ presenceFieldLabel('description') }}
        </span>
        <OmniEditor
          :model-value="taskDraftContentValue()"
          class="tasks-page__task-rich-editor"
          :editable="!isPresenceFieldLocked('description')"
          :people="peopleMentionLabels"
          :clients="clientMentionLabels"
          :tasks="taskMentionLabels"
          content-type="html"
          min-height="320px"
          max-height="52vh"
          placeholder="Press '/' for commands, ':' for emojis, '@' to mention..."
          @update:model-value="updateTaskDraftContent"
        />
      </div>

      <label v-if="isModalFieldVisible('archived')" class="tasks-page__task-archived">
        <span>Task arquivada</span>
        <USwitch v-model="taskDraft.archived" />
      </label>
    </div>

    <template #footer>
      <div class="tasks-page__task-footer flex w-full items-center justify-between gap-2">
        <UButton
          icon="i-lucide-trash-2"
          label="Excluir"
          color="error"
          variant="ghost"
          :disabled="!taskDraft.id"
          @click="deleteCurrentDraftTask"
        />
        <div class="flex items-center gap-2">
          <span class="tasks-page__task-autosave-status">
            <UIcon
              :name="taskSaving ? 'i-lucide-loader-circle' : 'i-lucide-check'"
              :class="{ 'animate-spin': taskSaving }"
            />
            {{ taskSaving ? 'Salvando...' : 'Salvo automatico' }}
          </span>
          <UButton label="Fechar" color="neutral" variant="ghost" @click="closeTaskEditor" />
        </div>
      </div>
    </template>
  </OmniEntityDrawer>

  <TaskVideoUploadDialog
    :open="taskVideoItemDialogOpen"
    :file-names="taskVideoPendingFileNames"
    :items="taskVideoChecklistItems"
    @update:open="!$event && cancelTaskVideoUpload()"
    @confirm="confirmTaskVideoUpload"
  />
</template>

<style scoped>
.tasks-page__task-editor {
  width: 100%;
  min-width: 0;
}

.tasks-page__task-rich-editor {
  height: 52vh;
  min-width: 0;
}

.tasks-page__task-rich-editor :deep(.omni-editor__instance > .relative) {
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
}
</style>
