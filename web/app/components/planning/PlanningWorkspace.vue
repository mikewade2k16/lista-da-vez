<script setup lang="ts">
import { computed, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { CalendarClock, Settings } from 'lucide-vue-next'

import PlanningGoalsPanel from '~/components/planning/PlanningGoalsPanel.vue'
import PlanningOperationPanel from '~/components/planning/PlanningOperationPanel.vue'
import PlanningSchedulePanel from '~/components/planning/PlanningSchedulePanel.vue'
import PlanningSectionSidebar from '~/components/planning/PlanningSectionSidebar.vue'
import PlanningSettingsConnector from '~/components/planning/PlanningSettingsConnector.vue'
import PlanningWorkspaceState from '~/components/planning/PlanningWorkspaceState.vue'
import { usePlanningWorkspaceContext } from '~/composables/usePlanningWorkspaceContext'
import { usePlanningWeekCopy } from '~/composables/usePlanningWeekCopy'
import { usePlanningPublication } from '~/composables/usePlanningPublication'
import type { PlanningSectionId, StoreLocationType } from '~/domain/planning/types'
import { useMultiStoreStore } from '~/stores/multistore'
import { usePlanningStore } from '~/stores/planning'
import { useUiStore } from '~/stores/ui'

const planning = usePlanningStore()
const multiStore = useMultiStoreStore()
const ui = useUiStore()
const props = defineProps<{ section: PlanningSectionId }>()
const activeSection = computed(() => props.section)
const settingsOpen = ref(false)
const locationTypePending = ref(false)
const {
  activeStoreId,
  weekStart,
  selectedMonth,
  selectedPeriod,
  scheduleStatus,
  scheduleVersion,
  activeStore,
  activePolicy,
  activeStaff,
  weekDates,
  activeShifts,
  storeShifts,
  issues,
  allocations,
  showWorkspaceHero,
  selectedTarget,
  scheduledHours,
  contractedHours,
  hardIssueCount,
  warningCount,
  openDayCount,
  coveredDayCount,
  invalidShiftTemplateCount,
} = storeToRefs(planning)

const {
  bootPending,
  clientReady,
  bootError,
  selectedGoalsMonth,
  selectedGoalsStoreId,
  selectedGoalsPeriod,
  activeManagedStores,
  storeOptions,
  allowAllGoalStores,
  canViewPlanning,
  canEditPlanning,
  activeAuthStoreId,
  planningContextPending,
  planningContextError,
  scheduleHistory,
  saveError,
  lastSavedAt,
  refreshPlanningContext,
  persistSchedule,
  generateSchedule,
  updateGoalsStore,
  updateGoalsMonth,
  updateGoalsPeriod,
  loadPlanningStores,
} = usePlanningWorkspaceContext(activeSection)
const planningReadonly = computed(
  () =>
    !canEditPlanning.value ||
    scheduleStatus.value === 'published' ||
    scheduleStatus.value === 'loading' ||
    scheduleStatus.value === 'saving' ||
    planningContextPending.value,
)
const coveragePercent = computed(() =>
  openDayCount.value > 0 ? Math.round((coveredDayCount.value / openDayCount.value) * 100) : 0,
)
const targetPeriodLabel = computed(() =>
  selectedPeriod.value === 'month' ? 'do mês' : `da semana ${selectedPeriod.value.slice(1)}`,
)
const goalPlanningAllocations = computed(() =>
  selectedGoalsStoreId.value === activeStoreId.value &&
  selectedGoalsMonth.value === selectedMonth.value &&
  selectedGoalsPeriod.value === selectedPeriod.value
    ? allocations.value
    : [],
)

async function changeLocationType(value: StoreLocationType): Promise<void> {
  const storeId = activeStoreId.value
  if (
    !canEditPlanning.value ||
    !storeId ||
    locationTypePending.value ||
    activeStore.value?.locationType === value
  )
    return

  locationTypePending.value = true
  try {
    const result = await multiStore.updateStore(storeId, {
      storeType: value === 'shopping' ? 'shopping' : 'bairro',
    })
    if (result?.ok === false) {
      ui.error(result.message || 'Não foi possível atualizar o tipo da loja.')
      return
    }
    if (activeStoreId.value === storeId) {
      planning.updateLocationType(value)
      await persistSchedule('saved', true)
    }
    ui.success(
      `Funcionamento alterado para ${value === 'shopping' ? 'Shopping' : 'Loja de rua'}.`,
      'Tipo da loja atualizado',
    )
  } finally {
    locationTypePending.value = false
  }
}

async function generateAutomaticSchedule() {
  if (planningReadonly.value) return
  await generateSchedule()
}

const { publishSchedule, reopenSchedule } = usePlanningPublication(
  canEditPlanning,
  scheduleStatus,
  hardIssueCount,
  activeShifts,
  activeStoreId,
  weekStart,
  scheduleVersion,
  lastSavedAt,
  saveError,
  persistSchedule,
)

const { copyPreviousWeek, clearWeek, replicateMonthWeeks } = usePlanningWeekCopy(
  planningReadonly,
  planningContextPending,
  () => persistSchedule(),
)

async function updateOperatingDay(...args: Parameters<typeof planning.updateOperatingDay>) {
  if (planningReadonly.value) return
  planning.updateOperatingDay(...args)
  await persistSchedule('saved', true)
}

async function persistUpdatedConfiguration() {
  if (planningReadonly.value) return
  await persistSchedule('saved', true)
}

async function updateSchedule(mutator: () => unknown) {
  if (planningReadonly.value) return
  const changed = mutator()
  if (changed === false) return
  await persistSchedule()
}
</script>

<template>
  <section v-if="clientReady && canViewPlanning" class="planning-workspace">
    <header v-if="showWorkspaceHero" class="planning-workspace__hero">
      <div>
        <span class="planning-workspace__eyebrow">
          <CalendarClock :size="15" :stroke-width="2.2" />
          Planejamento de equipe
        </span>
        <h1>Escalas e metas por loja</h1>
        <p>Cada área possui seus próprios filtros e responsabilidades.</p>
      </div>
      <div class="planning-workspace__hero-actions">
        <button type="button" :disabled="!canEditPlanning" @click="settingsOpen = true">
          <Settings :size="15" />
          Configurações
        </button>
      </div>
    </header>

    <PlanningWorkspaceState
      v-if="bootPending"
      state="loading"
      title="Carregando planejamento…"
      message="Buscando as lojas disponíveis para o seu perfil."
    />
    <PlanningWorkspaceState
      v-else-if="bootError"
      state="error"
      title="Não foi possível abrir o planejamento"
      :message="bootError"
      retry-label="Recarregar lojas"
      @retry="loadPlanningStores"
    />
    <div v-else-if="activeStore && activePolicy" class="planning-workspace__shell omni-glass">
      <PlanningSectionSidebar
        :model-value="activeSection"
        :can-edit="canEditPlanning"
        @open-settings="settingsOpen = true"
      />

      <main class="planning-workspace__content">
        <PlanningWorkspaceState
          v-if="activeSection === 'operation' && planningContextPending"
          state="loading"
          title="Carregando funcionamento…"
          message="Buscando os horários e as regras atuais da loja."
        />
        <PlanningWorkspaceState
          v-else-if="activeSection === 'operation' && planningContextError"
          state="error"
          title="Não foi possível carregar o funcionamento"
          :message="planningContextError"
          @retry="refreshPlanningContext"
        />
        <PlanningOperationPanel
          v-else-if="activeSection === 'operation'"
          :store="activeStore"
          :policy="activePolicy"
          :staff="activeStaff"
          :store-id="activeStoreId"
          :store-options="storeOptions"
          :pending="locationTypePending"
          :readonly="!canEditPlanning"
          @update:store="planning.setActiveStore"
          @update:location-type="changeLocationType"
          @update:operating-day="updateOperatingDay"
        />

        <PlanningSchedulePanel
          v-else-if="activeSection === 'schedule'"
          :store-id="activeStoreId"
          :store-options="storeOptions"
          :month="selectedMonth"
          :period="selectedPeriod"
          :status="scheduleStatus"
          :dates="weekDates"
          :target-period-label="targetPeriodLabel"
          :can-edit="canEditPlanning"
          :scheduled-hours="scheduledHours"
          :contracted-hours="contractedHours"
          :coverage-percent="coveragePercent"
          :covered-day-count="coveredDayCount"
          :open-day-count="openDayCount"
          :hard-issue-count="hardIssueCount"
          :warning-count="warningCount"
          :invalid-shift-template-count="invalidShiftTemplateCount"
          :context-pending="planningContextPending"
          :context-error="planningContextError"
          :save-error="saveError"
          :last-saved-at="lastSavedAt"
          :target="selectedTarget"
          :store="activeStore"
          :staff="activeStaff"
          :calendar-shifts="storeShifts"
          :issues="issues"
          :history="scheduleHistory"
          :readonly="planningReadonly"
          @update:store="planning.setActiveStore"
          @update:month="planning.setGoalReference($event, selectedPeriod)"
          @update:period="planning.setGoalReference(selectedMonth, $event)"
          @publish="publishSchedule"
          @reopen="reopenSchedule"
          @copy-previous="copyPreviousWeek"
          @replicate-month="replicateMonthWeeks"
          @clear-week="clearWeek"
          @apply-default="generateAutomaticSchedule"
          @retry-context="refreshPlanningContext"
          @retry-save="persistSchedule()"
          @generate="generateAutomaticSchedule"
          @assign="
            (staffId, isoDate) => updateSchedule(() => planning.assignStaffToDay(staffId, isoDate))
          "
          @move="
            (staffId, fromDate, toDate) =>
              updateSchedule(() => planning.moveShift(staffId, fromDate, toDate))
          "
          @place="
            (staffId, fromDate, toDate, templateId) =>
              updateSchedule(() => planning.placeShift(staffId, fromDate, toDate, templateId))
          "
          @change="
            (staffId, isoDate, templateId) =>
              updateSchedule(() => planning.setShift(staffId, isoDate, templateId))
          "
          @remove="
            (staffId, isoDate) => updateSchedule(() => planning.setShift(staffId, isoDate, 'off'))
          "
        />

        <PlanningGoalsPanel
          v-else
          :stores="activeManagedStores"
          :active-store-id="activeAuthStoreId"
          :can-view="canViewPlanning"
          :can-edit="canEditPlanning"
          :allow-all-store-scope="allowAllGoalStores"
          :selected-month="selectedGoalsMonth"
          :selected-store-id="selectedGoalsStoreId"
          :selected-period="selectedGoalsPeriod"
          :planning-allocations="goalPlanningAllocations"
          @update:selected-month="updateGoalsMonth"
          @update:selected-store-id="updateGoalsStore"
          @update:selected-period="updateGoalsPeriod"
        />
      </main>
    </div>

    <PlanningWorkspaceState
      v-else
      state="empty"
      title="Nenhuma loja disponível"
      message="O Planejamento será liberado quando uma loja ativa estiver vinculada ao seu acesso."
    />

    <PlanningSettingsConnector
      v-if="activeStore && activePolicy"
      v-model="settingsOpen"
      :readonly="planningReadonly"
      @changed="persistUpdatedConfiguration"
    />
  </section>
  <p v-else-if="clientReady" class="planning-workspace__empty-access">
    Seu perfil não possui permissão para acessar o Planejamento.
  </p>
</template>

<style scoped src="./planning-workspace.css"></style>
