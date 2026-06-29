<script setup lang="ts">
import { computed, provide, reactive, watch } from 'vue'
import { defineLazyComponent } from '~/utils/lazy-component'
import AdminPageHeader from '../../core/components/admin/AdminPageHeader.vue'
import CoreSkeleton from '../../core/components/CoreSkeleton.vue'
import { TASKS_PAGE_CONTEXT_KEY, useTasksPageContext } from '../composables/useTasksPageContext'
import TrackingBoardView from '../components/TrackingBoardView.vue'

// Modal da task carregado sob demanda (clicar num card abre o mesmo modal do Tasks).
const TasksTaskModal = defineLazyComponent(() => import('../components/TasksTaskModal.vue'))

// @ts-ignore Nuxt macro available at runtime in this page.
definePageMeta({
  layout: 'dashboard',
  workspaceId: 'tasks',
  pageLabel: 'Tracking',
})

const context = useTasksPageContext()
provide(TASKS_PAGE_CONTEXT_KEY, context)

const {
  pageBootstrapping,
  activeProject,
  projectModel,
  projectOptions,
  boardColumns,
  isTracking,
  taskEditorCssVars,
  taskEditorOpen,
  taskEditorMode,
} = context

// Card de tracking foca em nome/tempo/cliente/responsavel — tudo configuravel. O nome e sempre
// mostrado. Persistido como preferencia de visao (localStorage), nao e dado de negocio.
const FIELDS_KEY = 'omni.tracking.cardFields.v1'
const fields = reactive({ time: true, client: true, responsible: true })

if (import.meta.client) {
  try {
    const raw = localStorage.getItem(FIELDS_KEY)
    if (raw) {
      const parsed = JSON.parse(raw)
      fields.time = parsed.time !== false
      fields.client = parsed.client !== false
      fields.responsible = parsed.responsible !== false
    }
  } catch {
    // pref invalida: mantem defaults
  }
  watch(
    fields,
    (value) => {
      try {
        localStorage.setItem(FIELDS_KEY, JSON.stringify(value))
      } catch {
        // storage indisponivel: ignora (defaults em memoria)
      }
    },
    { deep: true },
  )
}

const fieldOptions = [
  { key: 'time' as const, label: 'Tempo' },
  { key: 'client' as const, label: 'Cliente' },
  { key: 'responsible' as const, label: 'Responsavel' },
]

const trackedCount = computed(() =>
  boardColumns.value.reduce(
    (total, column) => total + column.tasks.filter((task) => isTracking(task.id)).length,
    0,
  ),
)
</script>

<template>
  <section
    class="tracking-page"
    :class="{ 'tracking-page--side-editor-open': taskEditorOpen && taskEditorMode === 'side' }"
    :style="taskEditorCssVars"
  >
    <AdminPageHeader
      eyebrow="Tasks"
      title="Tracking"
      :description="`${trackedCount} task${trackedCount === 1 ? '' : 's'} em andamento (play/pause)`"
    />

    <div v-if="pageBootstrapping" class="tracking-page__skeleton">
      <CoreSkeleton variant="block" width="280px" height="40px" />
      <div class="mt-4 grid gap-3 xl:grid-cols-3">
        <div
          v-for="column in 3"
          :key="column"
          class="rounded-[var(--radius-sm)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] p-4"
        >
          <CoreSkeleton variant="block" width="120px" height="16px" />
          <div class="mt-3">
            <CoreSkeleton variant="card" :count="2" />
          </div>
        </div>
      </div>
    </div>

    <template v-else>
      <div class="tracking-page__toolbar">
        <USelectMenu
          v-if="projectOptions.length > 1"
          v-model="projectModel"
          :items="projectOptions"
          value-key="value"
          label-key="label"
          size="sm"
          class="tracking-page__project"
        />

        <div class="tracking-page__toolbar-spacer"></div>

        <UPopover :content="{ side: 'bottom', align: 'end' }">
          <UButton
            icon="i-lucide-sliders-horizontal"
            color="neutral"
            variant="ghost"
            size="sm"
            label="Campos"
          />
          <template #content>
            <div class="tracking-page__fields-menu">
              <p class="tracking-page__fields-title">Mostrar no card</p>
              <label
                v-for="option in fieldOptions"
                :key="option.key"
                class="tracking-page__fields-row"
              >
                <span>{{ option.label }}</span>
                <USwitch v-model="fields[option.key]" size="sm" />
              </label>
            </div>
          </template>
        </UPopover>
      </div>

      <UAlert
        v-if="!activeProject"
        color="warning"
        variant="soft"
        icon="i-lucide-folder-open"
        title="Sem projeto ativo"
        description="Crie ou selecione um projeto na pagina de Tasks."
      />

      <TrackingBoardView v-else :fields="fields" />
    </template>

    <TasksTaskModal />
  </section>
</template>

<style scoped>
.tracking-page {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding-bottom: 1rem;
}

.tracking-page__skeleton {
  padding: 0.5rem 0;
}

.tracking-page__toolbar {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.4rem 0.55rem;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-md);
  background: rgb(var(--surface-2));
}

.tracking-page__project {
  min-width: 200px;
  max-width: 280px;
}

.tracking-page__toolbar-spacer {
  flex: 1 1 auto;
}

.tracking-page__fields-menu {
  min-width: 200px;
  padding: 0.5rem;
}

.tracking-page__fields-title {
  margin-bottom: 0.4rem;
  color: rgb(var(--muted));
  font-size: 0.72rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.tracking-page__fields-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.4rem 0.35rem;
  border-radius: var(--radius-sm);
  color: rgb(var(--text));
  font-size: 0.85rem;
  cursor: pointer;
}

.tracking-page__fields-row:hover {
  background: rgb(var(--surface));
}
</style>
