import { reactive, ref } from 'vue'
import { expect, it } from 'vitest'
import { useFinanceSheetSources } from './useFinanceSheetSources'
import { makeLine } from '../utils/finance-helpers'
import { financeRecurringRowId } from '../utils/finance-ids'
import type { FinanceRecurringClientEntry } from '../types/finances'

it('does not rewrite persisted amounts when configuration changes', () => {
  const configDraft = reactive({
    categories: [],
    fixedAccounts: [
      {
        id: 'fixed',
        name: 'Monthly',
        kind: 'saida' as const,
        categoryId: '',
        defaultAmount: 100,
        notes: '',
        members: [],
      },
    ],
    recurringEntries: [],
  })
  const draft = reactive({
    entradas: [],
    saidas: [{ ...makeLine(), fixedAccountId: 'fixed', amount: 80, description: 'Old snapshot' }],
  })
  const sources = useFinanceSheetSources({
    configDraft,
    draft,
    clientRecurringEntries: ref([]),
    selectedSheetId: ref('november'),
    queuePersist: () => {},
  })
  configDraft.fixedAccounts[0]!.defaultAmount = 300
  expect(draft.saidas[0]!.amount).toBe(80)
  expect(draft.saidas[0]!.description).toBe('Old snapshot')
  sources.syncAllFixedRows()
  expect(draft.saidas.find((row) => row.fixedAccountId === 'fixed')?.amount).toBe(300)
})
it('initializes new monthly sheets with different recurring keys and preserves legacy rows', () => {
  const clients = ref<FinanceRecurringClientEntry[]>([
    {
      id: 'client',
      coreTenantId: 'client',
      name: 'Test client',
      amount: 100,
      dueDay: '10',
      billingMode: 'single',
      stores: [],
    },
  ])
  const build = (id: string, existing = false) => {
    const draft = reactive({
      entradas: existing
        ? [{ ...makeLine(), id: financeRecurringRowId('client'), amount: 75 }]
        : [],
      saidas: [],
    })
    const src = useFinanceSheetSources({
      configDraft: { categories: [], fixedAccounts: [], recurringEntries: [] },
      draft,
      clientRecurringEntries: clients,
      selectedSheetId: ref(id),
      queuePersist: () => {},
    })
    src.syncAllFixedRows()
    return draft.entradas.find((row) => row.description === 'Mensalidade Test client')!
  }
  expect(build('november').id).not.toBe(build('december').id)
  expect(build('legacy', true).id).toBe(financeRecurringRowId('client'))
})
