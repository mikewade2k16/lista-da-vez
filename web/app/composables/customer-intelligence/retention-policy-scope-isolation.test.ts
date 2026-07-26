import { effectScope, nextTick, reactive, ref } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { RetentionPolicyVersion } from '~/domain/customer-intelligence/retention-policy-types'

const cleanupFns: Array<() => void> = []
const fetchRetentionPolicyVersions = vi.fn()
const createRetentionPolicyDraft = vi.fn()
const publishRetentionPolicyVersion = vi.fn()
const customerScope = reactive({
  scopeKey: 'owner:client-a',
  clientAccountId: 'client-a',
})
const access = {
  canViewSources: ref(true),
  canManageSources: ref(true),
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

vi.mock('~/domain/customer-intelligence/retention-policy-api', () => ({
  fetchRetentionPolicyVersions: (...args: unknown[]) => fetchRetentionPolicyVersions(...args),
  createRetentionPolicyDraft: (...args: unknown[]) => createRetentionPolicyDraft(...args),
  publishRetentionPolicyVersion: (...args: unknown[]) => publishRetentionPolicyVersion(...args),
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

function policy(
  id: string,
  clientMarker: string,
  overrides: Partial<RetentionPolicyVersion> = {},
): RetentionPolicyVersion {
  return {
    id,
    accountId: 'owner',
    policyKey: `customer_profile.${clientMarker}`,
    version: 1,
    status: 'draft',
    snapshotTtlSeconds: 7_776_000,
    onExpiry: 'tombstone',
    legalHoldBehavior: 'preserve',
    blockReingestion: true,
    revision: 1,
    createdAt: '2026-07-23T12:00:00Z',
    ...overrides,
  }
}

async function flushPromises(): Promise<void> {
  await nextTick()
  await Promise.resolve()
  await Promise.resolve()
}

describe('retention policy scope isolation and governance', () => {
  beforeEach(() => {
    cleanupFns.length = 0
    customerScope.scopeKey = 'owner:client-a'
    customerScope.clientAccountId = 'client-a'
    access.canViewSources.value = true
    access.canManageSources.value = true
    access.clientScopeReady.value = true
    fetchRetentionPolicyVersions.mockReset()
    createRetentionPolicyDraft.mockReset()
    publishRetentionPolicyVersion.mockReset()
  })

  afterEach(() => {
    while (cleanupFns.length) cleanupFns.pop()?.()
  })

  it('clears the previous client synchronously, aborts and ignores its late response', async () => {
    const lateClientA = deferred<RetentionPolicyVersion[]>()
    fetchRetentionPolicyVersions
      .mockResolvedValueOnce([policy('client-a-v1', 'client_a')])
      .mockReturnValueOnce(lateClientA.promise)
      .mockRejectedValueOnce(new Error('client-b policies unavailable'))

    const { useRetentionPolicies } = await import('./useRetentionPolicies')
    const vueScope = effectScope()
    const state = vueScope.run(() => useRetentionPolicies())
    if (!state) throw new Error('Falha ao criar estado de retention policies.')
    await flushPromises()
    expect(state.policies.value.map((item) => item.id)).toEqual(['client-a-v1'])

    const oldRefresh = state.load()
    const oldSignal = fetchRetentionPolicyVersions.mock.calls[1]?.[1] as AbortSignal
    customerScope.clientAccountId = 'client-b'
    customerScope.scopeKey = 'owner:client-b'

    expect(oldSignal.aborted).toBe(true)
    expect(state.policies.value).toEqual([])
    expect(state.selectedPolicyKey.value).toBe('')
    expect(state.error.value).toBeNull()
    expect(state.loading.value).toBe(true)

    lateClientA.resolve([policy('client-a-late', 'client_a')])
    await oldRefresh
    await flushPromises()

    expect(state.policies.value).toEqual([])
    expect(state.error.value?.message).toBe('client-b policies unavailable')
    expect(state.loading.value).toBe(false)
    vueScope.stop()
  })

  it('creates a draft without publishing and rehydrates the authoritative list', async () => {
    const initial = policy('published-v1', 'default', {
      policyKey: 'customer_profile.default',
      status: 'published',
      revision: 2,
    })
    const created = policy('draft-v2', 'default', {
      policyKey: 'customer_profile.default',
      version: 2,
    })
    fetchRetentionPolicyVersions
      .mockResolvedValueOnce([initial])
      .mockResolvedValueOnce([created, initial])
    createRetentionPolicyDraft.mockResolvedValueOnce(created)

    const { useRetentionPolicies } = await import('./useRetentionPolicies')
    const vueScope = effectScope()
    const state = vueScope.run(() => useRetentionPolicies())
    if (!state) throw new Error('Falha ao criar estado de retention policies.')
    await flushPromises()

    await expect(
      state.createDraft({
        policyKey: 'customer_profile.default',
        snapshotTtlSeconds: 7_776_000,
        onExpiry: 'tombstone',
      }),
    ).resolves.toBe(true)

    expect(createRetentionPolicyDraft).toHaveBeenCalledTimes(1)
    expect(publishRetentionPolicyVersion).not.toHaveBeenCalled()
    expect(fetchRetentionPolicyVersions).toHaveBeenCalledTimes(2)
    expect(state.selectedDraft.value?.id).toBe('draft-v2')
    vueScope.stop()
  })

  it('publishes only a selected draft with catalog reason and aborts on scope change', async () => {
    const draft = policy('draft-v2', 'default', {
      policyKey: 'customer_profile.default',
      version: 2,
      revision: 3,
    })
    const latePublish = deferred<RetentionPolicyVersion>()
    fetchRetentionPolicyVersions
      .mockResolvedValueOnce([draft])
      .mockResolvedValueOnce([policy('client-b-v1', 'client_b')])
    publishRetentionPolicyVersion.mockReturnValueOnce(latePublish.promise)

    const { useRetentionPolicies } = await import('./useRetentionPolicies')
    const vueScope = effectScope()
    const state = vueScope.run(() => useRetentionPolicies())
    if (!state) throw new Error('Falha ao criar estado de retention policies.')
    await flushPromises()

    await expect(
      state.publishDraft(draft, {
        expectedRevision: 3,
        reasonCode: 'legal_review_approved',
        approvalReference: 'invalid approval with spaces',
      }),
    ).resolves.toBe(false)
    expect(publishRetentionPolicyVersion).not.toHaveBeenCalled()
    expect(state.error.value?.reasonCode).toBe('retention_policy_publication_metadata_invalid')

    const oldPublish = state.publishDraft(draft, {
      expectedRevision: 3,
      reasonCode: 'legal_review_approved',
      approvalReference: 'LEGAL-RETENTION-3',
    })
    const mutationSignal = publishRetentionPolicyVersion.mock.calls[0]?.[3] as AbortSignal
    customerScope.clientAccountId = 'client-b'
    customerScope.scopeKey = 'owner:client-b'

    expect(mutationSignal.aborted).toBe(true)
    expect(state.policies.value).toEqual([])
    expect(state.savingAction.value).toBe('')

    latePublish.resolve({ ...draft, status: 'published', revision: 4 })
    await expect(oldPublish).resolves.toBe(false)
    await flushPromises()

    expect(state.policies.value.map((item) => item.id)).toEqual(['client-b-v1'])
    expect(fetchRetentionPolicyVersions).toHaveBeenCalledTimes(2)
    vueScope.stop()
  })
})
