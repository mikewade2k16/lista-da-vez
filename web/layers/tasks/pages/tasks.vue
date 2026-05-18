<script setup lang="ts">
import { provide } from 'vue'
import AdminPageHeader from '../../core/components/admin/AdminPageHeader.vue'
import CoreSkeleton from '../../core/components/CoreSkeleton.vue'
import { TASKS_PAGE_CONTEXT_KEY, useTasksPageContext } from '../composables/useTasksPageContext'
import TasksFilterBar from '../components/TasksFilterBar.vue'
import TasksBoardView from '../components/TasksBoardView.vue'
import TasksTableView from '../components/TasksTableView.vue'
import TasksProjectSettings from '../components/TasksProjectSettings.vue'
import TasksTaskModal from '../components/TasksTaskModal.vue'

// @ts-ignore Nuxt macro available at runtime in this page.
definePageMeta({
  layout: 'dashboard',
  workspaceId: 'tasks',
  pageLabel: 'Tasks'
})

const context = useTasksPageContext()
provide(TASKS_PAGE_CONTEXT_KEY, context)

const { pageBootstrapping, activeProject, viewMode, taskEditorCssVars, legacyMigrationNotice, tasksErrorMessage, taskEditorOpen, taskEditorMode } = context
</script>

<template>
  <section class="tasks-page space-y-4" :class="{ 'tasks-page--side-editor-open': taskEditorOpen && taskEditorMode === 'side' }"
    :style="taskEditorCssVars">
    <AdminPageHeader eyebrow="Tasks" title="Orquestrador Tasks"
      description="Paginas notion-like com board, tabela, campos e colunas configuraveis." />

    <div v-if="pageBootstrapping" class="grid gap-4" aria-live="polite">
      <div class="rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] p-4">
        <div class="flex flex-wrap items-center gap-3">
          <CoreSkeleton variant="block" width="280px" height="40px" />
          <CoreSkeleton variant="block" width="136px" height="36px" />
          <CoreSkeleton variant="block" width="136px" height="36px" />
          <CoreSkeleton variant="block" width="136px" height="36px" />
        </div>
        <div class="mt-4 flex flex-wrap gap-2">
          <CoreSkeleton variant="block" width="220px" height="32px" />
          <CoreSkeleton variant="block" width="180px" height="32px" />
          <CoreSkeleton variant="block" width="180px" height="32px" />
          <CoreSkeleton variant="block" width="180px" height="32px" />
        </div>
      </div>

      <div class="grid gap-3 xl:grid-cols-3">
        <div v-for="column in 3" :key="column"
          class="rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] p-4">
          <CoreSkeleton variant="block" width="132px" height="18px" />
          <div class="mt-4">
            <CoreSkeleton variant="card" :count="3" />
          </div>
        </div>
      </div>
    </div>

    <template v-else>
      <UAlert v-if="legacyMigrationNotice" color="info" variant="soft" icon="i-lucide-database-zap"
        title="Tasks agora usa o servidor como fonte principal"
        description="O estado legado em localStorage foi descartado para iniciar a migracao do prototipo local para a API real." />

      <UAlert v-if="tasksErrorMessage" color="error" variant="soft" icon="i-lucide-circle-alert"
        title="Nao foi possivel sincronizar Tasks" :description="tasksErrorMessage" />

      <TasksFilterBar />

      <UAlert v-if="!activeProject" class="tasks-page__empty-project" color="warning" variant="soft"
        icon="i-lucide-folder-open" title="Sem projeto ativo"
        description="Crie ou selecione um projeto para comecar." />

      <TasksBoardView v-else-if="viewMode === 'board'" />
      <TasksTableView v-else />
    </template>

    <TasksProjectSettings />
    <TasksTaskModal />
  </section>
</template>

<style>
.tasks-page__board {
  display: grid;
  grid-auto-flow: column;
  grid-auto-columns: minmax(300px, 1fr);
  align-items: start;
}

.tasks-page__board-wrap {
  scrollbar-gutter: stable;
  transition: width 0.18s ease, max-width 0.18s ease;
}

