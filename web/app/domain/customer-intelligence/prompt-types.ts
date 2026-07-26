export const CANONICAL_PROCESS_KEYS = [
  'conversation.triage',
  'conversation.reply',
  'conversation.handoff_summary',
  'memory.extract',
  'profile.summary',
  'recommendation.follow_up',
  'recommendation.offer',
  'recommendation.important_dates',
  'source.suggest',
  'portfolio.opportunity',
  'media.image_analysis',
  'media.document_analysis',
  'quality.review',
] as const

export type CanonicalProcessKey = (typeof CANONICAL_PROCESS_KEYS)[number]

export interface PromptVariableDescriptor {
  key: string
  type: string
  classification: string
  source: string
  maxLength?: number
}

export interface PromptPolicyField {
  key: string
  label: string
  type: 'number' | 'boolean' | 'select' | 'text'
  description?: string
  min?: number
  max?: number
  options?: Array<{ value: string; label: string }>
}

export interface PromptProcessDefinition {
  processKey: string
  definitionId: string
  name: string
  description: string
  owner: string
  status: string
  inputSchemaVersion: string
  outputSchemaVersion: string
  allowedVariables: PromptVariableDescriptor[]
  policySchema?: PromptPolicyField[]
}

export interface PromptVersionView {
  id: string
  version: number
  status: 'draft' | 'validated' | 'published' | 'archived' | string
  revision: number
  promptText?: string
  config?: Record<string, string | number | boolean>
  checksum?: string
  createdAt?: string
  publishedAt?: string | null
}

export interface PromptEvaluationView {
  id: string
  status: string
  qualityScore?: number | null
  safetyScore?: number | null
  schemaScore?: number | null
  cost?: number | null
  latencyMs?: number | null
  violations?: string[]
}

export interface PromptBindingView {
  id: string
  revision: number
  scope: string
  activeVersionId?: string | null
  canaryVersionId?: string | null
  rolloutMode?: string
  agentVersionId?: string | null
}

export interface PromptPublishAgentOption {
  agentId: string
  agentVersionId: string
  label: string
}

export interface PromptProcessView {
  process: PromptProcessDefinition
  draft?: PromptVersionView | null
  published?: PromptVersionView | null
  versions: PromptVersionView[]
  evaluations: PromptEvaluationView[]
  effectiveBinding?: PromptBindingView | null
  publishAgents: PromptPublishAgentOption[]
  rollbackTargetVersionId?: string | null
  canEdit: boolean
  canTest: boolean
  canPublish: boolean
  canRollback: boolean
}

export interface PromptPipelineView {
  pipelineKey: string
  name: string
  status: string
  activeVersion?: number | null
  draftVersion?: number | null
  processKeys: string[]
}

export interface LegacyManagedCapability {
  key: 'transcription' | 'video_summary' | string
  owner: string
  status: string
  deepLink?: string
}

export interface PromptCatalogView {
  processes: PromptProcessDefinition[]
  pipelines: PromptPipelineView[]
  legacyManagedCapabilities: LegacyManagedCapability[]
}

export interface PromptDraftInput {
  promptText: string
  config: Record<string, string | number | boolean>
  expectedRevision: number
}
