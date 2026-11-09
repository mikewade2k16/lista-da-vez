// Camada de dados das planilhas financeiras.
//
// FONTE: API Go real (back/internal/modules/finance, /v1/finance/sheets) via
// createApiRequest — substituiu o mock BFF Nitro em AC-12 (web/server/ removido).
// X-Account-Id entra pelo provider global; escopo (coreTenantId) via
// useCoreAccountStore. SEM realtime ainda (re-busca sob demanda; ver AGENT.md).
import type {
  FinanceDetailResponse,
  FinanceLineMutationResponse,
  FinanceLineItem,
  FinanceSheetItem,
  FinanceSheetListItem,
  FinancesListResponse,
  FinanceMutationResponse,
} from '../types/finances'
import {
  normalizeFinanceApiText as normalizeText,
  normalizePeriod,
  normalizeCoreTenantId,
  normalizeDate,
  normalizeRows,
  normalizeSheetId,
} from '../utils/finance-api-normalize'
import { useCoreAccountStore } from '../../core/stores/account'
import { useAuthStore } from '~/stores/auth'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

interface FinancesFetchOptions {
  q?: string
  coreTenantId?: string
  period?: string
}

interface FinanceCreatePayload {
  title?: string
  period?: string
  status?: string
  notes?: string
  entradas?: FinanceLineItem[]
  saidas?: FinanceLineItem[]
  coreTenantId?: string
}

interface FinanceUpdatePayload {
  expectedUpdatedAt?: string
  title?: string
  period?: string
  status?: string
  notes?: string
  entradas?: FinanceLineItem[]
  saidas?: FinanceLineItem[]
  coreTenantId?: string
}

interface FinanceLinePatchPayload {
  effective?: boolean
  effectiveDate?: string
}

const DEFAULT_FETCH_LIMIT = 240
const FINANCE_SHEETS_API_BASE = '/v1/finance/sheets'
const COLLECTION_KEY = '__collection__'