.tasks-page__board-column-body {
  display: flex;
  flex-direction: column;
}

.tasks-page__board-card--draft {
  order: -1;
}

@media (min-width: 1024px) {
  .tasks-page--side-editor-open .tasks-page__board-wrap {
    width: max(22rem, calc(100vw - var(--tasks-editor-width, 720px) - 2rem));
    max-width: 100%;
    padding-bottom: 0.5rem;
  }
}

.tasks-page__board-column {
  min-height: 350px;
  border-top-width: 3px;
  transition: box-shadow 0.16s ease, border-color 0.16s ease, transform 0.16s ease;
}

.tasks-page__board-column--drop {
  box-shadow: inset 0 0 0 2px rgb(var(--primary) / 0.45);
}

.tasks-page__board-column--indigo {
  border-top-color: rgb(129 140 248);
}

.tasks-page__board-column--slate {
  border-top-color: rgb(148 163 184);
}

.tasks-page__board-column--blue {
  border-top-color: rgb(59 130 246);
}

.tasks-page__board-column--amber {
  border-top-color: rgb(245 158 11);
}

.tasks-page__board-column--emerald {
  border-top-color: rgb(16 185 129);
}

.tasks-page__board-column--violet {
  border-top-color: rgb(139 92 246);
}

.tasks-page__board-column--rose {
  border-top-color: rgb(244 63 94);
}

.tasks-page__board-column-color {
  width: 0.65rem;
  height: 0.65rem;
  border-radius: 999px;
  background: currentColor;
  color: rgb(var(--primary));
  flex: 0 0 auto;
}

.tasks-page__board-column--slate .tasks-page__board-column-color {
  color: rgb(148 163 184);
}

.tasks-page__board-column--blue .tasks-page__board-column-color {
  color: rgb(59 130 246);
}

.tasks-page__board-column--amber .tasks-page__board-column-color {
  color: rgb(245 158 11);
}

.tasks-page__board-column--emerald .tasks-page__board-column-color {
  color: rgb(16 185 129);
}

.tasks-page__board-column--violet .tasks-page__board-column-color {
  color: rgb(139 92 246);
}

.tasks-page__board-column--rose .tasks-page__board-column-color {
  color: rgb(244 63 94);
}

.tasks-page__board-column-body {
  min-height: 390px;
}

.tasks-page__board-column-handle {
  cursor: grab;
}

.tasks-page__board-column-handle:active {
  cursor: grabbing;
}

.tasks-page__board-card {
  box-shadow: var(--shadow-xs);
  position: relative;
}

.tasks-page__board-card-handle {
  width: 1.25rem;
  min-width: 1.25rem;
  height: 1.8rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  color: rgb(var(--muted));
  cursor: grab;
  opacity: 0.55;
  transition: opacity 0.16s ease, color 0.16s ease, background 0.16s ease;
}

.tasks-page__board-card-handle:active {
  cursor: grabbing;
}

.tasks-page__board-card:hover .tasks-page__board-card-handle,
.tasks-page__board-card:focus-within .tasks-page__board-card-handle {
  opacity: 1;
}

.tasks-page__board-card-handle:hover {
  color: rgb(var(--text));
  background: rgb(var(--surface-2));
}

.tasks-page__board-card-handle svg,
.tasks-page__board-card-handle .iconify {
  width: 0.9rem;
  height: 0.9rem;
}

.tasks-page__board-card--paused {
  border-color: rgb(234 179 8 / 0.7) !important;
}

.tasks-page__board-card--running {
  border-color: rgb(34 197 94 / 0.7) !important;
}

.tasks-page__board-card-pause-dot {
  position: absolute;
  bottom: 0.45rem;
  right: 0.45rem;
  width: 0.5rem;
  height: 0.5rem;
  border-radius: 50%;
  background: rgb(234 179 8);
}

.tasks-page__board-card--drop-before::before {
  content: "";
  position: absolute;
  inset-inline: 0.5rem;
  top: -0.45rem;
  height: 0.2rem;
  border-radius: 999px;
  background: rgb(var(--primary));
  box-shadow: 0 0 0 3px rgb(var(--primary) / 0.16);
}

