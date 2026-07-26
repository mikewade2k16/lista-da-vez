export type SourceSuggestionStatus = 'proposed' | 'accepted' | 'rejected' | 'expired' | 'unknown'

export type SourceSuggestionReviewStatus = 'accepted' | 'rejected'

export interface SourceSuggestionReviewReason {
  value: string
  label: string
}

export interface SourceSuggestionView {
  id: string
  relationshipId: string
  sourceKey: string
  gapCodes: string[]
  rationaleCode: string
  rationale: string
  confidence: number
  status: SourceSuggestionStatus
  expiresAt: string
  createdAt: string
  allowedActions: SourceSuggestionReviewStatus[]
}

export interface SourceSuggestionReviewInput {
  status: SourceSuggestionReviewStatus
  reason: string
}

export const SOURCE_SUGGESTION_REVIEW_REASONS: Readonly<
  Record<SourceSuggestionReviewStatus, readonly SourceSuggestionReviewReason[]>
> = {
  accepted: [
    { value: 'source_relevant', label: 'Fonte relevante para este cliente' },
    { value: 'profile_gap_confirmed', label: 'Lacuna de dados confirmada' },
  ],
  rejected: [
    { value: 'source_not_relevant', label: 'Fonte nao relevante' },
    { value: 'data_already_available', label: 'Dados ja disponiveis' },
    { value: 'privacy_or_consent_risk', label: 'Risco de privacidade ou consentimento' },
  ],
}

export function validSourceSuggestionReviewReason(
  status: SourceSuggestionReviewStatus,
  reason: string,
): boolean {
  const normalized = String(reason || '').trim()
  return SOURCE_SUGGESTION_REVIEW_REASONS[status].some((option) => option.value === normalized)
}
