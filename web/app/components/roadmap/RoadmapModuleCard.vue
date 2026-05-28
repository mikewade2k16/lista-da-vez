<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  ROADMAP_MODULE_STATUS_LABEL,
  ROADMAP_PRIORITY_LABEL,
  type ModulePriority,
  type ModuleStatus,
  type RoadmapModule,
} from '~/components/roadmap/roadmap-data'
import {
  dashboardModuleProgress,
  dashboardTaskProgress,
  dashboardTaskShare,
  formatDashboardPercent,
  isDashboardTaskDone,
  type DashboardCounts,
  type DashboardTaskRow,
} from '~/stores/roadmap'

interface EditableModule extends RoadmapModule {
  sourceId?: string
  isGlobal?: boolean
}

const props = defineProps<{
  module: EditableModule
  editable?: boolean
  tasks?: DashboardTaskRow[]
  counts?: DashboardCounts
}>()

const emit = defineEmits<{
  (
    e: 'update',
    payload: { status: ModuleStatus; priority: ModulePriority; description: string },
  ): void
  (e: 'delete'): void
  (e: 'open'): void
  (e: 'open-task', task: DashboardTaskRow): void
}>()

const editing = ref(false)
const draftStatus = ref<ModuleStatus>(props.module.status)
const draftPriority = ref<ModulePriority>(props.module.priority)
const draftDescription = ref(props.module.description)

watch(
  () => [props.module.status, props.module.priority, props.module.description],
  () => {
    draftStatus.value = props.module.status
    draftPriority.value = props.module.priority
    draftDescription.value = props.module.description
  },
)

const statusLabel = computed(() => ROADMAP_MODULE_STATUS_LABEL[props.module.status])
const priorityLabel = computed(() => ROADMAP_PRIORITY_LABEL[props.module.priority])
const moduleProgress = computed(() =>
  dashboardModuleProgress({
    tasks: props.tasks || [],
    counts: props.counts || { total: 0, idea: 0, planning: 0, inProgress: 0, done: 0 },
  }),
)
const taskShare = computed(() => dashboardTaskShare(props.tasks?.length || 0))

const STATUS_OPTIONS: ModuleStatus[] = ['pending', 'in_progress', 'beta', 'done']
const PRIORITY_OPTIONS: ModulePriority[] = ['P0', 'P1', 'P2', 'P3']

function startEdit() {
  draftStatus.value = props.module.status
  draftPriority.value = props.module.priority
  draftDescription.value = props.module.description
  editing.value = true
}

function cancelEdit() {
  editing.value = false
}

function save() {
  emit('update', {
    status: draftStatus.value,
    priority: draftPriority.value,
    description: draftDescription.value.trim(),
  })
  editing.value = false
}

function taskProgress(task: DashboardTaskRow) {
  return dashboardTaskProgress(task)
}
</script>

