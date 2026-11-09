import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useFinanceDocumentsStore } from '~/stores/finance-documents'
import { useAuthStore } from '~/stores/auth'
import { downloadText } from '~/utils/report-export'
import { documentPreparation } from '../utils/document-preparation'
import { getApiErrorMessage } from '~/utils/api-client'
import { useFinanceAccountContext } from './finance-account-context'
import { confirmFiscalAction } from '../utils/fiscal-confirm'
import type { FinanceDocumentReview } from '../types/documents'
export function useFinanceDocumentReview(
  id: () => string,
  version: () => string,
  dirty: () => boolean,
) {
  const store = useFinanceDocumentsStore()
  const account = useFinanceAccountContext()
  const auth = useAuthStore()
  const review = ref<FinanceDocumentReview | null>(null)
  const opened = ref(false)
  const busy = ref(false)
  const error = ref('')
  const scope = computed(
    () => `${account.activeAccountId}:${auth.user?.id || ''}:${id()}:${version()}`,
  )
  let generation = 0
  async function load() {
    const ticket = ++generation
    review.value = null
    error.value = ''
    busy.value = true
    if (!id() || !account.activeAccountId) {
      busy.value = false
      return
    }
    try {
      const data = await store.review(account.activeAccountId, id())
      if (ticket === generation) review.value = data
    } catch (cause) {
      if (ticket === generation)
        error.value = getApiErrorMessage(cause, 'Não foi possível carregar a revisão.')
    } finally {
      if (ticket === generation) busy.value = false
    }
  }
  async function toggle(value: boolean) {
    opened.value = value
    if (value) await load()
  }
  async function approve(value: boolean) {
    if (!review.value || busy.value || dirty() || (value && !review.value.ready)) return
    const ticket = generation
    const snapshot = review.value
    const key = scope.value
    const confirmed = await confirmFiscalAction(
      value
        ? 'Confirmar a revisão dos dados exibidos? Esta ação não emite nota nem autoriza emissão no portal.'
        : 'Cancelar a confirmação desta revisão? O histórico será preservado.',
      () => `${scope.value}:${review.value?.hash}:${dirty()}`,
    )
    if (!confirmed || ticket !== generation || key !== scope.value || dirty()) return
    busy.value = true
    error.value = ''
    try {
      const data = await store.approveReview(account.activeAccountId, id(), snapshot, value)
      if (ticket === generation) review.value = data
    } catch (cause) {
      if (ticket === generation)
        error.value = getApiErrorMessage(
          cause,
          'Não foi possível confirmar. Recarregue a revisão e confira os dados novamente.',
        )
    } finally {
      if (ticket === generation) busy.value = false
    }
  }
  async function downloadPreparation() {
    if (busy.value || dirty() || !id()) return
    const ticket = ++generation
    const documentId = id()
    const accountId = account.activeAccountId
    busy.value = true
    error.value = ''
    try {
      const data = await store.review(accountId, documentId)
      if (ticket !== generation || dirty()) return
      review.value = data
      downloadText(
        JSON.stringify(documentPreparation(data), null, 2),
        `finance-preparacao-${documentId}.json`,
        'application/json;charset=utf-8',
      )
    } catch (cause) {
      if (ticket === generation)
        error.value = getApiErrorMessage(
          cause,
          'Não foi possível baixar os dados. Recarregue e tente novamente.',
        )
    } finally {
      if (ticket === generation) busy.value = false
    }
  }
  watch(scope, () => {
    generation++
    review.value = null
    error.value = ''
    busy.value = false
    if (opened.value) void load()
  })
  onBeforeUnmount(() => generation++)
  return { review, opened, busy, error, load, toggle, approve, downloadPreparation }
}
