import { computed } from 'vue'
import type { ComputedRef } from 'vue'
import { PERF_ROWS, PERF_RUN, type PerfRow } from './perf-data'

export type PerfMode = PerfRow['mode']

// Linha consolidada por rota: junta os dois modos (in-app + cold) lado a lado.
export interface PerfRouteRow {
  path: string
  inapp: PerfRow | null
  cold: PerfRow | null
  capped: boolean
  // T3 usado para ordenar a tabela: pior dos dois modos disponiveis.
  worstT3: number | null
}

// Linha "achatada" de um unico modo, usada pelo ranking.
export interface PerfRouteSummary {
  path: string
  mode: PerfMode
  t1: number | null
  t2: number | null
  t3: number | null
  capped: boolean
}

export interface PerformanceData {
  run: typeof PERF_RUN
  rows: ComputedRef<PerfRouteRow[]>
  inappSummaries: ComputedRef<PerfRouteSummary[]>
  coldSummaries: ComputedRef<PerfRouteSummary[]>
  totalRoutes: ComputedRef<number>
  cappedCount: ComputedRef<number>
  slowestInapp: ComputedRef<PerfRouteSummary | null>
  slowestCold: ComputedRef<PerfRouteSummary | null>
}

function toSummary(row: PerfRow): PerfRouteSummary {
  return { path: row.path, mode: row.mode, t1: row.t1, t2: row.t2, t3: row.t3, capped: row.capped }
}

function worstT3(inapp: PerfRow | null, cold: PerfRow | null): number | null {
  const values = [inapp?.t3, cold?.t3].filter((value): value is number => typeof value === 'number')
  return values.length ? Math.max(...values) : null
}

export function usePerformanceData(): PerformanceData {
  const rows = computed<PerfRouteRow[]>(() => {
    const byPath = new Map<string, PerfRouteRow>()
    const order: string[] = []

    for (const row of PERF_ROWS) {
      let entry = byPath.get(row.path)
      if (!entry) {
        entry = { path: row.path, inapp: null, cold: null, capped: false, worstT3: null }
        byPath.set(row.path, entry)
        order.push(row.path)
      }
      if (row.mode === 'inapp') {
        entry.inapp = row
      } else {
        entry.cold = row
      }
      if (row.capped) {
        entry.capped = true
      }
    }

    const result = order.map((path) => {
      const entry = byPath.get(path) as PerfRouteRow
      entry.worstT3 = worstT3(entry.inapp, entry.cold)
      return entry
    })

    return result.sort((left, right) => (right.worstT3 ?? -1) - (left.worstT3 ?? -1))
  })

  const inappSummaries = computed<PerfRouteSummary[]>(() =>
    PERF_ROWS.filter((row) => row.mode === 'inapp').map(toSummary),
  )

  const coldSummaries = computed<PerfRouteSummary[]>(() =>
    PERF_ROWS.filter((row) => row.mode === 'cold').map(toSummary),
  )

  const totalRoutes = computed(() => rows.value.length)

  const cappedCount = computed(() => rows.value.filter((row) => row.capped).length)

  const slowest = (summaries: PerfRouteSummary[]): PerfRouteSummary | null => {
    let peak: PerfRouteSummary | null = null
    for (const summary of summaries) {
      if (summary.t3 === null) {
        continue
      }
      if (!peak || (peak.t3 ?? 0) < summary.t3) {
        peak = summary
      }
    }
    return peak
  }

  const slowestInapp = computed(() => slowest(inappSummaries.value))
  const slowestCold = computed(() => slowest(coldSummaries.value))

  return {
    run: PERF_RUN,
    rows,
    inappSummaries,
    coldSummaries,
    totalRoutes,
    cappedCount,
    slowestInapp,
    slowestCold,
  }
}
