import { effectScope, nextTick, reactive, ref } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type {
  IntelligenceAuditEventView,
  IntelligenceAuditPage,
  IntelligenceObservationView,
} from '~/domain/customer-intelligence/audit-types'

const cleanupFns: Array<() => void> = []
const fetchRelationshipObservations = vi.fn()
const fetchIntelligenceAuditEvents = vi.fn()
const fetchIntelligenceObservation = vi.fn()
const revealIntelligenceObservation = vi.fn()
const customerScope = reactive({
  scopeKey: 'owner:client-a',
  clientAccountId: 'client-a',
})
const access = {
  canViewIntelligenceProfile: ref(true),
  canViewAudit: ref(true),
  clientScopeReady: ref(true),
}

vi.mock('vue', async () => {
  const actual = await vi.importActual<typeof import('vue')>('vue')
  return {
    ...actual,
    onBeforeUnmount: (handler: () => void) => {
      cleanupFns.push(handler)
    },
  }
})

vi.mock('~/stores/auth', () => ({
  useAuthStore: () => ({ accessToken: 'token' }),
}))

vi.mock('~/stores/customer-intelligence', () => ({
  useCustomerIntelligenceStore: () => customerScope,
}))

vi.mock('./useCustomerIntelligenceAccess', () => ({
  useCustomerIntelligenceAccess: () => access,
}))

vi.mock('~/utils/api-client', () => ({
  createApiRequest: () => vi.fn(),
}))

vi.mock('~/domain/customer-intelligence/observation-api', () => ({
  fetchRelationshipObservations: (...args: unknown[]) => fetchRelationshipObservations(...args),
}))

vi.mock('~/domain/customer-intelligence/audit-api', () => ({
  fetchIntelligenceAuditEvents: (...args: unknown[]) => fetchIntelligenceAuditEvents(...args),
  fetchIntelligenceObservation: (...args: unknown[]) => fetchIntelligenceObservation(...args),
  revealIntelligenceObservation: (...args: unknown[]) => revealIntelligenceObservation(...args),
}))

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, resolve, reject }
}

function observation(id: string, sensitivity = 'internal'): IntelligenceObservationView {
  return {
    id,
    sourceKey: 'erp',
    provenanceRef: `erp:customer:${id}`,
    sensitivity,
    purposeKey: 'customer_profile',
    retentionState: 'active',
    observedAt: '2026-07-23T10:00:00Z',
    snapshotFields: [{ label: 'status', displayValue: id, masked: false }],
  }
}

function auditEvent(id: string): IntelligenceAuditEventView {
  return {
    id,
    action: 'source.observation_ingested',
    entityType: 'source_observation',
    entityRef: `observation-${id}`,
    occurredAt: '2026-07-23T10:00:00Z',
    actor: { type: 'system' },
    observationRef: `observation-${id}`,
    canOpenObservation: true,
    canNavigate: false,
  }
}

function auditPage(id: string): IntelligenceAuditPage {
  return {
    items: [auditEvent(id)],
    nextCursor: '',
    filterOptions: {
      actions: [],
      entityTypes: [],
      statuses: [],
      provenances: [],
    },
  }
}

async function flushPromises(): Promise<void> {
  await nextTick()
  await Promise.resolve()
  await Promise.resolve()
}