<template>
  <article
    class="roadmap-module-card"
    :data-status="module.status"
    :data-priority="module.priority"
  >
    <header class="roadmap-module-card__head">
      <div class="roadmap-module-card__title-row">
        <h3 class="roadmap-module-card__title">{{ module.label }}</h3>
        <span class="roadmap-module-card__route">{{ module.route }}</span>
        <span
          v-if="module.isGlobal === false"
          class="roadmap-module-card__override"
          title="Override por account"
        >
          override
        </span>
      </div>
      <div class="roadmap-module-card__badges">
        <span class="roadmap-module-card__badge roadmap-module-card__badge--priority">
          {{ priorityLabel }}
        </span>
        <span
          class="roadmap-module-card__badge roadmap-module-card__badge--status"
          :data-status="module.status"
        >
          {{ statusLabel }}
        </span>
        <button
          v-if="editable && !editing"
          type="button"
          class="roadmap-module-card__edit-btn"
          @click="startEdit"
        >
          Editar
        </button>
        <button
          v-if="editable && !editing && module.isGlobal === false"
          type="button"
          class="roadmap-module-card__delete-btn"
          title="Apagar override (volta ao seed global)"
          @click="emit('delete')"
        >
          Apagar
        </button>
        <button
          v-if="!editing"
          type="button"
          class="roadmap-module-card__open-btn"
          @click="emit('open')"
        >
          Abrir
        </button>
      </div>
    </header>

    <template v-if="!editing">
      <p class="roadmap-module-card__desc">{{ module.description }}</p>

      <div class="roadmap-module-card__progress">
        <div class="roadmap-module-card__progress-head">
          <span>Progresso do modulo</span>
          <strong>{{ moduleProgress }}%</strong>
        </div>
        <div class="roadmap-module-card__progress-bar" aria-hidden="true">
          <span :style="{ width: `${moduleProgress}%` }"></span>
        </div>
      </div>

      <div v-if="counts && counts.total > 0" class="roadmap-module-card__counts">
        <span class="roadmap-module-card__count roadmap-module-card__count--idea">
          <strong>{{ counts.idea }}</strong>
          ideias
        </span>
        <span class="roadmap-module-card__count roadmap-module-card__count--planning">
          <strong>{{ counts.planning }}</strong>
          planej.
        </span>
        <span class="roadmap-module-card__count roadmap-module-card__count--in-progress">
          <strong>{{ counts.inProgress }}</strong>
          execucao
        </span>
        <span class="roadmap-module-card__count roadmap-module-card__count--done">
          <strong>{{ counts.done }}</strong>
          feitos
        </span>
      </div>

      <div v-if="tasks?.length" class="roadmap-module-card__tasks">
        <span class="roadmap-module-card__tasks-label">
          Tasks do roadmap
          <small>cada uma vale {{ formatDashboardPercent(taskShare) }}%</small>
        </span>
        <ul class="roadmap-module-card__tasks-list">
          <li
            v-for="t in tasks"
            :key="t.id"
            class="roadmap-module-card__task"
            :data-task-status="t.status || 'planning'"
          >
            <button
              type="button"
              class="roadmap-module-card__task-link"
              @click.stop="emit('open-task', t)"
            >
              <span class="roadmap-module-card__task-title">{{ t.title }}</span>
              <span v-if="t.status" class="roadmap-module-card__task-status">
                {{ t.status }}
              </span>
              <span
                class="roadmap-module-card__task-progress"
                :class="{ 'roadmap-module-card__task-progress--done': isDashboardTaskDone(t) }"
              >
                {{ taskProgress(t) }}%
              </span>
            </button>
          </li>
        </ul>
      </div>

      <div v-if="module.scope?.length" class="roadmap-module-card__scope">
        <span class="roadmap-module-card__scope-label">Escopo</span>
        <ul class="roadmap-module-card__scope-list">
          <li v-for="item in module.scope" :key="item">{{ item }}</li>
        </ul>
      </div>

      <footer v-if="module.dependsOn?.length" class="roadmap-module-card__foot">
        <span class="roadmap-module-card__deps-label">Depende de</span>
        <span v-for="dep in module.dependsOn" :key="dep" class="roadmap-module-card__dep">
          {{ dep }}
        </span>
      </footer>
    </template>

    <form v-else class="roadmap-module-card__form" @submit.prevent="save">
      <div class="roadmap-module-card__form-row">
        <label class="roadmap-module-card__field">
          <span class="roadmap-module-card__field-label">Status</span>
          <select v-model="draftStatus" class="roadmap-module-card__select">
            <option v-for="s in STATUS_OPTIONS" :key="s" :value="s">
              {{ ROADMAP_MODULE_STATUS_LABEL[s] }}
            </option>
          </select>
        </label>
        <label class="roadmap-module-card__field">
          <span class="roadmap-module-card__field-label">Prioridade</span>
          <select v-model="draftPriority" class="roadmap-module-card__select">
            <option v-for="p in PRIORITY_OPTIONS" :key="p" :value="p">
              {{ ROADMAP_PRIORITY_LABEL[p] }}
            </option>
          </select>
        </label>
      </div>

      <label class="roadmap-module-card__field">
        <span class="roadmap-module-card__field-label">Descricao</span>
        <textarea
          v-model="draftDescription"
          class="roadmap-module-card__textarea"
          rows="4"
        ></textarea>
      </label>

      <div class="roadmap-module-card__actions">
        <button
          type="button"
          class="roadmap-module-card__btn roadmap-module-card__btn--ghost"
          @click="cancelEdit"
        >
          Cancelar
        </button>
        <button type="submit" class="roadmap-module-card__btn roadmap-module-card__btn--primary">
          Salvar
        </button>
      </div>
    </form>
  </article>
</template>

<style scoped>
.roadmap-module-card {
  display: grid;
  gap: 0.7rem;
  padding: 1rem 1.1rem;
  border: 1px solid var(--admin-header-border);
  border-radius: 14px;
  background: var(--admin-header-panel-bg);
  color: var(--text-main);
  transition:
    border-color 0.16s ease,
    transform 0.16s ease;
}

.roadmap-module-card:hover {
  border-color: rgb(var(--ring) / 0.4);
  transform: translateY(-1px);
}

.roadmap-module-card__head {
  display: grid;
  gap: 0.45rem;
}

.roadmap-module-card__title-row {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: 0.55rem;
}

.roadmap-module-card__title {
  margin: 0;
  font-size: 1.05rem;
  font-weight: 700;
  line-height: 1.2;
  color: var(--text-main);
}

.roadmap-module-card__route {
  font-family: ui-monospace, SFMono-Regular, monospace;
  font-size: 0.72rem;
  color: var(--text-muted);
}

