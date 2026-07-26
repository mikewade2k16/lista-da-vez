import { computed, onBeforeUnmount, ref, watch } from 'vue'
import {
  fetchCustomerDataControlState,
  updateCustomerDataCapability,
  updateCustomerDataWriter,
} from '~/domain/customer-data/control-plane-api'
import {
  fetchIntelligenceCapability,
  updateIntelligenceCapability,
} from '~/domain/customer-intelligence/control-plane-api'
import {
  CUSTOMER_DATA_CAPABILITY_DEFINITIONS,
  INTELLIGENCE_CAPABILITY_DEFINITIONS,
  type CustomerDataCapabilityKey,
  type CustomerDataCapabilityMode,
  type CustomerDataCapabilityState,
  type CustomerDataControlState,
  type CustomerDataWriterKey,
  type CustomerDataWriterMode,
  type CustomerDataWriterState,
  type IntelligenceCapabilityKey,
  type IntelligenceCapabilityView,
} from '~/domain/customer-intelligence/control-plane-types'
import {
  classifyCustomerApiError,
  type CustomerApiErrorState,
} from '~/domain/customer-intelligence/api-error'
import { useAuthStore } from '~/stores/auth'
import { useCustomerIntelligenceStore } from '~/stores/customer-intelligence'
import { createApiRequest } from '~/utils/api-client'
import { useCustomerIntelligenceAccess } from './useCustomerIntelligenceAccess'

interface CustomerDataWriterSaveOptions {
  reason: string
  watermark?: string
  sourceChecksum?: string
  targetChecksum?: string
}

function controlValidationError(message: string, reasonCode: string): CustomerApiErrorState {
  return {
    kind: 'error',
    message,
    reasonCode,
    statusCode: 422,
  }
}

function createIdempotencyKey(prefix: string): string {
  const randomPart =
    typeof globalThis.crypto?.randomUUID === 'function'
      ? globalThis.crypto.randomUUID()
      : `${Date.now()}-${Math.random().toString(36).slice(2)}`
  return `${prefix}:${randomPart}`
}

function optionalText(value?: string): string | undefined {
  const normalized = String(value ?? '').trim()
  return normalized || undefined
}

