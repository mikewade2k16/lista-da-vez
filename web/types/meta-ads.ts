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
}

export interface MetaAdsAdAccount {
  id: string
  metaAdAccountId: string
  name: string
  currency: string
  status: string
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

// --- Assistente MCP (fase MA, §12 do plano canonico) ---
// Contrato CONGELADO com o backend Go:
//   GET  /v1/meta-ads/assistant/messages?limit=50 → MetaAdsAssistantMessage[]
//   POST /v1/meta-ads/assistant/messages {message, adAccountId}
//        → { messages: MetaAdsAssistantMessage[], syncTriggered: boolean }
//   GET  /v1/meta-ads/assistant/health → MetaAdsAssistantHealth (200 sempre)

export interface MetaAdsAssistantAction {
  tool: string
  summary: string
  status: 'ok' | 'error'
}

export interface MetaAdsAssistantMessage {
  id: string
  role: 'user' | 'assistant'
  content: string
  actions: MetaAdsAssistantAction[]
  createdAt: string
}

export interface MetaAdsAssistantHealth {
  ok: boolean
  claudeAuth: boolean
  detail: string
}

export interface MetaAdsAssistantSettings {
  model: string
  systemPrompt: string
}
