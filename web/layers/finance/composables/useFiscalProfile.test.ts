import { effectScope, nextTick, reactive } from 'vue'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { emptyFiscalProfile } from '../types/fiscal'
import type { FiscalProfileView } from '../types/fiscal'
import { useFiscalProfile } from './useFiscalProfile'

const fixtures = vi.hoisted(() => ({
  account: null as unknown as {
    activeAccountId: string
    enabledModules: string[]
    permissions: string[]
  },
  auth: { role: 'platform_admin', user: { id: 'user' } },
  store: {
    getProfile: vi.fn(),
    portalStatus: vi.fn(),
    testPortal: vi.fn(),
    connectPortal: vi.fn(),
    portalSessionStatus: vi.fn(),
    listTemplates: vi.fn(),
    saveProfile: vi.fn(),
    saveCredentials: vi.fn(),
    deleteCredentials: vi.fn(),
    saveTemplate: vi.fn(),
    deleteTemplate: vi.fn(),
  },
}))
vi.mock('vue', async (original) => ({
  ...(await original<typeof import('vue')>()),
  onMounted: (callback: () => void) => callback(),
  onBeforeUnmount: () => {},
}))
vi.mock('~/stores/auth', () => ({ useAuthStore: () => fixtures.auth }))
vi.mock('../../core/stores/account', () => ({ useCoreAccountStore: () => fixtures.account }))
vi.mock('~/stores/fiscal-profile', () => ({ useFiscalProfileStore: () => fixtures.store }))
vi.mock('~/utils/api-client', () => ({ getApiErrorMessage: () => 'Falha ao salvar' }))
const scopes: ReturnType<typeof effectScope>[] = []
function editor() {
  const scope = effectScope()
  scopes.push(scope)
  return scope.run(() => useFiscalProfile())!
}
function profile(name: string): FiscalProfileView {
  return {
    profile: { ...emptyFiscalProfile(), legalName: name },
    updatedAt: 'revision',
    credentialsSet: false,
    credentialsUpdatedAt: '',
    missingFields: [],
  }
}
beforeEach(() => {
  vi.clearAllMocks()
  fixtures.account = reactive({
    activeAccountId: 'account-a',
    enabledModules: ['finance'],
    permissions: ['finance.sheets.view'],
  })
  fixtures.store.getProfile.mockResolvedValue(profile('Account A'))
  fixtures.store.listTemplates.mockResolvedValue([])
})
afterEach(() => scopes.splice(0).forEach((scope) => scope.stop()))
it('does not fetch for an account without the module, including platform admin', async () => {
  fixtures.account.enabledModules = []
  const state = editor()
  await nextTick()
  expect(state.enabled.value).toBe(false)
  expect(fixtures.store.getProfile).not.toHaveBeenCalled()
})
it('discards a previous account response and clears the draft on context change', async () => {
  let resolveOld!: (value: FiscalProfileView) => void
  fixtures.store.getProfile.mockImplementation((account: string) =>
    account === 'account-a'
      ? new Promise<FiscalProfileView>((resolve) => {
          resolveOld = resolve
        })
      : Promise.resolve(profile('Account B')),
  )
  const state = editor()
  fixtures.account.activeAccountId = 'account-b'
  await vi.waitFor(() => expect(state.draft.legalName).toBe('Account B'))
  resolveOld(profile('Old Account A'))
  await nextTick()
  await nextTick()
  expect(state.draft.legalName).toBe('Account B')
})
it('captures scope and payload while saving; an old response cannot hydrate the new account', async () => {
  const state = editor()
  await vi.waitFor(() => expect(state.view.value).not.toBeNull())
  let finish!: (value: FiscalProfileView) => void
  fixtures.store.saveProfile.mockImplementation(
    () =>
      new Promise<FiscalProfileView>((resolve) => {
        finish = resolve
      }),
  )
  state.draft.legalName = 'Edited A'
  const saving = state.saveProfile()
  fixtures.account.activeAccountId = 'account-b'
  fixtures.store.getProfile.mockResolvedValue(profile('Account B'))
  await vi.waitFor(() => expect(state.view.value).not.toBeNull())
  finish(profile('Saved A'))
  await saving
  expect(fixtures.store.saveProfile).toHaveBeenCalledWith(
    'account-a',
    expect.objectContaining({
      profile: expect.objectContaining({ legalName: 'Edited A' }),
      expectedUpdatedAt: 'revision',
    }),
  )
  expect(state.draft.legalName).not.toBe('Saved A')
})
it('retains an unsaved profile when the request fails', async () => {
  const state = editor()
  await vi.waitFor(() => expect(state.view.value).not.toBeNull())
  fixtures.store.saveProfile.mockRejectedValue(new Error('network'))
  state.draft.legalName = 'Unsaved'
  state.profileDirty.value = true
  expect(await state.saveProfile()).toBe(false)
  expect(state.draft.legalName).toBe('Unsaved')
  expect(state.profileDirty.value).toBe(true)
  expect(state.error.value).toBe('Falha ao salvar')
})

it('tests the saved access version and drops a late result after switching accounts', async () => {
  const state = editor()
  await vi.waitFor(() => expect(state.view.value).not.toBeNull())
  let finish!: (value: {
    status: string
    message: string
    checkedAt: string
    profileVersion: string
  }) => void
  fixtures.store.testPortal.mockImplementation(
    () =>
      new Promise((resolve) => {
        finish = resolve
      }),
  )
  const pending = state.testPortalAccess()
  expect(fixtures.store.testPortal).toHaveBeenCalledWith('account-a', 'revision')
  fixtures.account.activeAccountId = 'account-b'
  await nextTick()
  await vi.waitFor(() => expect(state.view.value).not.toBeNull())
  finish({ status: 'authenticated', message: 'QA', checkedAt: 'test', profileVersion: 'revision' })
  await pending
  expect(state.portalResult.value).toBeNull()
})

it('uses saved credentials to connect and does not send a password in the session request', async () => {
  const state = editor()
  await vi.waitFor(() => expect(state.view.value).not.toBeNull())
  fixtures.store.connectPortal.mockResolvedValue({
    status: 'authenticated',
    message: 'Sessão salva',
    expiresAt: 'test',
    verifiedAt: 'test',
    reused: true,
  })
  expect(await state.connectPortalSession()).toBe(true)
  expect(fixtures.store.connectPortal).toHaveBeenCalledWith('account-a', 'revision')
  expect(state.sessionResult.value?.reused).toBe(true)
})
