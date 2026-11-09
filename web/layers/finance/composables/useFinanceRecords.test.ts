import { effectScope, nextTick, reactive } from 'vue'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { useFinanceReceipts, useFinanceRetention, useFiscalCustomers } from './useFinanceRecords'

const f = vi.hoisted(() => ({
  account: null as unknown as { activeAccountId: string },
  store: {
    listReceipts: vi.fn(),
    unlinkReceipt: vi.fn(),
    removeOriginal: vi.fn(),
    getRetention: vi.fn(),
    retentionHistory: vi.fn(),
    updateRetention: vi.fn(),
    listCustomers: vi.fn(),
    customerVersions: vi.fn(),
    updateCustomer: vi.fn(),
  },
}))
vi.mock('vue', async (original) => ({
  ...(await original<typeof import('vue')>()),
  onMounted: (fn: () => void) => fn(),
  onBeforeUnmount: () => {},
}))
vi.mock('../../core/stores/account', () => ({ useCoreAccountStore: () => f.account }))
vi.mock('~/stores/finance-records', () => ({ useFinanceRecordsStore: () => f.store }))
vi.mock('~/utils/api-client', () => ({
  getApiErrorMessage: (_: unknown, fallback: string) => fallback,
}))

const scopes: ReturnType<typeof effectScope>[] = []
function run<T>(fn: () => T): T {
  const scope = effectScope()
  scopes.push(scope)
  return scope.run(fn) as T
}
const flush = async () => {
  for (let i = 0; i < 5; i++) await nextTick()
}
const settings = (days: number, version: number) => ({
  receiptOriginalDays: days,
  version,
  isDefault: version === 0,
  stored: 1,
  storedBytes: 10,
  purged: 0,
  notStored: 0,
  storageAvailable: true,
})

beforeEach(() => {
  f.account = reactive({ activeAccountId: 'acc-a' })
  Object.values(f.store).forEach((fn) => fn.mockReset())
})
afterEach(() => scopes.splice(0).forEach((scope) => scope.stop()))

it('retention keeps the user edit while reloading and saves with the read version', async () => {
  f.store.getRetention.mockResolvedValue(settings(90, 0))
  const retention = run(() => useFinanceRetention())
  await flush()
  expect(retention.days.value).toBe(90)
  retention.days.value = 30
  retention.touched.value = true
  expect(retention.dirty.value).toBe(true)
  f.store.updateRetention.mockResolvedValue(settings(30, 1))
  await retention.save()
  expect(f.store.updateRetention).toHaveBeenCalledWith('acc-a', 30, 0)
  expect(retention.settings.value?.version).toBe(1)
  expect(retention.dirty.value).toBe(false)
  retention.days.value = 0
  await retention.save()
  expect(retention.error.value).toContain('entre 1 e 3650')
})

it('receipts drop the old account response when the account changes', async () => {
  let resolveOld: (value: unknown) => void = () => {}
  f.store.listReceipts.mockImplementationOnce(
    () => new Promise((resolve) => (resolveOld = resolve)),
  )
  const receipts = run(() => useFinanceReceipts())
  f.store.listReceipts.mockResolvedValueOnce({
    items: [{ document: { id: 'b1' } }],
    page: 1,
    more: false,
  })
  f.account.activeAccountId = 'acc-b'
  await flush()
  resolveOld({ items: [{ document: { id: 'a1' } }], page: 1, more: false })
  await flush()
  expect(receipts.items.value.map((item) => item.document.id)).toEqual(['b1'])
})

it('receipt actions reload from the server after success and report failures', async () => {
  f.store.listReceipts.mockResolvedValue({ items: [], page: 1, more: false })
  const receipts = run(() => useFinanceReceipts())
  await flush()
  f.store.unlinkReceipt.mockResolvedValue({ unlinked: true })
  await receipts.unlink('d1')
  expect(f.store.listReceipts).toHaveBeenCalledTimes(2)
  expect(receipts.status.value).toContain('continua na planilha')
  f.store.removeOriginal.mockRejectedValue(new Error('boom'))
  await receipts.removeOriginal('d1')
  expect(receipts.error.value).toBe('Não foi possível concluir a ação.')
})

it('customer edit sends the expected version and replaces the item', async () => {
  const customer = {
    id: 'c1',
    name: 'João',
    document: '52998224725',
    serviceDescription: '',
    amount: 500,
    version: 1,
    lastUsedAt: '',
    updatedAt: '',
  }
  f.store.listCustomers.mockResolvedValue([customer])
  const state = run(() => useFiscalCustomers())
  await flush()
  state.edit(customer)
  state.draft.value.amount = 650
  f.store.updateCustomer.mockResolvedValue({ ...customer, amount: 650, version: 2 })
  await state.save(customer)
  expect(f.store.updateCustomer).toHaveBeenCalledWith('acc-a', 'c1', {
    name: 'João',
    serviceDescription: '',
    amount: 650,
    expectedVersion: 1,
  })
  expect(state.customers.value[0]?.version).toBe(2)
  expect(state.editingId.value).toBe('')
  expect(state.status.value).toBe('Salvo como versão 2.')
})
