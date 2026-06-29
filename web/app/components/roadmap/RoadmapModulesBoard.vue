<script setup lang="ts">
import { computed, onMounted, provide, ref, watch } from 'vue'
import { Plus } from 'lucide-vue-next'
import { defineLazyComponent } from '~/utils/lazy-component'
import RoadmapModuleCard from '~/components/roadmap/RoadmapModuleCard.vue'
import RoadmapModuleForm from '~/components/roadmap/RoadmapModuleForm.vue'
import RoadmapModuleTasksModal from '~/components/roadmap/RoadmapModuleTasksModal.vue'
import {
  ROADMAP_PRIORITY_LABEL,
  type ModulePriority,
  type ModuleStatus,
  type RoadmapModule,
} from '~/components/roadmap/roadmap-data'
import {
  useRoadmapStore,
  type DashboardModuleRow,
  type DashboardTaskRow,
  type RoadmapModuleRow,
} from '~/stores/roadmap'
import {
  TASKS_PAGE_CONTEXT_KEY,
  useTasksPageContext,
} from '../../../layers/tasks/composables/useTasksPageContext'

const TasksTaskModal = defineLazyComponent(
  () => import('../../../layers/tasks/components/TasksTaskModal.vue'),
)

type CategoryFilter = 'all' | NonNullable<RoadmapModule['category']>
type StatusFilter = 'idea' | 'planning' | 'in_progress' | 'done'

const PRIORITY_ORDER: ModulePriority[] = ['P0', 'P1', 'P2', 'P3']
const CATEGORY_FILTERS: { id: CategoryFilter; label: string }[] = [
  { id: 'all', label: 'Todas' },
  { id: 'atendimento', label: 'Atendimento' },
  { id: 'tools', label: 'Tools' },
  { id: 'operacao-comercial', label: 'Operacao comercial' },
  { id: 'indicadores', label: 'Indicadores' },
  { id: 'manage', label: 'Manage' },
]
const STATUS_CHIPS: { id: StatusFilter; label: string }[] = [
  { id: 'idea', label: 'Ideia' },
  { id: 'planning', label: 'Planejamento' },
  { id: 'in_progress', label: 'Execucao' },
  { id: 'done', label: 'Feito' },
]

const activeCategory = ref<CategoryFilter>('all')
const activeStatuses = ref<Set<StatusFilter>>(new Set(['planning', 'in_progress']))
const showForm = ref(false)
const showAll = ref(false)
const selectedModuleId = ref('')
const store = useRoadmapStore()
const tasksContext = useTasksPageContext()
provide(TASKS_PAGE_CONTEXT_KEY, tasksContext)

const { taskSaving } = tasksContext

onMounted(() => {
  if (!store.modules.length) {
    void store.fetchAll()
  } else if (store.backendAvailable) {
    void store.fetchDashboard()
  }
})

function isCurated(entry: DashboardModuleRow): boolean {
  if (entry.counts.inProgress > 0 || entry.counts.planning > 0) return true
  if (entry.tasks.length > 0) return true
  return entry.module.status === 'in_progress' || entry.module.status === 'beta'
}

function matchesStatusFilter(entry: DashboardModuleRow): boolean {
  if (activeStatuses.value.size === 0) return true
  if (entry.counts.total === 0) {
    if (entry.module.status === 'pending') return activeStatuses.value.has('idea')
    if (entry.module.status === 'done') return activeStatuses.value.has('done')
    return activeStatuses.value.has('in_progress')
  }
  if (activeStatuses.value.has('idea') && entry.counts.idea > 0) return true
  if (activeStatuses.value.has('planning') && entry.counts.planning > 0) return true
  if (activeStatuses.value.has('in_progress') && entry.counts.inProgress > 0) return true
  if (activeStatuses.value.has('done') && entry.counts.done > 0) return true
  return false
}

const dashboardEntries = computed<DashboardModuleRow[]>(() => {
  if (store.dashboard.length) return store.dashboard
  return store.modules.map((m) => ({
    module: m,
    tasks: [],
    counts: {
      total: 0,
      idea: 0,
      planning: 0,
      inProgress: 0,
      done: 0,
    },
  }))
})

const filteredEntries = computed<DashboardModuleRow[]>(() => {
  return dashboardEntries.value.filter((entry) => {
    if (activeCategory.value !== 'all' && entry.module.category !== activeCategory.value) {
      return false
    }
    if (!showAll.value && !isCurated(entry)) return false
    if (!matchesStatusFilter(entry)) return false
    return true
  })
})

