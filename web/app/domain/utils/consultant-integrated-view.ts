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
  stores?: ErpStoreRow[]
  queueStats?: {
    byConsultant?: QueueStatRow[]
  }
}

// DTO do payout pré-calculado no back (GET /v1/erp/crm). O front é DISPLAY:
// nada é recalculado aqui — apenas lido e propagado até as rows/cards.
// Defensivo: campos podem vir ausentes/null antes do rebuild do back.
export interface ErpPayout {
  amount: number
  ratePercent: number
  base?: number
  group?: string
  ruleLabel?: string
  penaltyApplied?: number
}

export type StoreType = 'shopping' | 'bairro'

// Origem da meta da loja + flags de gap (contrato congelado /v1/erp/crm).
export type StoreGoalSource = 'own' | 'consultant-sum' | 'none'
// Origem da meta do consultor + flags de gap (contrato congelado /v1/erp/crm).
export type ConsultantGoalSource = 'own' | 'store-split' | 'none'

// Payout por loja (gerente/caixa) + tipo de loja, lido da métrica de loja do back.
export interface ErpStorePayout {
  storeType: StoreType
  storeSold: number
  storeGoal: number
  storeProgress: number
  managerPayout: ErpPayout | null
  supportPayout: ErpPayout | null
  // Flags de gap de meta da loja (contrato congelado; ausentes antes do rebuild do back).
  storeGoalSource: StoreGoalSource
  missingStoreGoal: boolean
  missingTicketGoal: boolean
  missingPaGoal: boolean
  splitConsultantCount: number
}

interface ErpConsultantRow {
  profileConsultantId?: unknown
  profileStoreId?: unknown
  salesCents?: unknown
  ticketAverageCents?: unknown
  paScore?: unknown
  orders?: unknown
  payout?: unknown
  // Metas EFETIVAS calculadas no back (com herança da loja). Display só.
  avgTicketGoalCents?: unknown
  paGoal?: unknown
  monthlyGoalCents?: unknown
  // Flags de gap por consultor (contrato congelado /v1/erp/crm).
  goalSource?: unknown
  missingMonthlyGoal?: unknown
  missingTicketGoal?: unknown
  missingPaGoal?: unknown
}

interface ErpStoreRow {
  storeSlug?: unknown
  storeCode?: unknown
  profileStoreId?: unknown
  storeType?: unknown
  storeSold?: unknown
  salesCents?: unknown
  storeGoal?: unknown
  storeProgress?: unknown
  goalProgress?: unknown
  managerPayout?: unknown
  supportPayout?: unknown
  // Metas EFETIVAS da loja (para o popover semear o input).
  avgTicketGoalCents?: unknown
  paGoal?: unknown
  // Flags de gap por loja (contrato congelado /v1/erp/crm).
  storeGoalSource?: unknown
  missingStoreGoal?: unknown
  missingTicketGoal?: unknown
  missingPaGoal?: unknown
  splitConsultantCount?: unknown
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
  payout: ErpPayout | null
  // Metas efetivas (com herança da loja) vindas do back; 0 quando não cadastradas.
  avgTicketGoal: number
  paGoal: number
  monthlyGoal: number
  // Flags de gap por consultor (contrato congelado; default seguro antes do rebuild).
  goalSource: ConsultantGoalSource
  missingMonthlyGoal: boolean
  missingTicketGoal: boolean
  missingPaGoal: boolean
}

function normalizeConsultantGoalSource(value: unknown): ConsultantGoalSource {
  const text = normalizeText(value).toLowerCase()
  if (text === 'own') return 'own'
  if (text === 'store-split') return 'store-split'
  return 'none'
}

function normalizeStoreGoalSource(value: unknown): StoreGoalSource {
  const text = normalizeText(value).toLowerCase()
  if (text === 'own') return 'own'
  if (text === 'consultant-sum') return 'consultant-sum'
  return 'none'
}

// Lê o payout pré-calculado do back sem recalcular. Retorna null quando ausente
// para o display cair em "—"/R$ 0 (a página funciona antes do rebuild do back).
export function normalizeErpPayout(value: unknown): ErpPayout | null {
  if (!value || typeof value !== 'object') return null
  const source = value as Record<string, unknown>
  if (source.amount === undefined && source.ratePercent === undefined) return null
  return {
    amount: Math.max(0, Number(source.amount || 0) || 0),
    ratePercent: Math.max(0, Number(source.ratePercent || 0) || 0),
    base: Number(source.base || 0) || 0,
    group: normalizeText(source.group),
    ruleLabel: normalizeText(source.ruleLabel),
    penaltyApplied: Number(source.penaltyApplied || 0) || 0,
  }
}

function normalizeStoreType(value: unknown): StoreType {
  return normalizeText(value).toLowerCase() === 'shopping' ? 'shopping' : 'bairro'
}

