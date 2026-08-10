<script setup lang="ts">
import { computed, ref } from 'vue'

import PlanningSetupPanel from '~/components/planning/PlanningSetupPanel.vue'
import PlanningRulesEditor from '~/components/planning/PlanningRulesEditor.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import AppToggleSwitch from '~/components/ui/AppToggleSwitch.vue'
import OmniEntityDrawer from '~/components/ui/OmniEntityDrawer.vue'
import type {
  PlanningLaborPolicy,
  PlanningCoverageRule,
  PlanningHoliday,
  PlanningShiftTemplate,
  PlanningStaffMember,
  PlanningStaffException,
  PlanningStore,
  StoreLocationType,
  WeekdayId,
  WorkShiftTemplateId,
} from '~/domain/planning/types'

const props = defineProps<{
  modelValue: boolean
  store: PlanningStore
  policy: PlanningLaborPolicy
  policies: PlanningLaborPolicy[]
  activePolicyId: string
  staff: PlanningStaffMember[]
  readonly?: boolean
  showWorkspaceHero: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  'select:policy': [policyId: string]
  'update:show-workspace-hero': [value: boolean]
  'update:shift-template': [
    locationType: StoreLocationType,
    templateId: WorkShiftTemplateId,
    patch: Pick<PlanningShiftTemplate, 'name'>,
  ]
  'update:policy': [patch: Partial<PlanningLaborPolicy>]
  'update:staff': [staffId: string, patch: Partial<PlanningStaffMember>]
  'toggle:availability': [staffId: string, weekday: WeekdayId]
  'update:coverage': [locationType: StoreLocationType, patch: Partial<PlanningCoverageRule>]
  'add:holiday': [holiday: PlanningHoliday]
  'remove:holiday': [isoDate: string]
  'add:exception': [exception: PlanningStaffException]
  'remove:exception': [exceptionId: string]
}>()

const mode = ref<'side' | 'center' | 'fullscreen'>('side')
const width = ref(780)
const policyOptions = computed(() =>
  props.policies.map((policy) => ({
    value: policy.id,
    label: policy.label,
    meta: `${policy.maxDailyHours}h/dia · ${policy.minDaysOff} folga(s)`,
  })),
)
</script>

<template>
  <OmniEntityDrawer
    v-model:mode="mode"
    v-model:width="width"
    :model-value="modelValue"
    title="Configurações do planejamento"
    :subtitle="`${store.name} · aplicadas em todas as áreas`"
    @update:model-value="emit('update:modelValue', $event)"
  >
    <div class="planning-settings">
      <div class="planning-settings__intro">
        <strong>Configurações compartilhadas</strong>
        <small>
          Modelos de turno, política e disponibilidade são usados pelo gerador de escalas.
        </small>
      </div>

      <AppSelectField
        label="Política ativa"
        :model-value="activePolicyId"
        :options="policyOptions"
        :show-leading-icon="false"
        :disabled="readonly"
        @update:model-value="emit('select:policy', $event)"
      />

      <div class="planning-settings__preference">
        <span>
          <strong>Cabeçalho da página</strong>
          <small>Exibe o título e a descrição acima do planejamento.</small>
        </span>
        <AppToggleSwitch
          :model-value="showWorkspaceHero"
          label="Exibir cabeçalho"
          compact
          :disabled="readonly"
          @update:model-value="emit('update:show-workspace-hero', $event)"
        />
      </div>

      <PlanningSetupPanel
        section="settings"
        :store="store"
        :policy="policy"
        :staff="staff"
        :readonly="readonly"
        @update:shift-template="
          (locationType, templateId, patch) =>
            emit('update:shift-template', locationType, templateId, patch)
        "
        @update:policy="emit('update:policy', $event)"
        @update:staff="(staffId, patch) => emit('update:staff', staffId, patch)"
        @toggle:availability="(staffId, weekday) => emit('toggle:availability', staffId, weekday)"
      />

      <PlanningRulesEditor
        :store="store"
        :staff="staff"
        :readonly="readonly"
        @update:coverage="(locationType, patch) => emit('update:coverage', locationType, patch)"
        @update:staff="(staffId, patch) => emit('update:staff', staffId, patch)"
        @add:holiday="emit('add:holiday', $event)"
        @remove:holiday="emit('remove:holiday', $event)"
        @add:exception="emit('add:exception', $event)"
        @remove:exception="emit('remove:exception', $event)"
      />
    </div>

    <template #footer>
      <button
        class="planning-settings__close"
        type="button"
        @click="emit('update:modelValue', false)"
      >
        Fechar
      </button>
    </template>
  </OmniEntityDrawer>
</template>

<style scoped>
.planning-settings {
  display: grid;
  gap: 0.85rem;
}
.planning-settings__intro {
  display: grid;
  gap: 0.18rem;
  border: 1px solid rgb(var(--primary) / 0.24);
  border-radius: 0.75rem;
  padding: 0.7rem 0.8rem;
  background: rgb(var(--primary) / 0.07);
}
.planning-settings__intro strong {
  color: var(--text-main);
  font-size: 0.8rem;
}
.planning-settings__intro small {
  color: var(--text-muted);
  font-size: 0.68rem;
}
.planning-settings__preference {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  border: 1px solid rgb(var(--border) / 0.62);
  border-radius: 0.7rem;
  padding: 0.65rem 0.75rem;
}
.planning-settings__preference > span {
  display: grid;
  gap: 0.12rem;
}
.planning-settings__preference small {
  color: var(--text-muted);
  font-size: 0.68rem;
}
.planning-settings__close {
  min-height: 2.2rem;
  border: 1px solid rgb(var(--border) / 0.72);
  border-radius: 0.65rem;
  padding: 0.45rem 0.8rem;
  background: rgb(var(--surface-2));
  color: var(--text-main);
  font-size: 0.72rem;
  font-weight: 800;
  cursor: pointer;
}
</style>
