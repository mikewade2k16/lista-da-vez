<script setup lang="ts">
import PlanningShiftTemplatesEditor from '~/components/planning/PlanningShiftTemplatesEditor.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import AppToggleSwitch from '~/components/ui/AppToggleSwitch.vue'
import { planningStaffDisplayName } from '~/domain/planning/staff-display'
import { WEEKDAY_DEFINITIONS } from '~/domain/planning/types'
import type {
  PlanningLaborPolicy,
  PlanningOperatingDay,
  PlanningStaffMember,
  PlanningStore,
  PlanningShiftTemplate,
  StoreLocationType,
  WeekdayId,
  WorkShiftTemplateId,
} from '~/domain/planning/types'

const props = withDefaults(
  defineProps<{
    store: PlanningStore
    policy: PlanningLaborPolicy
    staff: PlanningStaffMember[]
    section?: 'operation' | 'settings' | 'all'
    locationTypePending?: boolean
    readonly?: boolean
  }>(),
  { section: 'all', locationTypePending: false, readonly: false },
)

const emit = defineEmits<{
  'update:location-type': [value: StoreLocationType]
  'update:operating-day': [weekday: WeekdayId, patch: Partial<PlanningOperatingDay>]
  'update:shift-template': [
    locationType: StoreLocationType,
    templateId: WorkShiftTemplateId,
    patch: Pick<PlanningShiftTemplate, 'name'>,
  ]
  'update:policy': [patch: Partial<PlanningLaborPolicy>]
  'update:staff': [staffId: string, patch: Partial<PlanningStaffMember>]
  'toggle:availability': [staffId: string, weekday: WeekdayId]
}>()

const locationOptions = [
  { value: 'shopping', label: 'Shopping', meta: 'Horário ampliado e domingos configuráveis' },
  { value: 'street', label: 'Loja de rua', meta: 'Calendário comercial configurável' },
]

function numberValue(event: Event): number {
  return Number((event.target as HTMLInputElement).value) || 0
}

function textValue(event: Event): string {
  return (event.target as HTMLInputElement).value
}

function weekdayLabel(weekday: WeekdayId): string {
  return WEEKDAY_DEFINITIONS.find((day) => day.id === weekday)?.shortLabel || weekday
}

function operatingDay(weekday: WeekdayId): PlanningOperatingDay | undefined {
  return props.store.operatingHoursByLocationType[props.store.locationType].find(
    (day) => day.weekday === weekday,
  )
}

function updateShiftTemplate(
  locationType: StoreLocationType,
  templateId: WorkShiftTemplateId,
  patch: Pick<PlanningShiftTemplate, 'name'>,
): void {
  emit('update:shift-template', locationType, templateId, patch)
}
</script>

