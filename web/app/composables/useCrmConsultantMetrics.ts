import { computed, ref, watch, type ComputedRef, type Ref } from 'vue'

import { useCrmStore } from '~/stores/crm'
import type { CRMConsultantMetric, QueueConsultantStats, QueueStats } from '~/stores/crm'
import { useErpStore } from '~/stores/erp'
import type { ErpConsultantLinkEmployee } from '~/stores/erp'
import { useUiStore } from '~/stores/ui'

const managementStoreSlug = 'gerencia-multiloja'

export type MergedCrmConsultant = CRMConsultantMetric & {
  queue?: QueueConsultantStats | null
  matched: boolean
  linkEmployee?: ErpConsultantLinkEmployee | null
}

type UseCrmConsultantMetricsOptions = {
  consultantRows: ComputedRef<CRMConsultantMetric[]>
  queueStats: ComputedRef<QueueStats | null>
  ready: Ref<boolean>
  canManageConsultantLinks: ComputedRef<boolean>
}

function normalizeConsultantLookupKey(value: unknown) {
  return String(value || '')
    .trim()
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, ' ')
    .trim()
}

function normalizeConsultantLinkEmployeeId(value: unknown) {
  return String(value || '')
    .trim()
    .toLowerCase()
}

function normalizeConsultantLinkStoreCode(value: unknown) {
  const normalized = String(value || '')
    .trim()
    .toLowerCase()
  const digits = normalized.replace(/\D+/g, '')
  if (digits.length >= 8) return digits
  return normalized
}

function normalizeQueueStoreKey(value: unknown) {
  return String(value || '')
    .trim()
    .toLowerCase()
}

export function consultantLinkKey(storeCode: unknown, employeeId: unknown) {
  const normalizedEmployeeId = normalizeConsultantLinkEmployeeId(employeeId)
  if (!normalizedEmployeeId) return ''
  return `${normalizeConsultantLinkStoreCode(storeCode)}\u0000${normalizedEmployeeId}`
}

function queueConsultantKey(storeKey: unknown, consultantKey: unknown) {
  const normalizedConsultantKey = String(consultantKey || '').trim()
  if (!normalizedConsultantKey) return ''
  return `${normalizeQueueStoreKey(storeKey)}\u0000${normalizedConsultantKey}`
}

function hasResolvedQueueIdentity(row: CRMConsultantMetric | MergedCrmConsultant) {
  return Boolean(String(row.profileConsultantId || row.profileConsultantName || '').trim())
}

export function linkStatusLabel(status?: string | null) {
  switch (status) {
    case 'manual':
      return 'manual'
    case 'employee_code':
      return 'codigo'
    case 'name_exact':
      return 'nome'
    case 'ambiguous':
      return 'ambiguo'
    case 'unmatched':
      return 'sem vinculo'
    default:
      return 'pendente'
  }
}

export function linkStatusClass(status?: string | null) {
  switch (status) {
    case 'manual':
    case 'employee_code':
      return 'crm-badge--ok'
    case 'name_exact':
      return 'crm-badge--info'
    case 'ambiguous':
      return 'crm-badge--warn'
    default:
      return 'crm-badge--neutral'
  }
}