const groupedByPriority = computed(() => {
  return PRIORITY_ORDER.map((priority) => ({
    priority,
    label: ROADMAP_PRIORITY_LABEL[priority],
    entries: filteredEntries.value.filter((e) => e.module.priority === priority),
  })).filter((g) => g.entries.length > 0)
})

const totals = computed(() => {
  const counts = { total: 0, idea: 0, planning: 0, inProgress: 0, done: 0 }
  for (const e of filteredEntries.value) {
    counts.total += 1
    counts.idea += e.counts.idea
    counts.planning += e.counts.planning
    counts.inProgress += e.counts.inProgress
    counts.done += e.counts.done
  }
  return counts
})

const selectedEntry = computed(
  () => dashboardEntries.value.find((entry) => entry.module.id === selectedModuleId.value) || null,
)

const moduleDrawerOpen = computed({
  get: () => Boolean(selectedEntry.value),
  set: (open: boolean) => {
    if (!open) selectedModuleId.value = ''
  },
})

function toggleStatus(s: StatusFilter) {
  const next = new Set(activeStatuses.value)
  if (next.has(s)) next.delete(s)
  else next.add(s)
  activeStatuses.value = next
}

async function handleUpdate(
  m: RoadmapModuleRow,
  payload: { status: ModuleStatus; priority: ModulePriority; description: string },
) {
  try {
    await store.updateModule(m.id, payload)
  } catch (err) {
    console.error('roadmap.modules.update failed', err)
  }
}

async function handleCreate(payload: {
  sourceId: string
  label: string
  route: string
  status: ModuleStatus
  priority: ModulePriority
  category: string
  description: string
}) {
  try {
    await store.createModule(payload)
    showForm.value = false
  } catch (err) {
    console.error('roadmap.modules.create failed', err)
  }
}

async function handleDelete(m: RoadmapModuleRow) {
  if (!window.confirm(`Apagar o modulo "${m.label}"? Vira o seed global.`)) return
  try {
    await store.deleteModule(m.id)
  } catch (err) {
    console.error('roadmap.modules.delete failed', err)
  }
}

function openModule(entry: DashboardModuleRow) {
  selectedModuleId.value = entry.module.id
}

async function openTask(task: DashboardTaskRow) {
  tasksContext.setTaskEditorMode('center')
  const opened = await tasksContext.openTaskEditorById(task.id)
  if (!opened) {
    console.warn('roadmap.modules.open_task not_found', task.id)
  }
}

async function createTaskForModule(moduleId: string) {
  await tasksContext.ensureTasksWorkspaceReady()
  tasksContext.setTaskEditorMode('center')
  tasksContext.openTaskEditorForRoadmapModule(moduleId)
}

watch(
  () => taskSaving.value,
  (saving, previous) => {
    if (previous && !saving) {
      void store.fetchDashboard()
    }
  },
)
</script>