<template>
  <section class="planning-setup" aria-label="Configurações do planejamento">
    <details v-if="section !== 'settings'" class="planning-setup__group" open>
      <summary class="planning-setup__summary">
        <span>
          <strong>Loja e funcionamento</strong>
          <small>
            {{ store.locationType === 'shopping' ? 'Shopping' : 'Loja de rua' }} ·
            {{ store.timezone }}
          </small>
        </span>
        <span class="planning-setup__summary-badge">
          {{
            store.operatingHoursByLocationType[store.locationType].filter((day) => day.isOpen)
              .length
          }}
          dias abertos
        </span>
      </summary>

      <div class="planning-setup__body">
        <div class="planning-setup__form-grid">
          <AppSelectField
            label="Tipo de localização"
            :model-value="store.locationType"
            :options="locationOptions"
            :show-leading-icon="false"
            :disabled="locationTypePending || readonly"
            @update:model-value="emit('update:location-type', $event as StoreLocationType)"
          />

          <label class="planning-setup__field">
            <span>Fuso horário</span>
            <input :value="store.timezone" type="text" disabled />
          </label>
        </div>

        <div class="planning-setup__hours" aria-label="Horários de funcionamento">
          <div
            v-for="definition in WEEKDAY_DEFINITIONS"
            :key="definition.id"
            class="planning-setup__hours-row"
          >
            <AppToggleSwitch
              :model-value="operatingDay(definition.id)?.isOpen === true"
              :label="definition.label"
              compact
              :disabled="readonly"
              @update:model-value="emit('update:operating-day', definition.id, { isOpen: $event })"
            />
            <div class="planning-setup__time-range">
              <input
                :value="operatingDay(definition.id)?.opensAt"
                type="time"
                :disabled="readonly || !operatingDay(definition.id)?.isOpen"
                :aria-label="`Abertura de ${definition.label}`"
                @change="
                  emit('update:operating-day', definition.id, { opensAt: textValue($event) })
                "
              />
              <span>até</span>
              <input
                :value="operatingDay(definition.id)?.closesAt"
                type="time"
                :disabled="readonly || !operatingDay(definition.id)?.isOpen"
                :aria-label="`Fechamento de ${definition.label}`"
                @change="
                  emit('update:operating-day', definition.id, { closesAt: textValue($event) })
                "
              />
            </div>
          </div>
        </div>
      </div>
    </details>

    <PlanningShiftTemplatesEditor
      v-if="section !== 'operation'"
      :active-location-type="store.locationType"
      :templates-by-location-type="store.shiftTemplatesByLocationType"
      :readonly="readonly"
      @update="updateShiftTemplate"
    />

    <details v-if="section !== 'operation'" class="planning-setup__group">
      <summary class="planning-setup__summary">
        <span>
          <strong>Política de jornada</strong>
          <small>Parâmetros experimentais; não representam validação jurídica</small>
        </span>
        <span class="planning-setup__summary-badge">
          {{ policy.maxDailyHours }}h/dia · {{ policy.maxConsecutiveDays }} dias seguidos
        </span>
      </summary>

      <div class="planning-setup__body planning-setup__policy-grid">
        <label class="planning-setup__field">
          <span>Máximo diário</span>
          <input
            :value="policy.maxDailyHours"
            type="number"
            min="1"
            max="12"
            step="0.5"
            :disabled="readonly"
            @change="emit('update:policy', { maxDailyHours: numberValue($event) })"
          />
        </label>
        <label class="planning-setup__field">
          <span>Dias consecutivos</span>
          <input
            :value="policy.maxConsecutiveDays"
            type="number"
            min="1"
            max="7"
            :disabled="readonly"
            @change="emit('update:policy', { maxConsecutiveDays: numberValue($event) })"
          />
        </label>
        <label class="planning-setup__field">
          <span>Folgas mínimas/semana</span>
          <input
            :value="policy.minDaysOff"
            type="number"
            min="0"
            max="6"
            :disabled="readonly"
            @change="emit('update:policy', { minDaysOff: numberValue($event) })"
          />
        </label>
        <label class="planning-setup__field">
          <span>Intervalo após</span>
          <input
            :value="policy.breakAfterHours"
            type="number"
            min="1"
            max="12"
            step="0.5"
            :disabled="readonly"
            @change="emit('update:policy', { breakAfterHours: numberValue($event) })"
          />
        </label>
        <label class="planning-setup__field">
          <span>Intervalo mínimo (min)</span>
          <input
            :value="policy.minBreakMinutes"
            type="number"
            min="0"
            max="180"
            step="15"
            :disabled="readonly"
            @change="emit('update:policy', { minBreakMinutes: numberValue($event) })"
          />
        </label>
      </div>
    </details>

    <details v-if="section !== 'operation'" class="planning-setup__group">
      <summary class="planning-setup__summary">
        <span>
          <strong>Equipe e disponibilidade</strong>
          <small>Carga, limite individual e peso para distribuição da meta</small>
        </span>
        <span class="planning-setup__summary-badge">{{ staff.length }} pessoas</span>
      </summary>

      <div class="planning-setup__body planning-setup__staff-list">
        <article v-for="member in staff" :key="member.id" class="planning-setup__staff-card">
          <header>
            <span>
              <strong>{{ planningStaffDisplayName(member) }}</strong>
              <small>{{ member.employeeCode }} · {{ member.jobRole }}</small>
            </span>
            <div class="planning-setup__staff-inputs">
              <label>
                <span>Semanal</span>
                <input
                  :value="member.weeklyHours"
                  type="number"
                  min="1"
                  max="60"
                  :disabled="readonly"
                  @change="emit('update:staff', member.id, { weeklyHours: numberValue($event) })"
                />
              </label>
              <label>
                <span>Máx./dia</span>
                <input
                  :value="member.maxDailyHours"
                  type="number"
                  min="1"
                  max="12"
                  step="0.5"
                  :disabled="readonly"
                  @change="emit('update:staff', member.id, { maxDailyHours: numberValue($event) })"
                />
              </label>
              <label>
                <span>Peso meta</span>
                <input
                  :value="member.targetWeight"
                  type="number"
                  min="0"
                  max="3"
                  step="0.05"
                  :disabled="readonly"
                  @change="emit('update:staff', member.id, { targetWeight: numberValue($event) })"
                />
              </label>
            </div>
          </header>
          <div class="planning-setup__availability" aria-label="Disponibilidade semanal">
            <button
              v-for="day in WEEKDAY_DEFINITIONS"
              :key="day.id"
              type="button"
              :class="{ 'is-active': member.availableDays.includes(day.id) }"
              :aria-pressed="member.availableDays.includes(day.id)"
              :disabled="readonly"
              @click="emit('toggle:availability', member.id, day.id)"
            >
              {{ weekdayLabel(day.id) }}
            </button>
          </div>
        </article>
      </div>
    </details>
  </section>
</template>

<style scoped src="./planning-setup.css"></style>
