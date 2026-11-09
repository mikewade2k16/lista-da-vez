import { beforeEach, afterEach, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { useCoreAccountStore, type AccountSummary } from './account'
const f = vi.hoisted(() => ({ api: vi.fn() }))
vi.mock('~/utils/api-client', () => ({ AUTH_TOKEN_COOKIE: 'qa', createApiRequest: () => f.api }))
beforeEach(() => {
  setActivePinia(createPinia())
  f.api.mockReset()
  vi.stubGlobal('useRuntimeConfig', () => ({}))
  vi.stubGlobal('useCookie', () => ref(''))
})
afterEach(() => vi.unstubAllGlobals())
it('keeps the last selected client when an earlier context response arrives late', async () => {
  const s = useCoreAccountStore()
  s.accounts = ['a', 'b'].map(
    (id) =>
      ({
        id,
        name: id,
        slug: id,
        organizationId: '',
        planCode: 'qa',
        modules: ['finance'],
        isAgency: false,
        organizationName: '',
      }) satisfies AccountSummary,
  )
  let resolve!: (value: unknown) => void
  f.api.mockImplementation((path: string) =>
    path.endsWith('=a')
      ? new Promise((done) => (resolve = done))
      : Promise.resolve({
          context: { account: s.accounts[1], permissions: ['finance.sheets.view'] },
        }),
  )
  const old = s.switchAccount('a')
  await s.switchAccount('b')
  resolve({ context: { account: s.accounts[0], permissions: ['core.account.manage'] } })
  await old
  expect(s.activeAccountId).toBe('b')
  expect(s.context?.account.id).toBe('b')
  expect(s.permissions).toEqual(['finance.sheets.view'])
  await expect(s.switchAccount('foreign')).rejects.toThrow()
  expect(s.activeAccountId).toBe('b')
})
