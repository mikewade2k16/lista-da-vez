<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { Plus, Trash2 } from 'lucide-vue-next'

import AppToggleSwitch from '~/components/ui/AppToggleSwitch.vue'
import { planningStaffDisplayName } from '~/domain/planning/staff-display'
import type {
  PlanningCoverageRule,
  PlanningExceptionKind,
  PlanningHoliday,
  PlanningStaffException,
  PlanningStaffMember,
  PlanningStore,
  StoreLocationType,
} from '~/domain/planning/types'

const props = defineProps<{
  store: PlanningStore
  staff: PlanningStaffMember[]
  readonly?: boolean
}>()

const emit = defineEmits<{
  'update:coverage': [locationType: StoreLocationType, patch: Partial<PlanningCoverageRule>]
  'update:staff': [staffId: string, patch: Partial<PlanningStaffMember>]
  'add:holiday': [holiday: PlanningHoliday]
  'remove:holiday': [isoDate: string]
  'add:exception': [exception: PlanningStaffException]
  'remove:exception': [exceptionId: string]
}>()

const selectedLocationType = ref<StoreLocationType>(props.store.locationType)
const holidayDraft = reactive({
  isoDate: '',
  name: '',
  isOpen: false,
  opensAt: '10:00',
  closesAt: '18:00',
})
const exceptionDraft = reactive({
  staffId: '',
  isoDate: '',
  kind: 'vacation' as PlanningExceptionKind,
  allDay: true,
  startsAt: '09:00',
  endsAt: '18:00',
  notes: '',
})

const coverage = computed(() => props.store.coverageByLocationType[selectedLocationType.value])
const exceptionKinds: Array<{ value: PlanningExceptionKind; label: string }> = [
  { value: 'vacation', label: 'Férias' },
  { value: 'medical_leave', label: 'Atestado' },
  { value: 'training', label: 'Treinamento' },
  { value: 'meeting', label: 'Reunião' },
  { value: 'time_bank', label: 'Banco de horas' },
  { value: 'exceptional_day_off', label: 'Folga excepcional' },
]

function numberValue(event: Event): number {
  return Math.max(0, Number((event.target as HTMLInputElement).value) || 0)
}

function textValue(event: Event): string {
  return (event.target as HTMLInputElement).value
}

function exceptionLabel(kind: PlanningExceptionKind): string {
  return exceptionKinds.find((item) => item.value === kind)?.label || kind
}

function addHoliday() {
  if (!holidayDraft.isoDate || !holidayDraft.name.trim()) return
  emit('add:holiday', { ...holidayDraft, name: holidayDraft.name.trim() })
  holidayDraft.isoDate = ''
  holidayDraft.name = ''
}

