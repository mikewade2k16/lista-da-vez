import { createSSRApp, defineComponent, h, provide, ref } from 'vue'
import { renderToString } from 'vue/server-renderer'
import { expect, it, vi } from 'vitest'
import type { AccountContext } from '../../core/stores/account'
import { FINANCE_CHAT_ACCOUNT, useFinanceAccountContext } from './finance-account-context'
const root = vi.hoisted(() => ({ activeAccountId: 'page-a', permissions: ['page-only'] }))
vi.mock('../../core/stores/account', () => ({ useCoreAccountStore: () => root }))
it('reads only the provided chat account, without changing page scope or permissions', async () => {
  const selected = ref('chat-b')
  const context = ref({
    account: { id: 'chat-b', modules: ['finance'] },
    permissions: ['finance.sheets.view'],
  } as AccountContext)
  let chat!: ReturnType<typeof useFinanceAccountContext>
  const Child = defineComponent({
    setup() {
      chat = useFinanceAccountContext()
      return () => h('span', chat.activeAccountId)
    },
  })
  const app = createSSRApp(
    defineComponent({
      setup() {
        provide(FINANCE_CHAT_ACCOUNT, { accountId: selected, context })
        return () => h(Child)
      },
    }),
  )
  expect(await renderToString(app)).toContain('chat-b')
  expect(root.activeAccountId).toBe('page-a')
  expect(chat.permissions).toEqual(['finance.sheets.view'])
  selected.value = 'chat-c'
  context.value = null
  expect(chat.activeAccountId).toBe('chat-c')
  expect(chat.permissions).toEqual([])
  expect(chat.enabledModules).toEqual([])
  expect(root.activeAccountId).toBe('page-a')
  expect(root.permissions).toEqual(['page-only'])
})