.tasks-page__board-card--draft {
  box-shadow: 0 0 0 1px rgb(var(--primary) / 0.24);
}

.tasks-page__board-card-timer {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  font-size: 0.75rem;
  font-variant-numeric: tabular-nums;
  color: rgb(var(--color-primary-500));
}

.tasks-page__board-card-actions {
  opacity: 0;
  transform: translateY(-0.15rem);
  transition: opacity 0.16s ease, transform 0.16s ease;
}

.tasks-page__board-card-actions svg {
  width: 0.7rem;
  height: 0.7rem;
}

.tasks-page__board-card:hover .tasks-page__board-card-actions,
.tasks-page__board-card:focus-within .tasks-page__board-card-actions {
  opacity: 1;
  transform: translateY(0);
}

.tasks-page__board-card-title {
  color: rgb(var(--text));
}

.tasks-page__board-card-title-input input {
  min-height: 1.8rem;
  padding-inline: 0;
  color: rgb(var(--text));
  font-size: 0.875rem;
  font-weight: 700;
}

.tasks-page__board-card-description {
  white-space: pre-wrap;
}

.tasks-page__board-card-presence {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  max-width: 100%;
  margin: -0.1rem 0 0.35rem 1.45rem;
  color: rgb(var(--primary));
  font-size: 0.72rem;
  font-weight: 700;
  line-height: 1.15;
}

.tasks-page__board-card-presence > span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tasks-page__board-card-presence .tasks-page__presence-avatar {
  width: 1.2rem;
  height: 1.2rem;
}

.tasks-page__board-card-inline {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  gap: 0.35rem;
  width: fit-content;
}

.tasks-page__board-card-inline .omni-select-menu-input,
.tasks-page__task-property-control {
  width: auto;
  max-width: 100%;
}

.tasks-page__board-card-inline .omni-select-menu-input__control,
.tasks-page__board-card-inline .omni-select-menu-input__base,
.tasks-page__task-property-control .omni-select-menu-input__control,
.tasks-page__task-property-control .omni-select-menu-input__base {
  width: auto;
}

.tasks-page__board-card-inline button,
.tasks-page__board-card-date input {
  min-height: 1.75rem;
  font-size: 0.75rem;
}

.tasks-page__board-add-card {
  width: 100%;
  min-height: 2.25rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.4rem;
  border: 1px dashed rgb(var(--border));
  border-radius: var(--radius-sm);
  color: rgb(var(--muted));
  background: rgb(var(--surface) / 0.56);
  font-size: 0.8rem;
  font-weight: 700;
  transition: border-color 0.16s ease, color 0.16s ease, background 0.16s ease;
}

.tasks-page__board-add-card:hover {
  border-color: rgb(var(--primary) / 0.55);
  color: rgb(var(--primary));
  background: rgb(var(--primary) / 0.08);
}

.tasks-page__column-menu {
  border-radius: var(--radius-sm);
  border: 1px solid rgb(var(--border));
  background: rgb(var(--surface));
  box-shadow: var(--shadow-md);
}

.tasks-page__column-editor-popover,
.tasks-page__task-mode-menu {
  border-radius: var(--radius-sm);
  border: 1px solid rgb(var(--border));
  background: rgb(var(--surface));
  box-shadow: var(--shadow-md);
}

.tasks-page__column-menu-item {
  width: 100%;
  min-height: 2rem;
  display: flex;
  align-items: center;
  gap: 0.5rem;
  border-radius: var(--radius-sm);
  padding: 0.35rem 0.5rem;
  color: rgb(var(--text));
  font-size: 0.85rem;
  text-align: left;
}

.tasks-page__column-menu-item:hover:not(:disabled) {
  background: rgb(var(--surface-2));
}

.tasks-page__column-menu-item:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.tasks-page__column-menu-item--danger {
  color: rgb(var(--error));
}

