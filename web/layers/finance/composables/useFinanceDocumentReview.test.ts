import { effectScope, reactive } from 'vue'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import type { FinanceDocumentReview } from '../types/documents'
import { emptyFiscalProfile, emptyFiscalTemplate } from '../types/fiscal'
import { useFinanceDocumentReview } from './useFinanceDocumentReview'
const f = vi.hoisted(() => ({
  account: null as unknown as { activeAccountId: string },
  store: { review: vi.fn(), approveReview: vi.fn() },
  confirm: vi.fn(),
  download: vi.fn(),
}))
vi.mock('~/utils/report-export', () => ({
  downloadText: (...args: unknown[]) => f.download(...args),
}))
vi.mock('~/stores/finance-documents', () => ({ useFinanceDocumentsStore: () => f.store }))
vi.mock('~/stores/auth', () => ({ useAuthStore: () => ({ user: { id: 'user' } }) }))
vi.mock('../../core/stores/account', () => ({ useCoreAccountStore: () => f.account }))
vi.mock('../utils/fiscal-confirm', () => ({
  confirmFiscalAction: (...args: unknown[]) => f.confirm(...args),
}))
vi.mock('~/utils/api-client', () => ({ getApiErrorMessage: () => 'Falha' }))
vi.mock('vue', async (original) => ({
  ...(await original<typeof import('vue')>()),
  onBeforeUnmount: () => {},
}))
const scopes: ReturnType<typeof effectScope>[] = []
const snapshot = () =>
  ({
    hash: 'hash',
    draftVersion: 'v1',
    profileVersion: 'p1',
    ready: true,
    approvalId: '',
    approvedAt: '',
  }) as FinanceDocumentReview
function editor() {
  const scope = effectScope()
  scopes.push(scope)
  return scope.run(() =>
    useFinanceDocumentReview(
      () => 'draft',
      () => 'v1',
      () => false,
    ),
  )!
}
beforeEach(() => {
  vi.clearAllMocks()
  f.account = reactive({ activeAccountId: 'a' })
  f.store.review.mockResolvedValue(snapshot())
  f.confirm.mockResolvedValue(true)
})
afterEach(() => scopes.splice(0).forEach((s) => s.stop()))
it('loads authoritative review and persists explicit confirmation', async () => {
  const state = editor()
  await state.toggle(true)
  f.store.approveReview.mockResolvedValue({ ...snapshot(), approvalId: 'approved' })
  await state.approve(true)
  expect(state.review.value?.approvalId).toBe('approved')
  expect(f.store.approveReview.mock.calls[0]?.slice(0, 2)).toEqual(['a', 'draft'])
})
it('does not approve when the user cancels', async () => {
  const state = editor()
  await state.toggle(true)
  f.confirm.mockResolvedValue(false)
  await state.approve(true)
  expect(f.store.approveReview).not.toHaveBeenCalled()
})
it('drops confirmation after an account switch while the modal was open', async () => {
  const state = editor()
  await state.toggle(true)
  let resolve!: (v: boolean) => void
  f.confirm.mockImplementation(
    () =>
      new Promise<boolean>((r) => {
        resolve = r
      }),
  )
  const pending = state.approve(true)
  f.account.activeAccountId = 'b'
  await vi.waitFor(() => expect(f.store.review).toHaveBeenCalledTimes(2))
  resolve(true)
  await pending
  expect(f.store.approveReview).not.toHaveBeenCalled()
})
it('preserves the review and shows save errors', async () => {
  const state = editor()
  await state.toggle(true)
  f.store.approveReview.mockRejectedValue(new Error('network'))
  await state.approve(true)
  expect(state.review.value?.hash).toBe('hash')
  expect(state.error.value).toBe('Falha')
})

it('downloads a freshly read version and never treats review as emission authorization', async () => {
  const state = editor()
  await state.toggle(true)
  const data = {
    ...snapshot(),
    draftVersion: 'v2',
    issuer: emptyFiscalProfile(),
    missingFields: ['Tomador'],
    fields: {
      documentType: 'comprovante' as const,
      amount: 12,
      paymentDate: '',
      payerDocument: '',
      payeeDocument: '',
      payerName: '',
      payeeName: '',
      customerName: '',
      customerDocument: '',
      competenceDate: '',
      service: emptyFiscalTemplate().fields,
      templateId: '',
    },
  }
  f.store.review.mockResolvedValue(data)
  await state.downloadPreparation()
  expect(f.store.review).toHaveBeenCalledTimes(2)
  const payload = JSON.parse(f.download.mock.calls[0]![0] as string)
  expect(payload.draftVersion).toBe('v2')
  expect(payload.emissionAuthorized).toBe(false)
  expect(payload.status).toBe('ready-for-review')
})
it('does not download a late response from a previous account', async () => {
  const state = editor()
  await state.toggle(true)
  let resolve!: (data: FinanceDocumentReview) => void
  f.store.review.mockImplementationOnce(
    () =>
      new Promise<FinanceDocumentReview>((r) => {
        resolve = r
      }),
  )
  const pending = state.downloadPreparation()
  f.account.activeAccountId = 'b'
  await vi.waitFor(() => expect(f.store.review).toHaveBeenCalledTimes(3))
  resolve(snapshot())
  await pending
  expect(f.download).not.toHaveBeenCalled()
})
