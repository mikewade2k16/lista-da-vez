<script setup lang="ts">
import { storeToRefs } from 'pinia'
import PlanningSettingsDrawer from './PlanningSettingsDrawer.vue'
import { usePlanningStore } from '~/stores/planning'

defineProps<{ modelValue: boolean; readonly: boolean }>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  changed: []
}>()

const planning = usePlanningStore()
const { activeStore, activePolicy, policies, activePolicyId, activeStaff, showWorkspaceHero } =
  storeToRefs(planning)

function change(mutator: () => void) {
  mutator()
  emit('changed')
}
</script>

<template>
  <PlanningSettingsDrawer
    v-if="activeStore && activePolicy"
    :model-value="modelValue"
    :store="activeStore"
    :policy="activePolicy"
    :policies="policies"
    :active-policy-id="activePolicyId"
    :staff="activeStaff"
    :readonly="readonly"
    :show-workspace-hero="showWorkspaceHero"
    @update:model-value="emit('update:modelValue', $event)"
    @select:policy="(policyId) => change(() => planning.setActivePolicy(policyId))"
    @update:show-workspace-hero="(value) => change(() => planning.setShowWorkspaceHero(value))"
    @update:shift-template="
      (locationType, templateId, patch) =>
        change(() => planning.updateShiftTemplate(locationType, templateId, patch))
    "
    @update:policy="(patch) => change(() => planning.updatePolicy(patch))"
    @update:staff="(staffId, patch) => change(() => planning.updateStaffMember(staffId, patch))"
    @toggle:availability="
      (staffId, weekday) => change(() => planning.toggleStaffAvailability(staffId, weekday))
    "
    @update:coverage="
      (locationType, patch) => change(() => planning.updateCoverageRule(locationType, patch))
    "
    @add:holiday="(holiday) => change(() => planning.addHoliday(holiday))"
    @remove:holiday="(isoDate) => change(() => planning.removeHoliday(isoDate))"
    @add:exception="(exception) => change(() => planning.addStaffException(exception))"
    @remove:exception="(exceptionId) => change(() => planning.removeStaffException(exceptionId))"
  />
</template>
