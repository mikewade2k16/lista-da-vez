import { computed, onBeforeUnmount, ref, watch } from 'vue'
import {
  createRetentionPolicyDraft,
  fetchRetentionPolicyVersions,
  publishRetentionPolicyVersion,
} from '~/domain/customer-intelligence/retention-policy-api'
import {
  RETENTION_PUBLICATION_REASON_OPTIONS,
  validRetentionDraftCommand,
  validRetentionPublishCommand,
  type RetentionPolicyDraftCommand,
  type RetentionPolicyPublishCommand,
  type RetentionPolicyVersion,
} from '~/domain/customer-intelligence/retention-policy-types'
import {
  classifyCustomerApiError,
  type CustomerApiErrorState,
} from '~/domain/customer-intelligence/api-error'
import { useAuthStore } from '~/stores/auth'
import { useCustomerIntelligenceStore } from '~/stores/customer-intelligence'
import { createApiRequest } from '~/utils/api-client'
import { useCustomerIntelligenceAccess } from './useCustomerIntelligenceAccess'

type RetentionMutation = 'create_draft' | 'publish' | ''

interface RetentionSelection {
  policyKey?: string
  versionId?: string
}

function validationError(message: string, reasonCode: string): CustomerApiErrorState {
  return { kind: 'error', message, reasonCode, statusCode: 422 }
}