.tasks-page__task-mode-item {
  width: 100%;
  min-height: 2rem;
  display: flex;
  align-items: center;
  gap: 0.5rem;
  border-radius: var(--radius-sm);
  padding: 0.35rem 0.5rem;
  color: rgb(var(--text));
  font-size: 0.85rem;
  text-align: left;
}

.tasks-page__task-mode-item:hover {
  background: rgb(var(--surface-2));
}

.tasks-page__table-add-row {
  width: 100%;
  min-height: 2.5rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.4rem;
  border: 1px dashed rgb(var(--border));
  border-top: 0;
  border-radius: 0 0 var(--radius-md) var(--radius-md);
  color: rgb(var(--muted));
  background: rgb(var(--surface));
  font-size: 0.85rem;
  font-weight: 700;
}

.tasks-page__table-add-row:hover {
  color: rgb(var(--primary));
  background: rgb(var(--primary) / 0.08);
}

.tasks-page__settings-switch-row,
.tasks-page__task-archived {
  background: rgb(var(--surface));
}

.tasks-page__task-overlay {
  width: min(var(--tasks-editor-width, 720px), calc(100vw - 1rem)) !important;
  max-width: min(var(--tasks-editor-width, 720px), calc(100vw - 1rem)) !important;
}

.tasks-page__task-overlay--center {
  right: auto !important;
  left: 50% !important;
  top: 50% !important;
  bottom: auto !important;
  width: min(880px, calc(100vw - 2rem)) !important;
  max-width: min(880px, calc(100vw - 2rem)) !important;
  height: min(840px, calc(100vh - 2rem)) !important;
  transform: translate(-50%, -50%) !important;
  border-radius: var(--radius-md) !important;
}

.tasks-page__task-overlay--fullscreen {
  inset: 0 !important;
  width: 100vw !important;
  max-width: 100vw !important;
  height: 100vh !important;
  border-radius: 0 !important;
}

.tasks-page__task-modal-header {
  min-height: 2.25rem;
}

.tasks-page__presence-stack {
  display: inline-flex;
  align-items: center;
  min-width: 0;
  padding-right: 0.25rem;
}

.tasks-page__presence-avatar {
  border: 2px solid rgb(var(--surface));
  box-shadow: var(--shadow-sm);
}

.tasks-page__presence-avatar + .tasks-page__presence-avatar {
  margin-left: -0.45rem;
}

.tasks-page__presence-more,
.tasks-page__presence-idle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 1.25rem;
  border: 1px solid rgb(var(--border));
  border-radius: 999px;
  padding: 0 0.4rem;
  background: rgb(var(--surface-2));
  color: rgb(var(--muted));
  font-size: 0.72rem;
  font-weight: 700;
}

.tasks-page__presence-more {
  margin-left: -0.35rem;
}

.tasks-page__presence-field {
  flex-basis: 100%;
  margin-left: 1.45rem;
  color: rgb(var(--primary));
  font-size: 0.72rem;
  font-weight: 700;
  line-height: 1.15;
}

.tasks-page__presence-field--title,
.tasks-page__presence-field--editor {
  display: inline-flex;
  margin-left: 0;
  margin-bottom: 0.35rem;
}

.tasks-page__presence-field--inline {
  flex-basis: auto;
  margin-left: 0.35rem;
}

.tasks-page__task-editor {
  position: relative;
  max-width: 860px;
  margin-inline: auto;
  padding: 0rem 0 2rem;
}

.tasks-page__task-resize-handle {
  position: absolute;
  left: -0.75rem;
  top: -3.5rem;
  bottom: -3rem;
  width: 0.75rem;
  cursor: col-resize;
}

.tasks-page__task-resize-handle::after {
  content: "";
  position: absolute;
  left: 0.3rem;
  top: 4rem;
  bottom: 2rem;
  width: 1px;
  background: rgb(var(--border));
  opacity: 0;
  transition: opacity 0.16s ease;
}

.tasks-page__task-resize-handle:hover::after {
  opacity: 1;
}

.tasks-page__task-title-row {
  margin-bottom: 1rem;
}

