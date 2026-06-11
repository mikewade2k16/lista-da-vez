<script setup>
import SettingsConsultantManager from '~/components/settings/SettingsConsultantManager.vue'
import SettingsProductManager from '~/components/settings/SettingsProductManager.vue'
import SettingsTabs from '~/components/settings/SettingsTabs.vue'
import SettingsCrmGoalsSection from '~/components/settings/sections/SettingsCrmGoalsSection.vue'
import SettingsAlertsSection from '~/components/settings/sections/SettingsAlertsSection.vue'
import SettingsModalSection from '~/components/settings/sections/SettingsModalSection.vue'
import SettingsOperationSection from '~/components/settings/sections/SettingsOperationSection.vue'
import SettingsOptionTabSection from '~/components/settings/sections/SettingsOptionTabSection.vue'
import SettingsReasonInputSection from '~/components/settings/sections/SettingsReasonInputSection.vue'
import SettingsWorkspaceHeader from '~/components/settings/sections/SettingsWorkspaceHeader.vue'
import { useSettingsWorkspace } from '~/composables/useSettingsWorkspace'

const props = defineProps({
  state: {
    type: Object,
    required: true,
  },
})

const ctx = useSettingsWorkspace(props)
</script>

<template>
  <section class="admin-panel" data-testid="settings-panel">
    <SettingsWorkspaceHeader :runtime-settings-notice="ctx.runtimeSettingsNotice" />

    <SettingsTabs
      :tabs="ctx.visibleTabs"
      :active-tab="ctx.activeTab"
      @update:active-tab="ctx.activeTab = $event"
    />

    <SettingsOperationSection v-if="ctx.activeTab === 'operacao'" :ctx="ctx" />

    <SettingsReasonInputSection
      v-else-if="ctx.activeTab === 'cancelamento'"
      :ctx="ctx"
      :config="ctx.reasonInputSectionConfigs.cancelamento"
    />

    <SettingsReasonInputSection
      v-else-if="ctx.activeTab === 'parada'"
      :ctx="ctx"
      :config="ctx.reasonInputSectionConfigs.parada"
    />

    <SettingsModalSection v-else-if="ctx.activeTab === 'modal'" :ctx="ctx" />

    <SettingsProductManager
      v-else-if="ctx.activeTab === 'produtos'"
      :products="ctx.state.productCatalog || []"
      @add="ctx.addProduct"
      @update="ctx.updateProduct"
      @remove="ctx.removeProduct"
    />

    <SettingsConsultantManager
      v-else-if="ctx.activeTab === 'consultores'"
      :consultants="ctx.state.roster || []"
      :disabled="!ctx.canEditConsultants"
      @add="ctx.addConsultant"
      @update="ctx.updateConsultant"
      @archive="ctx.archiveConsultant"
    />

    <SettingsCrmGoalsSection v-else-if="ctx.activeTab === 'metas-crm'" :ctx="ctx" />

    <SettingsAlertsSection v-else-if="ctx.activeTab === 'alertas'" :ctx="ctx" />

    <SettingsOptionTabSection
      v-else-if="ctx.optionTabConfigs[ctx.activeTab]"
      :ctx="ctx"
      :config="ctx.optionTabConfigs[ctx.activeTab]"
    />
  </section>
</template>
