<script setup lang="ts">
import { computed } from 'vue'
import { CheckCircle2, Circle, ListChecks, Plus, X } from 'lucide-vue-next'
import {
  dashboardModuleProgress,
  dashboardTaskProgress,
  dashboardTaskShare,
  formatDashboardPercent,
  isDashboardTaskDone,
  normalizeDashboardTaskStatus,
  type DashboardModuleRow,
  type DashboardTaskRow,
} from '~/stores/roadmap'
import { ROADMAP_MODULE_STATUS_LABEL, ROADMAP_PRIORITY_LABEL } from './roadmap-data'

const props = defineProps<{
  entry: DashboardModuleRow | null
  open: boolean
  tasksLoading?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:open', value: boolean): void
  (e: 'open-task', task: DashboardTaskRow): void
  (e: 'create-task', moduleId: string): void
}>()

const drawerOpen = computed({
  get: () => props.open,
  set: (value: boolean) => emit('update:open', value),
})

const progress = computed(() => (props.entry ? dashboardModuleProgress(props.entry) : 0))
const progressStyle = computed(() => ({
  '--roadmap-module-progress': `${progress.value}%`,
  '--roadmap-module-progress-deg': `${progress.value * 3.6}deg`,
}))

const doneTasks = computed(
  () => props.entry?.tasks.filter((task) => isDashboardTaskDone(task)).length || 0,
)
const taskShare = computed(() => dashboardTaskShare(props.entry?.tasks.length || 0))

function taskProgress(task: DashboardTaskRow) {
  return dashboardTaskProgress(task)
}

function taskStageLabel(task: DashboardTaskRow) {
  const status = normalizeDashboardTaskStatus(task.status)
  if (isDashboardTaskDone(task)) return 'Feita'
  if (['running', 'in_progress', 'doing', 'em_andamento', 'execucao'].includes(status)) {
    return 'Execucao'
  }
  if (['idea', 'ideia'].includes(status)) return 'Ideia'
  return task.status || 'Planejamento'
}

function close() {
  drawerOpen.value = false
}
</script>

<template>
  <USlideover
    v-model:open="drawerOpen"
    :overlay="true"
    :modal="true"
    :dismissible="true"
    :ui="{ content: 'roadmap-module-drawer' }"
  >
    <template #header>
      <div class="roadmap-module-drawer__header">
        <div class="roadmap-module-drawer__identity">
          <span class="roadmap-module-drawer__icon" aria-hidden="true">
            <ListChecks :size="18" :stroke-width="2.2" />
          </span>
          <div class="roadmap-module-drawer__title-block">
            <strong class="roadmap-module-drawer__title">
              {{ entry?.module.label || 'Modulo' }}
            </strong>
            <span v-if="entry?.module.route" class="roadmap-module-drawer__route">
              {{ entry.module.route }}
            </span>
          </div>
        </div>

        <div class="roadmap-module-drawer__header-actions">
          <button
            v-if="entry"
            type="button"
            class="roadmap-module-drawer__primary-btn"
            @click="emit('create-task', entry.module.id)"
          >
            <Plus :size="15" :stroke-width="2.4" aria-hidden="true" />
            Nova task
          </button>
          <button
            type="button"
            class="roadmap-module-drawer__icon-btn"
            title="Fechar"
            @click="close"
          >
            <X :size="16" :stroke-width="2.4" aria-hidden="true" />
          </button>
        </div>
      </div>
    </template>

    <template #body>
      <div v-if="entry" class="roadmap-module-drawer__body">
        <section class="roadmap-module-drawer__summary">
          <div class="roadmap-module-drawer__progress-ring" :style="progressStyle">
            <span>{{ progress }}%</span>
          </div>
          <div class="roadmap-module-drawer__summary-main">
            <div class="roadmap-module-drawer__badges">
              <span class="roadmap-module-drawer__badge">
                {{ ROADMAP_PRIORITY_LABEL[entry.module.priority] }}
              </span>
              <span class="roadmap-module-drawer__badge">
                {{ ROADMAP_MODULE_STATUS_LABEL[entry.module.status] }}
              </span>
              <span class="roadmap-module-drawer__badge roadmap-module-drawer__badge--done">
                {{ doneTasks }}/{{ entry.tasks.length }} feitas
              </span>
            </div>
            <p class="roadmap-module-drawer__description">{{ entry.module.description }}</p>
            <div class="roadmap-module-drawer__bar" aria-hidden="true">
              <span :style="{ width: `${progress}%` }"></span>
            </div>
          </div>
        </section>

        <section class="roadmap-module-drawer__tasks-section">
          <header class="roadmap-module-drawer__section-head">
            <h3>Tasks do modulo</h3>
            <span v-if="entry.tasks.length">
              Cada task vale {{ formatDashboardPercent(taskShare) }}%
            </span>
          </header>

          <p v-if="tasksLoading && !entry.tasks.length" class="roadmap-module-drawer__empty">
            Carregando tasks...
          </p>

          <ul v-else-if="entry.tasks.length" class="roadmap-module-drawer__task-list">
            <li v-for="task in entry.tasks" :key="task.id">
              <button
                type="button"
                class="roadmap-module-drawer__task"
                :class="{ 'roadmap-module-drawer__task--done': isDashboardTaskDone(task) }"
                @click="emit('open-task', task)"
              >
                <span class="roadmap-module-drawer__task-check" aria-hidden="true">
                  <CheckCircle2 v-if="isDashboardTaskDone(task)" :size="17" :stroke-width="2.4" />
                  <Circle v-else :size="17" :stroke-width="2.2" />
                </span>
                <span class="roadmap-module-drawer__task-main">
                  <strong>{{ task.title }}</strong>
                  <span>{{ taskStageLabel(task) }}</span>
                </span>
                <span class="roadmap-module-drawer__task-metric">
                  {{ taskProgress(task) }}%
                  <small>task</small>
                </span>
                <span class="roadmap-module-drawer__task-metric">
                  {{ formatDashboardPercent(taskShare) }}%
                  <small>peso</small>
                </span>
              </button>
            </li>
          </ul>

          <div v-else class="roadmap-module-drawer__empty">
            <p>Nenhuma task vinculada a este modulo ainda.</p>
            <button
              type="button"
              class="roadmap-module-drawer__primary-btn"
              @click="emit('create-task', entry.module.id)"
            >
              <Plus :size="15" :stroke-width="2.4" aria-hidden="true" />
              Criar primeira task
            </button>
          </div>
        </section>
      </div>
    </template>
  </USlideover>