export function useCrmConsultantMetrics(options: UseCrmConsultantMetricsOptions) {
  const crmStore = useCrmStore()
  const erpStore = useErpStore()
  const ui = useUiStore()
  const consultantLinkDraftByRow = ref<Record<string, string>>({})

  const managementConsultantRows = computed(() =>
    options.consultantRows.value.filter((row) => row.storeSlug === managementStoreSlug),
  )

  const consultantLinkOptions = computed(() => erpStore.consultantLinks?.consultants || [])

  const consultantLinkEmployeesByKey = computed(() => {
    const map = new Map<string, ErpConsultantLinkEmployee>()
    for (const row of erpStore.consultantLinks?.employees || []) {
      const employeeId = normalizeConsultantLinkEmployeeId(row.erpEmployeeId)
      if (!employeeId) continue

      const scopedKey = consultantLinkKey(row.erpStoreCode, employeeId)
      if (scopedKey) map.set(scopedKey, row)

      const globalKey = consultantLinkKey('', employeeId)
      if (globalKey && !map.has(globalKey)) {
        map.set(globalKey, row)
      }
    }
    return map
  })

  function findConsultantLinkEmployee(row: CRMConsultantMetric) {
    const employeeId = row.erpEmployeeId || row.consultantId
    return (
      consultantLinkEmployeesByKey.value.get(consultantLinkKey(row.storeCnpj, employeeId)) ||
      consultantLinkEmployeesByKey.value.get(consultantLinkKey('', employeeId)) ||
      null
    )
  }

  const queueById = computed(() => {
    const map = new Map<string, QueueConsultantStats>()
    for (const row of options.queueStats.value?.byConsultant ?? []) {
      const personId = String(row.personId || '').trim()
      if (!personId) continue

      const scopedKey = queueConsultantKey(row.storeSlug || row.storeId, personId)
      if (scopedKey) map.set(scopedKey, row)

      const globalKey = queueConsultantKey('', personId)
      if (globalKey && !map.has(globalKey)) {
        map.set(globalKey, row)
      }
    }
    return map
  })

  const queueByName = computed(() => {
    const map = new Map<string, QueueConsultantStats>()
    for (const row of options.queueStats.value?.byConsultant ?? []) {
      const personName = normalizeConsultantLookupKey(row.personName)
      if (!personName) continue

      const scopedKey = queueConsultantKey(row.storeSlug || row.storeId, personName)
      if (scopedKey) map.set(scopedKey, row)

      const globalKey = queueConsultantKey('', personName)
      if (globalKey && !map.has(globalKey)) {
        map.set(globalKey, row)
      }
    }
    return map
  })

  function findQueueForConsultant(row: CRMConsultantMetric) {
    const storeKey = row.storeSlug
    const linkedId = String(row.profileConsultantId || '').trim()
    if (linkedId) {
      const scopedQueue = queueById.value.get(queueConsultantKey(storeKey, linkedId))
      if (scopedQueue) return scopedQueue

      const globalQueue = queueById.value.get(queueConsultantKey('', linkedId))
      if (globalQueue) return globalQueue
    }

    const linkedName = normalizeConsultantLookupKey(row.profileConsultantName)
    if (linkedName) {
      const scopedQueue = queueByName.value.get(queueConsultantKey(storeKey, linkedName))
      if (scopedQueue) return scopedQueue

      const globalQueue = queueByName.value.get(queueConsultantKey('', linkedName))
      if (globalQueue) return globalQueue
    }

    const consultantName = normalizeConsultantLookupKey(row.consultantName)
    if (!consultantName) return null

    return (
      queueByName.value.get(queueConsultantKey(storeKey, consultantName)) ||
      queueByName.value.get(queueConsultantKey('', consultantName)) ||
      null
    )
  }

  const mergedConsultants = computed<MergedCrmConsultant[]>(() => {
    return options.consultantRows.value
      .filter((row) => row.storeSlug !== managementStoreSlug)
      .map((row) => {
        const queue = findQueueForConsultant(row)
        return { ...row, queue, matched: !!queue, linkEmployee: findConsultantLinkEmployee(row) }
      })
  })

  const visibleConsultantLinkEmployeeIds = computed(() => {
    const ids = new Set<string>()
    for (const row of mergedConsultants.value) {
      const employeeId = String(row.erpEmployeeId || row.consultantId || '').trim()
      if (employeeId) ids.add(employeeId)
    }
    return Array.from(ids)
  })

  const unmatchedCount = computed(
    () =>
      mergedConsultants.value.filter(
        (row) => !row.queue && !hasResolvedQueueIdentity(row) && options.queueStats.value,
      ).length,
  )

  async function submitFilters() {
    await crmStore.applyFilters()
  }

  async function refreshConsultantLinks() {
    if (!options.canManageConsultantLinks.value) return
    if (!visibleConsultantLinkEmployeeIds.value.length) return

    const result = await erpStore.fetchConsultantLinks({
      employeeIds: visibleConsultantLinkEmployeeIds.value,
    })
    if (!result.ok && result.message) {
      ui.error(result.message)
    }
  }

  async function autoLinkConsultants() {
    if (!visibleConsultantLinkEmployeeIds.value.length) return

    const result = await erpStore.autoLinkConsultants({
      employeeIds: visibleConsultantLinkEmployeeIds.value,
    })
    if (!result.ok && result.message) {
      ui.error(result.message)
      return
    }

    await submitFilters()
    ui.success('Vinculos automaticos aplicados.')
  }

  async function saveConsultantLink(row: MergedCrmConsultant) {
    const draftKey = consultantLinkKey(row.storeCnpj, row.erpEmployeeId || row.consultantId)
    const consultantId = String(consultantLinkDraftByRow.value[draftKey] || '').trim()
    if (!consultantId) {
      ui.error('Selecione um consultor da Lista de Vez.')
      return
    }

    const result = await erpStore.upsertConsultantLink({
      erpStoreCode: normalizeConsultantLinkStoreCode(row.storeCnpj),
      erpEmployeeId: row.erpEmployeeId || row.consultantId,
      erpEmployeeName: row.consultantName,
      consultantId,
      employeeIds: visibleConsultantLinkEmployeeIds.value,
    })
    if (!result.ok && result.message) {
      ui.error(result.message)
      return
    }

    await submitFilters()
    ui.success('Vinculo ERP atualizado.')
  }

  async function removeConsultantLink(row: MergedCrmConsultant) {
    if (!row.linkEmployee?.linkId) return

    const result = await erpStore.deleteConsultantLink({
      linkId: row.linkEmployee.linkId,
      employeeIds: visibleConsultantLinkEmployeeIds.value,
    })
    if (!result.ok && result.message) {
      ui.error(result.message)
      return
    }

    await submitFilters()
    ui.success('Vinculo ERP removido.')
  }

  function updateConsultantLinkDraft(payload: { key: string; value: string }) {
    const draftKey = String(payload?.key || '').trim()
    if (!draftKey) return

    consultantLinkDraftByRow.value = {
      ...consultantLinkDraftByRow.value,
      [draftKey]: String(payload?.value || '').trim(),
    }
  }

  watch(
    () =>
      `${options.ready.value}:${options.canManageConsultantLinks.value}:${visibleConsultantLinkEmployeeIds.value.join(',')}`,
    () => {
      if (
        !options.ready.value ||
        !options.canManageConsultantLinks.value ||
        !visibleConsultantLinkEmployeeIds.value.length
      ) {
        return
      }
      void refreshConsultantLinks()
    },
    { immediate: true },
  )

  watch(
    () => [mergedConsultants.value, consultantLinkEmployeesByKey.value] as const,
    ([rows]) => {
      const nextDrafts: Record<string, string> = {}
      for (const row of rows) {
        const key = consultantLinkKey(row.storeCnpj, row.erpEmployeeId || row.consultantId)
        if (!key) continue
        nextDrafts[key] = row.linkEmployee?.linkedConsultantId || row.profileConsultantId || ''
      }
      consultantLinkDraftByRow.value = nextDrafts
    },
    { immediate: true },
  )

  return {
    erpStore,
    mergedConsultants,
    managementConsultantRows,
    unmatchedCount,
    consultantLinkOptions,
    consultantLinkDraftByRow,
    refreshConsultantLinks,
    autoLinkConsultants,
    saveConsultantLink,
    removeConsultantLink,
    updateConsultantLinkDraft,
    consultantLinkKey,
    linkStatusLabel,
    linkStatusClass,
  }
}
