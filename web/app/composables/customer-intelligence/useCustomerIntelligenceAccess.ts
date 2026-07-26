import { computed, watch } from 'vue'
import { useAuthStore } from '~/stores/auth'
import { useCustomerIntelligenceStore } from '~/stores/customer-intelligence'
import { useCoreAccountStore } from '../../../layers/core/stores/account'

const PERMISSIONS = {
  subjectsView: ['customer_data.subjects.view', 'customer_data.relationships.view'],
  offlineView: ['customer_data.offline_interactions.view'],
  offlineManage: ['customer_data.offline_interactions.manage'],
  offlineImport: ['customer_data.offline_interactions.import'],
  segmentsView: ['customer_data.segments.view'],
  segmentsManage: ['customer_data.segments.manage'],
  segmentsPublish: ['customer_data.segments.publish'],
  segmentsEvaluate: ['customer_data.segments.evaluate'],
  segmentsExport: ['customer_data.segments.export'],
  customerDataCapabilitiesManage: ['customer_data.capabilities.manage'],
  profileView: ['customer_intelligence.profile.view'],
  profileManage: ['customer_intelligence.profile.manage'],
  sourcesView: ['customer_intelligence.sources.view'],
  sourcesManage: ['customer_intelligence.sources.manage'],
  promptsView: ['customer_intelligence.prompts.view'],
  promptsManage: ['customer_intelligence.prompts.manage'],
  promptsPublish: ['customer_intelligence.prompts.publish'],
  runsView: ['customer_intelligence.runs.view'],
  auditView: ['customer_intelligence.audit.view'],
  portfolioView: ['customer_intelligence.portfolio.view'],
  portfolioManage: ['customer_intelligence.portfolio.manage'],
  agentsManage: ['customer_intelligence.agents.manage'],
} as const