function addException() {
  if (!exceptionDraft.staffId || !exceptionDraft.isoDate) return
  emit('add:exception', {
    ...exceptionDraft,
    id: `exception-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    notes: exceptionDraft.notes.trim(),
  })
  exceptionDraft.isoDate = ''
  exceptionDraft.notes = ''
}
</script>

<template>
  <section class="planning-rules">
    <details class="planning-rules__group">
      <summary>
        <span>
          <strong>Necessidade mínima por horário</strong>
          <small>Abertura, pico e fechamento configuráveis por tipo de loja</small>
        </span>
        <b>{{ coverage.enabled ? 'Ativa' : 'Desativada' }}</b>
      </summary>
      <div class="planning-rules__body">
        <div class="planning-rules__tabs">
          <button
            v-for="locationType in ['street', 'shopping'] as const"
            :key="locationType"
            type="button"
            :class="{ 'is-active': selectedLocationType === locationType }"
            @click="selectedLocationType = locationType"
          >
            {{ locationType === 'shopping' ? 'Shopping' : 'Loja de rua' }}
          </button>
        </div>
        <AppToggleSwitch
          :model-value="coverage.enabled"
          label="Aplicar mínimos na geração e validação"
          :disabled="readonly"
          @update:model-value="emit('update:coverage', selectedLocationType, { enabled: $event })"
        />
        <div class="planning-rules__grid">
          <label>
            <span>Mínimo na abertura</span>
            <input
              :value="coverage.openingMinimum"
              type="number"
              min="0"
              max="50"
              :disabled="readonly || !coverage.enabled"
              @change="
                emit('update:coverage', selectedLocationType, {
                  openingMinimum: numberValue($event),
                })
              "
            />
          </label>
          <label>
            <span>Mínimo no pico</span>
            <input
              :value="coverage.peakMinimum"
              type="number"
              min="0"
              max="50"
              :disabled="readonly || !coverage.enabled"
              @change="
                emit('update:coverage', selectedLocationType, { peakMinimum: numberValue($event) })
              "
            />
          </label>
          <label>
            <span>Mínimo no fechamento</span>
            <input
              :value="coverage.closingMinimum"
              type="number"
              min="0"
              max="50"
              :disabled="readonly || !coverage.enabled"
              @change="
                emit('update:coverage', selectedLocationType, {
                  closingMinimum: numberValue($event),
                })
              "
            />
          </label>
          <label>
            <span>Início do pico</span>
            <input
              :value="coverage.peakStartsAt"
              type="time"
              :disabled="readonly || !coverage.enabled"
              @change="
                emit('update:coverage', selectedLocationType, { peakStartsAt: textValue($event) })
              "
            />
          </label>
          <label>
            <span>Fim do pico</span>
            <input
              :value="coverage.peakEndsAt"
              type="time"
              :disabled="readonly || !coverage.enabled"
              @change="
                emit('update:coverage', selectedLocationType, { peakEndsAt: textValue($event) })
              "
            />
          </label>
        </div>
      </div>
    </details>

    <details class="planning-rules__group">
      <summary>
        <span>
          <strong>Domingos e feriados</strong>
          <small>Elegibilidade e alternância por funcionário</small>
        </span>
        <b>{{ staff.length }} pessoas</b>
      </summary>
      <div class="planning-rules__body planning-rules__people">
        <article v-for="member in staff" :key="member.id">
          <strong>{{ planningStaffDisplayName(member) }}</strong>
          <AppToggleSwitch
            :model-value="member.worksSundays"
            label="Trabalha aos domingos"
            compact
            :disabled="readonly"
            @update:model-value="emit('update:staff', member.id, { worksSundays: $event })"
          />
          <AppToggleSwitch
            :model-value="member.alternateSundays"
            label="Alternar domingos"
            compact
            :disabled="readonly || !member.worksSundays"
            @update:model-value="emit('update:staff', member.id, { alternateSundays: $event })"
          />
          <label>
            <span>Grupo do revezamento</span>
            <select
              :value="member.sundayRotationOffset"
              :disabled="readonly || !member.worksSundays || !member.alternateSundays"
              @change="
                emit('update:staff', member.id, {
                  sundayRotationOffset: Number(textValue($event)) === 1 ? 1 : 0,
                })
              "
            >
              <option :value="0">Grupo A</option>
              <option :value="1">Grupo B</option>
            </select>
          </label>
          <AppToggleSwitch
            :model-value="member.worksHolidays"
            label="Trabalha em feriados"
            compact
            :disabled="readonly"
            @update:model-value="emit('update:staff', member.id, { worksHolidays: $event })"
          />
        </article>
      </div>
    </details>

    <details class="planning-rules__group">
      <summary>
        <span>
          <strong>Feriados</strong>
          <small>Fechamento ou expediente especial por data</small>
        </span>
        <b>{{ store.holidays.length }} cadastrados</b>
      </summary>
      <div class="planning-rules__body">
        <div class="planning-rules__grid">
          <label>
            <span>Data</span>
            <input v-model="holidayDraft.isoDate" type="date" :disabled="readonly" />
          </label>
          <label>
            <span>Nome</span>
            <input v-model="holidayDraft.name" type="text" :disabled="readonly" />
          </label>
          <AppToggleSwitch v-model="holidayDraft.isOpen" label="Loja abre" :disabled="readonly" />
          <label>
            <span>Abertura especial</span>
            <input
              v-model="holidayDraft.opensAt"
              type="time"
              :disabled="readonly || !holidayDraft.isOpen"
            />
          </label>
          <label>
            <span>Fechamento especial</span>
            <input
              v-model="holidayDraft.closesAt"
              type="time"
              :disabled="readonly || !holidayDraft.isOpen"
            />
          </label>
        </div>
        <button class="planning-rules__add" type="button" :disabled="readonly" @click="addHoliday">
          <Plus :size="14" />
          Adicionar feriado
        </button>
        <ul class="planning-rules__list">
          <li v-for="holiday in store.holidays" :key="holiday.isoDate">
            <span>
              <strong>{{ holiday.name }}</strong>
              <small>
                {{ holiday.isoDate }} ·
                {{ holiday.isOpen ? `${holiday.opensAt}–${holiday.closesAt}` : 'Fechado' }}
              </small>
            </span>
            <button
              type="button"
              :disabled="readonly"
              @click="emit('remove:holiday', holiday.isoDate)"
            >
              <Trash2 :size="14" />
            </button>
          </li>
        </ul>
      </div>
    </details>

    <details class="planning-rules__group">
      <summary>
        <span>
          <strong>Férias, folgas e ausências</strong>
          <small>Bloqueios integrais ou por horário</small>
        </span>
        <b>{{ store.exceptions.length }} cadastradas</b>
      </summary>
      <div class="planning-rules__body">
        <div class="planning-rules__grid">
          <label>
            <span>Funcionário</span>
            <select v-model="exceptionDraft.staffId" :disabled="readonly">
              <option value="">Selecione</option>
              <option v-for="member in staff" :key="member.id" :value="member.id">
                {{ planningStaffDisplayName(member) }}
              </option>
            </select>
          </label>
          <label>
            <span>Tipo</span>
            <select v-model="exceptionDraft.kind" :disabled="readonly">
              <option v-for="kind in exceptionKinds" :key="kind.value" :value="kind.value">
                {{ kind.label }}
              </option>
            </select>
          </label>
          <label>
            <span>Data</span>
            <input v-model="exceptionDraft.isoDate" type="date" :disabled="readonly" />
          </label>
          <AppToggleSwitch
            v-model="exceptionDraft.allDay"
            label="Dia inteiro"
            :disabled="readonly"
          />
          <label>
            <span>Início</span>
            <input
              v-model="exceptionDraft.startsAt"
              type="time"
              :disabled="readonly || exceptionDraft.allDay"
            />
          </label>
          <label>
            <span>Fim</span>
            <input
              v-model="exceptionDraft.endsAt"
              type="time"
              :disabled="readonly || exceptionDraft.allDay"
            />
          </label>
          <label class="planning-rules__wide">
            <span>Observação</span>
            <input v-model="exceptionDraft.notes" type="text" :disabled="readonly" />
          </label>
        </div>
        <button
          class="planning-rules__add"
          type="button"
          :disabled="readonly"
          @click="addException"
        >
          <Plus :size="14" />
          Adicionar ausência
        </button>
        <ul class="planning-rules__list">
          <li v-for="exception in store.exceptions" :key="exception.id">
            <span>
              <strong>
                {{
                  planningStaffDisplayName(staff.find((member) => member.id === exception.staffId))
                }}
                · {{ exceptionLabel(exception.kind) }}
              </strong>
              <small>
                {{ exception.isoDate }} ·
                {{ exception.allDay ? 'Dia inteiro' : `${exception.startsAt}–${exception.endsAt}` }}
              </small>
            </span>
            <button
              type="button"
              :disabled="readonly"
              @click="emit('remove:exception', exception.id)"
            >
              <Trash2 :size="14" />
            </button>
          </li>
        </ul>
      </div>
    </details>
  </section>
</template>

<style scoped src="./planning-rules.css"></style>
