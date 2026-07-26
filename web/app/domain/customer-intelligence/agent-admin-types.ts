export const INTELLIGENCE_AI_PROVIDER_OPTIONS = [
  { value: 'openai', label: 'OpenAI' },
  { value: 'gemini', label: 'Google Gemini' },
  { value: 'glm', label: 'GLM' },
] as const

export type IntelligenceAIProvider = (typeof INTELLIGENCE_AI_PROVIDER_OPTIONS)[number]['value']
export type IntelligenceModelStatus = 'enabled' | 'disabled'
export type IntelligenceAgentStatus = 'enabled' | 'disabled'

export interface IntelligenceModel {
  id: string
  provider: IntelligenceAIProvider
  model: string
  baseUrl: string
  status: IntelligenceModelStatus
  config: Record<string, unknown>
  revision: number
}

export interface IntelligenceModelWriteInput {
  id?: string
  provider: IntelligenceAIProvider
  model: string
  baseUrl: string
  status: IntelligenceModelStatus
  config: Record<string, unknown>
  revision: number
}

export interface IntelligenceCredentialSecretStatus {
  set: boolean
  last4: string
}

export interface IntelligenceCredential {
  id: string
  provider: IntelligenceAIProvider
  label: string
  secret: IntelligenceCredentialSecretStatus
  updatedAt: string
}

export interface IntelligenceCredentialWriteInput {
  provider: IntelligenceAIProvider
  label: string
  apiKey: string
}

export interface IntelligenceAgent {
  id: string
  name: string
  purpose: string
  status: IntelligenceAgentStatus
  activeVersionId: string
  updatedAt: string
  revision: number
}

export interface IntelligenceAgentCreateInput {
  clientAccountId: string
  slug: string
  name: string
}

export interface IntelligenceAgentPatchInput {
  name: string
  enabled: boolean
  expectedRevision: number
}

export interface IntelligenceAgentVersion {
  id: string
  agentId: string
  version: number
  status: string
  modelId: string
  credentialId: string
  temperature: number
  maxOutputTokens: number
  timeoutMs: number
  promptOverride: string
  config: Record<string, unknown>
}

export interface IntelligenceAgentVersionWriteInput {
  modelId: string
  credentialId: string
  temperature: number
  maxOutputTokens: number
  timeoutMs: number
  promptOverride: string
  config: Record<string, unknown>
}
