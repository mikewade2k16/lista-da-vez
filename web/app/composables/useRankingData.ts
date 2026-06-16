import { computed, type Ref } from 'vue'
import { computeScore360, useGamificationConfig } from '~/composables/useGamificationConfig'

export type RankingRow = {
  consultantId?: string
  consultantName?: string
  storeId?: string
  storeName?: string
  soldValue?: number
  attendances?: number
  conversions?: number
  conversionRate?: number
  ticketAverage?: number
  paScore?: number
  qualityScore?: number
  avgDurationMs?: number
  queueJumpServices?: number
  [key: string]: unknown
}

export type EnrichedRow = RankingRow & {
  rowKey: string
  score360: number
}

export type StoreAggRow = {
  rowKey: string
  consultantId: string
  consultantName: string
  storeId: string
  storeName: string
  attendances: number
  conversions: number
  soldValue: number
  conversionRate: number
  ticketAverage: number
  paScore: number
  qualityScore: number
  avgDurationMs: number
  queueJumpServices: number
  score360: number
  consultantCount: number
}

function normalizeText(value: unknown) {
  return String(value || '').trim()
}

export function buildRowKey(row: RankingRow) {
  return `${normalizeText(row.storeId)}:${normalizeText(row.consultantId)}`
}

export function getMetricValue(row: Record<string, unknown>, key: string) {
  if (key === 'score360') return Number(row.score360 || 0)
  return Number(row[key] || 0)
}

export function normalizeSearch(value: unknown) {
  return normalizeText(value).normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase()
}

export function buildStoreAggregates(rows: EnrichedRow[]): StoreAggRow[] {
  type AggEntry = {
    rowKey: string
    consultantId: string
    consultantName: string
    storeId: string
    storeName: string
    attendances: number
    conversions: number
    soldValue: number
    score360Weighted: number
    score360Weight: number
    ticketAverageWeighted: number
    ticketAverageWeight: number
    totalPieces: number
    qualityWeighted: number
    qualityWeight: number
    avgDurationTotal: number
    queueJumpServices: number
    consultantCount: number
  }
  const grouped = new Map<string, AggEntry>()

  rows.forEach((row) => {
    const storeId = normalizeText(row.storeId)
    if (!storeId) return
    const weight = Math.max(1, Number(row.attendances || 0))
    const current = grouped.get(storeId) || {
      rowKey: `store:${storeId}`,
      consultantId: storeId,
      consultantName: normalizeText(row.storeName) || 'Loja sem nome',
      storeId,
      storeName: normalizeText(row.storeName) || 'Loja sem nome',
      attendances: 0,
      conversions: 0,
      soldValue: 0,
      score360Weighted: 0,
      score360Weight: 0,
      ticketAverageWeighted: 0,
      ticketAverageWeight: 0,
      totalPieces: 0,
      qualityWeighted: 0,
      qualityWeight: 0,
      avgDurationTotal: 0,
      queueJumpServices: 0,
      consultantCount: 0,
    }
    current.attendances += Number(row.attendances || 0)
    current.conversions += Number(row.conversions || 0)
    current.soldValue += Number(row.soldValue || 0)
    current.score360Weighted += row.score360 * weight
    current.score360Weight += weight
    current.ticketAverageWeighted += Number(row.ticketAverage || 0) * weight
    current.ticketAverageWeight += weight
    current.totalPieces += Math.max(
      Number(row.conversions || 0),
      Number(row.paScore || 0) * Number(row.conversions || 0),
    )
    current.qualityWeighted += Number(row.qualityScore || 0) * weight
    current.qualityWeight += weight
    current.avgDurationTotal += Number(row.avgDurationMs || 0) * Number(row.attendances || 0)
    current.queueJumpServices += Number(row.queueJumpServices || 0)
    current.consultantCount += 1
    grouped.set(storeId, current)
  })

  return [...grouped.values()].map((entry) => ({
    rowKey: entry.rowKey,
    consultantId: entry.consultantId,
    consultantName: entry.consultantName,
    storeId: entry.storeId,
    storeName: entry.storeName,
    attendances: entry.attendances,
    conversions: entry.conversions,
    soldValue: entry.soldValue,
    conversionRate: entry.attendances > 0 ? (entry.conversions / entry.attendances) * 100 : 0,
    ticketAverage:
      entry.ticketAverageWeight > 0 ? entry.ticketAverageWeighted / entry.ticketAverageWeight : 0,
    paScore: entry.conversions > 0 ? Math.max(1, entry.totalPieces / entry.conversions) : 0,
    qualityScore: entry.qualityWeight > 0 ? entry.qualityWeighted / entry.qualityWeight : 0,
    avgDurationMs: entry.attendances > 0 ? entry.avgDurationTotal / entry.attendances : 0,
    queueJumpServices: entry.queueJumpServices,
    score360: entry.score360Weight > 0 ? entry.score360Weighted / entry.score360Weight : 0,
    consultantCount: entry.consultantCount,
  }))
}

