<script setup lang="ts">
import { computed, ref } from 'vue'

import PlanningScheduleCalendar from './PlanningScheduleCalendar.vue'
import PlanningScheduleHeader from './PlanningScheduleHeader.vue'
import PlanningSummaryMetrics from './PlanningSummaryMetrics.vue'
import PlanningWorkspaceState from './PlanningWorkspaceState.vue'
import AppCalendarPeriodRail from '~/components/ui/AppCalendarPeriodRail.vue'
import type {
  PlanningGoalPeriod,
  PlanningIssue,
  PlanningScheduleRevision,
  PlanningShift,
  PlanningStaffMember,
  PlanningStore,
  PlanningWeekDate,
  ScheduleStatus,
  ShiftTemplateId,
} from '~/domain/planning/types'
import { goalWeekPeriods, isGoalPeriodForMonth } from '~/utils/goal-periods'

const props = defineProps<{
  storeId: string
  storeOptions: Array<{ value: string; label: string; meta: string }>
  month: string
  period: PlanningGoalPeriod
  status: ScheduleStatus
  dates: PlanningWeekDate[]
  targetPeriodLabel: string
  canEdit: boolean
  scheduledHours: number
  contractedHours: number
  coveragePercent: number
  coveredDayCount: number
  openDayCount: number
  hardIssueCount: number
  warningCount: number
  invalidShiftTemplateCount: number
  contextPending: boolean
  contextError: string
  saveError: string
  lastSavedAt: string
  target: number
  store: PlanningStore
  staff: PlanningStaffMember[]
  calendarShifts: PlanningShift[]
  issues: PlanningIssue[]
  readonly: boolean
  history: PlanningScheduleRevision[]
}>()

const periodOptions = computed(() =>
  goalWeekPeriods(props.month).map((value, index) => ({
    value,
    label: `S${index + 1}`,
    title: `Semana ${index + 1}`,
  })),
)
const calendarView = ref<'month' | 'week'>('month')

function updateRailPeriod(value: string): void {
  if (value === 'month') {
    calendarView.value = 'month'
    return
  }
  if (isGoalPeriodForMonth(value, props.month) && value !== 'month') {
    calendarView.value = 'week'
    emit('update:period', value)
  }
}

const emit = defineEmits<{
  'update:store': [value: string]
  'update:month': [value: string]
  'update:period': [value: PlanningGoalPeriod]
  publish: []
  reopen: []
  copyPrevious: []
  replicateMonth: []
  clearWeek: []
  applyDefault: []
  retryContext: []
  retrySave: []
  generate: []
  assign: [staffId: string, isoDate: string]
  move: [staffId: string, fromDate: string, toDate: string]
  place: [staffId: string, fromDate: string, toDate: string, templateId: ShiftTemplateId]
  change: [staffId: string, isoDate: string, templateId: ShiftTemplateId]
  remove: [staffId: string, isoDate: string]
}>()
</script>

<template>
  <PlanningWorkspaceState
    v-if="contextPending && staff.length === 0"
    state="loading"
    title="Carregando escala…"
    message="Buscando horários, equipe, metas e a versão mais recente."
  />
  <PlanningWorkspaceState
    v-else-if="contextError && staff.length === 0"
    state="error"
    title="Não foi possível carregar a escala"
    :message="contextError"
    @retry="$emit('retryContext')"
  />
  <template v-else>
    <p v-if="contextError" class="planning-workspace__warning" role="alert">
      {{ contextError }}
    </p>
    <PlanningSummaryMetrics
      :scheduled-hours="scheduledHours"
      :contracted-hours="contractedHours"
      :coverage-percent="coveragePercent"
      :covered-day-count="coveredDayCount"
      :open-day-count="openDayCount"
      :hard-issue-count="hardIssueCount"
      :warning-count="warningCount"
      :staff-count="staff.length"
      :location-label="store.locationType === 'shopping' ? 'Loja em shopping' : 'Loja de rua'"
    />
    <p v-if="invalidShiftTemplateCount > 0" class="planning-workspace__warning" role="alert">
      {{ invalidShiftTemplateCount }} modelo(s) inválido(s) serão ignorados pelo gerador.
    </p>
    <p
      v-if="!contextPending && target <= 0"
      class="planning-workspace__warning is-goal"
      role="status"
    >
      Cadastre a meta {{ targetPeriodLabel }} na aba Metas para calcular o rateio da equipe.
    </p>
    <PlanningWorkspaceState
      v-if="staff.length === 0"
      state="empty"
      title="Nenhum funcionário elegível"
      message="Cadastre ou ative funcionários nesta loja para montar a escala."
    />
    <div v-else class="planning-schedule-calendar">
      <AppCalendarPeriodRail
        :model-value="calendarView === 'month' ? 'month' : period"
        :options="periodOptions"
        :disabled="status === 'saving'"
        aria-label="Semana da escala"
        @update:model-value="updateRailPeriod"
      />
      <PlanningScheduleCalendar
        :month="month"
        :view="calendarView"
        :store-id="storeId"
        :store-options="storeOptions"
        :status="status"
        :loading="contextPending"
        :store="store"
        :staff="staff"
        :dates="dates"
        :shifts="calendarShifts"
        :issues="issues"
        :readonly="readonly"
        @generate="$emit('generate')"
        @update:store="$emit('update:store', $event)"
        @update:month="$emit('update:month', $event)"
        @assign="(staffId, isoDate) => $emit('assign', staffId, isoDate)"
        @move="(staffId, fromDate, toDate) => $emit('move', staffId, fromDate, toDate)"
        @place="
          (staffId, fromDate, toDate, templateId) =>
            $emit('place', staffId, fromDate, toDate, templateId)
        "
        @change="(staffId, isoDate, templateId) => $emit('change', staffId, isoDate, templateId)"
        @remove="(staffId, isoDate) => $emit('remove', staffId, isoDate)"
      >
        <template #toolbar-actions>
          <PlanningScheduleHeader
            :status="status"
            :can-edit="canEdit"
            :history="history"
            :save-error="saveError"
            :last-saved-at="lastSavedAt"
            @publish="$emit('publish')"
            @reopen="$emit('reopen')"
            @copy-previous="$emit('copyPrevious')"
            @replicate-month="$emit('replicateMonth')"
            @clear-week="$emit('clearWeek')"
            @apply-default="$emit('applyDefault')"
            @retry-save="$emit('retrySave')"
          />
        </template>
      </PlanningScheduleCalendar>
    </div>
  </template>
</template>

<style scoped>
.planning-schedule-calendar {
  display: flex;
  min-width: 0;
  border: 1px solid rgb(var(--primary) / 0.42);
  border-radius: var(--radius-card);
  overflow: hidden;
}
.planning-schedule-calendar > :last-child {
  flex: 1;
  min-width: 0;
  border-radius: 0 var(--radius-card) var(--radius-card) 0;
  overflow: hidden;
}
.planning-schedule-calendar :deep(.app-calendar-period-rail) {
  border-right-color: rgb(var(--border) / 0.5);
}
.planning-schedule-calendar :deep(.app-calendar-surface) {
  border: 0;
}
</style>
