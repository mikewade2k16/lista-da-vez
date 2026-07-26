export type RecommendationPolicyValue = string | number | boolean

export interface RecommendationPolicyField {
  key: string
  label: string
  description?: string
  type: 'string' | 'number' | 'boolean' | 'select'
  min?: number
  max?: number
  options?: Array<{ value: string; label: string }>
  immutableFloor?: boolean
}

export interface RecommendationPolicyDefinition {
  policyKey: string
  name: string
  description: string
  fields: RecommendationPolicyField[]
  invariants: Array<{
    key: string
    label: string
    description: string
  }>
}

export interface RecommendationPolicyVersion {
  id: string
  versionNumber: number
  status: 'draft' | 'validated' | 'published' | 'superseded'
  revision: number
  values: Record<string, RecommendationPolicyValue>
  validationStatus?: 'pending' | 'valid' | 'invalid'
  validationReasonCodes?: string[]
  createdAt: string
  publishedAt?: string
}

export interface RecommendationPolicyView {
  definition: RecommendationPolicyDefinition
  draft?: RecommendationPolicyVersion
  effective?: RecommendationPolicyVersion
  versions: RecommendationPolicyVersion[]
  binding?: {
    id: string
    revision: number
    versionId: string
    scopeLabel: string
  }
  canEdit: boolean
  canPublish: boolean
  canRollback: boolean
}