export function useCustomerIntelligenceAccess() {
  const auth = useAuthStore()
  const accountStore = useCoreAccountStore()
  const intelligenceStore = useCustomerIntelligenceStore()

  const isPlatformAdmin = computed(() => auth.role === 'platform_admin')
  const activeAccount = computed(() => accountStore.activeAccount)
  const isAgencyAccount = computed(() => activeAccount.value?.isAgency === true)
  const contextReady = computed(
    () =>
      Boolean(accountStore.activeAccountId) &&
      (accountStore.platformView ||
        accountStore.context?.account?.id === accountStore.activeAccountId),
  )
  const hasCustomerDataModule = computed(
    () => accountStore.platformView || accountStore.enabledModules.includes('customer_data'),
  )
  const hasCustomerIntelligenceModule = computed(
    () =>
      accountStore.platformView || accountStore.enabledModules.includes('customer_intelligence'),
  )
  const permissionSet = computed(() => new Set(auth.effectivePermissionKeys))

  function hasAll(keys: readonly string[]): boolean {
    return isPlatformAdmin.value || keys.every((key) => permissionSet.value.has(key))
  }

  const clientOptions = computed(() =>
    accountStore.accounts
      .filter((account) => !account.isAgency)
      .map((account) => ({
        value: account.id,
        label: account.name,
        meta: account.organizationName,
      }))
      .sort((left, right) => left.label.localeCompare(right.label)),
  )

  const selectedClientAccountId = computed(() => intelligenceStore.clientAccountId)
  const hasExplicitClient = computed(() => Boolean(selectedClientAccountId.value))
  const clientScopeReady = computed(
    () => contextReady.value && (!isAgencyAccount.value || hasExplicitClient.value),
  )

  const canViewSubjects = computed(
    () => hasCustomerDataModule.value && hasAll(PERMISSIONS.subjectsView),
  )
  const canViewIntelligenceProfile = computed(
    () => hasCustomerIntelligenceModule.value && hasAll(PERMISSIONS.profileView),
  )
  const canManageIntelligenceProfile = computed(
    () => hasCustomerIntelligenceModule.value && hasAll(PERMISSIONS.profileManage),
  )
  const canViewOffline = computed(
    () => hasCustomerDataModule.value && hasAll(PERMISSIONS.offlineView),
  )
  const canManageOffline = computed(
    () => hasCustomerDataModule.value && hasAll(PERMISSIONS.offlineManage),
  )
  const canImportOffline = computed(
    () => hasCustomerDataModule.value && hasAll(PERMISSIONS.offlineImport),
  )
  const canViewSegments = computed(
    () => hasCustomerDataModule.value && hasAll(PERMISSIONS.segmentsView),
  )
  const canManageSegments = computed(
    () => hasCustomerDataModule.value && hasAll(PERMISSIONS.segmentsManage),
  )
  const canPublishSegments = computed(
    () => hasCustomerDataModule.value && hasAll(PERMISSIONS.segmentsPublish),
  )
  const canEvaluateSegments = computed(
    () => hasCustomerDataModule.value && hasAll(PERMISSIONS.segmentsEvaluate),
  )
  const canExportSegments = computed(
    () => hasCustomerDataModule.value && hasAll(PERMISSIONS.segmentsExport),
  )
  const canManageCustomerDataCapabilities = computed(
    () => hasCustomerDataModule.value && hasAll(PERMISSIONS.customerDataCapabilitiesManage),
  )
  const canViewSources = computed(
    () => hasCustomerIntelligenceModule.value && hasAll(PERMISSIONS.sourcesView),
  )
  const canManageSources = computed(
    () => hasCustomerIntelligenceModule.value && hasAll(PERMISSIONS.sourcesManage),
  )
  const canViewPrompts = computed(
    () => hasCustomerIntelligenceModule.value && hasAll(PERMISSIONS.promptsView),
  )
  const canManagePrompts = computed(
    () => hasCustomerIntelligenceModule.value && hasAll(PERMISSIONS.promptsManage),
  )
  const canPublishPrompts = computed(
    () => hasCustomerIntelligenceModule.value && hasAll(PERMISSIONS.promptsPublish),
  )
  const canViewRuns = computed(
    () => hasCustomerIntelligenceModule.value && hasAll(PERMISSIONS.runsView),
  )
  const canViewAudit = computed(
    () => hasCustomerIntelligenceModule.value && hasAll(PERMISSIONS.auditView),
  )
  const canViewPortfolio = computed(
    () =>
      hasCustomerIntelligenceModule.value &&
      isAgencyAccount.value &&
      hasAll(PERMISSIONS.portfolioView),
  )
  const canManagePortfolio = computed(
    () =>
      hasCustomerIntelligenceModule.value &&
      isAgencyAccount.value &&
      hasAll(PERMISSIONS.portfolioManage),
  )
  const canManageAgents = computed(
    () => hasCustomerIntelligenceModule.value && hasAll(PERMISSIONS.agentsManage),
  )

  function selectClient(clientAccountId: string): void {
    const normalized = String(clientAccountId || '').trim()
    if (isAgencyAccount.value) {
      const allowed = clientOptions.value.some((option) => option.value === normalized)
      intelligenceStore.setScope(accountStore.activeAccountId, allowed ? normalized : '')
      return
    }
    intelligenceStore.setScope(accountStore.activeAccountId, accountStore.activeAccountId)
  }

  watch(
    [() => accountStore.activeAccountId, () => accountStore.context?.account?.id, isAgencyAccount],
    () => {
      const activeAccountId = accountStore.activeAccountId
      const ownerChanged = intelligenceStore.ownerAccountId !== activeAccountId
      if (ownerChanged) {
        const defaultClient = isAgencyAccount.value ? '' : activeAccountId
        intelligenceStore.setScope(activeAccountId, defaultClient)
        return
      }
      if (!isAgencyAccount.value && intelligenceStore.clientAccountId !== activeAccountId) {
        intelligenceStore.setScope(activeAccountId, activeAccountId)
      }
    },
    { immediate: true },
  )

  watch(
    [canViewSubjects, canViewIntelligenceProfile, contextReady],
    () => {
      intelligenceStore.setReadAccess({
        subjects: contextReady.value && canViewSubjects.value,
        deterministicProfile: contextReady.value && canViewSubjects.value,
        intelligenceProfile: contextReady.value && canViewIntelligenceProfile.value,
      })
    },
    { immediate: true },
  )

  return {
    activeAccount,
    contextReady,
    isAgencyAccount,
    hasCustomerDataModule,
    hasCustomerIntelligenceModule,
    clientOptions,
    selectedClientAccountId,
    clientScopeReady,
    selectClient,
    canViewSubjects,
    canViewIntelligenceProfile,
    canManageIntelligenceProfile,
    canViewOffline,
    canManageOffline,
    canImportOffline,
    canViewSegments,
    canManageSegments,
    canPublishSegments,
    canEvaluateSegments,
    canExportSegments,
    canManageCustomerDataCapabilities,
    canViewSources,
    canManageSources,
    canViewPrompts,
    canManagePrompts,
    canPublishPrompts,
    canViewRuns,
    canViewAudit,
    canViewPortfolio,
    canManagePortfolio,
    canManageAgents,
  }
}