.tasks-page__task-title-input input {
  min-height: 3.25rem;
  padding-inline: 0;
  font-size: 2.25rem;
  font-weight: 800;
  letter-spacing: 0;
  color: rgb(var(--text));
}

.tasks-page__task-properties {
  display: grid;
  gap: 0.25rem;
  margin-bottom: 1rem;
}

.tasks-page__task-property-row {
  display: grid;
  grid-template-columns: minmax(9.5rem, 11rem) minmax(0, 1fr);
  align-items: center;
  gap: 1rem;
  min-height: 2.2rem;
}

.tasks-page__task-property-label {
  display: inline-flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.45rem;
  color: rgb(var(--muted));
  font-size: 0.88rem;
}

.tasks-page__task-property-label-main {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
}

.tasks-page__task-property-label svg,
.tasks-page__task-property-label .iconify {
  width: 1rem;
  height: 1rem;
}

.tasks-page__task-property-static {
  color: rgb(var(--muted));
  font-size: 0.88rem;
}

.tasks-page__task-date-input {
  width: fit-content;
  min-width: 10rem;
}

.tasks-page__task-date-input input {
  padding-inline: 0;
  color: rgb(var(--text));
}

.tasks-page__task-date-btn {
  min-width: 8rem;
  padding: 0.2rem 0.35rem;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: rgb(var(--text));
  font-size: 0.875rem;
  text-align: left;
  cursor: pointer;
  transition: background 0.14s ease;
}

.tasks-page__task-date-btn:hover {
  background: var(--admin-header-hover-bg);
}

.tasks-page__task-date-btn--empty {
  color: rgb(var(--muted));
}

.tasks-page__task-date-btn--end {
  color: rgb(var(--muted));
  font-size: 0.8rem;
}

.tasks-page__task-tracking-timer {
  font-size: 0.875rem;
  font-variant-numeric: tabular-nums;
  color: rgb(var(--color-primary-500));
  min-width: 3.5rem;
}

.tasks-page__task-description-bridge {
  margin: 0.5rem 0 1.25rem;
  border-top: 1px solid rgb(var(--border));
  border-bottom: 1px solid rgb(var(--border));
}

.tasks-page__task-description-bridge textarea {
  padding-inline: 0;
  resize: vertical;
}

.tasks-page__task-video-upload {
  margin: 1rem 0 1.25rem;
  border-top: 1px solid rgb(var(--border));
  border-bottom: 1px solid rgb(var(--border));
  padding: 0.85rem 0;
}

.tasks-page__task-video-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  margin-bottom: 0.65rem;
}

.tasks-page__task-video-title,
.tasks-page__task-video-action {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  color: rgb(var(--muted));
  font-size: 0.88rem;
}

.tasks-page__task-video-action {
  min-height: 2rem;
  border-radius: var(--radius-sm);
  padding: 0 0.5rem;
  color: rgb(var(--primary));
  cursor: pointer;
}

.tasks-page__task-video-action:hover {
  background: rgb(var(--primary) / 0.08);
}

.tasks-page__task-video-drop {
  min-height: 7rem;
  display: grid;
  place-items: center;
  gap: 0.25rem;
  border: 1px dashed rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.48);
  color: rgb(var(--muted));
  cursor: pointer;
}

.tasks-page__task-video-drop svg,
.tasks-page__task-video-drop .iconify {
  width: 1.4rem;
  height: 1.4rem;
}

.tasks-page__task-video-drop span {
  color: rgb(var(--text));
  font-size: 0.9rem;
  font-weight: 700;
}

.tasks-page__task-video-drop small {
  font-size: 0.75rem;
}

.tasks-page__task-video-list {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
  gap: 0.75rem;
  margin-top: 0.75rem;
}

.tasks-page__task-video-item {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 0.5rem;
  min-height: 0;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  padding: 0.45rem;
  background: rgb(var(--surface));
}

.tasks-page__task-video-item video {
  width: 100%;
  height: 14rem;
  border-radius: 6px;
  background: rgb(var(--surface-2));
  object-fit: contain;
}

