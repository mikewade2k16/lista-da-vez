import { beforeEach, describe, expect, it, vi } from 'vitest'

const ownerAccountId = '00000000-0000-4000-8000-000000000001'
const clientAccountId = '00000000-0000-4000-8000-000000000002'

const authStore = {
  role: 'platform_admin',
  effectivePermissionKeys: [] as string[],
}

const accountStore = {
  activeAccountId: ownerAccountId,
  activeAccount: {
    id: ownerAccountId,
    name: 'Agencia',
    organizationName: 'Agencia',
    isAgency: true,
  },
  context: { account: { id: ownerAccountId } },
  platformView: false,
  enabledModules: ['customer_data', 'customer_intelligence'],
  accounts: [
    {
      id: ownerAccountId,
      name: 'Agencia',
      organizationName: 'Agencia',
      isAgency: true,
    },
    {
      id: clientAccountId,
      name: 'Cliente',
      organizationName: 'Agencia',
      isAgency: false,
    },
  ],
}

const intelligenceStore = {
  ownerAccountId: '',
  clientAccountId: '',
  setScope: vi.fn((owner: string, client: string) => {
    intelligenceStore.ownerAccountId = owner
    intelligenceStore.clientAccountId = client
  }),
  setReadAccess: vi.fn(),
}

vi.mock('~/stores/auth', () => ({
  useAuthStore: () => authStore,
}))

vi.mock('~/stores/customer-intelligence', () => ({
  useCustomerIntelligenceStore: () => intelligenceStore,
}))

vi.mock('../../../layers/core/stores/account', () => ({
  useCoreAccountStore: () => accountStore,
}))

describe('customer intelligence client scope stability', () => {
  beforeEach(() => {
    vi.resetModules()
    intelligenceStore.ownerAccountId = ''
    intelligenceStore.clientAccountId = ''
    intelligenceStore.setScope.mockClear()
    intelligenceStore.setReadAccess.mockClear()
  })

  it('does not clear an agency client when another component creates the access composable', async () => {
    const { useCustomerIntelligenceAccess } = await import('./useCustomerIntelligenceAccess')

    const first = useCustomerIntelligenceAccess()
    first.selectClient(clientAccountId)
    expect(intelligenceStore.clientAccountId).toBe(clientAccountId)

    useCustomerIntelligenceAccess()

    expect(intelligenceStore.ownerAccountId).toBe(ownerAccountId)
    expect(intelligenceStore.clientAccountId).toBe(clientAccountId)
  })
})