</template>

<style>
.roadmap-module-drawer {
  width: min(880px, calc(100vw - 1rem)) !important;
  max-width: min(880px, calc(100vw - 1rem)) !important;
}

.roadmap-module-drawer__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  width: 100%;
}

.roadmap-module-drawer__identity {
  display: inline-flex;
  align-items: center;
  gap: 0.75rem;
  min-width: 0;
}

.roadmap-module-drawer__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2.15rem;
  height: 2.15rem;
  border: 1px solid rgb(var(--primary) / 0.28);
  border-radius: 8px;
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
  flex: 0 0 auto;
}

.roadmap-module-drawer__title-block {
  display: grid;
  min-width: 0;
}

.roadmap-module-drawer__title {
  overflow: hidden;
  color: rgb(var(--text));
  font-size: 1rem;
  font-weight: 800;
  line-height: 1.2;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.roadmap-module-drawer__route {
  overflow: hidden;
  color: rgb(var(--muted));
  font-family: ui-monospace, SFMono-Regular, monospace;
  font-size: 0.72rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.roadmap-module-drawer__header-actions {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  flex: 0 0 auto;
}

.roadmap-module-drawer__primary-btn,
.roadmap-module-drawer__icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--admin-header-border);
  border-radius: 8px;
  background: transparent;
  color: var(--text-main);
  cursor: pointer;
  transition:
    border-color 0.16s ease,
    background 0.16s ease,
    color 0.16s ease;
}

.roadmap-module-drawer__primary-btn {
  min-height: 2rem;
  gap: 0.4rem;
  padding: 0 0.75rem;
  border-color: rgb(var(--primary) / 0.35);
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
  font-size: 0.8rem;
  font-weight: 800;
}

.roadmap-module-drawer__icon-btn {
  width: 2rem;
  height: 2rem;
  color: rgb(var(--muted));
}

.roadmap-module-drawer__primary-btn:hover,
.roadmap-module-drawer__icon-btn:hover {
  border-color: rgb(var(--ring) / 0.4);
  background: var(--admin-header-hover-bg);
  color: var(--text-main);
}

.roadmap-module-drawer__body {
  display: grid;
  gap: 1rem;
}

.roadmap-module-drawer__summary {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 1rem;
  align-items: center;
  padding: 0.95rem;
  border: 1px solid var(--admin-header-border);
  border-radius: 8px;
  background: var(--admin-header-panel-bg);
}

.roadmap-module-drawer__progress-ring {
  display: grid;
  place-items: center;
  width: 5.2rem;
  height: 5.2rem;
  border-radius: 999px;
  background:
    radial-gradient(circle at center, var(--admin-header-panel-bg) 57%, transparent 59%),
    conic-gradient(
      rgb(var(--success)) var(--roadmap-module-progress-deg, 0deg),
      rgb(var(--muted) / 0.24) 0deg
    );
  color: rgb(var(--success));
  font-size: 1.1rem;
  font-weight: 900;
}