.roadmap-module-card__override {
  padding: 0.05rem 0.4rem;
  border-radius: 6px;
  background: rgb(var(--info) / 0.18);
  color: rgb(var(--info));
  font-size: 0.65rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.roadmap-module-card__badges {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 0.4rem;
  align-items: center;
}

.roadmap-module-card__badge {
  padding: 0.18rem 0.55rem;
  border-radius: 999px;
  font-size: 0.7rem;
  font-weight: 700;
  letter-spacing: 0.02em;
  line-height: 1.2;
  border: 1px solid transparent;
}

.roadmap-module-card__badge--priority {
  background: rgb(var(--primary) / 0.14);
  color: rgb(var(--primary));
  border-color: rgb(var(--primary) / 0.25);
}

.roadmap-module-card__badge--status[data-status='pending'] {
  background: rgb(var(--muted) / 0.4);
  color: var(--text-muted);
}

.roadmap-module-card__badge--status[data-status='in_progress'] {
  background: rgb(var(--info) / 0.15);
  color: rgb(var(--info));
}

.roadmap-module-card__badge--status[data-status='beta'] {
  background: rgb(var(--warning) / 0.18);
  color: rgb(var(--warning));
}

.roadmap-module-card__badge--status[data-status='done'] {
  background: rgb(var(--success) / 0.18);
  color: rgb(var(--success));
}

.roadmap-module-card__edit-btn,
.roadmap-module-card__delete-btn,
.roadmap-module-card__open-btn {
  padding: 0.2rem 0.65rem;
  border: 1px solid var(--admin-header-border);
  border-radius: 8px;
  background: transparent;
  color: var(--text-muted);
  font-size: 0.72rem;
  font-weight: 700;
  cursor: pointer;
  transition:
    border-color 0.16s ease,
    color 0.16s ease,
    background 0.16s ease;
}

.roadmap-module-card__edit-btn {
  margin-left: auto;
}

.roadmap-module-card__edit-btn:hover {
  border-color: rgb(var(--ring) / 0.4);
  background: var(--admin-header-hover-bg);
  color: var(--text-main);
}

.roadmap-module-card__delete-btn:hover {
  border-color: rgb(var(--danger) / 0.5);
  background: rgb(var(--danger) / 0.1);
  color: rgb(var(--danger));
}

.roadmap-module-card__open-btn {
  border-color: rgb(var(--primary) / 0.35);
  background: rgb(var(--primary) / 0.1);
  color: rgb(var(--primary));
}

.roadmap-module-card__open-btn:hover {
  border-color: rgb(var(--primary) / 0.48);
  background: rgb(var(--primary) / 0.18);
  color: rgb(var(--primary));
}

.roadmap-module-card__desc {
  margin: 0;
  font-size: 0.86rem;
  line-height: 1.45;
  color: var(--text-muted);
}

.roadmap-module-card__progress {
  display: grid;
  gap: 0.35rem;
}

.roadmap-module-card__progress-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 1rem;
  color: var(--text-muted);
  font-size: 0.72rem;
  font-weight: 800;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.roadmap-module-card__progress-head strong {
  color: rgb(var(--success));
  font-size: 0.86rem;
  letter-spacing: 0;
}

.roadmap-module-card__progress-bar {
  height: 0.45rem;
  overflow: hidden;
  border-radius: 999px;
  background: rgb(var(--muted) / 0.2);
}

.roadmap-module-card__progress-bar span {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, rgb(var(--primary)), rgb(var(--success)));
  transition: width 0.18s ease;
}

.roadmap-module-card__counts {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}

.roadmap-module-card__count {
  display: inline-flex;
  align-items: baseline;
  gap: 0.25rem;
  padding: 0.18rem 0.55rem;
  border: 1px solid var(--admin-header-border);
  border-radius: 999px;
  font-size: 0.7rem;
  font-weight: 600;
  color: var(--text-muted);
  background: var(--admin-header-panel-bg);
}

.roadmap-module-card__count strong {
  font-weight: 800;
  color: var(--text-main);
}

.roadmap-module-card__count--in-progress {
  border-color: rgb(var(--info) / 0.3);
  color: rgb(var(--info));
  background: rgb(var(--info) / 0.08);
}

.roadmap-module-card__count--in-progress strong {
  color: rgb(var(--info));
}

.roadmap-module-card__count--planning {
  border-color: rgb(var(--warning) / 0.3);
  color: rgb(var(--warning));
  background: rgb(var(--warning) / 0.08);
}

.roadmap-module-card__count--planning strong {
  color: rgb(var(--warning));
}

.roadmap-module-card__count--done {
  border-color: rgb(var(--success) / 0.3);
  color: rgb(var(--success));
  background: rgb(var(--success) / 0.08);
}