export function useRetentionPolicies() {
  const auth = useAuthStore()
  const scope = useCustomerIntelligenceStore()
  const access = useCustomerIntelligenceAccess()
  const api = createApiRequest(useRuntimeConfig(), () => auth.accessToken)

  const policies = ref<RetentionPolicyVersion[]>([])
  const selectedPolicyKey = ref('')
  const selectedDraftId = ref('')
  const loading = ref(false)
  const savingAction = ref<RetentionMutation>('')
  const error = ref<CustomerApiErrorState | null>(null)
  const canLoad = computed(() => access.canViewSources.value && access.clientScopeReady.value)
  const scopeKey = computed(() => currentScopeKey())
  const policyKeys = computed(() => [...new Set(policies.value.map((item) => item.policyKey))])
  const selectedVersions = computed(() =>
    policies.value.filter((item) => item.policyKey === selectedPolicyKey.value),
  )
  const selectedDrafts = computed(() =>
    selectedVersions.value.filter((item) => item.status === 'draft'),
  )
  const selectedDraft = computed(
    () => selectedDrafts.value.find((item) => item.id === selectedDraftId.value) ?? null,
  )
  const latestPublished = computed(
    () => selectedVersions.value.find((item) => item.status === 'published') ?? null,
  )

  let loadController: AbortController | null = null
  let mutationController: AbortController | null = null
  let loadGeneration = 0
  let mutationGeneration = 0

  function currentScopeKey(): string {
    return `${String(scope.scopeKey || '').trim()}:${String(scope.clientAccountId || '').trim()}`
  }

  function applySelection(items: RetentionPolicyVersion[], preferred: RetentionSelection): void {
    const keys = [...new Set(items.map((item) => item.policyKey))]
    const preferredKey = String(preferred.policyKey || selectedPolicyKey.value).trim()
    selectedPolicyKey.value = keys.includes(preferredKey) ? preferredKey : (keys[0] ?? '')
    const drafts = items.filter(
      (item) => item.policyKey === selectedPolicyKey.value && item.status === 'draft',
    )
    const preferredVersionId = String(preferred.versionId || selectedDraftId.value).trim()
    selectedDraftId.value = drafts.some((item) => item.id === preferredVersionId)
      ? preferredVersionId
      : (drafts[0]?.id ?? '')
  }

  function clear(): void {
    loadController?.abort()
    mutationController?.abort()
    loadController = null
    mutationController = null
    loadGeneration += 1
    mutationGeneration += 1
    policies.value = []
    selectedPolicyKey.value = ''
    selectedDraftId.value = ''
    loading.value = false
    savingAction.value = ''
    error.value = null
  }

  async function load(preferred: RetentionSelection = {}): Promise<boolean> {
    if (!canLoad.value) {
      clear()
      return false
    }
    loadController?.abort()
    const request = new AbortController()
    loadController = request
    const current = ++loadGeneration
    const requestedScopeKey = currentScopeKey()
    policies.value = []
    loading.value = true
    error.value = null
    try {
      const response = await fetchRetentionPolicyVersions(api, request.signal)
      if (
        request.signal.aborted ||
        current !== loadGeneration ||
        requestedScopeKey !== currentScopeKey()
      ) {
        return false
      }
      policies.value = response
      applySelection(response, preferred)
      return true
    } catch (cause) {
      if (
        request.signal.aborted ||
        current !== loadGeneration ||
        requestedScopeKey !== currentScopeKey()
      ) {
        return false
      }
      const classified = classifyCustomerApiError(
        cause,
        'Nao foi possivel carregar as policies de retencao.',
      )
      if (classified.kind !== 'aborted') error.value = classified
      return false
    } finally {
      if (current === loadGeneration) {
        loading.value = false
        loadController = null
      }
    }
  }

  function selectPolicy(policyKey: string): void {
    const normalized = String(policyKey || '').trim()
    selectedPolicyKey.value = policyKeys.value.includes(normalized) ? normalized : ''
    selectedDraftId.value =
      policies.value.find(
        (item) => item.policyKey === selectedPolicyKey.value && item.status === 'draft',
      )?.id ?? ''
  }

  function selectDraft(versionId: string): void {
    const normalized = String(versionId || '').trim()
    selectedDraftId.value = selectedDrafts.value.some((item) => item.id === normalized)
      ? normalized
      : ''
  }

  async function createDraft(input: RetentionPolicyDraftCommand): Promise<boolean> {
    if (!canLoad.value || !access.canManageSources.value || savingAction.value) return false
    if (!validRetentionDraftCommand(input)) {
      error.value = validationError(
        'Revise a chave, o prazo e a acao de expiracao do draft.',
        'retention_policy_draft_invalid',
      )
      return false
    }
    mutationController?.abort()
    const request = new AbortController()
    mutationController = request
    const current = ++mutationGeneration
    const requestedScopeKey = currentScopeKey()
    savingAction.value = 'create_draft'
    error.value = null
    try {
      const created = await createRetentionPolicyDraft(api, input, request.signal)
      if (
        request.signal.aborted ||
        current !== mutationGeneration ||
        requestedScopeKey !== currentScopeKey()
      ) {
        return false
      }
      return await load({ policyKey: created.policyKey, versionId: created.id })
    } catch (cause) {
      if (
        request.signal.aborted ||
        current !== mutationGeneration ||
        requestedScopeKey !== currentScopeKey()
      ) {
        return false
      }
      const classified = classifyCustomerApiError(
        cause,
        'Nao foi possivel criar o draft da retention policy.',
      )
      if (classified.kind !== 'aborted') error.value = classified
      return false
    } finally {
      if (current === mutationGeneration) {
        savingAction.value = ''
        mutationController = null
      }
    }
  }

  async function publishDraft(
    version: RetentionPolicyVersion,
    input: RetentionPolicyPublishCommand,
  ): Promise<boolean> {
    const currentVersion = policies.value.find((item) => item.id === version.id)
    if (
      !canLoad.value ||
      !access.canManageSources.value ||
      savingAction.value ||
      !currentVersion ||
      currentVersion.policyKey !== version.policyKey ||
      currentVersion.revision !== version.revision ||
      version.status !== 'draft' ||
      version.revision !== input.expectedRevision
    ) {
      return false
    }
    if (!validRetentionPublishCommand(input)) {
      error.value = validationError(
        'Selecione um motivo registrado e informe uma referencia de aprovacao valida.',
        'retention_policy_publication_metadata_invalid',
      )
      return false
    }
    mutationController?.abort()
    const request = new AbortController()
    mutationController = request
    const current = ++mutationGeneration
    const requestedScopeKey = currentScopeKey()
    savingAction.value = 'publish'
    error.value = null
    try {
      const published = await publishRetentionPolicyVersion(api, version.id, input, request.signal)
      if (
        request.signal.aborted ||
        current !== mutationGeneration ||
        requestedScopeKey !== currentScopeKey()
      ) {
        return false
      }
      return await load({ policyKey: published.policyKey })
    } catch (cause) {
      if (
        request.signal.aborted ||
        current !== mutationGeneration ||
        requestedScopeKey !== currentScopeKey()
      ) {
        return false
      }
      const classified = classifyCustomerApiError(
        cause,
        'Nao foi possivel publicar a retention policy.',
      )
      if (classified.kind !== 'aborted') error.value = classified
      return false
    } finally {
      if (current === mutationGeneration) {
        savingAction.value = ''
        mutationController = null
      }
    }
  }

  watch(
    [() => scope.scopeKey, canLoad, () => access.canManageSources.value],
    () => {
      clear()
      void load()
    },
    { immediate: true, flush: 'sync' },
  )
  onBeforeUnmount(clear)

  return {
    access,
    reasonOptions: RETENTION_PUBLICATION_REASON_OPTIONS,
    policies,
    policyKeys,
    selectedPolicyKey,
    selectedVersions,
    selectedDrafts,
    selectedDraftId,
    selectedDraft,
    latestPublished,
    loading,
    savingAction,
    error,
    scopeKey,
    load,
    selectPolicy,
    selectDraft,
    createDraft,
    publishDraft,
    clear,
  }
}
