import { computed, onBeforeUnmount, ref, watch } from 'vue'
import {
  createRecommendationPolicyDraft,
  fetchRecommendationPolicies,
  rollbackRecommendationPolicyBinding,
  runRecommendationPolicyAction,
  updateRecommendationPolicyDraft,
} from '~/domain/customer-intelligence/recommendation-policy-api'
import type {
  RecommendationPolicyValue,
  RecommendationPolicyView,
} from '~/domain/customer-intelligence/recommendation-policy-types'
import {
  classifyCustomerApiError,
  type CustomerApiErrorState,
} from '~/domain/customer-intelligence/api-error'
import { useAuthStore } from '~/stores/auth'
import { useCustomerIntelligenceStore } from '~/stores/customer-intelligence'
import { createApiRequest } from '~/utils/api-client'
import { useCustomerIntelligenceAccess } from './useCustomerIntelligenceAccess'

export function useRecommendationPolicies() {
  const auth = useAuthStore()
  const scope = useCustomerIntelligenceStore()
  const access = useCustomerIntelligenceAccess()
  const api = createApiRequest(useRuntimeConfig(), () => auth.accessToken)
  const policies = ref<RecommendationPolicyView[]>([])
  const selectedPolicyKey = ref('')
  const editorValues = ref<Record<string, RecommendationPolicyValue>>({})
  const baseline = ref('')
  const loading = ref(false)
  const saving = ref(false)
  const error = ref<CustomerApiErrorState | null>(null)
  let controller: AbortController | null = null
  let generation = 0

  const selected = computed(
    () =>
      policies.value.find((policy) => policy.definition.policyKey === selectedPolicyKey.value) ??
      null,
  )
  const dirty = computed(() => JSON.stringify(editorValues.value) !== baseline.value)

  function hydrate(): void {
    const version = selected.value?.draft ?? selected.value?.effective
    editorValues.value = { ...(version?.values ?? {}) }
    baseline.value = JSON.stringify(editorValues.value)
  }

  function selectPolicy(policyKey: string, discardDirty = false): boolean {
    if (dirty.value && !discardDirty) return false
    selectedPolicyKey.value = String(policyKey || '').trim()
    hydrate()
    return true
  }

  function clear(): void {
    controller?.abort()
    controller = null
    generation += 1
    policies.value = []
    selectedPolicyKey.value = ''
    editorValues.value = {}
    baseline.value = ''
    loading.value = false
    saving.value = false
    error.value = null
  }

  async function load(preserveSelection = false, force = false): Promise<void> {
    if (!access.canViewPortfolio.value || !access.contextReady.value) {
      clear()
      return
    }
    if (dirty.value && !force) return
    controller?.abort()
    const request = new AbortController()
    controller = request
    const current = ++generation
    loading.value = true
    error.value = null
    try {
      const response = await fetchRecommendationPolicies(api, request.signal)
      if (request.signal.aborted || current !== generation) return
      const previous = selectedPolicyKey.value
      policies.value = response
      selectedPolicyKey.value =
        (preserveSelection &&
          response.some((item) => item.definition.policyKey === previous) &&
          previous) ||
        response[0]?.definition.policyKey ||
        ''
      hydrate()
    } catch (cause) {
      if (request.signal.aborted || current !== generation) return
      error.value = classifyCustomerApiError(cause, 'Policies indisponiveis.')
    } finally {
      if (current === generation) loading.value = false
    }
  }

  async function ensureDraft(): Promise<boolean> {
    if (selected.value?.draft) return true
    if (!selected.value || !access.canManagePortfolio.value) return false
    saving.value = true
    try {
      await createRecommendationPolicyDraft(api, selected.value.definition.policyKey)
      await load(true, true)
      return Boolean(selected.value?.draft)
    } catch (cause) {
      error.value = classifyCustomerApiError(cause, 'Nao foi possivel criar o draft.')
      return false
    } finally {
      saving.value = false
    }
  }

  async function save(): Promise<boolean> {
    if (!(await ensureDraft()) || !selected.value?.draft || !dirty.value) return false
    saving.value = true
    try {
      await updateRecommendationPolicyDraft(
        api,
        selected.value.draft.id,
        editorValues.value,
        selected.value.draft.revision,
      )
      baseline.value = JSON.stringify(editorValues.value)
      await load(true, true)
      return true
    } catch (cause) {
      error.value = classifyCustomerApiError(cause, 'Nao foi possivel salvar a policy.')
      return false
    } finally {
      saving.value = false
    }
  }

  async function action(actionName: 'validate' | 'publish'): Promise<boolean> {
    const draft = selected.value?.draft
    if (!draft || dirty.value || saving.value) return false
    saving.value = true
    try {
      await runRecommendationPolicyAction(api, draft.id, actionName, draft.revision)
      await load(true, true)
      return true
    } catch (cause) {
      error.value = classifyCustomerApiError(cause, `Falha ao executar ${actionName}.`)
      return false
    } finally {
      saving.value = false
    }
  }

  async function rollback(): Promise<boolean> {
    const binding = selected.value?.binding
    if (!binding || !selected.value?.canRollback || saving.value) return false
    saving.value = true
    try {
      await rollbackRecommendationPolicyBinding(api, binding.id, binding.revision)
      await load(true, true)
      return true
    } catch (cause) {
      error.value = classifyCustomerApiError(cause, 'Rollback da policy falhou.')
      return false
    } finally {
      saving.value = false
    }
  }

  function setValue(key: string, value: RecommendationPolicyValue): void {
    if (!selected.value?.canEdit || !selected.value.draft) return
    editorValues.value = { ...editorValues.value, [key]: value }
  }

  watch(
    [() => scope.ownerAccountId, () => access.canViewPortfolio.value],
    () => {
      clear()
      void load(false, true)
    },
    { immediate: true },
  )
  onBeforeUnmount(clear)

  return {
    access,
    policies,
    selectedPolicyKey,
    selected,
    editorValues,
    dirty,
    loading,
    saving,
    error,
    selectPolicy,
    setValue,
    hydrate,
    ensureDraft,
    save,
    action,
    rollback,
    load,
    clear,
  }
}
