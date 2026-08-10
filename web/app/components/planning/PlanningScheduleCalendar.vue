<script setup lang="ts">
import { computed, ref } from 'vue'
import { GripVertical, Plus, Sparkles, Trash2 } from 'lucide-vue-next'
import AppMonthCalendarGrid from '~/components/ui/AppMonthCalendarGrid.vue'
import AppMonthInput from '~/components/ui/AppMonthInput.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import AppToolbarButton from '~/components/ui/AppToolbarButton.vue'
import OmniEntityDrawer from '~/components/ui/OmniEntityDrawer.vue'
import { usePlanningScheduleDrag } from '~/composables/usePlanningScheduleDrag'
import { shiftHours, shiftTemplatesForStore } from '~/domain/planning/scheduler'
import { planningStaffDisplayName } from '~/domain/planning/staff-display'
import type {
  PlanningIssue,
  PlanningShift,
  PlanningStaffMember,
  PlanningStore,
  PlanningWeekDate,
  ScheduleStatus,
  ShiftTemplateId,
} from '~/domain/planning/types'
import {
  addMonthsToKey,
  formatMonthTitle,
  weekdayLabels,
  type DayCellModel,
} from '~/utils/calendar'

type DrawerMode = 'side' | 'center' | 'fullscreen'
const props = defineProps<{
  month: string
  view: 'month' | 'week'
  storeId: string
  storeOptions: Array<{ value: string; label: string; meta: string }>
  status: ScheduleStatus
  loading?: boolean
  store: PlanningStore
  staff: PlanningStaffMember[]
  dates: PlanningWeekDate[]
  shifts: PlanningShift[]
  issues: PlanningIssue[]
  readonly?: boolean
}>()
const emit = defineEmits<{
  'update:store': [value: string]
  'update:month': [value: string]
  generate: []
  assign: [staffId: string, isoDate: string]
  move: [staffId: string, fromDate: string, toDate: string]
  place: [staffId: string, fromDate: string, toDate: string, templateId: ShiftTemplateId]
  change: [staffId: string, isoDate: string, templateId: ShiftTemplateId]
  remove: [staffId: string, isoDate: string]
}>()