.tasks-page__task-video-item > button {
  position: absolute;
  top: 0.65rem;
  right: 0.65rem;
  background: rgb(var(--surface) / 0.84);
  box-shadow: var(--shadow-sm);
}

.tasks-page__task-video-meta {
  padding-inline: 0.15rem;
}

.tasks-page__task-video-item p {
  color: rgb(var(--text));
  font-size: 0.86rem;
  font-weight: 700;
}

.tasks-page__task-video-item span {
  color: rgb(var(--muted));
  font-size: 0.74rem;
}

.tasks-page__task-relations {
  margin-top: 1.4rem;
  border-bottom: 1px solid rgb(var(--border));
  padding-bottom: 0.9rem;
}

.tasks-page__task-relations-title {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  margin-bottom: 0.45rem;
  color: rgb(var(--muted));
  font-size: 0.82rem;
  font-weight: 700;
}

.tasks-page__task-relations-loading {
  margin-left: auto;
}

.tasks-page__task-relations-list {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  list-style: none;
  padding: 0;
  margin: 0;
}

.tasks-page__task-relations-item {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  padding: 0.4rem 0.6rem;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2));
}

.tasks-page__task-relations-icon {
  flex-shrink: 0;
  color: rgb(var(--muted));
  width: 0.95rem;
  height: 0.95rem;
}

.tasks-page__task-relations-label {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: rgb(var(--text));
  font-size: 0.85rem;
}

.tasks-page__task-relations-type {
  color: rgb(var(--muted));
  font-size: 0.74rem;
  text-transform: uppercase;
  letter-spacing: 0.02em;
}

.tasks-page__task-relations-error {
  color: rgb(var(--error, 220 38 38));
  font-size: 0.78rem;
}

.tasks-page__task-comments {
  margin-top: 1.4rem;
  border-bottom: 1px solid rgb(var(--border));
  padding-bottom: 0.9rem;
}

.tasks-page__task-comments-title {
  margin-bottom: 0.45rem;
  color: rgb(var(--muted));
  font-size: 0.82rem;
  font-weight: 700;
}

.tasks-page__task-comment-input {
  display: flex;
  align-items: center;
  gap: 0.65rem;
}

.tasks-page__task-comment-input input {
  padding-inline: 0;
}

.tasks-page__task-rich-editor {
  margin-top: 1.25rem;
}

.tasks-page__task-rich-editor-wrap {
  margin-top: 1.25rem;
}

.tasks-page__task-rich-editor-wrap .tasks-page__task-rich-editor {
  margin-top: 0;
}

.tasks-page__task-archived {
  margin-top: 1rem;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  padding: 0.65rem 0.75rem;
}

.tasks-page__task-autosave-status {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  color: rgb(var(--muted));
  font-size: 0.78rem;
  white-space: nowrap;
}

.tasks-page__task-autosave-status svg,
.tasks-page__task-autosave-status .iconify {
  width: 0.95rem;
  height: 0.95rem;
}

@media (max-width: 720px) {
  .tasks-page__task-property-row {
    grid-template-columns: 1fr;
    gap: 0.25rem;
  }

  .tasks-page__task-title-input input {
    font-size: 1.75rem;
  }

  .tasks-page__task-overlay,
  .tasks-page__task-overlay--center {
    width: 100vw;
    max-width: 100vw;
    height: 100vh;
    border-radius: 0;
  }

  .tasks-page__task-video-item {
    min-height: 0;
  }

  .tasks-page__task-video-item video {
    height: 12rem;
  }
}

.tasks-toolbar {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  flex-wrap: wrap;
  padding: 0.4rem 0.55rem;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-md);
  background: rgb(var(--surface-2));
}

.tasks-toolbar__project {
  min-width: 200px;
  max-width: 280px;
  flex: 0 1 240px;
}

.tasks-toolbar__divider {
  width: 1px;
  height: 22px;
  background: rgb(var(--border));
  margin: 0 0.15rem;
  flex: 0 0 auto;
}