export function useCustomerControlPlane() {
  const auth = useAuthStore()
  const scope = useCustomerIntelligenceStore()
  const access = useCustomerIntelligenceAccess()
  const api = createApiRequest(useRuntimeConfig(), () => auth.accessToken)

  const capabilities = ref<IntelligenceCapabilityView[]>([])
  const draftModes = ref<Partial<Record<IntelligenceCapabilityKey, string>>>({})
  const canaryAllocationPercent = ref(5)
  const loading = ref(false)
  const savingKey = ref<IntelligenceCapabilityKey | ''>('')
  const error = ref<CustomerApiErrorState | null>(null)

  const customerDataState = ref<CustomerDataControlState | null>(null)
  const customerDataCapabilityDrafts = ref<
    Partial<Record<CustomerDataCapabilityKey, CustomerDataCapabilityMode>>
  >({})
  const customerDataWriterDrafts = ref<
    Partial<Record<CustomerDataWriterKey, CustomerDataWriterMode>>
  >({})
  const customerDataLoading = ref(false)
  const savingCustomerDataCapabilityKey = ref<CustomerDataCapabilityKey | ''>('')
  const savingCustomerDataWriterKey = ref<CustomerDataWriterKey | ''>('')
  const customerDataError = ref<CustomerApiErrorState | null>(null)

  let intelligenceController: AbortController | null = null
  let customerDataController: AbortController | null = null
  let intelligenceGeneration = 0
  let customerDataGeneration = 0

  const canReadIntelligence = computed(
    () =>
      access.hasCustomerIntelligenceModule.value &&
      access.canViewIntelligenceProfile.value &&
      access.clientScopeReady.value,
  )
  const canReadCustomerDataControls = computed(
    () =>
      access.hasCustomerDataModule.value &&
      access.canManageCustomerDataCapabilities.value &&
      access.clientScopeReady.value,
  )

  function clearIntelligence(): void {
    intelligenceController?.abort()
    intelligenceController = null
    intelligenceGeneration += 1
    capabilities.value = []
    draftModes.value = {}
    canaryAllocationPercent.value = 5
    loading.value = false
    savingKey.value = ''
    error.value = null
  }

  function clearCustomerData(): void {
    customerDataController?.abort()
    customerDataController = null
    customerDataGeneration += 1
    customerDataState.value = null
    customerDataCapabilityDrafts.value = {}
    customerDataWriterDrafts.value = {}
    customerDataLoading.value = false
    savingCustomerDataCapabilityKey.value = ''
    savingCustomerDataWriterKey.value = ''
    customerDataError.value = null
  }

  function clear(): void {
    clearIntelligence()
    clearCustomerData()
  }

  async function loadIntelligence(): Promise<void> {
    if (!canReadIntelligence.value) {
      clearIntelligence()
      return
    }
    intelligenceController?.abort()
    const request = new AbortController()
    intelligenceController = request
    const current = ++intelligenceGeneration
    loading.value = true
    error.value = null
    try {
      const response = await Promise.all(
        INTELLIGENCE_CAPABILITY_DEFINITIONS.map((definition) =>
          fetchIntelligenceCapability(api, definition.key, scope.clientAccountId, request.signal),
        ),
      )
      if (request.signal.aborted || current !== intelligenceGeneration) return
      const nextDrafts: Partial<Record<IntelligenceCapabilityKey, string>> = {}
      response.forEach((item) => {
        nextDrafts[item.key] = item.mode
        if (item.key === 'customer_intelligence.runtime') {
          const configured = Number(item.config?.canaryAllocationPercent)
          canaryAllocationPercent.value =
            Number.isInteger(configured) && configured >= 1 && configured <= 100 ? configured : 5
        }
      })
      capabilities.value = response
      draftModes.value = nextDrafts
    } catch (cause) {
      if (request.signal.aborted || current !== intelligenceGeneration) return
      error.value = classifyCustomerApiError(cause, 'Capabilities indisponiveis.')
    } finally {
      if (current === intelligenceGeneration) loading.value = false
    }
  }

  async function loadCustomerData(): Promise<void> {
    if (!canReadCustomerDataControls.value) {
      clearCustomerData()
      return
    }
    customerDataController?.abort()
    const request = new AbortController()
    customerDataController = request
    const current = ++customerDataGeneration
    customerDataLoading.value = true
    customerDataError.value = null
    try {
      const response = await fetchCustomerDataControlState(
        api,
        scope.clientAccountId,
        request.signal,
      )
      if (request.signal.aborted || current !== customerDataGeneration) return
      const capabilityDrafts: Partial<
        Record<CustomerDataCapabilityKey, CustomerDataCapabilityMode>
      > = {}
      response.capabilities.forEach((item) => {
        capabilityDrafts[item.capabilityKey] = item.mode
      })
      const writerDrafts: Partial<Record<CustomerDataWriterKey, CustomerDataWriterMode>> = {}
      response.writerStates.forEach((item) => {
        writerDrafts[item.entityKey] = item.mode
      })
      customerDataState.value = response
      customerDataCapabilityDrafts.value = capabilityDrafts
      customerDataWriterDrafts.value = writerDrafts
    } catch (cause) {
      if (request.signal.aborted || current !== customerDataGeneration) return
      customerDataError.value = classifyCustomerApiError(
        cause,
        'Estados do Customer Data indisponiveis.',
      )
    } finally {
      if (current === customerDataGeneration) customerDataLoading.value = false
    }
  }

  async function load(): Promise<void> {
    await Promise.all([loadIntelligence(), loadCustomerData()])
  }

  function capability(key: IntelligenceCapabilityKey): IntelligenceCapabilityView | null {
    return capabilities.value.find((item) => item.key === key) ?? null
  }

  function customerDataCapability(
    key: CustomerDataCapabilityKey,
  ): CustomerDataCapabilityState | null {
    return customerDataState.value?.capabilities.find((item) => item.capabilityKey === key) ?? null
  }

  function customerDataWriter(key: CustomerDataWriterKey): CustomerDataWriterState | null {
    return customerDataState.value?.writerStates.find((item) => item.entityKey === key) ?? null
  }

  function setDraftMode(key: IntelligenceCapabilityKey, mode: string): void {
    const definition = INTELLIGENCE_CAPABILITY_DEFINITIONS.find((item) => item.key === key)
    if (!definition?.modes.some((allowed) => allowed === mode)) return
    draftModes.value = { ...draftModes.value, [key]: mode }
  }

  function setCanaryAllocationPercent(value: number): void {
    const normalized = Math.trunc(Number(value))
    if (normalized >= 1 && normalized <= 100) {
      canaryAllocationPercent.value = normalized
    }
  }

  function intelligenceCapabilityDirty(key: IntelligenceCapabilityKey): boolean {
    const current = capability(key)
    const draftMode = draftModes.value[key]
    if (!current || !draftMode) return false
    if (draftMode !== current.mode) return true
    if (key !== 'customer_intelligence.runtime' || draftMode !== 'canary') {
      return false
    }
    return Number(current.config?.canaryAllocationPercent) !== canaryAllocationPercent.value
  }

  function setCustomerDataCapabilityDraft(key: CustomerDataCapabilityKey, mode: string): void {
    const definition = CUSTOMER_DATA_CAPABILITY_DEFINITIONS.find((item) => item.key === key)
    if (!definition?.modes.some((allowed) => allowed === mode)) return
    customerDataCapabilityDrafts.value = {
      ...customerDataCapabilityDrafts.value,
      [key]: mode as CustomerDataCapabilityMode,
    }
  }

  function setCustomerDataWriterDraft(key: CustomerDataWriterKey, mode: string): void {
    if (!(['legacy', 'shadow', 'new'] as const).some((allowed) => allowed === mode)) {
      return
    }
    customerDataWriterDrafts.value = {
      ...customerDataWriterDrafts.value,
      [key]: mode as CustomerDataWriterMode,
    }
  }

  function customerDataCapabilityModes(
    key: CustomerDataCapabilityKey,
  ): CustomerDataCapabilityMode[] {
    const current = customerDataCapability(key)?.mode ?? 'off'
    if (key !== 'core' && current === 'off') return ['off', 'shadow']
    return ['off', 'shadow', 'on']
  }

  function customerDataWriterModes(key: CustomerDataWriterKey): CustomerDataWriterMode[] {
    const current = customerDataWriter(key)?.mode ?? 'legacy'
    if (current === 'legacy') return ['legacy', 'shadow']
    if (current === 'new') return ['new', 'shadow']
    return ['legacy', 'shadow', 'new']
  }

  async function save(key: IntelligenceCapabilityKey): Promise<boolean> {
    const current = capability(key)
    const mode = draftModes.value[key]
    const definition = INTELLIGENCE_CAPABILITY_DEFINITIONS.find((item) => item.key === key)
    if (
      !current ||
      !mode ||
      !definition?.modes.some((allowed) => allowed === mode) ||
      !access.canManageIntelligenceProfile.value
    ) {
      return false
    }
    const clientAccountId = scope.clientAccountId
    savingKey.value = key
    error.value = null
    try {
      const config =
        key === 'customer_intelligence.runtime'
          ? {
              ...(mode === 'canary'
                ? { canaryAllocationPercent: canaryAllocationPercent.value }
                : {}),
              ...(typeof current.config?.bucketKeyVersion === 'string'
                ? { bucketKeyVersion: current.config.bucketKeyVersion }
                : {}),
            }
          : (current.config ?? {})
      await updateIntelligenceCapability(api, key, {
        clientAccountId,
        scopeKey: current.scopeKey || '',
        mode,
        config,
        expectedRevision: current.revision,
      })
      if (scope.clientAccountId === clientAccountId) await loadIntelligence()
      return true
    } catch (cause) {
      error.value = classifyCustomerApiError(cause, 'Nao foi possivel salvar a capability.')
      return false
    } finally {
      savingKey.value = ''
    }
  }

  async function saveCustomerDataCapability(
    key: CustomerDataCapabilityKey,
    reason: string,
  ): Promise<boolean> {
    const current = customerDataCapability(key)
    const mode = customerDataCapabilityDrafts.value[key]
    const normalizedReason = reason.trim()
    if (!current || !mode || !access.canManageCustomerDataCapabilities.value) {
      return false
    }
    if (!normalizedReason || normalizedReason.length > 1000) {
      customerDataError.value = controlValidationError(
        'Informe um motivo de ate 1000 caracteres para registrar na auditoria.',
        'control_state_reason_required',
      )
      return false
    }
    const clientAccountId = scope.clientAccountId
    savingCustomerDataCapabilityKey.value = key
    customerDataError.value = null
    try {
      await updateCustomerDataCapability(api, clientAccountId, key, {
        mode,
        expectedRevision: current.revision,
        idempotencyKey: createIdempotencyKey(`capability:${key}`),
        reason: normalizedReason,
      })
      if (scope.clientAccountId === clientAccountId) await loadCustomerData()
      return true
    } catch (cause) {
      customerDataError.value = classifyCustomerApiError(
        cause,
        'Nao foi possivel alterar a capability do Customer Data.',
      )
      return false
    } finally {
      savingCustomerDataCapabilityKey.value = ''
    }
  }

  async function saveCustomerDataWriter(
    key: CustomerDataWriterKey,
    options: CustomerDataWriterSaveOptions,
  ): Promise<boolean> {
    const current = customerDataWriter(key)
    const mode = customerDataWriterDrafts.value[key]
    const reason = options.reason.trim()
    if (!current || !mode || !access.canManageCustomerDataCapabilities.value) {
      return false
    }
    if (!reason || reason.length > 1000) {
      customerDataError.value = controlValidationError(
        'Informe um motivo de ate 1000 caracteres para registrar na auditoria.',
        'control_state_reason_required',
      )
      return false
    }
    const sourceChecksum = optionalText(options.sourceChecksum) ?? current.sourceChecksum
    const targetChecksum = optionalText(options.targetChecksum) ?? current.targetChecksum
    if (
      mode === 'new' &&
      (!sourceChecksum || !targetChecksum || sourceChecksum !== targetChecksum)
    ) {
      customerDataError.value = controlValidationError(
        'O cutover para new exige checksums de origem e destino preenchidos e iguais.',
        'writer_checksums_must_match',
      )
      return false
    }
    const clientAccountId = scope.clientAccountId
    savingCustomerDataWriterKey.value = key
    customerDataError.value = null
    try {
      await updateCustomerDataWriter(api, clientAccountId, key, {
        mode,
        watermark: optionalText(options.watermark) ?? current.watermark,
        sourceChecksum,
        targetChecksum,
        expectedRevision: current.revision,
        idempotencyKey: createIdempotencyKey(`writer:${key}`),
        reason,
      })
      if (scope.clientAccountId === clientAccountId) await loadCustomerData()
      return true
    } catch (cause) {
      customerDataError.value = classifyCustomerApiError(
        cause,
        'Nao foi possivel alterar o writer do Customer Data.',
      )
      return false
    } finally {
      savingCustomerDataWriterKey.value = ''
    }
  }

  watch(
    [() => scope.scopeKey, canReadIntelligence, canReadCustomerDataControls],
    () => void load(),
    { immediate: true },
  )
  onBeforeUnmount(clear)

  return {
    access,
    capabilities,
    draftModes,
    canaryAllocationPercent,
    loading,
    savingKey,
    error,
    customerDataState,
    customerDataCapabilityDrafts,
    customerDataWriterDrafts,
    customerDataLoading,
    savingCustomerDataCapabilityKey,
    savingCustomerDataWriterKey,
    customerDataError,
    capability,
    customerDataCapability,
    customerDataWriter,
    customerDataCapabilityModes,
    customerDataWriterModes,
    setDraftMode,
    setCanaryAllocationPercent,
    intelligenceCapabilityDirty,
    setCustomerDataCapabilityDraft,
    setCustomerDataWriterDraft,
    save,
    saveCustomerDataCapability,
    saveCustomerDataWriter,
    load,
    loadIntelligence,
    loadCustomerData,
    clear,
  }
}
