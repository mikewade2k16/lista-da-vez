export interface ExplanationPricing {
  version: string
  source: string
  validFrom: string
  validTo: string
  inputUsdPerMillion: string
  outputUsdPerMillion: string
}
export interface PlusExplanationConfig {
  enabled: boolean
  credentialId: string
  model: string
  taskClass: string
  maxInputBytes: number
  maxOutputTokens: number
  timeoutMs: number
  dailyRequests: number
  maxRunCostUsd: string
  pricing: ExplanationPricing | null
}
export const emptyPlusExplanation = (): PlusExplanationConfig => ({
  enabled: false,
  credentialId: '',
  model: '',
  taskClass: 'source_summary',
  maxInputBytes: 8000,
  maxOutputTokens: 512,
  timeoutMs: 10000,
  dailyRequests: 20,
  maxRunCostUsd: '',
  pricing: null,
})