.tasks-toolbar__filters {
  display: inline-flex;
  align-items: center;
  gap: 0.2rem;
  flex-wrap: nowrap;
}

.tasks-toolbar__filter-btn {
  position: relative;
}

.tasks-toolbar__filter-btn[data-active="true"]::after {
  content: "";
  position: absolute;
  top: 3px;
  right: 3px;
  width: 6px;
  height: 6px;
  border-radius: 999px;
  background: rgb(var(--primary));
  box-shadow: 0 0 0 1.5px rgb(var(--surface-2));
  transform: scale(1);
  transition: transform 120ms ease-out;
}

.tasks-toolbar__filter-popover {
  min-width: 240px;
  padding: 0.45rem;
}

.tasks-toolbar__search-group {
  display: inline-flex;
  align-items: center;
}

.tasks-toolbar__search {
  overflow: hidden;
  max-width: 0;
  opacity: 0;
  margin-right: 0;
  transition: max-width 200ms ease-out, opacity 160ms ease-out, margin-right 200ms ease-out;
}

.tasks-toolbar__search.is-open {
  max-width: 260px;
  opacity: 1;
  margin-right: 0.25rem;
}

.tasks-toolbar__search input {
  height: 1.85rem;
  font-size: 0.82rem;
}

.tasks-toolbar__chips {
  display: inline-flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.3rem;
  padding: 0 0.15rem;
}

.tasks-toolbar__chip {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  max-width: 220px;
  height: 1.7rem;
  padding: 0 0.15rem 0 0.55rem;
  border-radius: 999px;
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
  font-size: 0.75rem;
  font-weight: 500;
  line-height: 1;
  border: 1px solid rgb(var(--primary) / 0.25);
}

.tasks-toolbar__chip-key {
  color: rgb(var(--primary) / 0.75);
  font-weight: 600;
  font-size: 0.68rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.tasks-toolbar__chip-value {
  color: rgb(var(--text));
  max-width: 12rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 600;
}

.tasks-toolbar__chip-x {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.15rem;
  height: 1.15rem;
  border-radius: 999px;
  color: rgb(var(--primary));
  background: transparent;
  transition: background 120ms ease, color 120ms ease;
}

.tasks-toolbar__chip-x:hover {
  background: rgb(var(--primary) / 0.18);
  color: rgb(var(--text));
}

.tasks-toolbar__chip-x svg,
.tasks-toolbar__chip-x .iconify {
  width: 0.85rem;
  height: 0.85rem;
}

.tasks-toolbar__stats {
  display: inline-flex;
  align-items: center;
  gap: 0.2rem;
  padding: 0 0.15rem;
}

.tasks-toolbar__stat {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 1.6rem;
  height: 1.5rem;
  padding: 0 0.4rem;
  border-radius: var(--radius-sm);
  font-size: 0.72rem;
  font-weight: 700;
  line-height: 1;
  letter-spacing: 0.02em;
}

.tasks-toolbar__stat--neutral {
  background: rgb(var(--surface));
  color: rgb(var(--muted));
  border: 1px solid rgb(var(--border));
}

.tasks-toolbar__stat--primary {
  background: rgb(var(--primary) / 0.14);
  color: rgb(var(--primary));
}

.tasks-toolbar__stat--warning {
  background: rgb(var(--warning) / 0.18);
  color: rgb(var(--warning));
}

.tasks-toolbar__spacer {
  flex: 1 1 auto;
  min-width: 0.25rem;
}

.tasks-toolbar__view-seg {
  display: inline-flex;
  align-items: center;
  padding: 2px;
  gap: 2px;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface));
}

.tasks-toolbar__view-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.7rem;
  height: 1.7rem;
  border-radius: calc(var(--radius-sm) - 2px);
  color: rgb(var(--muted));
  background: transparent;
  transition: color 150ms ease, background 150ms ease;
}

.tasks-toolbar__view-btn svg,
.tasks-toolbar__view-btn .iconify {
  width: 1rem;
  height: 1rem;
}

.tasks-toolbar__view-btn:hover {
  color: rgb(var(--text));
  background: rgb(var(--surface-2));
}

