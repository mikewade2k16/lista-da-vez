import { effectScope, nextTick, reactive } from 'vue'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { useFinanceConversation } from './useFinanceConversation'
const f = vi.hoisted(() => ({
  account: null as unknown as { activeAccountId: string },
  store: { list: vi.fn(), send: vi.fn() },
}))
vi.mock('vue', async (original) => ({
  ...(await original<typeof import('vue')>()),
  onMounted: (fn: () => void) => fn(),
  onBeforeUnmount: () => {},
}))
vi.mock('~/stores/auth', () => ({ useAuthStore: () => ({ user: { id: 'user' } }) }))
vi.mock('../../core/stores/account', () => ({ useCoreAccountStore: () => f.account }))
vi.mock('~/stores/finance-conversation', () => ({ useFinanceConversationStore: () => f.store }))
vi.mock('~/utils/api-client', () => ({ getApiErrorMessage: () => 'Falha' }))
const scopes: ReturnType<typeof effectScope>[] = []
function editor() {
  const s = effectScope()
  scopes.push(s)
  return s.run(useFinanceConversation)!
}
beforeEach(() => {
  vi.clearAllMocks()
  f.account = reactive({ activeAccountId: 'a' })
  f.store.list.mockResolvedValue({ messages: [] })

  f.store.send.mockResolvedValue({
    messages: [{ id: '1', role: 'assistant', text: 'Cadastro', action: 'access' }],
  })
})
afterEach(() => scopes.splice(0).forEach((s) => s.stop()))
it('sends without AI and never sends an explicitly pasted password to the conversation endpoint', async () => {
  const s = editor()
  await vi.waitFor(() => expect(s.busy.value).toBe(false))
  s.draft.value = 'minha senha é secret-qa-value'
  await s.send()
  expect(f.store.send.mock.calls[0]?.[1].text).not.toContain('secret-qa-value')
  expect(s.action.value).toBe('access')
  expect(s.draft.value).toBe('')
})
it('retains draft and reuses the request ID after network failure', async () => {
  const s = editor()
  await vi.waitFor(() => expect(s.busy.value).toBe(false))
  s.draft.value = 'testar acesso'
  f.store.send.mockRejectedValueOnce(new Error('network'))
  await s.send()
  expect(s.draft.value).toBe('testar acesso')
  const id = f.store.send.mock.calls[0]?.[1].requestId
  await s.send()
  expect(f.store.send.mock.calls[1]?.[1].requestId).toBe(id)
})
it('discards an old account response', async () => {
  const s = editor()
  await vi.waitFor(() => expect(s.busy.value).toBe(false))
  let done!: (v: { messages: [] }) => void
  f.store.send.mockImplementationOnce(() => new Promise((r) => (done = r)))
  const pending = s.send('testar acesso')
  f.account.activeAccountId = 'b'
  await nextTick()
  await vi.waitFor(() => expect(s.busy.value).toBe(false))
  done({ messages: [] })
  await pending
  expect(s.messages.value).toEqual([])
  expect(s.scope.value).toBe('b:user')
})

it('binds the selected draft and changes the idempotency key when the document changes', async () => {
  const s = editor()
  await vi.waitFor(() => expect(s.busy.value).toBe(false))
  s.selectedDraftId.value = 'draft-a'
  f.store.send.mockRejectedValueOnce(new Error('network'))
  await s.send('Preparar nota')
  const first = f.store.send.mock.calls[0]?.[1]
  expect(first.draftId).toBe('draft-a')
  s.selectedDraftId.value = 'draft-b'
  await s.send('Preparar nota')
  const next = f.store.send.mock.calls[1]?.[1]
  expect(next.draftId).toBe('draft-b')
  expect(next.requestId).not.toBe(first.requestId)
})

it('loads history without reopening an old form', async () => {
  f.store.list.mockResolvedValue({
    messages: [{ id: 'old', role: 'assistant', text: 'Acesso', action: 'access' }],
  })
  const s = editor()
  await vi.waitFor(() => expect(s.busy.value).toBe(false))
  expect(s.messages.value).toHaveLength(1)
  expect(s.action.value).toBe('')
})
it('keeps a single question in the transcript without opening a form', async () => {
  f.store.send.mockResolvedValue({
    messages: [
      { id: 'u', role: 'user', text: 'criar nota', action: '' },
      { id: 'a', role: 'assistant', text: 'Para qual cliente?', action: 'ask:Tomador' },
    ],
  })
  const s = editor()
  await vi.waitFor(() => expect(s.busy.value).toBe(false))
  await s.send('criar nota')
  expect(s.messages.value[0]?.text).toBe('criar nota')
  expect(s.action.value).toBe('')
})

it('follows the draft chosen by the server instead of a stale selection', async () => {
  const s = editor()
  await vi.waitFor(() => expect(s.busy.value).toBe(false))
  s.selectedDraftId.value = 'receipt'
  f.store.send.mockResolvedValueOnce({
    messages: [
      {
        id: '2',
        role: 'assistant',
        text: 'Para qual cliente?',
        action: 'ask:Tomador',
        draftId: 'note',
      },
    ],
  })
  await s.send('criar nota')
  expect(s.selectedDraftId.value).toBe('')
  expect(s.activeDraftId.value).toBe('note')
  await s.send('Cliente QA')
  expect(f.store.send.mock.calls[1]?.[1].draftId).toBe('note')
})
