import { beforeEach, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import { createAccountingController } from './accounting-controller'
import { emptyAccountingFields, emptyIssuerProfile } from '../../types/accounting'
import type {
  AccountingContext,
  AccountingDocument,
  AccountingReview,
  AccountingTransport,
  Diagnosis,
  Issuer,
} from '../../types/accounting'
let context: AccountingContext
let api: ReturnType<typeof vi.fn>
let confirm: ReturnType<typeof vi.fn>
const profile = (): Issuer => ({
  id: 'issuer-a',
  version: 2,
  profileVersion: 3,
  profile: { ...emptyIssuerProfile(), legalName: 'Empresa do servidor' },
  emissionAvailable: false,
})
const diagnosis = (): Diagnosis => ({
  id: 'diagnosis-a',
  version: 4,
  profileVersion: 3,
  intent: 'service',
  competenceDate: '2026-11-04',
  answers: {},
  pending: [],
})
const document = (): AccountingDocument => ({
  id: 'doc-a',
  issuerId: 'issuer-a',
  version: 5,
  profileVersion: 3,
  kind: 'nfse',
  state: 'validated',
  fields: emptyAccountingFields('nfse'),
})
function controller() {
  return createAccountingController(
    () => context,
    api as AccountingTransport,
    confirm,
    () => 'unique-request-key',
  )
}
function ready() {
  const c = controller()
  c.state.issuer = profile()
  c.state.profile = profile().profile
  c.state.diagnosis = diagnosis()
  return c
}
beforeEach(() => {
  context = reactive({
    accountId: 'a',
    actorId: 'u',
    modules: ['finance', 'accounting'],
    permissions: [
      'accounting.issuers.view',
      'accounting.issuers.manage',
      'accounting.documents.view',
      'accounting.documents.manage',
      'accounting.documents.review',
    ],
  })
  api = vi.fn().mockResolvedValue([])
  confirm = vi.fn().mockResolvedValue(true)
})
it('does not fetch Accounting when module or permission is absent', async () => {
  const c = controller()
  context.modules = ['finance']
  await c.load()
  expect(api).not.toHaveBeenCalled()
  context.modules = ['accounting']
  context.permissions = []
  await c.load()
  expect(api).not.toHaveBeenCalled()
})
it('requires an explicit issuer selection and loads server data', async () => {
  api.mockResolvedValueOnce([{ id: 'issuer-a', version: 2 }])
  const c = controller()
  await c.load()
  expect(c.state.issuer).toBeNull()
  api
    .mockResolvedValueOnce(profile())
    .mockResolvedValueOnce([diagnosis()])
    .mockResolvedValueOnce([])
  await c.selectIssuer('issuer-a')
  expect(c.state.profile.legalName).toBe('Empresa do servidor')
  expect(c.state.diagnoses).toHaveLength(1)
})
it('does not fetch documents without the document permission', async () => {
  context.permissions = ['accounting.issuers.view']
  api.mockResolvedValueOnce(profile()).mockResolvedValueOnce([])
  const c = controller()
  await c.selectIssuer('issuer-a')
  expect(api).toHaveBeenCalledTimes(2)
})
it('ignores a late response after switching account', async () => {
  let resolve!: (data: unknown) => void
  api.mockImplementationOnce(
    () =>
      new Promise((r) => {
        resolve = r
      }),
  )
  const c = controller(),
    pending = c.load()
  const signal = api.mock.calls[0]![2].signal as AbortSignal
  context.accountId = 'b'
  c.reset()
  resolve([{ id: 'private-a' }])
  await pending
  expect(signal.aborted).toBe(true)
  expect(c.state.issuers).toEqual([])
})
it('never starts a follow-up request under the old account after a scope switch', async () => {
  let resolve!: (data: unknown) => void
  api.mockImplementationOnce(
    () =>
      new Promise((r) => {
        resolve = r
      }),
  )
  const c = controller(),
    pending = c.selectIssuer('issuer-a')
  await Promise.resolve()
  context.accountId = 'b'
  c.reset()
  resolve(profile())
  await pending
  expect(api).toHaveBeenCalledTimes(1)
  expect(c.state.issuer).toBeNull()
})
it('preserves dirty edits when navigation is cancelled', async () => {
  const c = ready()
  c.state.profileDirty = true
  c.state.profile.legalName = 'Edição'
  confirm.mockResolvedValue(false)
  await c.selectIssuer('issuer-b')
  expect(api).not.toHaveBeenCalled()
  expect(c.state.profile.legalName).toBe('Edição')
})
it('cancels a selection if the account changes during the discard confirmation', async () => {
  let resolve!: (v: boolean) => void
  confirm.mockImplementationOnce(
    () =>
      new Promise<boolean>((r) => {
        resolve = r
      }),
  )
  const c = ready()
  c.state.profileDirty = true
  const pending = c.selectIssuer('issuer-b')
  context.accountId = 'b'
  c.reset()
  resolve(true)
  await pending
  expect(api).not.toHaveBeenCalled()
})
it('saves with the original account, expected version, and hydrates authoritative data', async () => {
  const c = ready()
  c.state.profileDirty = true
  c.state.profile.legalName = 'Rascunho'
  api.mockResolvedValueOnce({ id: 'issuer-a' }).mockResolvedValueOnce(profile())
  await c.saveProfile()
  expect(api.mock.calls[0]![0]).toBe('a')
  expect(api.mock.calls[0]![2].body.expectedVersion).toBe(2)
  expect(c.state.profile.legalName).toBe('Empresa do servidor')
  expect(c.state.profileDirty).toBe(false)
})
it('preserves draft and dirty state on 409', async () => {
  const c = ready()
  c.state.profileDirty = true
  c.state.profile.legalName = 'Não perder'
  api.mockRejectedValueOnce({ data: { error: { code: 'version_conflict' } } })
  expect(await c.saveProfile()).toBe(false)
  expect(c.state.profileDirty).toBe(true)
  expect(c.state.profile.legalName).toBe('Não perder')
  expect(c.state.error).toContain('Recarregue')
})
it('resumes a diagnosis and preserves unknown fields as null', async () => {
  const c = ready()
  const d = diagnosis()
  d.answers.taxRegime = { value: null, confirmation: 'unknown', origin: null, evidenceRef: null }
  api.mockResolvedValueOnce(d)
  await c.selectDiagnosis(d.id)
  expect(c.state.competenceDate).toBe(d.competenceDate)
  expect(c.state.answers.taxRegime?.value).toBeNull()
  api.mockResolvedValueOnce(d)
  await c.saveDiagnosis()
  expect(api.mock.calls[1]![2].body.answers.taxRegime.value).toBeNull()
})
it('keeps a reader from mutating even through direct controller calls', async () => {
  const c = ready()
  context.permissions = ['accounting.issuers.view', 'accounting.documents.view']
  await c.saveProfile()
  await c.saveDiagnosis()
  await c.saveDocument()
  expect(api).not.toHaveBeenCalled()
})
it('prepares from explicit diagnosis and fields without AI or credentials', async () => {
  const c = ready()
  c.state.documentEditorOpen = true
  c.state.fields.amount = '125.00'
  c.state.documentDirty = true
  api.mockResolvedValueOnce({ id: 'doc-a' }).mockResolvedValueOnce(document())
  await c.saveDocument()
  const [account, path, options] = api.mock.calls[0]!
  expect(account).toBe('a')
  expect(path).toContain('/documents/prepare')
  expect(options.body.diagnosisId).toBe('diagnosis-a')
  expect(options.body.source).toBeNull()
  expect(options.body.fields.amount).toBe('125.00')
  expect(JSON.stringify(options.body)).not.toMatch(/password|apiKey|portalLogin/)
  expect(c.state.documentDirty).toBe(false)
})
it('does not revise a dirty document', async () => {
  const c = ready()
  c.state.document = document()
  c.state.documentDirty = true
  await c.loadReview()
  expect(api).not.toHaveBeenCalled()
})
it('requires separate review permission and explicit confirmation, never submits', async () => {
  const c = ready()
  c.state.document = document()
  const review: AccountingReview = {
    document: document(),
    profile: profile().profile,
    snapshotHash: 'server-hash',
    pending: [],
    emissionAvailable: false,
    connectorId: '',
    environment: '',
  }
  c.state.review = review
  confirm.mockResolvedValueOnce(false)
  await c.confirmReview()
  expect(api).not.toHaveBeenCalled()
  api
    .mockResolvedValueOnce({ reviewId: 'review-a' })
    .mockResolvedValueOnce({ ...document(), state: 'reviewed', version: 6 })
  await c.confirmReview()
  expect(api.mock.calls[0]![2].body.snapshotHash).toBe('server-hash')
  expect(c.state.review).toBeNull()
  expect(api.mock.calls.some((call) => /execute|authorizations/.test(call[1] as string))).toBe(
    false,
  )
})
it('drops a review confirmation after changing account', async () => {
  let resolve!: (v: boolean) => void
  confirm.mockImplementationOnce(
    () =>
      new Promise<boolean>((r) => {
        resolve = r
      }),
  )
  const c = ready()
  c.state.document = document()
  c.state.review = {
    document: document(),
    profile: profile().profile,
    snapshotHash: 'old',
    pending: [],
    emissionAvailable: false,
    connectorId: '',
    environment: '',
  }
  const pending = c.confirmReview()
  context.accountId = 'b'
  c.reset()
  resolve(true)
  await pending
  expect(api).not.toHaveBeenCalled()
})
it('does not infer a product draft from a service operation', async () => {
  const c = ready()
  await c.newDocument('nfe')
  expect(c.state.fields.service).toBeUndefined()
  expect(c.state.fields.items?.[0]?.ncm).toBeNull()
  expect(c.state.fields.amount).toBeNull()
})

it('closes the old document when resuming a different diagnosis', async () => {
  const c = ready()
  c.state.document = document()
  c.state.documentEditorOpen = true
  api.mockResolvedValueOnce({ ...diagnosis(), id: 'other-diagnosis' })
  await c.selectDiagnosis('other-diagnosis')
  expect(c.state.document).toBeNull()
  expect(c.state.documentEditorOpen).toBe(false)
})
it('loads the persisted diagnosis link when selecting a document', async () => {
  const c = ready()
  api
    .mockResolvedValueOnce({ ...document(), diagnosisId: 'diagnosis-linked' })
    .mockResolvedValueOnce({ ...diagnosis(), id: 'diagnosis-linked' })
  await c.selectDocument('doc-a')
  expect(c.state.diagnosis?.id).toBe('diagnosis-linked')
  expect(c.state.documentEditorOpen).toBe(true)
})
it('does not save a document until its type has been selected explicitly', async () => {
  const c = ready()
  await c.saveDocument()
  expect(api).not.toHaveBeenCalled()
})
it('treats profile and diagnosis edits as invalidating review', async () => {
  const c = ready()
  c.state.document = document()
  c.state.diagnosisDirty = true
  await c.loadReview()
  expect(api).not.toHaveBeenCalled()
})