.roadmap-module-card__count--done strong {
  color: rgb(var(--success));
}

.roadmap-module-card__tasks {
  display: grid;
  gap: 0.35rem;
  padding: 0.7rem 0.85rem;
  border-radius: 10px;
  background: rgb(var(--primary) / 0.06);
  border: 1px dashed rgb(var(--primary) / 0.25);
}

.roadmap-module-card__tasks-label {
  display: inline-flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.75rem;
  font-size: 0.65rem;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: rgb(var(--primary));
}

.roadmap-module-card__tasks-label small {
  color: var(--text-muted);
  font-size: 0.62rem;
  letter-spacing: 0;
  text-transform: none;
}

.roadmap-module-card__tasks-list {
  margin: 0;
  padding: 0;
  list-style: none;
  display: grid;
  gap: 0.3rem;
}

.roadmap-module-card__task-link {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  width: 100%;
  padding: 0.35rem 0.55rem;
  border: 1px solid transparent;
  border-radius: 7px;
  background: var(--admin-header-panel-bg);
  color: var(--text-main);
  font-size: 0.8rem;
  text-align: left;
  cursor: pointer;
  transition:
    border-color 0.16s ease,
    background 0.16s ease;
}

.roadmap-module-card__task-link:hover {
  border-color: rgb(var(--ring) / 0.3);
  background: var(--admin-header-hover-bg);
}

.roadmap-module-card__task-title {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.roadmap-module-card__task-status {
  flex-shrink: 0;
  padding: 0.02rem 0.45rem;
  border-radius: 999px;
  background: rgb(var(--muted) / 0.4);
  color: var(--text-muted);
  font-size: 0.65rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.roadmap-module-card__task-progress {
  flex: 0 0 auto;
  min-width: 2.4rem;
  padding: 0.04rem 0.35rem;
  border-radius: 6px;
  background: rgb(var(--muted) / 0.22);
  color: var(--text-muted);
  font-size: 0.68rem;
  font-weight: 850;
  text-align: center;
}

.roadmap-module-card__task-progress--done {
  background: rgb(var(--success) / 0.13);
  color: rgb(var(--success));
}

.roadmap-module-card__scope {
  display: grid;
  gap: 0.35rem;
  padding: 0.7rem 0.85rem;
  border-radius: 10px;
  background: rgb(var(--muted) / 0.18);
}

.roadmap-module-card__scope-label {
  font-size: 0.65rem;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.roadmap-module-card__scope-list {
  margin: 0;
  padding-left: 1.1rem;
  display: grid;
  gap: 0.25rem;
  font-size: 0.82rem;
  line-height: 1.4;
  color: var(--text-main);
}

.roadmap-module-card__foot {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 0.4rem;
  align-items: center;
}

.roadmap-module-card__deps-label {
  font-size: 0.7rem;
  color: var(--text-muted);
  font-weight: 600;
}

.roadmap-module-card__dep {
  padding: 0.12rem 0.5rem;
  border-radius: 8px;
  background: rgb(var(--ring) / 0.12);
  color: var(--text-main);
  font-size: 0.72rem;
  font-weight: 700;
}

.roadmap-module-card__form {
  display: grid;
  gap: 0.7rem;
}

.roadmap-module-card__form-row {
  display: grid;
  gap: 0.7rem;
  grid-template-columns: 1fr 1fr;
}

.roadmap-module-card__field {
  display: grid;
  gap: 0.3rem;
}

.roadmap-module-card__field-label {
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.roadmap-module-card__select,
.roadmap-module-card__textarea {
  width: 100%;
  padding: 0.45rem 0.65rem;
  border: 1px solid var(--admin-header-border);
  border-radius: 9px;
  background: var(--admin-header-panel-bg);
  color: var(--text-main);
  font-size: 0.85rem;
  font-family: inherit;
}

.roadmap-module-card__textarea {
  resize: vertical;
  line-height: 1.4;
}

.roadmap-module-card__actions {
  display: inline-flex;
  justify-content: flex-end;
  gap: 0.45rem;
}

.roadmap-module-card__btn {
  padding: 0.45rem 0.95rem;
  border: 1px solid var(--admin-header-border);
  border-radius: 10px;
  background: transparent;
  color: var(--text-main);
  font-size: 0.8rem;
  font-weight: 700;
  cursor: pointer;
  transition:
    border-color 0.16s ease,
    background 0.16s ease;
}

.roadmap-module-card__btn--ghost:hover {
  background: var(--admin-header-hover-bg);
}

.roadmap-module-card__btn--primary {
  background: rgb(var(--primary) / 0.18);
  border-color: rgb(var(--primary) / 0.4);
  color: rgb(var(--primary));
}

.roadmap-module-card__btn--primary:hover {
  background: rgb(var(--primary) / 0.28);
}
</style>
