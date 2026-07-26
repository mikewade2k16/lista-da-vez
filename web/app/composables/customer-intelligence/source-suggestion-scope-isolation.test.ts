import { effectScope, nextTick, reactive, ref } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { SourceSuggestionView } from '~/domain/customer-intelligence/source-suggestion-types'

const cleanupFns: Array<() => void> = []
const fetchSourceSuggestions = vi.fn()
const reviewSourceSuggestion = vi.fn()
const customerScope = reactive({
  scopeKey: 'owner:client-a',
  clientAccountId: 'client-a',
})
const access = {
  canViewIntelligenceProfile: ref(true),
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

vi.mock('~/domain/customer-intelligence/source-suggestion-api', () => ({
  fetchSourceSuggestions: (...args: unknown[]) => fetchSourceSuggestions(...args),
  reviewSourceSuggestion: (...args: unknown[]) => reviewSourceSuggestion(...args),
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

function suggestion(id: string): SourceSuggestionView {
  return {
    id,
    relationshipId: 'relationship-1',
    sourceKey: 'erp',
    gapCodes: ['purchase_history_missing'],
    rationaleCode: 'profile_gap',
    rationale: `Racional ${id}`,
    confidence: 0.8,
    status: 'proposed',
    expiresAt: '2099-08-01T12:00:00Z',
    createdAt: '2026-07-23T12:00:00Z',
    allowedActions: ['accepted', 'rejected'],
  }
}

async function flushPromises(): Promise<void> {
  await nextTick()
  await Promise.resolve()
  await Promise.resolve()
}

describe('customer intelligence source suggestion scope isolation', () => {
  beforeEach(() => {
    cleanupFns.length = 0
    customerScope.scopeKey = 'owner:client-a'
    customerScope.clientAccountId = 'client-a'
    access.canViewIntelligenceProfile.value = true
    access.canManageSources.value = true
    access.clientScopeReady.value = true
    fetchSourceSuggestions.mockReset()
    reviewSourceSuggestion.mockReset()
  })

  afterEach(() => {
    while (cleanupFns.length) cleanupFns.pop()?.()
  })

  it('clears the previous client in the same tick and ignores its late response', async () => {
    const lateClientA = deferred<SourceSuggestionView[]>()
    fetchSourceSuggestions
      .mockResolvedValueOnce([suggestion('client-a')])
      .mockReturnValueOnce(lateClientA.promise)
      .mockRejectedValueOnce(new Error('client-b unavailable'))

    const { useSourceSuggestions } = await import('./useSourceSuggestions')
    const vueScope = effectScope()
    const state = vueScope.run(() => useSourceSuggestions(ref('relationship-1')))
    if (!state) throw new Error('Falha ao criar estado de sugestoes de fontes.')
    await flushPromises()
    expect(state.items.value.map((item) => item.id)).toEqual(['client-a'])

    const oldRefresh = state.load()
    customerScope.clientAccountId = 'client-b'
    customerScope.scopeKey = 'owner:client-b'

    expect(state.items.value).toEqual([])
    expect(state.error.value).toBeNull()
    expect(state.loading.value).toBe(true)

    lateClientA.resolve([suggestion('client-a-late')])
    await oldRefresh
    await flushPromises()

    expect(state.items.value).toEqual([])
    expect(state.error.value?.message).toBe('client-b unavailable')
    expect(state.loading.value).toBe(false)
    vueScope.stop()
  })

  it('requires source management plus a registered reason and cancels review on scope change', async () => {
    const lateReview = deferred<SourceSuggestionView>()
    fetchSourceSuggestions
      .mockResolvedValueOnce([suggestion('client-a')])
      .mockResolvedValueOnce([suggestion('client-b')])
    reviewSourceSuggestion.mockReturnValueOnce(lateReview.promise)

    const { useSourceSuggestions } = await import('./useSourceSuggestions')
    const vueScope = effectScope()
    const state = vueScope.run(() => useSourceSuggestions(ref('relationship-1')))
    if (!state) throw new Error('Falha ao criar estado de sugestoes de fontes.')
    await flushPromises()
    const item = state.items.value[0]!

    access.canManageSources.value = false
    await expect(state.review(item, 'accepted', 'source_relevant')).resolves.toBe(false)
    expect(reviewSourceSuggestion).not.toHaveBeenCalled()

    access.canManageSources.value = true
    await expect(state.review(item, 'accepted', 'source_not_relevant')).resolves.toBe(false)
    expect(reviewSourceSuggestion).not.toHaveBeenCalled()
    expect(state.error.value?.reasonCode).toBe('source_suggestion_review_reason_invalid')

    const oldReview = state.review(item, 'accepted', 'source_relevant')
    expect(state.reviewingId.value).toBe('client-a')

    customerScope.clientAccountId = 'client-b'
    customerScope.scopeKey = 'owner:client-b'
    expect(state.items.value).toEqual([])
    expect(state.reviewingId.value).toBe('')

    lateReview.resolve({ ...item, status: 'accepted', allowedActions: [] })
    await expect(oldReview).resolves.toBe(false)
    await flushPromises()

    expect(state.items.value.map((entry) => entry.id)).toEqual(['client-b'])
    expect(fetchSourceSuggestions).toHaveBeenCalledTimes(2)
    vueScope.stop()
  })
})