export function useFinancesManager() {
  const coreAccount = useCoreAccountStore()
  const auth = useAuthStore()
  const runtimeConfig = useRuntimeConfig()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const sheets = ref<FinanceSheetListItem[]>([])
  const activeSheet = ref<FinanceSheetItem | null>(null)
  const detailLoading = ref(false)
  const loading = ref(false)
  const creating = ref(false)
  const deletingId = ref<string | null>(null)
  const errorMessage = ref('')
  const savingMap = ref<Record<string, boolean>>({})
  let activeDetailRequestId = 0

  function scopedCoreTenantId(override?: string) {
    return normalizeCoreTenantId(override || coreAccount.activeAccountId)
  }

  function keyFor(id: string, field: 'fetch' | 'create' | 'update' | 'delete') {
    return `${id || COLLECTION_KEY}:${field}`
  }

  function setSaving(key: string, value: boolean) {
    const next = { ...savingMap.value }
    if (value) {
      next[key] = true
    } else {
      const remaining = Object.fromEntries(Object.entries(next).filter(([name]) => name !== key))
      savingMap.value = remaining
      return
    }
    savingMap.value = next
  }

  function isFinanceSheetDetail(
    value: FinanceSheetListItem | FinanceSheetItem,
  ): value is FinanceSheetItem {
    return (
      Array.isArray((value as FinanceSheetItem).entradas) &&
      Array.isArray((value as FinanceSheetItem).saidas)
    )
  }

  function toSheetListItem(payload: FinanceSheetListItem | FinanceSheetItem): FinanceSheetListItem {
    return {
      id: payload.id,
      title: payload.title,
      period: payload.period,
      status: payload.status,
      notes: payload.notes,
      coreTenantId: payload.coreTenantId,
      clientName: payload.clientName,
      summary: payload.summary,
      preview: payload.preview,
      createdAt: payload.createdAt,
      updatedAt: payload.updatedAt,
    }
  }

  function upsertSheet(payload: FinanceSheetListItem | FinanceSheetItem) {
    const normalized = toSheetListItem(payload)
    const index = sheets.value.findIndex((item) => item.id === normalized.id)
    if (index < 0) {
      sheets.value.unshift(normalized)
    } else {
      sheets.value[index] = normalized
    }

    if (isFinanceSheetDetail(payload) && activeSheet.value?.id === payload.id) {
      activeSheet.value = payload
    }
  }

  function replaceActiveSheetLine(
    line: FinanceLineItem,
    summary: FinanceSheetItem['summary'],
    preview: string,
    updatedAt: string,
  ) {
    if (!activeSheet.value) return

    const applyToCollection = (collection: FinanceLineItem[]) => {
      const index = collection.findIndex((item) => item.id === line.id)
      if (index < 0) return false
      collection[index] = {
        ...collection[index],
        ...line,
      }
      return true
    }

    if (!applyToCollection(activeSheet.value.entradas)) {
      applyToCollection(activeSheet.value.saidas)
    }

    activeSheet.value.summary = { ...summary }
    activeSheet.value.preview = preview
    activeSheet.value.updatedAt = updatedAt
    upsertSheet(activeSheet.value)
  }

  async function fetchSheetDetail(id: string) {
    const sheetId = normalizeSheetId(id)
    if (!sheetId) return null

    detailLoading.value = true
    errorMessage.value = ''
    const accountId = coreAccount.activeAccountId
    const requestId = activeDetailRequestId + 1
    activeDetailRequestId = requestId

    try {
      const path = `${FINANCE_SHEETS_API_BASE}/${sheetId}`
      const response = (await apiRequest(path, {
        headers: { 'X-Account-Id': accountId },
      })) as FinanceDetailResponse
      if (requestId !== activeDetailRequestId || accountId !== coreAccount.activeAccountId) {
        return response.data
      }

      activeSheet.value = response.data
      upsertSheet(response.data)
      return response.data
    } catch {
      if (requestId === activeDetailRequestId) {
        errorMessage.value = 'Falha ao carregar detalhes da planilha financeira.'
      }
      return null
    } finally {
      if (requestId === activeDetailRequestId) {
        detailLoading.value = false
      }
    }
  }

  async function fetchSheets(options: FinancesFetchOptions = {}) {
    loading.value = true
    errorMessage.value = ''
    setSaving(keyFor(COLLECTION_KEY, 'fetch'), true)

    const accountId = coreAccount.activeAccountId
    const coreTenantId = scopedCoreTenantId(options.coreTenantId)

    try {
      const response = (await apiRequest(FINANCE_SHEETS_API_BASE, {
        headers: { 'X-Account-Id': accountId },
        query: {
          page: 1,
          limit: DEFAULT_FETCH_LIMIT,
          q: normalizeText(options.q, 120),
          coreTenantId: coreTenantId || undefined,
          period: normalizePeriod(options.period),
        },
      })) as FinancesListResponse
      if (accountId !== coreAccount.activeAccountId) return
      sheets.value = Array.isArray(response.data) ? response.data : []
      if (activeSheet.value && !sheets.value.some((item) => item.id === activeSheet.value?.id)) {
        activeSheet.value = null
      }
    } catch {
      errorMessage.value = 'Falha ao carregar planilhas financeiras.'
    } finally {
      loading.value = false
      setSaving(keyFor(COLLECTION_KEY, 'fetch'), false)
    }
  }

  async function createSheet(payload: FinanceCreatePayload = {}) {
    creating.value = true
    errorMessage.value = ''
    setSaving(keyFor(COLLECTION_KEY, 'create'), true)

    try {
      const response = (await apiRequest(FINANCE_SHEETS_API_BASE, {
        method: 'POST',
        body: {
          title: normalizeText(payload.title, 180),
          period: normalizePeriod(payload.period),
          status: normalizeText(payload.status, 120),
          notes: normalizeText(payload.notes, 12000),
          entradas: normalizeRows(payload.entradas, 'entrada'),
          saidas: normalizeRows(payload.saidas, 'saida'),
          coreTenantId: scopedCoreTenantId(payload.coreTenantId) || undefined,
        },
      })) as FinanceMutationResponse

      upsertSheet(response.data)
      activeSheet.value = response.data
      return response.data
    } catch {
      errorMessage.value = 'Falha ao criar planilha financeira.'
      return null
    } finally {
      creating.value = false
      setSaving(keyFor(COLLECTION_KEY, 'create'), false)
    }
  }

  async function updateSheet(
    id: string,
    payload: FinanceUpdatePayload,
    accountId = coreAccount.activeAccountId,
  ) {
    const sheetId = normalizeSheetId(id)
    if (!sheetId) return null

    errorMessage.value = ''
    setSaving(keyFor(sheetId, 'update'), true)

    try {
      const response = (await apiRequest(`${FINANCE_SHEETS_API_BASE}/${sheetId}`, {
        method: 'PUT',
        headers: { 'X-Account-Id': accountId },
        body: {
          expectedUpdatedAt: payload.expectedUpdatedAt,
          title: normalizeText(payload.title, 180),
          period: normalizePeriod(payload.period),
          status: normalizeText(payload.status, 120),
          notes: normalizeText(payload.notes, 12000),
          entradas: normalizeRows(payload.entradas, 'entrada') || [],
          saidas: normalizeRows(payload.saidas, 'saida') || [],
          coreTenantId: payload.coreTenantId || undefined,
        },
      })) as FinanceMutationResponse

      if (coreAccount.activeAccountId !== accountId) return response.data
      upsertSheet(response.data)
      if (activeSheet.value?.id === response.data.id) {
        activeSheet.value = response.data
      }
      return response.data
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, 'Falha ao salvar planilha financeira.')
      return null
    } finally {
      setSaving(keyFor(sheetId, 'update'), false)
    }
  }

  async function deleteSheet(id: string) {
    const sheetId = normalizeSheetId(id)
    if (!sheetId) return false

    deletingId.value = sheetId
    errorMessage.value = ''
    setSaving(keyFor(sheetId, 'delete'), true)

    try {
      await apiRequest(`${FINANCE_SHEETS_API_BASE}/${sheetId}`, {
        method: 'DELETE',
      })

      sheets.value = sheets.value.filter((item) => item.id !== sheetId)
      if (activeSheet.value?.id === sheetId) {
        activeSheet.value = null
      }
      return true
    } catch {
      errorMessage.value = 'Falha ao excluir planilha financeira.'
      return false
    } finally {
      deletingId.value = null
      setSaving(keyFor(sheetId, 'delete'), false)
    }
  }

  async function updateSheetLine(
    id: string,
    lineId: string,
    payload: FinanceLinePatchPayload,
    accountId = coreAccount.activeAccountId,
  ) {
    const sheetId = normalizeSheetId(id)
    if (!sheetId) return null

    errorMessage.value = ''
    setSaving(keyFor(sheetId, 'update'), true)

    try {
      const response = (await apiRequest(`${FINANCE_SHEETS_API_BASE}/${sheetId}/lines/${lineId}`, {
        method: 'PATCH',
        headers: { 'X-Account-Id': accountId },
        body: {
          effective: Object.prototype.hasOwnProperty.call(payload, 'effective')
            ? Boolean(payload.effective)
            : undefined,
          effectiveDate: Object.prototype.hasOwnProperty.call(payload, 'effectiveDate')
            ? normalizeDate(payload.effectiveDate)
            : undefined,
        },
      })) as FinanceLineMutationResponse

      if (accountId !== coreAccount.activeAccountId || activeSheet.value?.id !== id)
        return response.data
      replaceActiveSheetLine(
        response.data.line,
        response.data.summary,
        response.data.preview,
        response.data.updatedAt,
      )
      return response.data
    } catch {
      errorMessage.value = 'Falha ao salvar linha financeira.'
      return null
    } finally {
      setSaving(keyFor(sheetId, 'update'), false)
    }
  }

  async function changeSheetStatus(id: string, status: string) {
    const response = (await apiRequest(`${FINANCE_SHEETS_API_BASE}/${id}/status`, {
      method: 'PATCH',
      body: { status, expectedUpdatedAt: activeSheet.value?.updatedAt },
    })) as FinanceMutationResponse
    upsertSheet(response.data)
    return response.data
  }

  watch(
    () => coreAccount.activeAccountId,
    () => {
      activeDetailRequestId++
      sheets.value = []
      activeSheet.value = null
      void fetchSheets()
    },
  )

  return {
    sheets,
    activeSheet,
    detailLoading,
    loading,
    creating,
    deletingId,
    errorMessage,
    savingMap,
    fetchSheets,
    fetchSheetDetail,
    createSheet,
    updateSheet,
    changeSheetStatus,
    updateSheetLine,
    deleteSheet,
  }
}
