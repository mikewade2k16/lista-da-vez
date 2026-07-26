import { computed, onBeforeUnmount, ref, watch } from 'vue'
import {
  createPromptDraft,
  fetchPromptCatalog,
  fetchPromptProcessView,
  publishPromptVersion,
  rollbackPromptBinding,
  runPromptVersionAction,
  updatePromptDraft,
} from '~/domain/customer-intelligence/prompt-api'
import type {
  PromptCatalogView,
  PromptDraftInput,
  PromptProcessView,
} from '~/domain/customer-intelligence/prompt-types'
import {
  classifyCustomerApiError,
  type CustomerApiErrorState,
} from '~/domain/customer-intelligence/api-error'
import { useAuthStore } from '~/stores/auth'
import { useCustomerIntelligenceStore } from '~/stores/customer-intelligence'
import { createApiRequest } from '~/utils/api-client'
import { useCustomerIntelligenceAccess } from './useCustomerIntelligenceAccess'

export function usePromptStudio() {
  const auth = useAuthStore()
  const access = useCustomerIntelligenceAccess()
  const scopeStore = useCustomerIntelligenceStore()
  const api = createApiRequest(useRuntimeConfig(), () => auth.accessToken)
  const catalog = ref<PromptCatalogView>({
    processes: [],
    pipelines: [],
    legacyManagedCapabilities: [],
  })
  const selectedProcessKey = ref('')
  const processView = ref<PromptProcessView | null>(null)
  const editorPrompt = ref('')
  const editorConfig = ref<Record<string, string | number | boolean>>({})
  const selectedAgentVersionId = ref('')
  const baseline = ref('')
  const loading = ref(false)
  const loadingProcess = ref(false)
  const saving = ref(false)
  const error = ref<CustomerApiErrorState | null>(null)
  let controller: AbortController | null = null
  let generation = 0

  const dirty = computed(
    () =>
      JSON.stringify({ prompt: editorPrompt.value, config: editorConfig.value }) !== baseline.value,
  )
  const draft = computed(() => processView.value?.draft ?? null)

  function hydrateEditor(): void {
    const currentVersion = processView.value?.draft ?? processView.value?.published
    editorPrompt.value = currentVersion?.promptText ?? ''
    editorConfig.value = { ...(currentVersion?.config ?? {}) }
    baseline.value = JSON.stringify({
      prompt: editorPrompt.value,
      config: editorConfig.value,
    })
  }

  function clear(): void {
    controller?.abort()
    controller = null
    generation += 1
    catalog.value = { processes: [], pipelines: [], legacyManagedCapabilities: [] }
    selectedProcessKey.value = ''
    processView.value = null
    editorPrompt.value = ''
    editorConfig.value = {}
    selectedAgentVersionId.value = ''
    baseline.value = ''
    error.value = null
    loading.value = false
    loadingProcess.value = false
  }

  async function loadCatalog(): Promise<void> {
    if (!access.canViewPrompts.value || !access.clientScopeReady.value) {
      clear()
      return
    }
    controller?.abort()
    const request = new AbortController()
    controller = request
    const current = ++generation
    loading.value = true
    error.value = null
    try {
      catalog.value = await fetchPromptCatalog(api, scopeStore.clientAccountId, request.signal)
      if (request.signal.aborted || current !== generation) return
      const first = catalog.value.processes[0]?.processKey ?? ''
      loading.value = false
      if (first) await selectProcess(first, true)
    } catch (cause) {
      if (request.signal.aborted || current !== generation) return
      error.value = classifyCustomerApiError(cause, 'Prompt Studio indisponivel.')
    } finally {
      if (current === generation) loading.value = false
    }
  }

  async function selectProcess(processKey: string, discardDirty = false): Promise<boolean> {
    const normalized = String(processKey || '').trim()
    if (!normalized || (!discardDirty && dirty.value)) return false
    const request = new AbortController()
    controller?.abort()
    controller = request
    const current = ++generation
    selectedProcessKey.value = normalized
    loadingProcess.value = true
    error.value = null
    try {
      const response = await fetchPromptProcessView(
        api,
        normalized,
        scopeStore.clientAccountId,
        request.signal,
      )
      if (request.signal.aborted || current !== generation) return false
      processView.value = response
      const allowedAgents = new Set(response.publishAgents.map((item) => item.agentVersionId))
      const currentAgent = response.effectiveBinding?.agentVersionId ?? ''
      if (!allowedAgents.has(selectedAgentVersionId.value)) {
        selectedAgentVersionId.value = allowedAgents.has(currentAgent)
          ? currentAgent
          : (response.publishAgents[0]?.agentVersionId ?? '')
      }
      hydrateEditor()
      return true
    } catch (cause) {
      if (request.signal.aborted || current !== generation) return false
      processView.value = null
      error.value = classifyCustomerApiError(cause, 'Nao foi possivel abrir o processo.')
      return false
    } finally {
      if (current === generation) loadingProcess.value = false
    }
  }

  async function ensureDraft(): Promise<boolean> {
    if (draft.value) return true
    if (!access.canManagePrompts.value || !selectedProcessKey.value || !editorPrompt.value.trim()) {
      return false
    }
    saving.value = true
    try {
      await createPromptDraft(
        api,
        selectedProcessKey.value,
        scopeStore.clientAccountId,
        editorPrompt.value,
        processView.value?.published?.id ?? '',
      )
      return await selectProcess(selectedProcessKey.value, true)
    } catch (cause) {
      error.value = classifyCustomerApiError(cause, 'Nao foi possivel criar o draft.')
      return false
    } finally {
      saving.value = false
    }
  }

  async function saveDraft(): Promise<boolean> {
    if (!draft.value) return ensureDraft()
    if (saving.value) return false
    saving.value = true
    try {
      const input: PromptDraftInput = {
        promptText: editorPrompt.value,
        config: editorConfig.value,
        expectedRevision: draft.value.revision,
      }
      await updatePromptDraft(api, draft.value.id, input)
      await selectProcess(selectedProcessKey.value, true)
      return true
    } catch (cause) {
      error.value = classifyCustomerApiError(cause, 'Nao foi possivel salvar o draft.')
      return false
    } finally {
      saving.value = false
    }
  }

  async function runAction(action: 'validate' | 'test' | 'publish'): Promise<void> {
    if (!draft.value || saving.value) return
    if (action === 'publish') {
      if (
        !access.canPublishPrompts.value ||
        !processView.value?.canPublish ||
        !selectedAgentVersionId.value
      )
        return
      saving.value = true
      try {
        await publishPromptVersion(
          api,
          draft.value.id,
          scopeStore.clientAccountId,
          selectedAgentVersionId.value,
        )
        await selectProcess(selectedProcessKey.value, true)
      } catch (cause) {
        error.value = classifyCustomerApiError(cause, 'Falha ao publicar o prompt.')
      } finally {
        saving.value = false
      }
      return
    }
    if (!access.canManagePrompts.value) return
    saving.value = true
    try {
      await runPromptVersionAction(api, draft.value.id, action)
      await selectProcess(selectedProcessKey.value, true)
    } catch (cause) {
      error.value = classifyCustomerApiError(cause, `Falha ao executar ${action}.`)
    } finally {
      saving.value = false
    }
  }

  async function rollback(): Promise<void> {
    const bindingId = processView.value?.effectiveBinding?.id
    const targetVersionId = processView.value?.rollbackTargetVersionId
    if (
      saving.value ||
      !access.canPublishPrompts.value ||
      !processView.value?.canRollback ||
      !bindingId ||
      !targetVersionId
    )
      return
    saving.value = true
    try {
      await rollbackPromptBinding(api, bindingId, targetVersionId)
      await selectProcess(selectedProcessKey.value, true)
    } catch (cause) {
      error.value = classifyCustomerApiError(cause, 'Falha ao reverter o prompt.')
    } finally {
      saving.value = false
    }
  }

  function discardChanges(): void {
    hydrateEditor()
  }

  watch([() => scopeStore.scopeKey, () => access.canViewPrompts.value], () => void loadCatalog(), {
    immediate: true,
  })
  onBeforeUnmount(clear)

  return {
    catalog,
    selectedProcessKey,
    processView,
    editorPrompt,
    editorConfig,
    selectedAgentVersionId,
    loading,
    loadingProcess,
    saving,
    error,
    dirty,
    draft,
    selectProcess,
    ensureDraft,
    saveDraft,
    runAction,
    rollback,
    discardChanges,
    loadCatalog,
  }
}