const selectedDate = ref('')
const selectedStaffId = ref('')
const drawerMode = ref<DrawerMode>('center')
const drawerWidth = ref(760)
const weekdays = weekdayLabels('sunday')
const now = new Date()
const currentMonthKey = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`
const previousMonthKey = addMonthsToKey(currentMonthKey, -1)
const visibleDays = computed<DayCellModel[] | undefined>(() =>
  props.view === 'month'
    ? undefined
    : props.dates.map((date) => ({
        dateKey: date.isoDate,
        day: Number(date.isoDate.slice(-2)),
        inMonth: date.isoDate.startsWith(props.month),
      })),
)
const calendarTitle = computed(() =>
  props.view === 'month'
    ? formatMonthTitle(props.month)
    : `Semana · ${props.dates[0]?.dayLabel || ''} a ${props.dates.at(-1)?.dayLabel || ''}`,
)
const loadedDates = computed(() => new Set(props.dates.map((date) => date.isoDate)))
const {
  draggedShift,
  dragTarget,
  startStaffDrag,
  startShiftDrag,
  stopDrag,
  dropOnDay,
  dropOnLane,
  dragOverLane,
  dropToRemove,
} = usePlanningScheduleDrag({
  readonly: () => Boolean(props.readonly),
  loadedDates,
  assign: (staffId, isoDate) => emit('assign', staffId, isoDate),
  move: (staffId, fromDate, toDate) => emit('move', staffId, fromDate, toDate),
  place: (staffId, fromDate, toDate, templateId) =>
    emit('place', staffId, fromDate, toDate, templateId),
  remove: (staffId, isoDate) => emit('remove', staffId, isoDate),
})
const selectedShifts = computed(() => dayShifts(selectedDate.value))
const availableStaff = computed(() =>
  props.staff.filter(
    (person) => !selectedShifts.value.some((shift) => shift.staffId === person.id),
  ),
)
const staffOptions = computed(() =>
  availableStaff.value.map((person) => ({
    value: person.id,
    label: planningStaffDisplayName(person),
    meta: person.jobRole,
  })),
)
const templateOptions = computed(() => [
  { value: 'off', label: 'Folga' },
  ...shiftTemplatesForStore(props.store).map((template) => ({
    value: template.id,
    label: template.name,
    meta: `${template.startsAt}–${template.endsAt}`,
  })),
])
const shiftLanes = computed(() => shiftTemplatesForStore(props.store))

function member(id: string) {
  return props.staff.find((person) => person.id === id)
}
function memberLabel(id: string) {
  return planningStaffDisplayName(member(id))
}
function dayShifts(date: string) {
  return props.shifts
    .filter((shift) => shift.isoDate === date)
    .sort((a, b) => a.startsAt.localeCompare(b.startsAt))
}
function dayHours(date: string) {
  return dayShifts(date).reduce((total, shift) => total + shiftHours(shift), 0)
}
function dayIssueCount(date: string) {
  return props.issues.filter((issue) => issue.isoDate === date).length
}
function laneShifts(date: string, templateId: ShiftTemplateId) {
  return dayShifts(date).filter((shift) => shift.templateId === templateId)
}
function openDay(date: string) {
  selectedDate.value = date
  selectedStaffId.value = ''
}
function addSelectedStaff() {
  if (!selectedStaffId.value || !selectedDate.value || props.readonly) return
  emit('assign', selectedStaffId.value, selectedDate.value)
  selectedStaffId.value = ''
}
</script>

<template>
  <div class="planning-calendar-root">
    <AppMonthCalendarGrid
      compact
      class="planning-calendar"
      :month-key="month"
      :title="calendarTitle"
      :weekdays="weekdays"
      :visible-days="visibleDays"
    >
      <template #header-actions>
        <div class="planning-calendar__controls">
          <AppSelectField
            :model-value="storeId"
            :options="storeOptions"
            searchable
            :show-leading-icon="false"
            compact
            :disabled="status === 'saving'"
            @update:model-value="emit('update:store', $event)"
          />
          <AppMonthInput
            :model-value="month"
            :disabled="status === 'saving'"
            @update:model-value="emit('update:month', $event)"
          />
          <AppToolbarButton
            label="Mês anterior"
            :active="month === previousMonthKey"
            :disabled="status === 'saving'"
            @click="emit('update:month', previousMonthKey)"
          />
          <AppToolbarButton
            label="Este mês"
            :active="month === currentMonthKey"
            :disabled="status === 'saving'"
            @click="emit('update:month', currentMonthKey)"
          />
          <span v-if="loading" class="planning-calendar__loading">Atualizando…</span>
          <slot name="toolbar-actions"></slot>
          <button
            class="planning-calendar__primary is-icon"
            type="button"
            title="Gerar escala automática"
            aria-label="Gerar escala automática"
            :disabled="readonly"
            @click="emit('generate')"
          >
            <Sparkles :size="14" />
          </button>
        </div>
      </template>
      <div class="planning-calendar__roster">
        <span>Equipe disponível</span>
        <button
          v-for="person in staff"
          :key="person.id"
          type="button"
          :draggable="!readonly"
          @dragstart="startStaffDrag(person.id, $event)"
          @dragend="stopDrag"
        >
          <GripVertical :size="12" />
          <strong>{{ planningStaffDisplayName(person) }}</strong>
        </button>
      </div>
      <div
        v-if="draggedShift"
        class="planning-calendar__remove-zone"
        :class="{ 'is-drag-over': dragTarget === 'remove' }"
        @dragenter.prevent.stop="dragTarget = 'remove'"
        @dragover.prevent.stop
        @dragleave.self="dragTarget = ''"
        @drop.prevent.stop="dropToRemove($event)"
      >
        <Trash2 :size="15" />
        <strong>Solte aqui para remover da escala</strong>
      </div>
      <template #day="{ day }">
        <article
          class="planning-calendar__day"
          :class="{
            'is-outside': !day.inMonth,
            'is-loaded': loadedDates.has(day.dateKey),
            'is-week': view === 'week',
            'is-drag-over': dragTarget === `day:${day.dateKey}`,
          }"
          @dragenter.prevent="dragTarget = `day:${day.dateKey}`"
          @dragover.prevent
          @dragleave.self="dragTarget = ''"
          @drop.prevent.stop="dropOnDay(day.dateKey, $event)"
        >
          <button type="button" class="planning-calendar__day-head" @click="openDay(day.dateKey)">
            <span class="planning-calendar__number">{{ day.day }}</span>
            <small v-if="loadedDates.has(day.dateKey)">
              {{ dayShifts(day.dateKey).length }} pessoas · {{ dayHours(day.dateKey).toFixed(1) }}h
            </small>
          </button>
          <small v-if="dayIssueCount(day.dateKey)" class="planning-calendar__alert">
            {{ dayIssueCount(day.dateKey) }} alerta(s)
          </small>
          <div v-if="view === 'week'" class="planning-calendar__lanes">
            <section
              v-for="lane in shiftLanes"
              :key="lane.id"
              class="planning-calendar__lane"
              :class="{ 'is-drag-over': dragTarget === `${day.dateKey}:${lane.id}` }"
              :data-drop-date="day.dateKey"
              :data-drop-template="lane.id"
              @dragenter.prevent.stop="dragTarget = `${day.dateKey}:${lane.id}`"
              @dragover="dragOverLane(day.dateKey, lane.id, $event)"
              @dragleave.self="dragTarget = ''"
              @drop="dropOnLane(day.dateKey, lane.id, $event)"
            >
              <header>
                <strong>{{ lane.name }}</strong>
                <small>{{ lane.startsAt }}–{{ lane.endsAt }}</small>
              </header>
              <article
                v-for="shift in laneShifts(day.dateKey, lane.id)"
                :key="shift.staffId"
                class="planning-calendar__chip is-full"
                :draggable="!readonly"
                :data-drag-staff="shift.staffId"
                :data-drag-date="shift.isoDate"
                @dragstart.capture="startShiftDrag(shift, $event)"
                @dragend="stopDrag"
              >
                <GripVertical :size="11" />
                <button
                  type="button"
                  draggable="false"
                  class="planning-calendar__chip-copy"
                  @click.stop="openDay(day.dateKey)"
                >
                  <strong>{{ memberLabel(shift.staffId) }}</strong>
                  <small>{{ shift.startsAt }}–{{ shift.endsAt }}</small>
                </button>
                <button
                  type="button"
                  draggable="false"
                  class="planning-calendar__chip-remove"
                  :disabled="readonly"
                  :aria-label="`Remover ${memberLabel(shift.staffId)} da escala`"
                  @click.stop="emit('remove', shift.staffId, shift.isoDate)"
                >
                  <Trash2 :size="11" />
                </button>
              </article>
              <span
                v-if="!laneShifts(day.dateKey, lane.id).length"
                class="planning-calendar__lane-empty"
              >
                Solte aqui
              </span>
            </section>
          </div>
          <template v-else>
            <article
              v-for="shift in dayShifts(day.dateKey).slice(0, 3)"
              :key="shift.staffId"
              class="planning-calendar__chip is-month"
              :draggable="!readonly"
              @dragstart.stop="startShiftDrag(shift, $event)"
              @dragend="stopDrag"
            >
              <button
                type="button"
                class="planning-calendar__chip-copy"
                @click.stop="openDay(day.dateKey)"
              >
                {{ memberLabel(shift.staffId) }} · {{ shift.startsAt }}
              </button>
              <button
                type="button"
                class="planning-calendar__chip-remove"
                :disabled="readonly"
                :aria-label="`Remover ${memberLabel(shift.staffId)} da escala`"
                @click.stop="emit('remove', shift.staffId, shift.isoDate)"
              >
                <Trash2 :size="10" />
              </button>
            </article>
            <small v-if="dayShifts(day.dateKey).length > 3">
              +{{ dayShifts(day.dateKey).length - 3 }} itens
            </small>
          </template>
        </article>
      </template>
    </AppMonthCalendarGrid>

    <OmniEntityDrawer
      :model-value="Boolean(selectedDate)"
      :mode="drawerMode"
      :width="drawerWidth"
      preference-key="planning-schedule-day"
      :title="
        selectedDate ? `Escala de ${selectedDate.split('-').reverse().join('/')}` : 'Escala do dia'
      "
      :subtitle="`${selectedShifts.length} pessoas · ${dayHours(selectedDate).toFixed(1)}h`"
      @update:model-value="
        (value) => {
          if (!value) selectedDate = ''
        }
      "
      @update:mode="drawerMode = $event"
      @update:width="drawerWidth = $event"
    >
      <section class="planning-day-modal">
        <span class="planning-day-modal__label">Itens do dia</span>
        <article
          v-for="shift in selectedShifts"
          :key="shift.staffId"
          class="planning-day-modal__item"
        >
          <span>
            <strong>{{ memberLabel(shift.staffId) }}</strong>
            <small>
              {{ shift.startsAt }}–{{ shift.endsAt }} · {{ shiftHours(shift).toFixed(1) }}h
            </small>
          </span>
          <AppSelectField
            :model-value="shift.templateId"
            :options="templateOptions"
            :show-leading-icon="false"
            compact
            :disabled="readonly"
            @update:model-value="
              emit('change', shift.staffId, shift.isoDate, $event as ShiftTemplateId)
            "
          />
          <button
            type="button"
            :disabled="readonly"
            :aria-label="`Excluir ${memberLabel(shift.staffId)}`"
            @click="emit('remove', shift.staffId, shift.isoDate)"
          >
            <Trash2 :size="14" />
          </button>
        </article>
        <p v-if="!selectedShifts.length">Nenhum turno neste dia.</p>
      </section>
      <template #footer>
        <AppSelectField
          v-model="selectedStaffId"
          :options="staffOptions"
          :show-leading-icon="false"
          searchable
          compact
          placeholder="Adicionar consultor"
          :disabled="readonly || !loadedDates.has(selectedDate)"
        />
        <button
          class="planning-calendar__primary"
          type="button"
          :disabled="readonly || !selectedStaffId"
          @click="addSelectedStaff"
        >
          <Plus :size="14" />
          Adicionar
        </button>
      </template>
    </OmniEntityDrawer>
  </div>
</template>

<style scoped src="./planning-schedule-calendar.css"></style>
