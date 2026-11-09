import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useFinanceDocumentsStore } from '~/stores/finance-documents'
import { useFiscalProfileStore } from '~/stores/fiscal-profile'
import { useAuthStore } from '~/stores/auth'
import { useFinanceAccountContext } from './finance-account-context'
import { getApiErrorMessage } from '~/utils/api-client'
import type { FinanceDocumentDraft, FinanceDocumentFields } from '../types/documents'
import type { FiscalTemplate } from '../types/fiscal'
export function useFinanceDocuments(documentId: () => string = () => '') {
  const store = useFinanceDocumentsStore()
  const fiscal = useFiscalProfileStore()
  const auth = useAuthStore()
  const account = useFinanceAccountContext()
  const documents = ref<FinanceDocumentDraft[]>([])
  const templates = ref<FiscalTemplate[]>([])
  const current = ref<FinanceDocumentDraft | null>(null)
  const fields = ref<FinanceDocumentFields | null>(null)
  const busy = ref(false)
  const error = ref('')
  const status = ref('')
  const file = ref<File | null>(null)
  const dirty = computed(() =>
    Boolean(current.value && JSON.stringify(fields.value) !== JSON.stringify(current.value.fields)),
  )
  const scope = computed(() => `${account.activeAccountId}:${auth.user?.id || ''}`)
  let generation = 0
  function select(d: FinanceDocumentDraft) {
    current.value = d
    fields.value = JSON.parse(JSON.stringify(d.fields)) as FinanceDocumentFields
  }
  function accept(d: FinanceDocumentDraft) {
    documents.value = [d, ...documents.value.filter((item) => item.id !== d.id)].slice(0, 10)
    select(d)
  }
  async function load() {
    const ticket = ++generation
    current.value = null
    fields.value = null
    documents.value = []
    templates.value = []
    file.value = null
    error.value = ''
    status.value = ''
    busy.value = true
    const id = account.activeAccountId
    if (!id || !auth.user?.id) {
      busy.value = false
      return
    }
    try {
      const [docs, list] = await Promise.all([store.list(id), fiscal.listTemplates(id)])
      if (ticket !== generation) return
      documents.value = docs
      templates.value = list
      const target = documentId()
      if (target) {
        const selected = docs.find((item) => item.id === target) || (await store.get(id, target))
        if (ticket === generation) accept(selected)
      } else if (docs[0]) select(docs[0])
    } catch (cause) {
      if (ticket === generation)
        error.value = getApiErrorMessage(cause, 'Não foi possível carregar os documentos.')
    } finally {
      if (ticket === generation) busy.value = false
    }
  }
  async function extract() {
    if (!file.value || busy.value || dirty.value) return
    if (
      (![
        'image/jpeg',
        'image/png',
        'application/pdf',
        'text/plain',
        'text/csv',
        'application/x-ofx',
      ].includes(file.value.type) &&
        !/\.(pdf|jpe?g|png|txt|csv|ofx)$/i.test(file.value.name || '')) ||
      file.value.size > 8 * 1024 * 1024
    ) {
      error.value = 'Envie PDF, JPEG, PNG, TXT, CSV ou OFX de até 8 MB.'
      return
    }
    const ticket = generation
    busy.value = true
    error.value = ''
    status.value = ''
    try {
      const d = await store.extract(account.activeAccountId, file.value)
      if (ticket !== generation) return
      accept(d)
      file.value = null
      status.value = 'Documento lido. Rascunho salvo; confira e complete os campos.'
    } catch (cause) {
      if (ticket === generation)
        error.value = getApiErrorMessage(
          cause,
          'Não foi possível ler o documento. Confira o formato e tente novamente.',
        )
    } finally {
      if (ticket === generation) busy.value = false
    }
  }
  async function save() {
    if (!fields.value || !current.value || busy.value) return
    const ticket = generation
    busy.value = true
    error.value = ''
    status.value = ''
    try {
      const payload = JSON.parse(JSON.stringify(fields.value)) as FinanceDocumentFields
      if (typeof payload.amount !== 'number') payload.amount = null
      const d = await store.save(account.activeAccountId, current.value, payload)
      if (ticket === generation) {
        accept(d)
        status.value = 'Rascunho salvo. Nenhuma nota foi emitida.'
      }
    } catch (cause) {
      if (ticket === generation)
        error.value = getApiErrorMessage(
          cause,
          'Não foi possível salvar. Recarregue em caso de conflito.',
        )
    } finally {
      if (ticket === generation) busy.value = false
    }
  }
  async function reidentifyParties() {
    if (!current.value || busy.value || dirty.value) return
    const ticket = generation
    busy.value = true
    error.value = ''
    status.value = ''
    try {
      const d = await store.reidentifyParties(account.activeAccountId, current.value)
      if (ticket === generation) {
        accept(d)
        status.value =
          'Identificação atualizada nos campos vazios. Dados já preenchidos foram preservados.'
      }
    } catch (cause) {
      if (ticket === generation)
        error.value = getApiErrorMessage(
          cause,
          'Não foi possível atualizar a identificação. Recarregue em caso de conflito.',
        )
    } finally {
      if (ticket === generation) busy.value = false
    }
  }
  async function prepare() {
    if (!current.value || busy.value || dirty.value) return false
    const ticket = generation
    busy.value = true
    error.value = ''
    status.value = ''
    try {
      const result = await store.prepare(account.activeAccountId, current.value)
      if (ticket !== generation) return false
      accept(result.draft)
      status.value =
        result.session.message +
        (result.review.missingFields.length
          ? ' Falta completar: ' + result.review.missingFields.join(', ') + '.'
          : ' Confira os dados e confirme a revisão.')
      return true
    } catch (cause) {
      if (ticket === generation)
        error.value = getApiErrorMessage(
          cause,
          'Não foi possível preparar. Confira o acesso e recarregue em caso de conflito.',
        )
      return false
    } finally {
      if (ticket === generation) busy.value = false
    }
  }
  function usePartyAsCustomer(party: 'payer' | 'payee') {
    if (!fields.value || busy.value) return
    const name = party === 'payer' ? fields.value.payerName : fields.value.payeeName
    if (!name) return
    fields.value.customerName = name
    fields.value.customerDocument =
      (party === 'payer' ? fields.value.payerDocument : fields.value.payeeDocument) || ''
  }
  function applyTemplate(id: string) {
    const template = templates.value.find((item) => item.id === id)
    if (!fields.value) return
    fields.value.templateId = id
    if (template) fields.value.service = { ...template.fields }
  }
  onMounted(load)
  watch(scope, load)
  watch(documentId, (id) => {
    if (id && id !== current.value?.id && !dirty.value) void load()
  })
  onBeforeUnmount(() => generation++)
  return {
    prepare,
    documents,
    templates,
    current,
    fields,
    busy,
    error,
    status,
    file,
    dirty,
    select,
    load,
    extract,
    save,
    applyTemplate,
    reidentifyParties,
    usePartyAsCustomer,
  }
}
