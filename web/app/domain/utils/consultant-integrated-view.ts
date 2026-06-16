/**
 * Funções puras para construção da view integrada de consultores (ranking + ERP + cancelamento).
 * Sem efeitos colaterais; sem dependência de store ou composable.
 */

import { buildRankingRows } from '~/domain/utils/admin-metrics'
import {
  normalizeText,
  buildRowKey,
  type NormalizedConsultant,
} from '~/domain/utils/consultant-transforms'

export interface ErpCrmPayload {
  consultants?: ErpConsultantRow[]
  queueStats?: {
    byConsultant?: QueueStatRow[]
  }
}

interface ErpConsultantRow {
  profileConsultantId?: unknown
  profileStoreId?: unknown
  salesCents?: unknown
  ticketAverageCents?: unknown
  paScore?: unknown
  orders?: unknown
}

interface QueueStatRow {
  queueCancellationRate?: unknown
  personId?: unknown
  personName?: unknown
  storeId?: unknown
}

export interface ErpMetric {
  soldValue: number
  ticketAverage: number
  paScore: number
  erpOrders: number
}

export interface IntegratedRankingResponse {
  storeId: string
  tenantId: string
  monthlyRows: Record<string, unknown>[]
  dailyRows: Record<string, unknown>[]
  alerts: unknown[]
}

export function buildErpMetricsByConsultant(erpCrm: ErpCrmPayload | null): Map<string, ErpMetric> {
  const metrics = new Map<string, ErpMetric>()
  for (const row of Array.isArray(erpCrm?.consultants) ? (erpCrm?.consultants ?? []) : []) {
    const consultantId = normalizeText(row?.profileConsultantId)
    if (!consultantId) continue

    const metric: ErpMetric = {
      soldValue: Math.max(0, Number(row?.salesCents || 0) || 0) / 100,
      ticketAverage: Math.max(0, Number(row?.ticketAverageCents || 0) || 0) / 100,
      paScore: Math.max(0, Number(row?.paScore || 0) || 0),
      erpOrders: Math.max(0, Number(row?.orders || 0) || 0),
    }

    metrics.set(buildRowKey(row?.profileStoreId, consultantId), metric)
    if (!metrics.has(buildRowKey('', consultantId))) {
      metrics.set(buildRowKey('', consultantId), metric)
    }
  }
  return metrics
}

export function buildQueueCancellationByConsultant(
  erpCrm: ErpCrmPayload | null,
): Map<string, number> {
  const rates = new Map<string, number>()
  const byConsultant = Array.isArray(erpCrm?.queueStats?.byConsultant)
    ? (erpCrm?.queueStats?.byConsultant ?? [])
    : []
  for (const row of byConsultant) {
    const rate = Math.max(0, Number(row?.queueCancellationRate || 0) || 0)
    const personId = normalizeText(row?.personId)
    const nameKey = normalizeText(row?.personName).toLowerCase()

    if (personId) {
      rates.set(buildRowKey(row?.storeId, personId), rate)
      if (!rates.has(buildRowKey('', personId))) rates.set(buildRowKey('', personId), rate)
    }
    if (nameKey && !rates.has(buildRowKey('', nameKey))) {
      rates.set(buildRowKey('', nameKey), rate)
    }
  }
  return rates
}

export function resolveQueueCancellationRate(
  rates: Map<string, number>,
  consultant: { id?: unknown; name?: unknown; storeId?: unknown },
): number | undefined {
  const storeId = normalizeText(consultant?.storeId)
  const consultantId = normalizeText(consultant?.id)
  const nameKey = normalizeText(consultant?.name).toLowerCase()
  const candidates = [
    buildRowKey(storeId, consultantId),
    buildRowKey('', consultantId),
    buildRowKey('', nameKey),
  ]
  for (const key of candidates) {
    if (rates.has(key)) return rates.get(key)
  }
  return undefined
}

export function buildIntegratedRankingResponse(
  tenantId: string,
  roster: NormalizedConsultant[] = [],
  serviceHistory: Record<string, unknown>[] = [],
  erpCrm: ErpCrmPayload | null = null,
  dateFrom = '',
  dateTo = '',
): IntegratedRankingResponse {
  const rosterByConsultantId = new Map<string, NormalizedConsultant>(
    (Array.isArray(roster) ? roster : []).map((consultant) => [
      normalizeText(consultant?.id),
      consultant,
    ]),
  )
  const erpMetricsByConsultant = buildErpMetricsByConsultant(erpCrm)
  const cancellationByConsultant = buildQueueCancellationByConsultant(erpCrm)

  const mapRows = (rows: Record<string, unknown>[]) =>
    rows.map((row) => {
      const consultant = rosterByConsultantId.get(normalizeText(row?.consultantId))
      const erpMetric =
        erpMetricsByConsultant.get(buildRowKey(consultant?.storeId, row?.consultantId)) ||
        erpMetricsByConsultant.get(buildRowKey('', row?.consultantId)) ||
        null
      const cancellationRate = resolveQueueCancellationRate(cancellationByConsultant, {
        id: row?.consultantId,
        name: consultant?.name ?? (row?.consultantName as string | undefined),
        storeId: consultant?.storeId,
      })

      const mergedRow: Record<string, unknown> = {
        ...row,
        storeId: normalizeText(consultant?.storeId),
        storeName: normalizeText(consultant?.storeName),
      }

      if (typeof cancellationRate === 'number') {
        mergedRow.cancellationRate = cancellationRate
      }

      if (!erpMetric) {
        return mergedRow
      }

      return {
        ...mergedRow,
        soldValue: erpMetric.soldValue,
        ticketAverage: erpMetric.ticketAverage,
        paScore: erpMetric.paScore,
        erpOrders: erpMetric.erpOrders,
        soldValueSource: 'erp',
        ticketAverageSource: 'erp',
        paScoreSource: 'erp',
      }
    })

  return {
    storeId: '',
    tenantId: normalizeText(tenantId),
    monthlyRows: mapRows(
      buildRankingRows({ history: serviceHistory, roster, scope: 'month', dateFrom, dateTo }),
    ),
    dailyRows: mapRows(
      buildRankingRows({ history: serviceHistory, roster, scope: 'today', dateFrom, dateTo }),
    ),
    alerts: [],
  }
}
