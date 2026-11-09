import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useFinanceRecordsStore } from '~/stores/finance-records'
import { getApiErrorMessage } from '~/utils/api-client'
import { useCoreAccountStore } from '../../core/stores/account'
import type {
  FinanceFiscalCustomer,
  FinanceFiscalCustomerVersion,
  FinanceReceiptItem,
  FinanceRetentionEvent,
  FinanceRetentionSettings,
} from '../types/records'

// Base comum: recarrega ao trocar de conta, descarta respostas antigas (ticket) e
// cancela a requisição em voo ao desmontar ou trocar de conta.
function useAccountLoader(load: (accountId: string, signal: AbortSignal) => Promise<void>) {
  const account = useCoreAccountStore()
  const loading = ref(false)
  const error = ref('')
  let ticket = 0
  let controller: AbortController | null = null
  async function reload() {
    const current = ++ticket
    controller?.abort()
    controller = new AbortController()
    const accountId = account.activeAccountId
    if (!accountId) return
    loading.value = true
    error.value = ''
    try {
      await load(accountId, controller.signal)
    } catch (cause) {
      if (current === ticket && !controller.signal.aborted)
        error.value = getApiErrorMessage(cause, 'Não foi possível carregar os dados.')
    } finally {
      if (current === ticket) loading.value = false
    }
  }
  const isCurrent = (accountId: string) => accountId === account.activeAccountId
  onMounted(reload)
  watch(
    () => account.activeAccountId,
    () => void reload(),
  )
  onBeforeUnmount(() => controller?.abort())
  return { account, loading, error, reload, isCurrent }
}

export function useFinanceReceipts() {
  const store = useFinanceRecordsStore()
  const items = ref<FinanceReceiptItem[]>([])
  const page = ref(1)
  const more = ref(false)
  const busyId = ref('')
  const status = ref('')
  const base = useAccountLoader(async (accountId, signal) => {
    items.value = []
    const result = await store.listReceipts(accountId, 1, signal)
    if (!base.isCurrent(accountId)) return
    items.value = result.items
    page.value = 1
    more.value = result.more
  })
  async function loadMore() {
    const accountId = base.account.activeAccountId
    const result = await store.listReceipts(accountId, page.value + 1)
    if (!base.isCurrent(accountId)) return
    items.value = [...items.value, ...result.items]
    page.value = result.page
    more.value = result.more
  }
  async function act(
    documentId: string,
    run: (accountId: string) => Promise<unknown>,
    done: string,
  ) {
    busyId.value = documentId
    status.value = ''
    base.error.value = ''
    try {
      await run(base.account.activeAccountId)
      status.value = done
      await base.reload()
    } catch (cause) {
      base.error.value = getApiErrorMessage(cause, 'Não foi possível concluir a ação.')
    } finally {
      busyId.value = ''
    }
  }
  const unlink = (documentId: string) =>
    act(
      documentId,
      (accountId) => store.unlinkReceipt(accountId, documentId),
      'Vínculo desfeito. O lançamento continua na planilha.',
    )
  const removeOriginal = (documentId: string) =>
    act(
      documentId,
      (accountId) => store.removeOriginal(accountId, documentId),
      'Original apagado. Os dados extraídos continuam salvos.',
    )
  async function download(item: FinanceReceiptItem) {
    const accountId = base.account.activeAccountId
    busyId.value = item.document.id
    try {
      const blob = await store.downloadOriginal(accountId, item.document.id)
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = item.original?.fileName || item.document.sourceName || 'comprovante'
      link.click()
      setTimeout(() => URL.revokeObjectURL(url), 1000)
    } catch (cause) {
      base.error.value = getApiErrorMessage(cause, 'Não foi possível baixar o original.')
    } finally {
      busyId.value = ''
    }
  }
  return { ...base, items, more, busyId, status, loadMore, unlink, removeOriginal, download }
}

