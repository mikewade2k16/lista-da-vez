// Tipos do modulo meta-ads (espelham os JSON tags do backend Go em
// back/internal/modules/meta_ads/model*.go). Contrato congelado pelo plano
// docs/meta-ads/PLANO_INTEGRACAO_META_ADS.md — nao alterar shapes sem atualizar
// o backend e o store correspondente (web/app/stores/meta-ads.ts).

export interface MetaAdsConnection {
  connected: boolean
  name: string
  metaBusinessId: string
  status: string
  tokenExpiresAt: string | null
  revision: string
}

export interface MetaAdsAdAccount {
  id: string
  metaAdAccountId: string
  name: string
  currency: string
  status: string
  clientAccountId: string | null
}

export interface MetaAdsInstagramIdentity {
  igUserId: string
  username: string
  pageId: string
  pageName: string
  clientAccountId: string | null
}

export interface MetaAdsCampaign {
  id: string
  metaCampaignId: string
  name: string
  objective: string
  status: string
  dailyBudget: number | null
  lifetimeBudget: number | null
}

export interface MetaAdsInsightPoint {
  date: string
  impressions: number
  clicks: number
  spend: number
  reach: number
  ctr: number
  cpc: number
  cpm: number
  conversions: number
}

export interface MetaAdsOverviewKpis {
  spend: number
  impressions: number
  clicks: number
  ctr: number
  cpc: number
  conversions: number
}

export interface MetaAdsOverview {
  connection: MetaAdsConnection
  kpis: MetaAdsOverviewKpis
  adAccountId: string
}

export interface MetaAdsOAuthStart {
  authorizationUrl: string
  expiresAt: string
}
