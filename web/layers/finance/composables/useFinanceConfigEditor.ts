// Editor de configuracao financeira (categorias, contas fixas, recorrencias).
//
// Dominio independente do editor de planilha: nunca toca no `draft` da planilha.
// A re-sincronizacao das linhas fixas/recorrentes com a planilha e disparada
// pela pagina (watchers em configDraft/clientRecurringEntries -> editor.syncAllFixedRows).
import type { InjectionKey } from 'vue'
import type {
  FinanceCategoryConfig,
  FinanceConfigKind,
  FinanceFixedAccountConfig,
  FinanceRecurringClientEntry,
  FinanceRecurringEntryConfig,
} from '../types/finances'
import { createFinanceUuid, normalizeFinanceLinkedUuid } from '../utils/finance-ids'
import { normalizeText, CONFIG_AUTOSAVE_DEBOUNCE_MS } from '../utils/finance-helpers'
import { useCoreAccountStore } from '../../core/stores/account'

export function useFinanceConfigEditor() {
  const coreAccount = useCoreAccountStore()
  const { config, loading, saving, errorMessage, fetchConfig, saveConfig } =
    useFinancesConfigManager()

  const targetCoreTenantId = computed(() => String(coreAccount.activeAccountId || '').trim())

  const configDraft = reactive<{
    categories: FinanceCategoryConfig[]
    fixedAccounts: FinanceFixedAccountConfig[]
    recurringEntries: FinanceRecurringEntryConfig[]
  }>({
    categories: [],
    fixedAccounts: [],
    recurringEntries: [],
  })

  const clientRecurringEntries = ref<FinanceRecurringClientEntry[]>([])
  const configOpen = ref(false)

  const editingCategoryId = ref<string | null>(null)
  const editingFixedId = ref<string | null>(null)
  const categoryEditDraft = reactive<{ id: string; name: string; kind: FinanceConfigKind }>({
    id: '',
    name: '',
    kind: 'ambas',
  })
  const fixedEditDraft = reactive<{
    id: string
    name: string
    kind: FinanceConfigKind
    categoryId: string
  }>({
    id: '',
    name: '',
    kind: 'saida',
    categoryId: '',
  })
  const newCategory = reactive<{ name: string; kind: FinanceConfigKind; description: string }>({
    name: '',
    kind: 'ambas',
    description: '',
  })
  const newFixed = reactive<{
    name: string
    kind: FinanceConfigKind
    categoryId: string
    defaultAmount: number
    notes: string
  }>({
    name: '',
    kind: 'saida',
    categoryId: '',
    defaultAmount: 0,
    notes: '',
  })

  let saveConfigTimer: ReturnType<typeof setTimeout> | null = null
  let configPersistInFlight = false
  let configPersistQueued = false

  const categoryConfigOptions = computed(() =>
    configDraft.categories.map((category) => ({
      label: category.name,
      value: category.id,
    })),
  )

  function resolveCategoryNameById(categoryId: string) {
    if (!categoryId) return ''
    return configDraft.categories.find((category) => category.id === categoryId)?.name || ''
  }

  function recurringEntryForTenant(sourceCoreTenantId: string) {
    const normalized = String(sourceCoreTenantId || '')
      .trim()
      .toLowerCase()
    if (!normalized) return undefined
    return configDraft.recurringEntries.find(
      (item) =>
        String(item.sourceCoreTenantId || '')
          .trim()
          .toLowerCase() === normalized,
    )
  }

  function syncConfigDraft() {
    const source = config.value
    if (!source) return
    configDraft.categories = (source.categories || []).map((item) => ({ ...item }))
    configDraft.fixedAccounts = (source.fixedAccounts || []).map((item) => ({
      ...item,
      members: (item.members || []).map((member) => ({ ...member })),
    }))
    configDraft.recurringEntries = (source.recurringEntries || []).map((item) => ({ ...item }))
  }

  function buildConfigPersistPayload() {
    return {
      coreTenantId: targetCoreTenantId.value || undefined,
      categories: configDraft.categories.map((category) => ({ ...category })),
      fixedAccounts: configDraft.fixedAccounts.map((account) => ({
        ...account,
        members: (account.members || []).map((member) => ({ ...member })),
      })),
      recurringEntries: configDraft.recurringEntries.map((entry) => ({ ...entry })),
    }
  }

  async function persistConfig() {
    if (configPersistInFlight) {
      configPersistQueued = true
      return
    }
    configPersistInFlight = true
    configPersistQueued = false
    try {
      await saveConfig(buildConfigPersistPayload())
    } finally {
      configPersistInFlight = false
      if (configPersistQueued) {
        configPersistQueued = false
        void persistConfig()
      }
    }
  }

  function queueConfigPersist() {
    if (saveConfigTimer) clearTimeout(saveConfigTimer)
    saveConfigTimer = setTimeout(() => {
      saveConfigTimer = null
      void persistConfig()
    }, CONFIG_AUTOSAVE_DEBOUNCE_MS)
  }

  async function fetchClientRecurringEntries() {
    try {
      const response = await $fetch<{
        status: 'success'
        data: Array<{
          id: string
          coreTenantId: string
          name: string
          monthlyPaymentAmount: number
          paymentDueDay: string
          billingMode: 'single' | 'per_store'
          stores: Array<{ id: string; name: string; amount: number }>
        }>
      }>('/api/admin/finance-config/recurring-clients', {
        query: {
          limit: 300,
          coreTenantId: targetCoreTenantId.value || undefined,
        },
      })

      clientRecurringEntries.value = (response.data || [])
        .map((client) => {
          const stores = Array.isArray(client.stores)
            ? client.stores
                .map((store) => ({
                  id: String(store.id || '').trim(),
                  name: String(store.name || '').trim(),
                  amount: Number(store.amount || 0),
                }))
                .filter((store) => store.name)
            : []
          const billingMode = client.billingMode === 'per_store' ? 'per_store' : 'single'

          return {
            id: String(client.id || '').trim(),
            coreTenantId: String(client.coreTenantId || client.id || '').trim(),
            name: String(client.name || 'Cliente sem nome').trim(),
            amount:
              billingMode === 'per_store'
                ? Number(
                    stores.reduce((sum, store) => sum + Number(store.amount || 0), 0).toFixed(2),
                  )
                : Number(client.monthlyPaymentAmount || 0),
            dueDay: String(client.paymentDueDay || ''),
            billingMode,
            stores,
          } as FinanceRecurringClientEntry
        })
        .filter((client) => Number(client.amount || 0) > 0)
    } catch {
      clientRecurringEntries.value = []
    }
  }

  async function loadConfig() {
    await Promise.all([fetchConfig(targetCoreTenantId.value), fetchClientRecurringEntries()])
    syncConfigDraft()
  }

  async function openConfigPanel() {
    await loadConfig()
    configOpen.value = true
  }

  function addCategory() {
    const name = normalizeText(newCategory.name, 120)
    if (!name) return
    if (
      configDraft.categories.some((category) => category.name.toLowerCase() === name.toLowerCase())
    )
      return

    configDraft.categories.push({
      id: createFinanceUuid(),
      name,
      kind: newCategory.kind,
      description: normalizeText(newCategory.description, 400),
    })
    newCategory.name = ''
    newCategory.kind = 'ambas'
    newCategory.description = ''
    queueConfigPersist()
  }

  function startEditCategory(id: string) {
    const target = configDraft.categories.find((category) => category.id === id)
    if (!target) return
    categoryEditDraft.id = target.id
    categoryEditDraft.name = target.name
    categoryEditDraft.kind = target.kind
    editingCategoryId.value = id
  }

  function finishEditCategory() {
    const id = editingCategoryId.value
    if (!id) return
    const target = configDraft.categories.find((category) => category.id === id)
    if (!target) return
    const nextName = normalizeText(categoryEditDraft.name, 120)
    if (!nextName) return

    target.name = nextName
    target.kind = categoryEditDraft.kind
    cancelEditCategory()
    queueConfigPersist()
  }

  function cancelEditCategory() {
    editingCategoryId.value = null
    categoryEditDraft.id = ''
    categoryEditDraft.name = ''
    categoryEditDraft.kind = 'ambas'
  }

  function removeCategory(categoryId: string) {
    configDraft.categories = configDraft.categories.filter((category) => category.id !== categoryId)
    configDraft.fixedAccounts = configDraft.fixedAccounts.map((account) => ({
      ...account,
      categoryId: account.categoryId === categoryId ? '' : account.categoryId,
    }))
    queueConfigPersist()
  }

  function addFixedAccount() {
    const name = normalizeText(newFixed.name, 120)
    if (!name) return

    configDraft.fixedAccounts.push({
      id: createFinanceUuid(),
      name,
      kind: newFixed.kind,
      categoryId: normalizeFinanceLinkedUuid(newFixed.categoryId),
      defaultAmount: Number(newFixed.defaultAmount || 0),
      notes: normalizeText(newFixed.notes, 500),
      members: [],
    })
    newFixed.name = ''
    newFixed.kind = 'saida'
    newFixed.categoryId = ''
    newFixed.defaultAmount = 0
    newFixed.notes = ''
    queueConfigPersist()
  }

  function startEditFixed(id: string) {
    const target = configDraft.fixedAccounts.find((account) => account.id === id)
    if (!target) return
    fixedEditDraft.id = target.id
    fixedEditDraft.name = target.name
    fixedEditDraft.kind = target.kind
    fixedEditDraft.categoryId = target.categoryId
    editingFixedId.value = id
  }

  function finishEditFixed() {
    const id = editingFixedId.value
    if (!id) return
    const target = configDraft.fixedAccounts.find((account) => account.id === id)
    if (!target) return
    const nextName = normalizeText(fixedEditDraft.name, 120)
    if (!nextName) return

    target.name = nextName
    target.kind = fixedEditDraft.kind
    target.categoryId = normalizeFinanceLinkedUuid(fixedEditDraft.categoryId)
    cancelEditFixed()
    queueConfigPersist()
  }

  function cancelEditFixed() {
    editingFixedId.value = null
    fixedEditDraft.id = ''
    fixedEditDraft.name = ''
    fixedEditDraft.kind = 'saida'
    fixedEditDraft.categoryId = ''
  }

  function removeFixedAccount(id: string) {
    configDraft.fixedAccounts = configDraft.fixedAccounts.filter((account) => account.id !== id)
    queueConfigPersist()
  }

  function addFixedMember(account: FinanceFixedAccountConfig) {
    account.members.push({
      id: createFinanceUuid(),
      name: `Item ${account.members.length + 1}`,
      amount: 0,
    })
    queueConfigPersist()
  }

  function updateFixedAmountFromMembers(
    account: FinanceFixedAccountConfig,
    options: { preserveWhenEmpty?: boolean; persist?: boolean } = {},
  ) {
    const preserveWhenEmpty = options.preserveWhenEmpty !== false
    const hasAnyMember = account.members.length > 0
    if (!hasAnyMember && preserveWhenEmpty) return

    const sum = account.members.reduce((total, member) => total + Number(member.amount || 0), 0)
    if (hasAnyMember || !preserveWhenEmpty) {
      account.defaultAmount = Number(sum.toFixed(2))
    }
    if (options.persist !== false) {
      queueConfigPersist()
    }
  }

  function removeFixedMember(account: FinanceFixedAccountConfig, memberId: string) {
    account.members = account.members.filter((member) => member.id !== memberId)
    updateFixedAmountFromMembers(account, { preserveWhenEmpty: true, persist: true })
  }

  function upsertRecurringEntry(
    sourceCoreTenantId: string,
    patch: Partial<FinanceRecurringEntryConfig>,
  ) {
    const normalizedTenantId = String(sourceCoreTenantId || '').trim()
    if (!normalizedTenantId) return
    const index = configDraft.recurringEntries.findIndex(
      (item) =>
        String(item.sourceCoreTenantId || '')
          .trim()
          .toLowerCase() === normalizedTenantId.toLowerCase(),
    )
    if (index < 0) {
      configDraft.recurringEntries.push({
        sourceCoreTenantId: normalizedTenantId,
        adjustmentAmount: Number(patch.adjustmentAmount || 0),
        notes: patch.notes || '',
      })
    } else {
      const entry = configDraft.recurringEntries[index]!
      if (patch.adjustmentAmount !== undefined)
        entry.adjustmentAmount = Number(patch.adjustmentAmount || 0)
      if (patch.notes !== undefined) entry.notes = patch.notes
    }
    queueConfigPersist()
  }

  function setRecurringAdjustment(sourceCoreTenantId: string, rawValue: number) {
    upsertRecurringEntry(sourceCoreTenantId, { adjustmentAmount: Number(rawValue || 0) })
  }

  function setRecurringNotes(sourceCoreTenantId: string, notes: string) {
    upsertRecurringEntry(sourceCoreTenantId, { notes: normalizeText(notes, 240) })
  }

  onScopeDispose(() => {
    if (saveConfigTimer) clearTimeout(saveConfigTimer)
  })

  return {
    config,
    configDraft,
    clientRecurringEntries,
    configOpen,
    loading,
    saving,
    errorMessage,
    targetCoreTenantId,
    editingCategoryId,
    categoryEditDraft,
    newCategory,
    editingFixedId,
    fixedEditDraft,
    newFixed,
    categoryConfigOptions,
    resolveCategoryNameById,
    recurringEntryForTenant,
    syncConfigDraft,
    loadConfig,
    openConfigPanel,
    fetchConfig,
    fetchClientRecurringEntries,
    queueConfigPersist,
    addCategory,
    startEditCategory,
    finishEditCategory,
    cancelEditCategory,
    removeCategory,
    addFixedAccount,
    startEditFixed,
    finishEditFixed,
    cancelEditFixed,
    removeFixedAccount,
    addFixedMember,
    removeFixedMember,
    updateFixedAmountFromMembers,
    setRecurringAdjustment,
    setRecurringNotes,
  }
}

export type FinanceConfigEditor = ReturnType<typeof useFinanceConfigEditor>
export const FINANCE_CONFIG_KEY: InjectionKey<FinanceConfigEditor> = Symbol('finance-config-editor')
