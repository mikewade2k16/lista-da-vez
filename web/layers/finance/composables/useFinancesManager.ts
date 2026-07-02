// Camada de dados das planilhas financeiras.
//
// FONTE ATUAL: mock BFF Nitro (web/server/api/admin/finance-sheets). Adaptado do
// web-reference: usa `$fetch` (Nitro local, mesma origem) no lugar do antigo
// `useBffFetch`, escopo de cliente via useCoreAccountStore, e SEM realtime
// (o mock so re-busca sob demanda). Quando o back Go real entrar, trocar `$fetch`
// por createApiRequest (~/utils/api-client) com X-Account-Id e reativar realtime.
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
  isFinanceUuid,
  normalizeFinanceEntityId,
  normalizeFinanceLinkedUuid,
} from '../utils/finance-ids'
import { useCoreAccountStore } from '../../core/stores/account'

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
const FINANCE_SHEETS_API_BASE = '/api/admin/finance-sheets'
const COLLECTION_KEY = '__collection__'

function normalizeText(value: unknown, max = 12000) {
  return String(value ?? '')
    .replace(/\s+/g, ' ')
    .trim()
    .slice(0, max)
}

function normalizePeriod(value: unknown) {
  const raw = String(value ?? '').trim()
  return /^\d{4}-\d{2}$/.test(raw) ? raw : ''
}

function normalizeCoreTenantId(value: unknown) {
  return String(value ?? '')
    .trim()
    .toLowerCase()
}

function normalizeDate(value: unknown) {
  const raw = String(value ?? '').trim()
  if (!raw) return ''
  if (/^\d{4}-\d{2}-\d{2}$/.test(raw)) return raw
  return ''
}

function normalizeSheetId(value: unknown) {
  const raw = String(value ?? '')
    .trim()
    .toLowerCase()
  return isFinanceUuid(raw) ? raw : ''
}

function normalizeAmount(value: unknown, allowNegative = false) {
  if (typeof value === 'number' && Number.isFinite(value)) {
    const rounded = Number(value.toFixed(2))
    return allowNegative ? rounded : Math.max(0, rounded)
  }

  const raw = String(value ?? '').trim()
  if (!raw) return 0

  let normalized = raw
    .replace(/\s+/g, '')
    .replace(/^R\$/i, '')
    .replace(/[^\d,.-]/g, '')

  const hasComma = normalized.includes(',')
  const hasDot = normalized.includes('.')
  if (hasComma && hasDot) {
    if (normalized.lastIndexOf(',') > normalized.lastIndexOf('.')) {
      normalized = normalized.replace(/\./g, '').replace(',', '.')
    } else {
      normalized = normalized.replace(/,/g, '')
    }
  } else if (hasComma) {
    normalized = normalized.replace(/\./g, '').replace(',', '.')
  }

  const parsed = Number(normalized)
  if (!Number.isFinite(parsed)) return 0
  const rounded = Number(parsed.toFixed(2))
  return allowNegative ? rounded : Math.max(0, rounded)
}

function normalizeAdjustments(value: unknown) {
  if (!Array.isArray(value)) return []

  return value.map((item) => {
    const source =
      item && typeof item === 'object'
        ? (item as {
            id?: unknown
            amount?: unknown
            note?: unknown
            date?: unknown
          })
        : {}

    return {
      id: normalizeFinanceEntityId(source.id),
      amount: normalizeAmount(source.amount, true),
      note: normalizeText(source.note, 240),
      date: normalizeDate(source.date),
    }
  })
}

function normalizeRows(value: FinanceLineItem[] | undefined, _kind: 'entrada' | 'saida') {
  if (!Array.isArray(value)) return undefined

  return value.map((item) => {
    const adjustments = normalizeAdjustments(item?.adjustments)
    const adjustmentAmount =
      adjustments.length > 0
        ? Number(adjustments.reduce((sum, adjustment) => sum + adjustment.amount, 0).toFixed(2))
        : normalizeAmount(item?.adjustmentAmount, true)

    return {
      adjustments,
      id: normalizeFinanceEntityId(item?.id),
      description: normalizeText(item?.description, 260),
      category: normalizeText(item?.category, 120),
      effective: Boolean(item?.effective),
      effectiveDate: normalizeDate(item?.effectiveDate),
      amount: normalizeAmount(item?.amount, false),
      adjustmentAmount,
      fixedAccountId: normalizeFinanceLinkedUuid(item?.fixedAccountId),
      details: normalizeText(item?.details, 600),
    }
  })
}

export function useFinancesManager() {
  const coreAccount = useCoreAccountStore()

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
      delete next[key]
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
    const requestId = activeDetailRequestId + 1
    activeDetailRequestId = requestId

    try {
      const response = await $fetch<FinanceDetailResponse>(`${FINANCE_SHEETS_API_BASE}/${sheetId}`)
      if (requestId !== activeDetailRequestId) {
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

    const coreTenantId = scopedCoreTenantId(options.coreTenantId)

    try {
      const response = await $fetch<FinancesListResponse>(FINANCE_SHEETS_API_BASE, {
        query: {
          page: 1,
          limit: DEFAULT_FETCH_LIMIT,
          q: normalizeText(options.q, 120),
          coreTenantId: coreTenantId || undefined,
          period: normalizePeriod(options.period),
        },
      })
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
      const response = await $fetch<FinanceMutationResponse>(FINANCE_SHEETS_API_BASE, {
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
      })

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

  async function updateSheet(id: string, payload: FinanceUpdatePayload) {
    const sheetId = normalizeSheetId(id)
    if (!sheetId) return null

    errorMessage.value = ''
    setSaving(keyFor(sheetId, 'update'), true)

    try {
      const response = await $fetch<FinanceMutationResponse>(
        `${FINANCE_SHEETS_API_BASE}/${sheetId}`,
        {
          method: 'PUT',
          body: {
            title: normalizeText(payload.title, 180),
            period: normalizePeriod(payload.period),
            status: normalizeText(payload.status, 120),
            notes: normalizeText(payload.notes, 12000),
            entradas: normalizeRows(payload.entradas, 'entrada') || [],
            saidas: normalizeRows(payload.saidas, 'saida') || [],
            coreTenantId: scopedCoreTenantId(payload.coreTenantId) || undefined,
          },
        },
      )

      upsertSheet(response.data)
      if (activeSheet.value?.id === response.data.id) {
        activeSheet.value = response.data
      }
      return response.data
    } catch {
      errorMessage.value = 'Falha ao salvar planilha financeira.'
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
      await $fetch<{ status: 'success' }>(`${FINANCE_SHEETS_API_BASE}/${sheetId}`, {
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

  async function updateSheetLine(id: string, lineId: string, payload: FinanceLinePatchPayload) {
    const sheetId = normalizeSheetId(id)
    if (!sheetId) return null

    errorMessage.value = ''
    setSaving(keyFor(sheetId, 'update'), true)

    try {
      const response = await $fetch<FinanceLineMutationResponse>(
        `${FINANCE_SHEETS_API_BASE}/${sheetId}/lines/${lineId}`,
        {
          method: 'PATCH',
          body: {
            effective: Object.prototype.hasOwnProperty.call(payload, 'effective')
              ? Boolean(payload.effective)
              : undefined,
            effectiveDate: Object.prototype.hasOwnProperty.call(payload, 'effectiveDate')
              ? normalizeDate(payload.effectiveDate)
              : undefined,
          },
        },
      )

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

  watch(
    () => coreAccount.activeAccountId,
    () => {
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
    updateSheetLine,
    deleteSheet,
  }
}
