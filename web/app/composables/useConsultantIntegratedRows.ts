import { computed, onMounted, watch, type Ref } from 'vue'
import { storeToRefs } from 'pinia'
import { useAuthStore } from '~/stores/auth'
import { useOperationGoalsStore } from '~/stores/operation-goals'
import type {
  ConsultantGoalSource,
  ErpPayout,
  ErpStorePayout,
} from '~/domain/utils/consultant-integrated-view'

export interface ConsultantRosterItem {
  id: string
  name?: string
  role?: string
  storeId?: string
  storeName?: string
  storeCode?: string
  storeCity?: string
  monthlyGoal?: number
  commissionRate?: number
  conversionGoal?: number
  avgTicketGoal?: number
  paGoal?: number
  cancellationRate?: number
  [key: string]: unknown
}

export interface ConsultantRow extends ConsultantRosterItem {
  // name obrigatório (a base ConsultantRosterItem o deixa opcional): o row sempre
  // tem nome, e os consumidores (RosterEntry/GridRow/DrawerConsultant) exigem string.
  name: string
  liveStatusCode: string
  liveStatusLabel: string
  monthlyGoal: number
  soldValue: number
  dailySoldValue: number
  attendances: number
  conversions: number
  conversionRate: number
  ticketAverage: number
  paScore: number
  erpOrders: number
  soldValueSource: string
  ticketAverageSource: string
  paScoreSource: string
  qualityScore: number
  avgDurationMs: number
  queueJumpServices: number
  progress: number
  hitGoal: boolean
  remainingToGoal: number
  // Payout do consultor pré-calculado no back (% da PRÓPRIA venda). null = display "—"/R$ 0.
  payout: ErpPayout | null
  // Flags de gap por consultor (contrato congelado) p/ o aviso acionável inline.
  goalSource: ConsultantGoalSource
  missingMonthlyGoal: boolean
  missingTicketGoal: boolean
  missingPaGoal: boolean
}

function normalizeText(v: unknown) {
  return String(v || '').trim()
}

function buildRowKey(storeId: unknown, consultantId: unknown) {
  return `${normalizeText(storeId)}:${normalizeText(consultantId)}`
}

function normalizeStatusEntry(code: string, label: string) {
  return { code, label }
}

function currentMonthKey() {
  const now = new Date()
  return `${now.getUTCFullYear()}-${String(now.getUTCMonth() + 1).padStart(2, '0')}`
}

