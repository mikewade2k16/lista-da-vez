<script setup>
import SettingsTabs from '~/components/settings/SettingsTabs.vue'
import SettingsCatalogsSection from '~/components/settings/sections/SettingsCatalogsSection.vue'
import SettingsGamificationSection from '~/components/settings/sections/SettingsGamificationSection.vue'
import SettingsModalSection from '~/components/settings/sections/SettingsModalSection.vue'
import SettingsOperationSection from '~/components/settings/sections/SettingsOperationSection.vue'
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
  <section class="admin-panel settings-workspace" data-testid="settings-panel">
    <SettingsWorkspaceHeader :runtime-settings-notice="ctx.runtimeSettingsNotice" />

    <SettingsTabs
      :tabs="ctx.visibleTabs"
      :active-tab="ctx.activeTab"
      @update:active-tab="ctx.activeTab = $event"
    />

    <div class="settings-workspace__content">
      <SettingsOperationSection v-if="ctx.activeTab === 'operacao'" :ctx="ctx" />

      <SettingsModalSection v-else-if="ctx.activeTab === 'modal'" :ctx="ctx" />

      <SettingsCatalogsSection v-else-if="ctx.activeTab === 'catalogos'" :ctx="ctx" />

      <SettingsGamificationSection v-else-if="ctx.activeTab === 'gamificacao'" :ctx="ctx" />
    </div>
  </section>
</template>

<style scoped>
.admin-panel.settings-workspace {
  display: flex;
  flex-direction: column;
  align-content: normal;
  align-items: stretch;
  gap: 8px;
  padding: 8px;
  border-radius: 14px;
}

.settings-workspace > :deep(.settings-tabs) {
  position: relative;
  z-index: 2;
  box-sizing: border-box;
  flex: 0 0 38px;
  min-height: 38px;
  overflow-y: hidden;
}

.settings-workspace__content {
  position: relative;
  z-index: 1;
  flex: 0 0 auto;
  min-width: 0;
  isolation: isolate;
}

@media (max-width: 760px) {
  .admin-panel.settings-workspace {
    gap: 6px;
    padding: 6px;
    border-radius: 12px;
  }
}
</style>