export function useFinanceRetention() {
  const store = useFinanceRecordsStore()
  const settings = ref<FinanceRetentionSettings | null>(null)
  const history = ref<FinanceRetentionEvent[]>([])
  const days = ref(90)
  const touched = ref(false)
  const saving = ref(false)
  const status = ref('')
  const base = useAccountLoader(async (accountId, signal) => {
    settings.value = null
    const value = await store.getRetention(accountId, signal)
    if (!base.isCurrent(accountId)) return
    settings.value = value
    if (!touched.value) days.value = value.receiptOriginalDays
  })
  const dirty = computed(
    () => settings.value !== null && days.value !== settings.value.receiptOriginalDays,
  )
  async function loadHistory() {
    const accountId = base.account.activeAccountId
    const value = await store.retentionHistory(accountId)
    if (base.isCurrent(accountId)) history.value = value
  }
  async function save() {
    if (!settings.value || !Number.isInteger(days.value) || days.value < 1 || days.value > 3650) {
      base.error.value = 'Informe um prazo entre 1 e 3650 dias.'
      return
    }
    saving.value = true
    status.value = ''
    base.error.value = ''
    try {
      settings.value = await store.updateRetention(
        base.account.activeAccountId,
        days.value,
        settings.value.version,
      )
      days.value = settings.value.receiptOriginalDays
      touched.value = false
      status.value = 'Prazo salvo. Vale também para os originais já guardados.'
      if (history.value.length) await loadHistory()
    } catch (cause) {
      base.error.value = getApiErrorMessage(cause, 'Não foi possível salvar o prazo.')
    } finally {
      saving.value = false
    }
  }
  return { ...base, settings, history, days, touched, dirty, saving, status, save, loadHistory }
}

export function useFiscalCustomers() {
  const store = useFinanceRecordsStore()
  const customers = ref<FinanceFiscalCustomer[]>([])
  const versions = ref<Record<string, FinanceFiscalCustomerVersion[]>>({})
  const editingId = ref('')
  const draft = ref({ name: '', serviceDescription: '', amount: null as number | null })
  const saving = ref(false)
  const status = ref('')
  const base = useAccountLoader(async (accountId, signal) => {
    customers.value = []
    versions.value = {}
    const list = await store.listCustomers(accountId, signal)
    if (base.isCurrent(accountId)) customers.value = list
  })
  async function toggleVersions(id: string) {
    if (versions.value[id]) {
      const next = { ...versions.value }
      delete next[id]
      versions.value = next
      return
    }
    const accountId = base.account.activeAccountId
    try {
      const list = await store.customerVersions(accountId, id)
      if (base.isCurrent(accountId)) versions.value = { ...versions.value, [id]: list }
    } catch (cause) {
      base.error.value = getApiErrorMessage(cause, 'Não foi possível carregar o histórico.')
    }
  }
  function edit(customer: FinanceFiscalCustomer) {
    editingId.value = customer.id
    draft.value = {
      name: customer.name,
      serviceDescription: customer.serviceDescription,
      amount: customer.amount,
    }
    status.value = ''
  }
  async function save(customer: FinanceFiscalCustomer) {
    saving.value = true
    base.error.value = ''
    try {
      const saved = await store.updateCustomer(base.account.activeAccountId, customer.id, {
        ...draft.value,
        expectedVersion: customer.version,
      })
      customers.value = customers.value.map((item) => (item.id === saved.id ? saved : item))
      if (versions.value[saved.id]) {
        const next = { ...versions.value }
        delete next[saved.id]
        versions.value = next
      }
      editingId.value = ''
      status.value =
        saved.version === customer.version ? 'Nada mudou.' : `Salvo como versão ${saved.version}.`
    } catch (cause) {
      base.error.value = getApiErrorMessage(
        cause,
        'Não foi possível salvar o cliente. Recarregue e tente de novo.',
      )
    } finally {
      saving.value = false
    }
  }
  return {
    ...base,
    customers,
    versions,
    editingId,
    draft,
    saving,
    status,
    toggleVersions,
    edit,
    save,
    cancel: () => (editingId.value = ''),
  }
}
