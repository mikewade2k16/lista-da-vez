import { computed, reactive } from 'vue'
import { emptyAccountingFields, emptyIssuerProfile } from '../../types/accounting'
import type {
  AccountingContext,
  AccountingDocument,
  AccountingFields,
  AccountingReview,
  AccountingTransport,
  Diagnosis,
  Issuer,
  IssuerProfile,
} from '../../types/accounting'

export function createAccountingController(
  context: () => AccountingContext,
  transport: AccountingTransport,
  confirm: (message: string) => Promise<boolean>,
  key: () => string,
) {
  const state = reactive({
    issuers: [] as Array<{ id: string; version: number }>,
    pages: { issuers: 1, diagnoses: 1, documents: 1 },
    more: { issuers: false, diagnoses: false, documents: false },
    issuer: null as Issuer | null,
    profile: emptyIssuerProfile(),
    profileDirty: false,
    diagnoses: [] as Diagnosis[],
    diagnosis: null as Diagnosis | null,
    diagnosisDirty: false,
    answers: {} as Diagnosis['answers'],
    intent: 'service' as Diagnosis['intent'],
    competenceDate: '',
    documents: [] as AccountingDocument[],
    document: null as AccountingDocument | null,
    fields: emptyAccountingFields('nfse'),
    documentDirty: false,
    documentEditorOpen: false,
    kind: 'nfse' as 'nfse' | 'nfe',
    review: null as AccountingReview | null,
    busy: false,
    error: '',
    message: '',
  })
  let generation = 0
  let abort = new AbortController()
  const can = (permission: string) =>
    context().modules.includes('accounting') &&
    Boolean(context().accountId && context().actorId) &&
    context().permissions.includes(permission)
  const enabled = computed(() => can('accounting.issuers.view'))
  const dirty = computed(() => state.profileDirty || state.diagnosisDirty || state.documentDirty)
  const base = () => `/v1/accounting/issuers/${state.issuer?.id || ''}`
  const copy = <T>(value: T): T => JSON.parse(JSON.stringify(value)) as T
  const currentScope = () => `${context().accountId}:${context().actorId}`
  async function run<T>(
    work: (
      request: <V>(
        path: string,
        method?: 'GET' | 'POST' | 'PUT' | 'PATCH',
        body?: unknown,
      ) => Promise<V>,
    ) => Promise<T>,
    apply: (data: T) => void,
  ) {
    if (state.busy || !enabled.value) return false
    const ticket = generation,
      scope = currentScope(),
      accountId = context().accountId,
      signal = abort.signal
    state.busy = true
    state.error = ''
    state.message = ''
    try {
      const data = await work(
        <V>(path: string, method = 'GET' as 'GET' | 'POST' | 'PUT' | 'PATCH', body?: unknown) => {
          if (ticket !== generation || scope !== currentScope() || signal.aborted)
            throw new Error('scope_changed')
          return transport<V>(accountId, path, {
            method,
            body: body === undefined ? undefined : copy(body),
            signal,
            key: method === 'GET' ? undefined : key(),
          })
        },
      )
      if (ticket !== generation || scope !== currentScope()) return false
      apply(data)
      return true
    } catch (cause) {
      if (ticket === generation && scope === currentScope()) {
        const error = cause as { data?: { error?: { code?: string } }; message?: string }
        const code = error.data?.error?.code
        state.error =
          code?.includes('conflict') || code === 'snapshot_stale'
            ? 'Os dados mudaram. Recarregue antes de salvar novamente; suas edições foram preservadas.'
            : 'Não foi possível concluir. Confira as permissões e tente novamente.'
      }
      return false
    } finally {
      if (ticket === generation) state.busy = false
    }
  }
  function reset() {
    generation++
    abort.abort()
    abort = new AbortController()
    Object.assign(state, {
      issuers: [],
      pages: { issuers: 1, diagnoses: 1, documents: 1 },
      more: { issuers: false, diagnoses: false, documents: false },
      issuer: null,
      profile: emptyIssuerProfile(),
      profileDirty: false,
      diagnoses: [],
      diagnosis: null,
      diagnosisDirty: false,
      answers: {},
      competenceDate: '',
      intent: 'service',
      documents: [],
      document: null,
      fields: emptyAccountingFields('nfse'),
      documentDirty: false,
      documentEditorOpen: false,
      review: null,
      kind: 'nfse',
      busy: false,
      error: '',
      message: '',
    })
  }
  async function mayLeave() {
    return !dirty.value || (await confirm('Descartar as edições ainda não salvas?'))
  }
  function hydrateIssuer(issuer: Issuer) {
    state.issuer = issuer
    state.profile = copy(issuer.profile)
    state.profileDirty = false
  }
  function hydrateDocument(document: AccountingDocument) {
    state.document = document
    state.documentEditorOpen = true
    state.fields = copy(document.fields)
    state.kind = document.kind
    state.documentDirty = false
    state.review = null
  }
  async function load() {
    if (!enabled.value) return
    await run(
      (request) => request<Array<{ id: string; version: number }>>('/v1/accounting/issuers'),
      (value) => {
        state.issuers = value
        state.pages.issuers = 1
        state.more.issuers = value.length === 20
      },
    )
  }
  async function loadMore(kind: 'issuers' | 'diagnoses' | 'documents') {
    if (
      !state.more[kind] ||
      (kind !== 'issuers' && !state.issuer) ||
      (kind === 'documents' && !can('accounting.documents.view'))
    )
      return false
    const page = state.pages[kind] + 1
    const path =
      kind === 'issuers'
        ? '/v1/accounting/issuers'
        : `${base()}/${kind === 'diagnoses' ? 'diagnosis' : 'documents'}`
    return run(
      (request) => request<Array<{ id: string; version: number }>>(`${path}?page=${page}&limit=20`),
      (value) => {
        if (kind === 'issuers')
          state.issuers = [
            ...new Map([...state.issuers, ...value].map((item) => [item.id, item])).values(),
          ]
        else if (kind === 'diagnoses')
          state.diagnoses = [
            ...new Map(
              [...state.diagnoses, ...(value as Diagnosis[])].map((item) => [item.id, item]),
            ).values(),
          ]
        else
          state.documents = [
            ...new Map(
              [...state.documents, ...(value as AccountingDocument[])].map((item) => [
                item.id,
                item,
              ]),
            ).values(),
          ]
        state.pages[kind] = page
        state.more[kind] = value.length === 20
      },
    )
  }
  async function selectIssuer(id: string) {
    const scope = currentScope()
    if (state.busy || !(await mayLeave())) return false
    if (!enabled.value || !id || scope !== currentScope()) return false
    state.diagnosis = null
    state.answers = {}
    state.document = null
    state.documentEditorOpen = false
    state.review = null
    state.diagnosisDirty = false
    state.documentDirty = false
    return run(
      async (request) => {
        const issuer = await request<Issuer>(`/v1/accounting/issuers/${id}`)
        const diagnoses = await request<Diagnosis[]>(`/v1/accounting/issuers/${id}/diagnosis`)
        const documents = can('accounting.documents.view')
          ? await request<AccountingDocument[]>(`/v1/accounting/issuers/${id}/documents`)
          : []
        return { issuer, diagnoses, documents }
      },
      (value) => {
        hydrateIssuer(value.issuer)
        state.diagnoses = value.diagnoses
        state.documents = value.documents
        state.pages.diagnoses = 1
        state.pages.documents = 1
        state.more.diagnoses = value.diagnoses.length === 20
        state.more.documents = value.documents.length === 20
      },
    )
  }
  async function newIssuer() {
    const scope = currentScope()
    if (state.busy || !(await mayLeave()) || scope !== currentScope()) return
    state.issuer = null
    state.profile = emptyIssuerProfile()
    state.profileDirty = false
    state.diagnosis = null
    state.answers = {}
    state.diagnoses = []
    state.documents = []
    state.document = null
    state.documentEditorOpen = false
    state.review = null
    state.documentDirty = false
    state.diagnosisDirty = false
  }
  async function saveProfile() {
    if (!can('accounting.issuers.manage')) return false
    const issuer = state.issuer,
      profile: IssuerProfile = copy(state.profile)
    return run(
      async (request) => {
        const saved = await request<{ id: string }>(
          issuer ? base() : '/v1/accounting/issuers',
          issuer ? 'PATCH' : 'POST',
          { expectedVersion: issuer?.version || 0, profile },
        )
        return request<Issuer>(`/v1/accounting/issuers/${saved.id}`)
      },
      (value) => {
        hydrateIssuer(value)
        if (!state.issuers.some((item) => item.id === value.id))
          state.issuers.push({ id: value.id, version: value.version })
        state.message = 'Cadastro salvo no servidor.'
      },
    )
  }
  async function selectDiagnosis(id: string) {
    const scope = currentScope()
    if (state.busy || !(await mayLeave()) || scope !== currentScope()) return false
    if (!id) {
      state.diagnosis = null
      state.competenceDate = ''
      state.answers = {}
      state.document = null
      state.documentEditorOpen = false
      state.review = null
      state.diagnosisDirty = false
      return true
    }
    return run(
      (request) => request<Diagnosis>(`${base()}/diagnosis?diagnosisId=${encodeURIComponent(id)}`),
      (value) => {
        state.diagnosis = value
        state.intent = value.intent
        state.competenceDate = value.competenceDate || ''
        state.answers = copy(value.answers)
        state.document = null
        state.documentEditorOpen = false
        state.review = null
        state.diagnosisDirty = false
      },
    )
  }
  async function saveDiagnosis() {
    if (!can('accounting.issuers.manage') || !state.issuer || state.profileDirty) return false
    return run(
      (request) =>
        request<Diagnosis>(`${base()}/diagnosis`, 'PUT', {
          diagnosisId: state.diagnosis?.id || null,
          expectedVersion: state.diagnosis?.version || 0,
          expectedProfileVersion: state.issuer!.profileVersion,
          intent: state.intent,
          competenceDate: state.competenceDate || null,
          answers: state.answers,
        }),
      (value) => {
        state.diagnosis = value
        state.diagnosisDirty = false
        state.diagnoses = [...state.diagnoses.filter((item) => item.id !== value.id), value]
        state.message = 'Diagnóstico salvo.'
      },
    )
  }
  async function selectDocument(id: string) {
    const scope = currentScope()
    if (
      state.busy ||
      !can('accounting.documents.view') ||
      !(await mayLeave()) ||
      scope !== currentScope()
    )
      return false
    return run(
      async (request) => {
        const document = await request<AccountingDocument>(`${base()}/documents/${id}`)
        const diagnosis = document.diagnosisId
          ? await request<Diagnosis>(
              `${base()}/diagnosis?diagnosisId=${encodeURIComponent(document.diagnosisId)}`,
            )
          : null
        return { document, diagnosis }
      },
      (value) => {
        hydrateDocument(value.document)
        if (value.diagnosis) {
          state.diagnosis = value.diagnosis
          state.answers = copy(value.diagnosis.answers)
          state.intent = value.diagnosis.intent
          state.competenceDate = value.diagnosis.competenceDate || ''
          state.diagnosisDirty = false
        }
      },
    )
  }
  async function newDocument(kind: 'nfse' | 'nfe') {
    const scope = currentScope()
    if (state.busy || !(await mayLeave()) || scope !== currentScope()) return
    state.document = null
    state.documentEditorOpen = false
    state.review = null
    state.kind = kind
    state.documentEditorOpen = true
    state.fields = emptyAccountingFields(kind)
    state.fields.competenceDate = state.diagnosis?.competenceDate || null
    state.documentDirty = false
  }
  async function saveDocument() {
    if (
      !can('accounting.documents.manage') ||
      !state.documentEditorOpen ||
      !state.issuer ||
      !state.diagnosis ||
      state.profileDirty ||
      state.diagnosisDirty
    )
      return false
    const document = state.document,
      path = base(),
      fields: AccountingFields = copy(state.fields)
    return run(
      async (request) => {
        const saved = await request<{ id: string }>(
          document ? `${path}/documents/${document.id}` : `${path}/documents/prepare`,
          document ? 'PATCH' : 'POST',
          document
            ? { expectedVersion: document.version, fields }
            : {
                expectedVersion: 0,
                diagnosisId: state.diagnosis!.id,
                expectedDiagnosisVersion: state.diagnosis!.version,
                expectedProfileVersion: state.issuer!.profileVersion,
                kind: state.kind,
                source: null,
                fields,
              },
        )
        return request<AccountingDocument>(`${path}/documents/${saved.id}`)
      },
      (value) => {
        hydrateDocument(value)
        state.documents = [...state.documents.filter((item) => item.id !== value.id), value]
        state.message = 'Rascunho salvo; nenhuma emissão realizada.'
      },
    )
  }
  async function loadReview() {
    if (!state.document || dirty.value || !can('accounting.documents.view')) return false
    return run(
      (request) => request<AccountingReview>(`${base()}/documents/${state.document!.id}/review`),
      (value) => {
        state.review = value
      },
    )
  }
  async function validate() {
    if (!state.document || dirty.value || !can('accounting.documents.manage')) return false
    return run(
      async (request) => {
        const path = `${base()}/documents/${state.document!.id}`
        await request(path + '/validate', 'POST', { expectedVersion: state.document!.version })
        return request<AccountingDocument>(path)
      },
      (value) => {
        hydrateDocument(value)
        state.message = 'Dados validados localmente. Confira a revisão.'
      },
    )
  }
  async function confirmReview() {
    if (
      !state.review ||
      !state.document ||
      state.document.state !== 'validated' ||
      dirty.value ||
      state.review.pending.length ||
      !can('accounting.documents.review')
    )
      return false
    const ticket = generation,
      version = state.document.version,
      review = state.review,
      path = `${base()}/documents/${state.document.id}`
    if (
      !(await confirm('Confirmar a revisão destes dados? Esta confirmação não autoriza emissão.'))
    )
      return false
    if (ticket !== generation || dirty.value || state.document?.version !== version) return false
    return run(
      async (request) => {
        await request(path + '/reviews', 'POST', {
          expectedVersion: version,
          expectedProfileVersion: review.document.profileVersion,
          snapshotHash: review.snapshotHash,
          confirmed: true,
          connectorId: review.connectorId,
          environment: review.environment,
        })
        return request<AccountingDocument>(path)
      },
      (value) => {
        hydrateDocument(value)
        state.message = 'Revisão confirmada. Emissão exige autorização separada.'
      },
    )
  }
  return {
    state,
    enabled,
    dirty,
    can,
    reset,
    load,
    selectIssuer,
    loadMore,
    newIssuer,
    saveProfile,
    selectDiagnosis,
    saveDiagnosis,
    selectDocument,
    newDocument,
    saveDocument,
    loadReview,
    validate,
    confirmReview,
  }
}