describe('customer intelligence observation scope isolation', () => {
  beforeEach(() => {
    cleanupFns.length = 0
    customerScope.clientAccountId = 'client-a'
    customerScope.scopeKey = 'owner:client-a'
    access.canViewIntelligenceProfile.value = true
    access.canViewAudit.value = true
    access.clientScopeReady.value = true
    fetchRelationshipObservations.mockReset()
    fetchIntelligenceAuditEvents.mockReset()
    fetchIntelligenceObservation.mockReset()
    revealIntelligenceObservation.mockReset()
  })

  afterEach(() => {
    while (cleanupFns.length) cleanupFns.pop()?.()
  })

  it('limpa evidencias no mesmo tick e ignora refresh antigo quando o novo escopo falha', async () => {
    const lateClientA = deferred<IntelligenceObservationView[]>()
    fetchRelationshipObservations
      .mockResolvedValueOnce([observation('client-a')])
      .mockReturnValueOnce(lateClientA.promise)
      .mockRejectedValueOnce(new Error('client-b unavailable'))

    const { useRelationshipObservations } = await import('./useRelationshipObservations')
    const vueScope = effectScope()
    const state = vueScope.run(() => useRelationshipObservations(ref('relationship-1')))
    if (!state) throw new Error('Falha ao criar estado de observacoes.')
    await flushPromises()
    expect(state.items.value.map((item) => item.id)).toEqual(['client-a'])

    const oldRefresh = state.load()
    expect(state.loading.value).toBe(true)

    customerScope.clientAccountId = 'client-b'
    customerScope.scopeKey = 'owner:client-b'

    expect(state.items.value).toEqual([])
    expect(state.error.value).toBeNull()
    expect(state.loading.value).toBe(true)

    lateClientA.resolve([observation('client-a-late')])
    await oldRefresh
    await flushPromises()

    expect(state.items.value).toEqual([])
    expect(state.error.value?.message).toBe('client-b unavailable')
    expect(state.loading.value).toBe(false)
    vueScope.stop()
  })

  it('fecha o drawer, limpa auditoria e invalida detalhe antigo na troca de escopo', async () => {
    const lateDetail = deferred<IntelligenceObservationView>()
    fetchIntelligenceAuditEvents
      .mockResolvedValueOnce(auditPage('client-a'))
      .mockRejectedValueOnce(new Error('client-b audit unavailable'))
    fetchIntelligenceObservation.mockReturnValueOnce(lateDetail.promise)

    const { useIntelligenceAudit } = await import('./useIntelligenceAudit')
    const vueScope = effectScope()
    const state = vueScope.run(() => useIntelligenceAudit())
    if (!state) throw new Error('Falha ao criar estado de auditoria.')
    await flushPromises()
    expect(state.events.value.map((event) => event.id)).toEqual(['client-a'])

    const oldDetail = state.openObservation(state.events.value[0]!)
    expect(state.observationOpen.value).toBe(true)
    expect(state.loadingObservation.value).toBe(true)

    customerScope.clientAccountId = 'client-b'
    customerScope.scopeKey = 'owner:client-b'

    expect(state.events.value).toEqual([])
    expect(state.observation.value).toBeNull()
    expect(state.observationOpen.value).toBe(false)
    expect(state.loading.value).toBe(true)
    expect(state.loadingObservation.value).toBe(false)

    lateDetail.resolve(observation('client-a-late', 'personal'))
    await oldDetail
    await flushPromises()

    expect(state.events.value).toEqual([])
    expect(state.observation.value).toBeNull()
    expect(state.observationOpen.value).toBe(false)
    expect(state.error.value?.message).toBe('client-b audit unavailable')
    vueScope.stop()
  })

  it('descarta reveal tardio quando o escopo muda', async () => {
    const lateReveal = deferred<IntelligenceObservationView>()
    fetchIntelligenceAuditEvents
      .mockResolvedValueOnce(auditPage('client-a'))
      .mockRejectedValueOnce(new Error('client-b audit unavailable'))
    fetchIntelligenceObservation.mockResolvedValueOnce(
      observation('observation-client-a', 'personal'),
    )
    revealIntelligenceObservation.mockReturnValueOnce(lateReveal.promise)

    const { useIntelligenceAudit } = await import('./useIntelligenceAudit')
    const vueScope = effectScope()
    const state = vueScope.run(() => useIntelligenceAudit())
    if (!state) throw new Error('Falha ao criar estado de auditoria.')
    await flushPromises()
    await state.openObservation(state.events.value[0]!)

    const reveal = state.revealObservation('customer_support_investigation')
    expect(state.revealingObservation.value).toBe(true)

    customerScope.clientAccountId = 'client-b'
    customerScope.scopeKey = 'owner:client-b'

    expect(state.observation.value).toBeNull()
    expect(state.observationOpen.value).toBe(false)
    expect(state.revealingObservation.value).toBe(false)

    lateReveal.resolve({
      ...observation('observation-client-a', 'personal'),
      revealed: true,
      snapshotFields: [{ label: 'status', displayValue: 'segredo-client-a', masked: false }],
    })
    await reveal
    await flushPromises()

    expect(state.observation.value).toBeNull()
    expect(state.observationOpen.value).toBe(false)
    expect(state.revealingObservation.value).toBe(false)
    vueScope.stop()
  })
})
