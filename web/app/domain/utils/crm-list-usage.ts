import {
  classifyCrmListUsageRate,
  crmListUsageNormalThreshold,
  normalizeCrmListUsageMinOrders,
  normalizeCrmListUsageTiers,
  type CrmListUsageTier,
} from '~/domain/utils/crm-performance-policy'

export type CrmListUsageStatus = 'covered' | 'partial' | 'unused' | 'no-sales'

export type CrmListUsageQueue = {
  attendances?: number | null
}

export type CrmListUsageConsultant = {
  consultantId?: string | null
  consultantName?: string | null
  erpEmployeeId?: string | null
  profileConsultantId?: string | null
  profileConsultantName?: string | null
  orders?: number | null
  storeSlug?: string | null
  storeLabel?: string | null
  storeName?: string | null
  queue?: CrmListUsageQueue | null
}

export type CrmListUsageStoreSummary = {
  storeSlug: string
  storeLabel: string
  totalConsultants: number
  coveredConsultants: number
  partialConsultants: number
  unusedConsultants: number
  usageRate: number
  tierLabel: string
}

export type CrmListUsageConsultantSummary = {
  consultantKey: string
  consultantName: string
  storeSlug: string
  storeLabel: string
  orders: number
  attendances: number
  usageRate: number
  tierLabel: string
  eligibleForHighlight: boolean
}

export type CrmListUsageSummaryOptions = {
  tiers?: CrmListUsageTier[] | null
  minOrdersForHighlight?: number | null
}

export type CrmListUsageSummary = {
  totalConsultants: number
  coveredConsultants: number
  partialConsultants: number
  unusedConsultants: number
  usageRate: number
  tierLabel: string
  minHighlightRate: number
  minOrdersForHighlight: number
  hasPositiveStoreHighlight: boolean
  hasPositiveConsultantHighlight: boolean
  bestStore: CrmListUsageStoreSummary | null
  worstStore: CrmListUsageStoreSummary | null
  bestConsultant: CrmListUsageConsultantSummary | null
  worstConsultant: CrmListUsageConsultantSummary | null
}

function positiveNumber(value: unknown) {
  const numeric = Number(value || 0)
  return Number.isFinite(numeric) && numeric > 0 ? numeric : 0
}

function roundRate(value: number) {
  return Math.round(value * 10) / 10
}

export function crmListUsageOrders(row: CrmListUsageConsultant) {
  return positiveNumber(row.orders)
}

export function crmListUsageAttendances(row: CrmListUsageConsultant) {
  return positiveNumber(row.queue?.attendances)
}

export function crmListUsageCoverageRate(row: CrmListUsageConsultant) {
  const orders = crmListUsageOrders(row)
  if (!orders) return 0
  return Math.min(100, (crmListUsageAttendances(row) / orders) * 100)
}

export function crmListUsageStatus(row: CrmListUsageConsultant): CrmListUsageStatus {
  const orders = crmListUsageOrders(row)
  if (!orders) return 'no-sales'

  const attendances = crmListUsageAttendances(row)
  if (attendances >= orders) return 'covered'
  if (attendances > 0) return 'partial'
  return 'unused'
}

export function crmListUsageStatusLabel(status: CrmListUsageStatus) {
  switch (status) {
    case 'covered':
      return 'Coberto'
    case 'partial':
      return 'Parcial'
    case 'unused':
      return 'Sem uso'
    default:
      return '-'
  }
}

function emptyStoreSummary(storeSlug: string, storeLabel: string): CrmListUsageStoreSummary {
  return {
    storeSlug,
    storeLabel,
    totalConsultants: 0,
    coveredConsultants: 0,
    partialConsultants: 0,
    unusedConsultants: 0,
    usageRate: 0,
    tierLabel: '',
  }
}

function storeKey(row: CrmListUsageConsultant) {
  const slug = String(row.storeSlug || '').trim()
  if (slug) return slug
  return String(row.storeLabel || row.storeName || 'sem-loja')
    .trim()
    .toLowerCase()
}

function storeLabel(row: CrmListUsageConsultant) {
  return String(row.storeLabel || row.storeName || 'Sem loja').trim()
}

function consultantKey(row: CrmListUsageConsultant, index: number) {
  for (const value of [
    row.profileConsultantId,
    row.consultantId,
    row.erpEmployeeId,
    row.consultantName,
  ]) {
    const normalized = String(value || '')
      .trim()
      .toLowerCase()
    if (normalized) return normalized
  }
  return `row-${index}`
}

function consultantLabel(row: CrmListUsageConsultant) {
  return String(
    row.consultantName ||
      row.profileConsultantName ||
      row.profileConsultantId ||
      row.consultantId ||
      'Sem nome',
  ).trim()
}

function addUsage(
  map: Map<string, CrmListUsageConsultant>,
  key: string,
  row: CrmListUsageConsultant,
) {
  const current = map.get(key) || { ...row, orders: 0, queue: { attendances: 0 } }
  map.set(key, {
    ...current,
    orders: crmListUsageOrders(current) + crmListUsageOrders(row),
    queue: {
      attendances: crmListUsageAttendances(current) + crmListUsageAttendances(row),
    },
  })
}

function compareBestStore(left: CrmListUsageStoreSummary, right: CrmListUsageStoreSummary) {
  if (left.usageRate !== right.usageRate) return right.usageRate - left.usageRate
  if (left.coveredConsultants !== right.coveredConsultants) {
    return right.coveredConsultants - left.coveredConsultants
  }
  return left.storeLabel.localeCompare(right.storeLabel)
}