.roadmap-module-drawer__summary-main {
  display: grid;
  gap: 0.6rem;
  min-width: 0;
}

.roadmap-module-drawer__badges {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}

.roadmap-module-drawer__badge {
  padding: 0.16rem 0.55rem;
  border: 1px solid var(--admin-header-border);
  border-radius: 999px;
  background: rgb(var(--muted) / 0.22);
  color: var(--text-muted);
  font-size: 0.72rem;
  font-weight: 800;
}

.roadmap-module-drawer__badge--done {
  border-color: rgb(var(--success) / 0.3);
  background: rgb(var(--success) / 0.12);
  color: rgb(var(--success));
}

.roadmap-module-drawer__description {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.88rem;
  line-height: 1.45;
}

.roadmap-module-drawer__bar {
  height: 0.55rem;
  overflow: hidden;
  border-radius: 999px;
  background: rgb(var(--muted) / 0.2);
}

.roadmap-module-drawer__bar span {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, rgb(var(--primary)), rgb(var(--success)));
  transition: width 0.18s ease;
}

.roadmap-module-drawer__tasks-section {
  display: grid;
  gap: 0.7rem;
}

.roadmap-module-drawer__section-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 1rem;
}

.roadmap-module-drawer__section-head h3 {
  margin: 0;
  color: var(--text-main);
  font-size: 0.95rem;
  font-weight: 850;
}

.roadmap-module-drawer__section-head span {
  color: var(--text-muted);
  font-size: 0.78rem;
  font-weight: 700;
}

.roadmap-module-drawer__task-list {
  display: grid;
  gap: 0.45rem;
  margin: 0;
  padding: 0;
  list-style: none;
}

.roadmap-module-drawer__task {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto auto;
  gap: 0.75rem;
  align-items: center;
  width: 100%;
  min-height: 3.6rem;
  padding: 0.65rem 0.75rem;
  border: 1px solid var(--admin-header-border);
  border-radius: 8px;
  background: var(--admin-header-panel-bg);
  color: var(--text-main);
  text-align: left;
  cursor: pointer;
  transition:
    border-color 0.16s ease,
    background 0.16s ease,
    transform 0.16s ease;
}

.roadmap-module-drawer__task:hover {
  border-color: rgb(var(--ring) / 0.38);
  background: var(--admin-header-hover-bg);
  transform: translateY(-1px);
}

.roadmap-module-drawer__task-check {
  display: inline-flex;
  color: rgb(var(--muted));
}

.roadmap-module-drawer__task--done .roadmap-module-drawer__task-check {
  color: rgb(var(--success));
}

.roadmap-module-drawer__task-main {
  display: grid;
  gap: 0.12rem;
  min-width: 0;
}

.roadmap-module-drawer__task-main strong,
.roadmap-module-drawer__task-main span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.roadmap-module-drawer__task-main strong {
  color: var(--text-main);
  font-size: 0.88rem;
  font-weight: 800;
}

.roadmap-module-drawer__task-main span {
  color: var(--text-muted);
  font-size: 0.74rem;
  font-weight: 700;
}

.roadmap-module-drawer__task-metric {
  display: grid;
  place-items: center;
  min-width: 3.4rem;
  padding: 0.25rem 0.45rem;
  border-radius: 7px;
  background: rgb(var(--muted) / 0.18);
  color: var(--text-main);
  font-size: 0.82rem;
  font-weight: 900;
  line-height: 1.05;
}

.roadmap-module-drawer__task-metric small {
  color: var(--text-muted);
  font-size: 0.62rem;
  font-weight: 800;
  text-transform: uppercase;
}

.roadmap-module-drawer__task--done .roadmap-module-drawer__task-metric:first-of-type {
  background: rgb(var(--success) / 0.13);
  color: rgb(var(--success));
}

.roadmap-module-drawer__empty {
  display: grid;
  justify-items: center;
  gap: 0.65rem;
  margin: 0;
  padding: 1.6rem;
  border: 1px dashed var(--admin-header-border);
  border-radius: 8px;
  color: var(--text-muted);
  text-align: center;
}

.roadmap-module-drawer__empty p {
  margin: 0;
}

@media (max-width: 720px) {
  .roadmap-module-drawer {
    width: 100vw !important;
    max-width: 100vw !important;
  }

  .roadmap-module-drawer__header,
  .roadmap-module-drawer__section-head {
    align-items: stretch;
    flex-direction: column;
  }

  .roadmap-module-drawer__header-actions {
    justify-content: flex-end;
  }

  .roadmap-module-drawer__summary {
    grid-template-columns: 1fr;
    justify-items: start;
  }

  .roadmap-module-drawer__task {
    grid-template-columns: auto minmax(0, 1fr);
  }

  .roadmap-module-drawer__task-metric {
    grid-row: 2;
  }
}
</style>
