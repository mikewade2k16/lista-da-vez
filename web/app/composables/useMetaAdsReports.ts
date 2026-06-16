import { storeToRefs } from 'pinia'

import { useMetaAdsStore } from '~/stores/meta-ads'

// Composable fino de relatorios/metricas Meta Ads. Embrulha o useMetaAdsStore
// expondo o que OverviewCard e ReportChart precisam (KPIs, serie de insights,
// range e sync). Estado e logica vivem no store; aqui so projetamos refs.
export function useMetaAdsReports() {
  const store = useMetaAdsStore()
  const { overview, kpis, insights, range, pending, syncing, error } = storeToRefs(store)

  return {
    overview,
    kpis,
    insights,
    range,
    pending,
    syncing,
    error,
    loadOverview: store.loadOverview,
    loadInsights: store.loadInsights,
    setRange: store.setRange,
    sync: store.sync,
  }
}