function compareWorstStore(left: CrmListUsageStoreSummary, right: CrmListUsageStoreSummary) {
  if (left.usageRate !== right.usageRate) return left.usageRate - right.usageRate
  if (left.unusedConsultants !== right.unusedConsultants) {
    return right.unusedConsultants - left.unusedConsultants
  }
  return left.storeLabel.localeCompare(right.storeLabel)
}

function compareBestConsultant(
  left: CrmListUsageConsultantSummary,
  right: CrmListUsageConsultantSummary,
) {
  if (left.eligibleForHighlight !== right.eligibleForHighlight) {
    return left.eligibleForHighlight ? -1 : 1
  }
  if (left.usageRate !== right.usageRate) return right.usageRate - left.usageRate
  if (left.orders !== right.orders) return right.orders - left.orders
  return left.consultantName.localeCompare(right.consultantName)
}

function compareWorstConsultant(
  left: CrmListUsageConsultantSummary,
  right: CrmListUsageConsultantSummary,
) {
  if (left.usageRate !== right.usageRate) return left.usageRate - right.usageRate
  if (left.orders !== right.orders) return right.orders - left.orders
  return left.consultantName.localeCompare(right.consultantName)
}

export function buildCrmListUsageSummary(
  rows: CrmListUsageConsultant[],
  options: CrmListUsageSummaryOptions = {},
): CrmListUsageSummary {
  const tiers = normalizeCrmListUsageTiers(options.tiers)
  const minHighlightRate = crmListUsageNormalThreshold(tiers)
  const minOrdersForHighlight = normalizeCrmListUsageMinOrders(options.minOrdersForHighlight)
  const summary: CrmListUsageSummary = {
    totalConsultants: 0,
    coveredConsultants: 0,
    partialConsultants: 0,
    unusedConsultants: 0,
    usageRate: 0,
    tierLabel: '',
    minHighlightRate,
    minOrdersForHighlight,
    hasPositiveStoreHighlight: false,
    hasPositiveConsultantHighlight: false,
    bestStore: null,
    worstStore: null,
    bestConsultant: null,
    worstConsultant: null,
  }

  const consultants = new Map<string, CrmListUsageConsultant>()
  const stores = new Map<string, Map<string, CrmListUsageConsultant>>()

  rows.forEach((row, index) => {
    if (!crmListUsageOrders(row)) return
    const personKey = consultantKey(row, index)
    addUsage(consultants, personKey, row)
    const key = storeKey(row)
    const store = stores.get(key) || new Map<string, CrmListUsageConsultant>()
    stores.set(key, store)
    addUsage(store, personKey, row)
  })

  for (const row of consultants.values()) {
    const status = crmListUsageStatus(row)
    if (status === 'no-sales') continue
    summary.totalConsultants += 1
    if (status === 'covered') {
      summary.coveredConsultants += 1
    } else if (status === 'partial') {
      summary.partialConsultants += 1
    } else {
      summary.unusedConsultants += 1
    }
  }

  if (summary.totalConsultants > 0) {
    summary.usageRate = roundRate((summary.coveredConsultants / summary.totalConsultants) * 100)
  }
  summary.tierLabel = classifyCrmListUsageRate(summary.usageRate, tiers).label

  const storeSummaries = [...stores.entries()].map(([key, storeRows]) => {
    const firstRow = storeRows.values().next().value
    const store = emptyStoreSummary(key, firstRow ? storeLabel(firstRow) : key)
    let coverageTotal = 0
    for (const row of storeRows.values()) {
      const status = crmListUsageStatus(row)
      if (status === 'no-sales') continue
      store.totalConsultants += 1
      coverageTotal += crmListUsageCoverageRate(row)
      if (status === 'covered') {
        store.coveredConsultants += 1
      } else if (status === 'partial') {
        store.partialConsultants += 1
      } else {
        store.unusedConsultants += 1
      }
    }
    store.usageRate =
      store.totalConsultants > 0 ? roundRate(coverageTotal / store.totalConsultants) : 0
    store.tierLabel = classifyCrmListUsageRate(store.usageRate, tiers).label
    return store
  })

  const consultantSummaries = [...consultants.entries()].map(([key, row]) => {
    const usageRate = roundRate(crmListUsageCoverageRate(row))
    const orders = crmListUsageOrders(row)
    return {
      consultantKey: key,
      consultantName: consultantLabel(row),
      storeSlug: String(row.storeSlug || '').trim(),
      storeLabel: storeLabel(row),
      orders,
      attendances: crmListUsageAttendances(row),
      usageRate,
      tierLabel: classifyCrmListUsageRate(usageRate, tiers).label,
      eligibleForHighlight: usageRate >= minHighlightRate && orders >= minOrdersForHighlight,
    }
  })

  summary.bestStore = [...storeSummaries].sort(compareBestStore)[0] || null
  summary.worstStore = [...storeSummaries].sort(compareWorstStore)[0] || null
  summary.hasPositiveStoreHighlight = Boolean(
    summary.bestStore && summary.bestStore.usageRate >= minHighlightRate,
  )
  summary.bestConsultant = [...consultantSummaries].sort(compareBestConsultant)[0] || null
  summary.worstConsultant = [...consultantSummaries].sort(compareWorstConsultant)[0] || null
  summary.hasPositiveConsultantHighlight = Boolean(summary.bestConsultant?.eligibleForHighlight)

  return summary
}