// Indexa o payout/tipo de loja por TODAS as chaves que a loja pode aparecer
// (storeId, slug, código) para casar com o roster do front, que usa storeId.
export function buildErpStorePayoutByStore(
  erpCrm: ErpCrmPayload | null,
): Map<string, ErpStorePayout> {
  const byStore = new Map<string, ErpStorePayout>()

  // O store metric do back e indexado por slug/codigo, mas o roster/staff do front
  // usa o storeId (UUID) interno. Os consultores trazem storeSlug + profileStoreId
  // (UUID), entao montamos slug -> UUID para tambem indexar o payout de loja por UUID
  // e o card de gerente/caixa casar com a loja.
  const storeIdBySlug = new Map<string, string>()
  for (const row of Array.isArray(erpCrm?.consultants) ? (erpCrm?.consultants ?? []) : []) {
    const slug = normalizeText((row as Record<string, unknown>)?.storeSlug)
    const storeId = normalizeText(row?.profileStoreId)
    if (slug && storeId && !storeIdBySlug.has(slug)) {
      storeIdBySlug.set(slug, storeId)
    }
  }

  for (const row of Array.isArray(erpCrm?.stores) ? (erpCrm?.stores ?? []) : []) {
    const storePayout: ErpStorePayout = {
      storeType: normalizeStoreType(row?.storeType),
      storeSold: Math.max(0, Number(row?.storeSold ?? row?.salesCents ?? 0) || 0),
      storeGoal: Math.max(0, Number(row?.storeGoal || 0) || 0),
      storeProgress: Math.max(0, Number(row?.storeProgress ?? row?.goalProgress ?? 0) || 0),
      managerPayout: normalizeErpPayout(row?.managerPayout),
      supportPayout: normalizeErpPayout(row?.supportPayout),
      storeGoalSource: normalizeStoreGoalSource(row?.storeGoalSource),
      missingStoreGoal: Boolean(row?.missingStoreGoal),
      missingTicketGoal: Boolean(row?.missingTicketGoal),
      missingPaGoal: Boolean(row?.missingPaGoal),
      splitConsultantCount: Math.max(0, Number(row?.splitConsultantCount || 0) || 0),
    }

    const storeIdFromSlug = storeIdBySlug.get(normalizeText(row?.storeSlug))
    for (const key of [row?.profileStoreId, row?.storeSlug, row?.storeCode, storeIdFromSlug]) {
      const normalized = normalizeText(key)
      if (normalized && !byStore.has(normalized)) {
        byStore.set(normalized, storePayout)
      }
    }
  }
  return byStore
}

export interface IntegratedRankingResponse {
  storeId: string
  tenantId: string
  monthlyRows: Record<string, unknown>[]
  dailyRows: Record<string, unknown>[]
  alerts: unknown[]
  // Payout/tipo de loja pré-calculado pelo back, indexado por storeId/slug/código.
  // Serializado como objeto plano para sobreviver ao JSON.stringify do scope key.
  storePayoutByStore: Record<string, ErpStorePayout>
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
      payout: normalizeErpPayout(row?.payout),
      avgTicketGoal: Math.max(0, Number(row?.avgTicketGoalCents || 0) || 0) / 100,
      paGoal: Math.max(0, Number(row?.paGoal || 0) || 0),
      monthlyGoal: Math.max(0, Number(row?.monthlyGoalCents || 0) || 0) / 100,
      goalSource: normalizeConsultantGoalSource(row?.goalSource),
      missingMonthlyGoal: Boolean(row?.missingMonthlyGoal),
      missingTicketGoal: Boolean(row?.missingTicketGoal),
      missingPaGoal: Boolean(row?.missingPaGoal),
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
        // Payout do consultor pré-calculado no back (% da PRÓPRIA venda); display só.
        payout: erpMetric.payout,
        // Metas efetivas (com herança da loja) p/ o card mostrar meta + cor.
        erpAvgTicketGoal: erpMetric.avgTicketGoal,
        erpPaGoal: erpMetric.paGoal,
        erpMonthlyGoal: erpMetric.monthlyGoal,
        // Flags de gap por consultor (contrato congelado) p/ o aviso acionável inline.
        erpGoalSource: erpMetric.goalSource,
        erpMissingMonthlyGoal: erpMetric.missingMonthlyGoal,
        erpMissingTicketGoal: erpMetric.missingTicketGoal,
        erpMissingPaGoal: erpMetric.missingPaGoal,
        soldValueSource: 'erp',
        ticketAverageSource: 'erp',
        paScoreSource: 'erp',
      }
    })

  const storePayoutByStore = buildErpStorePayoutByStore(erpCrm)

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
    storePayoutByStore: Object.fromEntries(storePayoutByStore),
  }
}
