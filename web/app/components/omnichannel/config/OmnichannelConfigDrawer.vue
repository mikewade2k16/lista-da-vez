<script setup lang="ts">
import { computed, watch } from 'vue'
import AutomationAiConfigDrawer from '~/components/omnichannel/automation/AutomationAiConfigDrawer.vue'
import { useOmnichannelAutomationMvp } from '~/composables/omnichannel/useOmnichannelAutomationMvp'
import { useAuthStore } from '~/stores/auth'
import { useCoreAccountStore } from '../../../../layers/core/stores/account'

const props = defineProps<{ open: boolean; initialTab?: string }>()
const emit = defineEmits<{ 'update:open': [value: boolean] }>()

const auth = useAuthStore()
const accountStore = useCoreAccountStore()
const isPlatformAdmin = computed(() => auth.role === 'platform_admin')
const canManage = (permission: string) =>
  isPlatformAdmin.value || auth.effectivePermissionKeys.includes(permission)

const { profiles, profile, instances, agents, loadingProfile, saving, load, selectClient, save } =
  useOmnichannelAutomationMvp()

const canManageSettings = computed(() => canManage('omnichannel.settings.manage'))
const canManageInstances = computed(() => canManage('omnichannel.instances.manage'))
const canManageAgents = computed(() => canManage('omnichannel.agents.manage'))
const canAudit = computed(() => canManage('omnichannel.audit.view'))

async function loadForCurrentClient(): Promise<void> {
  await load()
  const currentClientId = String(accountStore.activeAccountId || '').trim()
  if (currentClientId && profiles.value.some((item) => item.client.id === currentClientId)) {
    await selectClient(currentClientId)
  }
}

watch(
  () => props.open,
  (open) => {
    if (open) void loadForCurrentClient()
  },
  { immediate: true },
)
</script>

<template>
  <AutomationAiConfigDrawer
    :open="open"
    :profile="profile"
    :profiles="profiles"
    :instances="instances"
    :agents="agents"
    :loading-profile="loadingProfile"
    :saving="saving"
    :can-manage-settings="canManageSettings"
    :can-manage-instances="canManageInstances"
    :can-manage-agents="canManageAgents"
    :can-audit="canAudit"
    :initial-tab="initialTab"
    @save="save"
    @options-changed="load"
    @update:open="emit('update:open', $event)"
  />
</template>
