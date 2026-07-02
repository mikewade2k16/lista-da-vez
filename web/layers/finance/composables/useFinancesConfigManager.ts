// Camada de dados da configuracao financeira (categorias, contas fixas, recorrencias).
//
// FONTE ATUAL: mock BFF Nitro (web/server/api/admin/finance-config). Adaptado do
// web-reference: `$fetch` no lugar de `useBffFetch`, escopo por useCoreAccountStore.
import type {
  FinanceCategoryConfig,
  FinanceRecurringEntryConfig,
  FinanceConfigResponse,
  FinanceFixedAccountConfig,
} from '../types/finances'
import { isFinanceUuid, normalizeFinanceEntityId } from '../utils/finance-ids'
import { useCoreAccountStore } from '../../core/stores/account'

interface FinanceConfigState {
  categories: FinanceCategoryConfig[]
  fixedAccounts: FinanceFixedAccountConfig[]
  recurringEntries: FinanceRecurringEntryConfig[]
  updatedAt: string
}

const FINANCE_CONFIG_API_BASE = '/api/admin/finance-config'

function normalizeText(value: unknown, max = 300) {
  return String(value ?? '')
    .replace(/\s+/g, ' ')
    .trim()
    .slice(0, max)
}

function normalizeAmount(value: unknown) {
  const parsed = Number(value ?? 0)
  if (!Number.isFinite(parsed)) return 0
  return Number(parsed.toFixed(2))
}

export function useFinancesConfigManager() {
  const coreAccount = useCoreAccountStore()
  const loading = ref(false)
  const saving = ref(false)
  const errorMessage = ref('')
  const config = ref<FinanceConfigState | null>(null)

  function resolveScopedCoreTenantId(coreTenantId?: string) {
    return normalizeText(coreTenantId, 90) || normalizeText(coreAccount.activeAccountId, 90)
  }

  async function fetchConfig(coreTenantId?: string) {
    loading.value = true
    errorMessage.value = ''

    try {
      const resolvedCoreTenantId = resolveScopedCoreTenantId(coreTenantId)
      const response = await $fetch<FinanceConfigResponse>(FINANCE_CONFIG_API_BASE, {
        query: {
          coreTenantId: resolvedCoreTenantId || undefined,
        },
      })
      config.value = response.data
      return response.data
    } catch {
      errorMessage.value = 'Falha ao carregar configuracoes financeiras.'
      return null
    } finally {
      loading.value = false
    }
  }

  async function saveConfig(payload: {
    coreTenantId?: string
    categories?: FinanceCategoryConfig[]
    fixedAccounts?: FinanceFixedAccountConfig[]
    recurringEntries?: FinanceRecurringEntryConfig[]
  }) {
    saving.value = true
    errorMessage.value = ''

    try {
      const normalizedCategories = Array.isArray(payload.categories)
        ? payload.categories.map((category) => {
            const rawId = normalizeText(category.id, 90)
            return {
              rawId,
              id: normalizeFinanceEntityId(rawId),
              name: normalizeText(category.name, 120),
              kind: category.kind,
              description: normalizeText(category.description, 400),
            }
          })
        : undefined
      const categoryIdMap = new Map(
        (normalizedCategories || []).map((category) => [category.rawId, category.id] as const),
      )
      const coreTenantId = resolveScopedCoreTenantId(payload.coreTenantId)

      const response = await $fetch<FinanceConfigResponse>(FINANCE_CONFIG_API_BASE, {
        method: 'PUT',
        body: {
          coreTenantId: coreTenantId || undefined,
          categories: normalizedCategories
            ? normalizedCategories.map((category) => ({
                id: category.id,
                name: category.name,
                kind: category.kind,
                description: category.description,
              }))
            : undefined,
          fixedAccounts: Array.isArray(payload.fixedAccounts)
            ? payload.fixedAccounts.map((account) => ({
                id: normalizeFinanceEntityId(account.id),
                name: normalizeText(account.name, 120),
                kind: account.kind,
                categoryId: (() => {
                  const rawCategoryId = normalizeText(account.categoryId, 90)
                  if (!rawCategoryId) return ''
                  if (categoryIdMap.has(rawCategoryId))
                    return categoryIdMap.get(rawCategoryId) || ''
                  return isFinanceUuid(rawCategoryId) ? rawCategoryId.toLowerCase() : ''
                })(),
                defaultAmount: normalizeAmount(account.defaultAmount),
                notes: normalizeText(account.notes, 500),
                members: Array.isArray(account.members)
                  ? account.members.map((member) => ({
                      id: normalizeFinanceEntityId(member.id),
                      name: normalizeText(member.name, 120),
                      amount: normalizeAmount(member.amount),
                    }))
                  : [],
              }))
            : undefined,
          recurringEntries: Array.isArray(payload.recurringEntries)
            ? payload.recurringEntries.map((entry) => ({
                sourceCoreTenantId: normalizeText(entry.sourceCoreTenantId, 90) || undefined,
                adjustmentAmount: normalizeAmount(entry.adjustmentAmount),
                notes: normalizeText(entry.notes, 240),
              }))
            : undefined,
        },
      })

      config.value = response.data
      return response.data
    } catch {
      errorMessage.value = 'Falha ao salvar configuracoes financeiras.'
      return null
    } finally {
      saving.value = false
    }
  }

  return {
    config,
    loading,
    saving,
    errorMessage,
    fetchConfig,
    saveConfig,
  }
}