.tasks-toolbar__view-btn.is-active {
  color: rgb(var(--primary));
  background: rgb(var(--primary) / 0.14);
}

.tasks-toolbar__menu {
  min-width: 220px;
  padding: 0.25rem;
  border-radius: var(--radius-sm);
  border: 1px solid rgb(var(--border));
  background: rgb(var(--surface));
  box-shadow: var(--shadow-md);
}

.tasks-toolbar__menu-item {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.4rem 0.5rem;
  border-radius: var(--radius-sm);
  color: rgb(var(--text));
  font-size: 0.85rem;
  text-align: left;
  background: transparent;
}

.tasks-toolbar__menu-item:hover:not(:disabled) {
  background: rgb(var(--surface-2));
}

.tasks-toolbar__menu-item:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.tasks-toolbar__menu-info {
  margin-top: 0.25rem;
  padding: 0.45rem 0.55rem;
  border-top: 1px solid rgb(var(--border));
  color: rgb(var(--muted));
  font-size: 0.75rem;
}

.tasks-toolbar__menu-info strong {
  color: rgb(var(--text));
  font-weight: 600;
}

.tasks-toolbar__menu-info-sep {
  margin: 0 0.35rem;
  opacity: 0.5;
}

@media (max-width: 879px) {
  .tasks-toolbar__stats {
    display: none;
  }

  .tasks-toolbar__search.is-open {
    max-width: 160px;
  }
}

.tasks-page__board-card-inline .omni-select-menu-input__trailing {
  display: none;
}

.tasks-page__board-card-inline .omni-select-menu-input__base {
  min-height: 0;
  padding: 0;
  background: transparent;
  border: none;
  box-shadow: none;
}

.tasks-page__board-card-people .omni-select-menu-input__selected-badge {
  background: transparent !important;
  padding: 0.1rem 0.3rem;
  color: rgb(var(--text));
  gap: 0.4rem;
}

.tasks-page__board-card-people .omni-select-menu-input__selected-badge-label {
  color: rgb(var(--text));
  font-weight: 500;
}

.tasks-page__board-card-people .omni-select-menu-input__placeholder-badge {
  background: transparent;
  color: rgb(var(--muted));
}

.tasks-page__board-card-inline .omni-select-menu-input__selected-badge {
  position: relative;
}

.tasks-page__board-card-inline .omni-select-menu-input__selected-badge-clear {
  position: absolute;
  top: -6px;
  right: -6px;
  width: 14px;
  height: 14px;
  padding: 0;
  border-radius: 999px;
  background: rgb(var(--surface));
  border: 1px solid rgb(var(--border));
  color: rgb(var(--text));
  opacity: 0;
  transition: opacity 120ms ease;
  z-index: 2;
}

.tasks-page__board-card-inline .omni-select-menu-input__selected-badge:hover .omni-select-menu-input__selected-badge-clear,
.tasks-page__board-card-inline .omni-select-menu-input__selected-badge-clear:focus-visible {
  opacity: 1;
}

.tasks-page__board-card-duedate {
  width: fit-content;
  border: none;
  background: transparent;
  padding: 0;
  cursor: pointer;
  font-size: 0.875rem;
}

.tasks-page__draft-add-list {
  width: 100%;
  gap: 0.05rem;
}

.tasks-page__draft-add-row {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.18rem 0.25rem;
  margin: 0;
  background: transparent;
  border: 0;
  color: rgb(var(--muted));
  font-size: 0.78rem;
  font-weight: 400;
  text-align: left;
  cursor: pointer;
  border-radius: var(--radius-xs, 4px);
  width: auto;
  align-self: flex-start;
  transition: color 120ms ease, background 120ms ease;
}

.tasks-page__draft-add-row:hover {
  color: rgb(var(--text));
  background: rgb(var(--surface-2));
}

.tasks-page__draft-add-row-icon {
  width: 0.9rem;
  height: 0.9rem;
  flex-shrink: 0;
  opacity: 0.85;
}
</style>
