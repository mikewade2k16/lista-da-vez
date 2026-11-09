import { effectScope, nextTick, reactive } from 'vue'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { emptyFiscalTemplate } from '../types/fiscal'
import type { FinanceDocumentDraft } from '../types/documents'
import { useFinanceDocuments } from './useFinanceDocuments'
const f = vi.hoisted(() => ({
  account: null as unknown as { activeAccountId: string },
  store: {
    list: vi.fn(),
    get: vi.fn(),
    prepare: vi.fn(),
    extract: vi.fn(),
    save: vi.fn(),
    reidentifyParties: vi.fn(),
  },
  fiscal: { listTemplates: vi.fn() },
}))
vi.mock('vue', async (original) => ({
  ...(await original<typeof import('vue')>()),
  onMounted: (fn: () => void) => fn(),
  onBeforeUnmount: () => {},
}))
vi.mock('~/stores/auth', () => ({ useAuthStore: () => ({ user: { id: 'user' } }) }))
vi.mock('../../core/stores/account', () => ({ useCoreAccountStore: () => f.account }))
vi.mock('~/stores/finance-documents', () => ({ useFinanceDocumentsStore: () => f.store }))
vi.mock('~/stores/fiscal-profile', () => ({ useFiscalProfileStore: () => f.fiscal }))
vi.mock('~/utils/api-client', () => ({ getApiErrorMessage: () => 'Falha' }))
const scopes: ReturnType<typeof effectScope>[] = []
function doc(id: string): FinanceDocumentDraft {
  return {
    id,
    sourceName: id,
    extractedText: 'OCR',
    updatedAt: 'v1',
    warnings: [],
    missingFields: [],
    fields: {
      documentType: 'comprovante',
      amount: 22000,
      paymentDate: '2026-10-08',
      payerDocument: '',
      payeeDocument: '',
      payerName: 'QA pagador',
      payeeName: 'QA recebedor',
      customerName: '',
      customerDocument: '',
      competenceDate: '',
      service: emptyFiscalTemplate().fields,
      templateId: '',
    },
  }
}
function editor() {
  const scope = effectScope()
  scopes.push(scope)
  return scope.run(useFinanceDocuments)!
}
beforeEach(() => {
  vi.clearAllMocks()
  f.account = reactive({ activeAccountId: 'a' })
  f.store.list.mockResolvedValue([doc('a')])
  f.fiscal.listTemplates.mockResolvedValue([])
})
afterEach(() => scopes.splice(0).forEach((s) => s.stop()))
it('loads persisted fields without assigning receipt parties to fiscal roles', async () => {
  const state = editor()
  await vi.waitFor(() => expect(state.current.value?.id).toBe('a'))
  expect(state.fields.value?.amount).toBe(22000)
  expect(state.fields.value?.customerName).toBe('')
  expect(state.fields.value?.competenceDate).toBe('')
})
it('drops a late upload response after switching accounts', async () => {
  const state = editor()
  await vi.waitFor(() => expect(state.current.value).not.toBeNull())
  let resolve!: (d: FinanceDocumentDraft) => void
  f.store.extract.mockImplementation(
    () =>
      new Promise<FinanceDocumentDraft>((r) => {
        resolve = r
      }),
  )
  state.file.value = { type: 'image/jpeg', size: 100 } as File
  const pending = state.extract()
  f.account.activeAccountId = 'b'
  f.store.list.mockResolvedValue([doc('b')])
  await nextTick()
  await vi.waitFor(() => expect(state.current.value?.id).toBe('b'))
  resolve(doc('old'))
  await pending
  expect(state.current.value?.id).toBe('b')
})
it('retains unsaved edits after failure and sends the authoritative version', async () => {
  const state = editor()
  await vi.waitFor(() => expect(state.current.value).not.toBeNull())
  state.fields.value!.customerName = 'Revisado'
  f.store.save.mockRejectedValue(new Error('network'))
  await state.save()
  expect(state.dirty.value).toBe(true)
  expect(state.fields.value!.customerName).toBe('Revisado')
  expect(f.store.save.mock.calls[0]?.[1].updatedAt).toBe('v1')
  expect(state.error.value).toBe('Falha')
})

it('accepts PDF with generic MIME and rejects unsupported or oversized files', async () => {
  const state = editor()
  await vi.waitFor(() => expect(state.current.value).not.toBeNull())
  f.store.extract.mockResolvedValue(doc('pdf'))
  state.file.value = { name: 'statement.PDF', type: 'application/octet-stream', size: 100 } as File
  await state.extract()
  expect(state.current.value?.id).toBe('pdf')
  state.file.value = {
    name: 'statement.docx',
    type: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
    size: 100,
  } as File
  await state.extract()
  expect(f.store.extract).toHaveBeenCalledTimes(1)
  expect(state.error.value).toContain('PDF')
  state.file.value = {
    name: 'statement.pdf',
    type: 'application/pdf',
    size: 9 * 1024 * 1024,
  } as File
  await state.extract()
  expect(f.store.extract).toHaveBeenCalledTimes(1)
})

it('rehydrates missing parties from the saved result and blocks recovery with unsaved edits', async () => {
  const state = editor()
  await vi.waitFor(() => expect(state.current.value).not.toBeNull())
  const recovered = doc('a')
  recovered.fields.payerName = 'Nome recuperado'
  recovered.updatedAt = 'v2'
  f.store.reidentifyParties.mockResolvedValue(recovered)
  await state.reidentifyParties()
  expect(state.fields.value?.payerName).toBe('Nome recuperado')
  expect(state.current.value?.updatedAt).toBe('v2')
  state.fields.value!.payerName = 'Edição pendente'
  await state.reidentifyParties()
  expect(f.store.reidentifyParties).toHaveBeenCalledTimes(1)
})

it('uses a chosen party as customer without leaking a previous document or assigning competence', async () => {
  const state = editor()
  await vi.waitFor(() => expect(state.current.value).not.toBeNull())
  state.fields.value!.payeeDocument = '13017462000163'
  state.usePartyAsCustomer('payee')
  expect(state.fields.value!.customerName).toBe('QA recebedor')
  expect(state.fields.value!.customerDocument).toBe('13017462000163')
  expect(state.dirty.value).toBe(true)
  state.usePartyAsCustomer('payer')
  expect(state.fields.value!.customerDocument).toBe('')
  expect(state.fields.value!.competenceDate).toBe('')
  expect(f.store.save).not.toHaveBeenCalled()
})

it('prepares the authoritative draft and does not prepare while edits are unsaved', async () => {
  const state = editor()
  await vi.waitFor(() => expect(state.current.value).not.toBeNull())
  const prepared = doc('a')
  prepared.updatedAt = 'v2'
  f.store.prepare.mockResolvedValue({
    draft: prepared,
    session: { message: 'Sessão verificada' },
    review: { missingFields: ['Competência'] },
  })
  expect(await state.prepare()).toBe(true)
  expect(state.current.value?.updatedAt).toBe('v2')
  expect(state.status.value).toContain('Competência')
  state.fields.value!.customerName = 'Edição pendente'
  expect(await state.prepare()).toBe(false)
  expect(f.store.prepare).toHaveBeenCalledTimes(1)
})
