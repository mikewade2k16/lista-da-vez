import { computed, effectScope, nextTick, reactive, ref, watch } from 'vue'
import { afterEach, expect, it, vi } from 'vitest'
import { useFinanceSheetEditor } from './useFinanceSheetEditor'
import type { FinanceSheetItem } from '../types/finances'

vi.mock('../../core/stores/account', () => ({
  useCoreAccountStore: () => ({ activeAccountId: 'account', permissions: [] }),
}))
vi.mock('~/stores/auth', () => ({ useAuthStore: () => ({ role: 'platform_admin' }) }))
vi.mock('./useFinanceAutosave', () => ({
  useFinanceAutosave: () => ({
    state: reactive({ pending: false, saving: false, failed: false }),
    schedule: vi.fn(),
    flush: vi.fn(async () => true),
    has: () => false,
    label: ref('Salvo'),
  }),
}))
afterEach(() => vi.unstubAllGlobals())

it('keeps configured rows when selection hydrates a newly created sheet', async () => {
  for (const [name, value] of Object.entries({ computed, nextTick, reactive, ref, watch }))
    vi.stubGlobal(name, value)
  const sheet: FinanceSheetItem = {
    id: 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa',
    title: 'New',
    period: '2026-11',
    status: 'aberta',
    notes: '',
    clientName: '',
    preview: '',
    createdAt: '',
    updatedAt: '',
    entradas: [],
    saidas: [],
    summary: {
      expectedIn: 0,
      effectiveIn: 0,
      expectedOut: 0,
      effectiveOut: 0,
      expectedBalance: 0,
      effectiveBalance: 0,
    },
  }
  const activeSheet = ref<FinanceSheetItem | null>(null)
  const sheets = ref<FinanceSheetItem[]>([])
  vi.stubGlobal('useFinancesManager', () => ({
    activeSheet,
    sheets,
    detailLoading: ref(false),
    creating: ref(false),
    deletingId: ref(null),
    errorMessage: ref(''),
    fetchSheets: vi.fn(),
    fetchSheetDetail: vi.fn(),
    updateSheet: vi.fn(),
    updateSheetLine: vi.fn(),
    changeSheetStatus: vi.fn(),
    deleteSheet: vi.fn(),
    createSheet: vi.fn(async () => {
      activeSheet.value = sheet
      sheets.value = [sheet]
      return sheet
    }),
  }))
  const scope = effectScope()
  try {
    const editor = scope.run(() =>
      useFinanceSheetEditor({
        configDraft: {
          categories: [],
          recurringEntries: [],
          fixedAccounts: [
            {
              id: 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb',
              name: 'Fixed test',
              kind: 'saida',
              categoryId: '',
              defaultAmount: 125,
              notes: '',
              members: [],
            },
          ],
        },
        clientRecurringEntries: ref([]),
      }),
    )!
    await editor.onCreateSheet()
    await nextTick()
    expect(editor.draft.saidas.find((row) => row.fixedAccountId)?.amount).toBe(125)
  } finally {
    scope.stop()
  }
})