export function useRankingData(monthlyRows: Ref<unknown[]>, dailyRows: Ref<unknown[]>) {
  const { scoreWeights } = useGamificationConfig()

  const monthlyConsultantMaxSold = computed(() =>
    Math.max(...(monthlyRows.value as RankingRow[]).map((row) => Number(row.soldValue || 0)), 1),
  )
  const monthlyConsultantMaxPa = computed(() =>
    Math.max(...(monthlyRows.value as RankingRow[]).map((row) => Number(row.paScore || 0)), 0.01),
  )

  function enrichRows(rows: RankingRow[], maxSold: number, maxPa: number): EnrichedRow[] {
    return rows.map((row) => ({
      ...row,
      rowKey: buildRowKey(row),
      consultantName: normalizeText(row.consultantName) || 'Consultor sem nome',
      storeId: normalizeText(row.storeId),
      storeName: normalizeText(row.storeName) || 'Loja sem nome',
      score360: computeScore360(
        {
          conversionRate: Number(row.conversionRate || 0),
          soldValue: Number(row.soldValue || 0),
          qualityScore: Number(row.qualityScore || 0),
          paScore: Number(row.paScore || 0),
          queueJumpServices: Number(row.queueJumpServices || 0),
          attendances: Number(row.attendances || 0),
        },
        { maxSold, maxPa, weights: scoreWeights.value },
      ),
    }))
  }

  const enrichedMonthly = computed(() =>
    enrichRows(
      monthlyRows.value as RankingRow[],
      monthlyConsultantMaxSold.value,
      monthlyConsultantMaxPa.value,
    ),
  )

  const enrichedDaily = computed(() =>
    enrichRows(
      dailyRows.value as RankingRow[],
      monthlyConsultantMaxSold.value,
      monthlyConsultantMaxPa.value,
    ),
  )

  const dailyScoreMap = computed(() => {
    const map = new Map<string, number>()
    enrichedDaily.value.forEach((row) => map.set(row.rowKey, row.score360))
    return map
  })

  const monthlyStoreRows = computed(() => buildStoreAggregates(enrichedMonthly.value))

  const dailyStoreScoreMap = computed(() => {
    const map = new Map<string, number>()
    buildStoreAggregates(enrichedDaily.value).forEach((row) => map.set(row.rowKey, row.score360))
    return map
  })

  function variationFor(rowKey: string, monthlyScore: number) {
    const daily = dailyScoreMap.value.get(rowKey)
    if (typeof daily !== 'number' || daily === 0 || monthlyScore === 0) return null
    return ((daily - monthlyScore) / monthlyScore) * 100
  }

  function storeVariationFor(rowKey: string, monthlyScore: number) {
    const daily = dailyStoreScoreMap.value.get(rowKey)
    if (typeof daily !== 'number' || daily === 0 || monthlyScore === 0) return null
    return ((daily - monthlyScore) / monthlyScore) * 100
  }

  return {
    enrichedMonthly,
    enrichedDaily,
    monthlyStoreRows,
    dailyScoreMap,
    dailyStoreScoreMap,
    variationFor,
    storeVariationFor,
  }
}
