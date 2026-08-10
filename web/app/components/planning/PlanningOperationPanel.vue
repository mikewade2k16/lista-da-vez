<script setup lang="ts">
import PlanningSetupPanel from './PlanningSetupPanel.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import type {
  PlanningLaborPolicy,
  PlanningOperatingDay,
  PlanningStaffMember,
  PlanningStore,
  StoreLocationType,
  WeekdayId,
} from '~/domain/planning/types'

defineProps<{
  store: PlanningStore
  policy: PlanningLaborPolicy
  staff: PlanningStaffMember[]
  storeId: string
  storeOptions: Array<{ value: string; label: string; meta: string }>
  pending: boolean
  readonly: boolean
}>()

defineEmits<{
  'update:store': [storeId: string]
  'update:location-type': [value: StoreLocationType]
  'update:operating-day': [weekday: WeekdayId, patch: Partial<PlanningOperatingDay>]
}>()
</script>

<template>
  <header class="planning-workspace__section-head">
    <span>
      <strong>Horário de funcionamento</strong>
      <small>Configure os dias e horários físicos desta loja.</small>
    </span>
    <div class="planning-workspace__filters is-store-only">
      <AppSelectField
        label="Loja"
        :model-value="storeId"
        :options="storeOptions"
        searchable
        :show-leading-icon="false"
        @update:model-value="$emit('update:store', $event)"
      />
    </div>
  </header>
  <PlanningSetupPanel
    section="operation"
    :store="store"
    :policy="policy"
    :staff="staff"
    :location-type-pending="pending"
    :readonly="readonly"
    @update:location-type="$emit('update:location-type', $event)"
    @update:operating-day="(weekday, patch) => $emit('update:operating-day', weekday, patch)"
  />
</template>
