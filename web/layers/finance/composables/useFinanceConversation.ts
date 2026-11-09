import { computed, onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { useAuthStore } from '~/stores/auth'
import { useFinanceConversationStore } from '~/stores/finance-conversation'
import type { FinanceAccessMessage } from '~/stores/finance-conversation'
import { useFinanceAccountContext } from './finance-account-context'
import { getApiErrorMessage } from '~/utils/api-client'
export function useFinanceConversation() {
  const auth = useAuthStore(),
    account = useFinanceAccountContext(),
    store = useFinanceConversationStore()
  const messages = ref<FinanceAccessMessage[]>([]),
    draft = ref(''),
    error = ref(''),
    busy = ref(false),
    action = ref('')
  const scope = computed(() => `${account.activeAccountId}:${auth.user?.id || ''}`)
  const selectedDraftId = ref('')
  const activeDraftId = computed(
    () =>
      selectedDraftId.value || [...messages.value].reverse().find((m) => m.draftId)?.draftId || '',
  )
  let generation = 0
  let pending: { text: string; id: string; draftId: string } | null = null
  async function load() {
    const ticket = ++generation
    messages.value = []
    draft.value = ''
    error.value = ''
    action.value = ''
    pending = null
    selectedDraftId.value = ''
    if (!account.activeAccountId || !auth.user?.id) {
      busy.value = false
      return
    }
    busy.value = true
    try {
      const out = await store.list(account.activeAccountId)
      if (ticket === generation) messages.value = out.messages
    } catch (cause) {
      if (ticket === generation)
        error.value = getApiErrorMessage(cause, 'Não foi possível carregar a conversa.')
    } finally {
      if (ticket === generation) busy.value = false
    }
  }
  async function send(text = draft.value) {
    text = text.trim()
    if (
      /(?:senha|password|passwd|pwd|token|cookies?|authorization|api[_ -]?key)\s*(?::|=|é\s|eh\s)\s*\S+/i.test(
        text,
      )
    )
      text = 'Quero cadastrar meu acesso pelo campo protegido'
    if (!text || busy.value) return false
    const ticket = generation
    if (pending?.text !== text || pending?.draftId !== activeDraftId.value)
      pending = { text, id: crypto.randomUUID(), draftId: activeDraftId.value }
    const requestId = pending.id
    busy.value = true
    error.value = ''
    try {
      const out = await store.send(account.activeAccountId, {
        requestId,
        text,
        ...(activeDraftId.value ? { draftId: activeDraftId.value } : {}),
      })
      if (ticket !== generation) return false
      messages.value = out.messages
      const reply = [...out.messages].reverse().find((m) => m.role === 'assistant')
      // The server decides which draft the conversation moved to (e.g. a new note
      // after an expense receipt); a stale selection must not keep overriding it.
      if (selectedDraftId.value && reply?.draftId && reply.draftId !== selectedDraftId.value)
        selectedDraftId.value = ''
      const nextAction = reply?.action || ''
      action.value = ['access', 'document', 'profile', 'prepare'].includes(nextAction)
        ? nextAction
        : ''
      draft.value = ''
      pending = null
      return true
    } catch (cause) {
      if (ticket === generation)
        error.value = getApiErrorMessage(cause, 'Não foi possível enviar. Tente novamente.')
      return false
    } finally {
      if (ticket === generation) busy.value = false
    }
  }
  onMounted(load)
  watch(scope, load)
  onBeforeUnmount(() => generation++)
  return {
    selectedDraftId,
    activeDraftId,
    messages,
    draft,
    error,
    busy,
    scope,
    action,
    send,
    load,
  }
}