export function useConsultantIntegratedRows(
  roster: Ref<ConsultantRosterItem[]>,
  ranking: Ref<Record<string, unknown> | null>,
  overview: Ref<Record<string, unknown> | null>,
) {
  const auth = useAuthStore()
  const operationGoalsStore = useOperationGoalsStore()
  const { goals: operationGoalRows } = storeToRefs(operationGoalsStore)

  async function ensureGoalsLoaded() {
    if (!auth.isAuthenticated || !auth.activeTenantId) return
    try {
      await operationGoalsStore.loadGoals({
        tenantId: auth.activeTenantId,
        month: currentMonthKey(),
      })
    } catch {
      // silencioso: fallback do roster
    }
  }

  onMounted(() => {
    void ensureGoalsLoaded()
  })
  watch(
    () => [auth.isAuthenticated, auth.activeTenantId],
    () => {
      void ensureGoalsLoaded()
    },
  )

  const goalByConsultantId = computed(() => {
    const map = new Map<string, number>()
    for (const row of (operationGoalRows.value || []) as Array<Record<string, unknown>>) {
      if (row?.scope !== 'consultant' || !row?.consultantId) continue
      map.set(normalizeText(row.consultantId), Number(row.monthlyGoal) || 0)
    }
    return map
  })

  const goalByStoreId = computed(() => {
    const map = new Map<string, number>()
    for (const row of (operationGoalRows.value || []) as Array<Record<string, unknown>>) {
      if (row?.scope !== 'store' || !row?.storeId) continue
      map.set(normalizeText(row.storeId), Number(row.monthlyGoal) || 0)
    }
    return map
  })

  function resolveMonthlyGoal(consultant: ConsultantRosterItem) {
    const cg = goalByConsultantId.value.get(normalizeText(consultant.id))
    if (typeof cg === 'number' && cg > 0) return cg
    const sg = goalByStoreId.value.get(normalizeText(consultant.storeId))
    if (typeof sg === 'number' && sg > 0) return sg
    return Math.max(0, Number(consultant.monthlyGoal || 0))
  }

  function resolveRankingRow(map: Map<string, unknown>, consultant: ConsultantRosterItem) {
    return (
      (map.get(buildRowKey(consultant.storeId, consultant.id)) as Record<string, unknown>) ||
      (map.get(buildRowKey('', consultant.id)) as Record<string, unknown>) ||
      {}
    )
  }

  const monthlyRowsMap = computed(() => {
    const map = new Map<string, unknown>()
    for (const row of ((ranking.value?.monthlyRows as unknown[]) || []) as Array<
      Record<string, unknown>
    >) {
      map.set(buildRowKey(row.storeId, row.consultantId), row)
    }
    return map
  })

  const dailyRowsMap = computed(() => {
    const map = new Map<string, unknown>()
    for (const row of ((ranking.value?.dailyRows as unknown[]) || []) as Array<
      Record<string, unknown>
    >) {
      map.set(buildRowKey(row.storeId, row.consultantId), row)
    }
    return map
  })

  const statusMap = computed(() => {
    const map = new Map<string, { code: string; label: string }>()
    const ov = overview.value as Record<string, Array<Record<string, unknown>>> | null
    ;(ov?.activeServices || []).forEach((item) =>
      map.set(
        buildRowKey(item.storeId, item.personId),
        normalizeStatusEntry('service', 'Em atendimento'),
      ),
    )
    ;(ov?.waitingList || []).forEach((item) =>
      map.set(buildRowKey(item.storeId, item.personId), normalizeStatusEntry('queue', 'Na fila')),
    )
    ;(ov?.pausedEmployees || []).forEach((item) => {
      const code = normalizeText(item.pauseKind) === 'assignment' ? 'assignment' : 'paused'
      map.set(
        buildRowKey(item.storeId, item.personId),
        normalizeStatusEntry(code, code === 'assignment' ? 'Em tarefa' : 'Pausado'),
      )
    })
    ;(ov?.availableConsultants || []).forEach((item) =>
      map.set(
        buildRowKey(item.storeId, item.personId),
        normalizeStatusEntry('available', 'Disponivel'),
      ),
    )
    return map
  })

  const consultantRows = computed<ConsultantRow[]>(() =>
    roster.value.map((consultant) => {
      const monthly = resolveRankingRow(monthlyRowsMap.value, consultant)
      const daily = resolveRankingRow(dailyRowsMap.value, consultant)
      const liveStatus =
        statusMap.value.get(buildRowKey(consultant.storeId, consultant.id)) ||
        normalizeStatusEntry('available', 'Disponivel')
      // Prefere a meta EFETIVA do back (com herança da loja) p/ o % bater com o cálculo.
      const backMonthlyGoal = Math.max(0, Number(monthly.erpMonthlyGoal || 0) || 0)
      const monthlyGoal = backMonthlyGoal > 0 ? backMonthlyGoal : resolveMonthlyGoal(consultant)
      const soldValue = Math.max(0, Number(monthly.soldValue || 0))
      const attendances = Math.max(0, Number(monthly.attendances || 0))
      const conversions = Math.max(0, Number(monthly.conversions || 0))
      const progress = monthlyGoal > 0 ? (soldValue / monthlyGoal) * 100 : 0
      const cancellationRate =
        typeof monthly.cancellationRate === 'number'
          ? Math.max(0, monthly.cancellationRate)
          : consultant.cancellationRate
      return {
        ...consultant,
        name: normalizeText(consultant.name),
        cancellationRate,
        liveStatusCode: liveStatus.code,
        liveStatusLabel: liveStatus.label,
        monthlyGoal,
        soldValue,
        dailySoldValue: Math.max(0, Number(daily.soldValue || 0)),
        attendances,
        conversions,
        conversionRate: Math.max(0, Number(monthly.conversionRate || 0)),
        ticketAverage: Math.max(0, Number(monthly.ticketAverage || 0)),
        paScore: Math.max(0, Number(monthly.paScore || 0)),
        erpOrders: Math.max(0, Number(monthly.erpOrders || 0)),
        soldValueSource: normalizeText(monthly.soldValueSource),
        ticketAverageSource: normalizeText(monthly.ticketAverageSource),
        paScoreSource: normalizeText(monthly.paScoreSource),
        qualityScore: Math.max(0, Number(monthly.qualityScore || 0)),
        avgDurationMs: Math.max(0, Number(monthly.avgDurationMs || 0)),
        queueJumpServices: Math.max(0, Number(monthly.queueJumpServices || 0)),
        progress,
        hitGoal: monthlyGoal > 0 && soldValue >= monthlyGoal,
        remainingToGoal: Math.max(0, monthlyGoal - soldValue),
        // Metas de ticket/PA: preferem a meta EFETIVA do back (herança da loja),
        // caindo na meta individual do roster só quando o back não trouxe.
        avgTicketGoal:
          Math.max(0, Number(monthly.erpAvgTicketGoal || 0) || 0) ||
          Math.max(0, Number(consultant.avgTicketGoal || 0)),
        paGoal:
          Math.max(0, Number(monthly.erpPaGoal || 0) || 0) ||
          Math.max(0, Number(consultant.paGoal || 0)),
        // Lido do payload do back (não recalculado): row.payout (% própria venda).
        payout: (monthly.payout as ErpPayout | null) ?? null,
        // Flags de gap (contrato congelado): default seguro antes do rebuild do back.
        goalSource: (normalizeText(monthly.erpGoalSource) || 'none') as ConsultantGoalSource,
        missingMonthlyGoal: Boolean(monthly.erpMissingMonthlyGoal),
        missingTicketGoal: Boolean(monthly.erpMissingTicketGoal),
        missingPaGoal: Boolean(monthly.erpMissingPaGoal),
      }
    }),
  )

  // Payout/tipo de loja vindo do back, indexado por storeId. Display só.
  const storePayoutByStoreId = computed<Record<string, ErpStorePayout>>(() => {
    const source = (ranking.value?.storePayoutByStore as Record<string, ErpStorePayout>) || {}
    const map: Record<string, ErpStorePayout> = {}
    for (const [key, value] of Object.entries(source)) {
      const normalized = normalizeText(key)
      if (normalized && value) map[normalized] = value
    }
    return map
  })

  const storeConversionAvgByStoreId = computed(() => {
    const result: Record<string, number> = {}
    const grouped = new Map<string, { attendances: number; conversions: number }>()
    consultantRows.value.forEach((row) => {
      const sid = normalizeText(row.storeId)
      const cur = grouped.get(sid) || { attendances: 0, conversions: 0 }
      cur.attendances += row.attendances
      cur.conversions += row.conversions
      grouped.set(sid, cur)
    })
    grouped.forEach((v, storeId) => {
      result[storeId] = v.attendances > 0 ? (v.conversions / v.attendances) * 100 : 0
    })
    return result
  })

  const rankingPositionByKey = computed(() => {
    const positions: Record<string, number> = {}
    const sorted = [...consultantRows.value].sort((a, b) => b.soldValue - a.soldValue)
    sorted.forEach((row, index) => {
      positions[`${normalizeText(row.storeId)}:${normalizeText(row.id)}`] = index + 1
    })
    return positions
  })

  const storeTotalSoldByStoreId = computed(() => {
    const result: Record<string, number> = {}
    consultantRows.value.forEach((row) => {
      const sid = normalizeText(row.storeId)
      result[sid] = (result[sid] || 0) + row.soldValue
    })
    return result
  })

  // Progresso da meta da LOJA: total vendido pelos consultores / meta de loja.
  // Fallback de meta: soma das metas individuais da loja quando nao ha meta de loja cadastrada.
  const storeProgressByStoreId = computed(() => {
    const result: Record<string, { storeSold: number; storeGoal: number; progress: number }> = {}
    const soldByStore = storeTotalSoldByStoreId.value
    const individualGoalByStore = new Map<string, number>()
    consultantRows.value.forEach((row) => {
      const sid = normalizeText(row.storeId)
      individualGoalByStore.set(sid, (individualGoalByStore.get(sid) || 0) + (row.monthlyGoal || 0))
    })

    new Set(consultantRows.value.map((row) => normalizeText(row.storeId))).forEach((sid) => {
      // Prefere os números da loja vindos do back (payout DTO); cai no cálculo
      // local só quando o back ainda não trouxe a métrica daquela loja.
      const backStore = storePayoutByStoreId.value[sid]
      const localSold = soldByStore[sid] || 0
      const storeSold = backStore?.storeSold || localSold
      const cadastredStoreGoal = goalByStoreId.value.get(sid) || 0
      const storeGoal =
        backStore?.storeGoal ||
        (cadastredStoreGoal > 0 ? cadastredStoreGoal : individualGoalByStore.get(sid) || 0)
      const progress =
        backStore?.storeProgress || (storeGoal > 0 ? (storeSold / storeGoal) * 100 : 0)
      result[sid] = { storeSold, storeGoal, progress }
    })
    return result
  })

  return {
    consultantRows,
    storeConversionAvgByStoreId,
    rankingPositionByKey,
    storeTotalSoldByStoreId,
    storeProgressByStoreId,
    storePayoutByStoreId,
  }
}