<template>
  <div class="roadmap-modules-board">
    <header class="roadmap-modules-board__header">
      <div class="roadmap-modules-board__intro">
        <h3 class="roadmap-modules-board__title">Dashboard de Modulos</h3>
        <p class="roadmap-modules-board__text">
          Modulos em execucao ou planejamento aparecem aqui. Tasks vinculadas ao Roadmap (em
          <code>/tasks</code>
          ) ficam agregadas embaixo do modulo. Use os chips de status pra filtrar.
          <span v-if="!store.backendAvailable" class="roadmap-modules-board__badge-ro">
            Modo leitura
          </span>
          <span v-else class="roadmap-modules-board__badge-live">Persistido no banco</span>
        </p>
      </div>

      <div class="roadmap-modules-board__side">
        <div class="roadmap-modules-board__stats">
          <div class="roadmap-modules-board__stat">
            <span class="roadmap-modules-board__stat-value">{{ totals.total }}</span>
            <span class="roadmap-modules-board__stat-label">Modulos</span>
          </div>
          <div class="roadmap-modules-board__stat roadmap-modules-board__stat--in-progress">
            <span class="roadmap-modules-board__stat-value">{{ totals.inProgress }}</span>
            <span class="roadmap-modules-board__stat-label">Execucao</span>
          </div>
          <div class="roadmap-modules-board__stat roadmap-modules-board__stat--planning">
            <span class="roadmap-modules-board__stat-value">{{ totals.planning }}</span>
            <span class="roadmap-modules-board__stat-label">Planejamento</span>
          </div>
          <div class="roadmap-modules-board__stat roadmap-modules-board__stat--done">
            <span class="roadmap-modules-board__stat-value">{{ totals.done }}</span>
            <span class="roadmap-modules-board__stat-label">Feitos</span>
          </div>
        </div>
        <button
          v-if="store.backendAvailable"
          type="button"
          class="roadmap-modules-board__add-btn"
          @click="showForm = !showForm"
        >
          <Plus :size="15" :stroke-width="2.2" aria-hidden="true" />
          {{ showForm ? 'Fechar' : 'Novo modulo' }}
        </button>
      </div>
    </header>

    <p v-if="store.error" class="roadmap-modules-board__error">{{ store.error }}</p>

    <RoadmapModuleForm v-if="showForm" @submit="handleCreate" @cancel="showForm = false" />

    <div class="roadmap-modules-board__filters-bar">
      <div class="roadmap-modules-board__chip-group" aria-label="Status">
        <button
          v-for="chip in STATUS_CHIPS"
          :key="chip.id"
          type="button"
          class="roadmap-modules-board__chip"
          :class="{ 'is-active': activeStatuses.has(chip.id) }"
          @click="toggleStatus(chip.id)"
        >
          {{ chip.label }}
        </button>
      </div>

      <nav class="roadmap-modules-board__filters" aria-label="Filtros por categoria">
        <button
          v-for="cat in CATEGORY_FILTERS"
          :key="cat.id"
          type="button"
          class="roadmap-modules-board__filter"
          :class="{ 'is-active': activeCategory === cat.id }"
          @click="activeCategory = cat.id"
        >
          {{ cat.label }}
        </button>
      </nav>

      <label class="roadmap-modules-board__show-all">
        <input v-model="showAll" type="checkbox" />
        <span>Mostrar todos (inclui ideias futuras)</span>
      </label>
    </div>

    <p v-if="store.loading && !filteredEntries.length" class="roadmap-modules-board__empty">
      Carregando...
    </p>

    <section
      v-for="group in groupedByPriority"
      :key="group.priority"
      class="roadmap-modules-board__group"
      :data-priority="group.priority"
    >
      <header class="roadmap-modules-board__group-head">
        <h4 class="roadmap-modules-board__group-title">{{ group.label }}</h4>
        <span class="roadmap-modules-board__group-count">{{ group.entries.length }}</span>
      </header>
      <div class="roadmap-modules-board__grid">
        <RoadmapModuleCard
          v-for="entry in group.entries"
          :key="entry.module.id"
          :module="entry.module"
          :tasks="entry.tasks"
          :counts="entry.counts"
          :editable="store.backendAvailable"
          @open="openModule(entry)"
          @open-task="openTask"
          @update="(payload) => handleUpdate(entry.module, payload)"
          @delete="handleDelete(entry.module)"
        />
      </div>
    </section>

    <p v-if="!groupedByPriority.length && !store.loading" class="roadmap-modules-board__empty">
      Nenhum modulo nos filtros atuais. Tente "Mostrar todos" ou desmarque algum chip.
    </p>

    <RoadmapModuleTasksModal
      v-model:open="moduleDrawerOpen"
      :entry="selectedEntry"
      :tasks-loading="tasksContext.pageBootstrapping.value"
      @open-task="openTask"
      @create-task="createTaskForModule"
    />
    <TasksTaskModal />
  </div>
</template>

<style scoped>
.roadmap-modules-board {
  display: grid;
  gap: 1.2rem;
}

.roadmap-modules-board__header {
  display: grid;
  gap: 1rem;
}

@media (min-width: 760px) {
  .roadmap-modules-board__header {
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: center;
  }
}

.roadmap-modules-board__intro {
  display: grid;
  gap: 0.35rem;
}

.roadmap-modules-board__title {
  margin: 0;
  font-size: 1.25rem;
  color: var(--text-main);
}

.roadmap-modules-board__text {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.88rem;
  line-height: 1.45;
  max-width: 64ch;
}

.roadmap-modules-board__text code {
  font-family: ui-monospace, SFMono-Regular, monospace;
  font-size: 0.85em;
  padding: 0.05rem 0.3rem;
  border-radius: 4px;
  background: rgb(var(--muted) / 0.4);
}

.roadmap-modules-board__badge-ro,
.roadmap-modules-board__badge-live {
  display: inline-block;
  margin-left: 0.4rem;
  padding: 0.05rem 0.5rem;
  border-radius: 999px;
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.roadmap-modules-board__badge-ro {
  background: rgb(var(--muted) / 0.4);
  color: var(--text-muted);
}

.roadmap-modules-board__badge-live {
  background: rgb(var(--success) / 0.18);
  color: rgb(var(--success));
}

.roadmap-modules-board__side {
  display: grid;
  gap: 0.65rem;
  justify-items: end;
}

.roadmap-modules-board__stats {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 0.65rem;
}

.roadmap-modules-board__stat {
  display: grid;
  place-items: center;
  min-width: 4.5rem;
  padding: 0.55rem 0.7rem;
  border: 1px solid var(--admin-header-border);
  border-radius: 11px;
  background: var(--admin-header-panel-bg);
  color: var(--text-main);
}

.roadmap-modules-board__stat--in-progress {
  border-color: rgb(var(--info) / 0.4);
  color: rgb(var(--info));
}

.roadmap-modules-board__stat--planning {
  border-color: rgb(var(--warning) / 0.4);
  color: rgb(var(--warning));
}

.roadmap-modules-board__stat--done {
  border-color: rgb(var(--success) / 0.4);
  color: rgb(var(--success));
}

.roadmap-modules-board__stat-value {
  font-size: 1.25rem;
  font-weight: 800;
  line-height: 1;
}

.roadmap-modules-board__stat-label {
  margin-top: 0.18rem;
  font-size: 0.68rem;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  font-weight: 700;
}

.roadmap-modules-board__add-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.5rem 0.95rem;
  border: 1px solid rgb(var(--success) / 0.4);
  border-radius: 10px;
  background: rgb(var(--success) / 0.14);
  color: rgb(var(--success));
  font-size: 0.82rem;
  font-weight: 700;
  cursor: pointer;
  transition:
    background 0.16s ease,
    border-color 0.16s ease;
}

.roadmap-modules-board__add-btn:hover {
  background: rgb(var(--success) / 0.22);
}

.roadmap-modules-board__error {
  margin: 0;
  padding: 0.7rem 0.9rem;
  border-radius: 10px;
  background: rgb(var(--danger) / 0.14);
  color: rgb(var(--danger));
  font-size: 0.85rem;
}

.roadmap-modules-board__filters-bar {
  display: grid;
  gap: 0.55rem;
}

.roadmap-modules-board__chip-group {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 0.4rem;
}

.roadmap-modules-board__chip {
  padding: 0.4rem 0.85rem;
  border: 1px solid var(--admin-header-border);
  border-radius: 999px;
  background: transparent;
  color: var(--text-muted);
  font-size: 0.8rem;
  font-weight: 700;
  cursor: pointer;
  transition:
    border-color 0.16s ease,
    background 0.16s ease,
    color 0.16s ease;
}

.roadmap-modules-board__chip:hover {
  border-color: rgb(var(--ring) / 0.32);
  color: var(--text-main);
}

.roadmap-modules-board__chip.is-active {
  border-color: rgb(var(--primary) / 0.5);
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary));
}

.roadmap-modules-board__filters {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 0.4rem;
}

.roadmap-modules-board__filter {
  padding: 0.42rem 0.85rem;
  border: 1px solid var(--admin-header-border);
  border-radius: 999px;
  background: transparent;
  color: var(--text-muted);
  font-size: 0.78rem;
  font-weight: 700;
  cursor: pointer;
  transition:
    border-color 0.16s ease,
    color 0.16s ease,
    background 0.16s ease;
}

.roadmap-modules-board__filter:hover {
  border-color: rgb(var(--ring) / 0.32);
  color: var(--text-main);
}

.roadmap-modules-board__filter.is-active {
  border-color: rgb(var(--primary) / 0.4);
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
}

.roadmap-modules-board__show-all {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  font-size: 0.78rem;
  color: var(--text-muted);
  cursor: pointer;
}

.roadmap-modules-board__show-all input {
  width: 0.95rem;
  height: 0.95rem;
  accent-color: rgb(var(--primary));
}

.roadmap-modules-board__group {
  display: grid;
  gap: 0.7rem;
}

.roadmap-modules-board__group-head {
  display: inline-flex;
  align-items: center;
  gap: 0.6rem;
}

.roadmap-modules-board__group-title {
  margin: 0;
  font-size: 0.85rem;
  font-weight: 800;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.roadmap-modules-board__group-count {
  padding: 0.05rem 0.5rem;
  border-radius: 999px;
  background: rgb(var(--muted) / 0.4);
  color: var(--text-muted);
  font-size: 0.72rem;
  font-weight: 700;
}

.roadmap-modules-board__grid {
  display: grid;
  gap: 0.85rem;
  grid-template-columns: repeat(auto-fill, minmax(min(340px, 100%), 1fr));
}

.roadmap-modules-board__empty {
  padding: 2rem;
  text-align: center;
  color: var(--text-muted);
  border: 1px dashed var(--admin-header-border);
  border-radius: 12px;
}
</style>
