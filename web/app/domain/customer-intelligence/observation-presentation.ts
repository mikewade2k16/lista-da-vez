import type { IntelligenceObservationView } from './audit-types'

type ObservationField = IntelligenceObservationView['snapshotFields'][number]

const VISIBLE_SENSITIVITIES = new Set(['public', 'internal'])
const REVEALABLE_SENSITIVITIES = new Set(['personal', 'sensitive', 'restricted'])
const OPAQUE_PROVENANCE_PATTERN = /^obsref:v1:[A-Za-z0-9_-]{43}$/

export const PROTECTED_OBSERVATION_VALUE = '[conteudo protegido]'
export const PROTECTED_OBSERVATION_REFERENCE = 'Referencia protegida'

export function isProtectedObservationSensitivity(sensitivity: string): boolean {
  const normalized = String(sensitivity || '')
    .trim()
    .toLowerCase()
  return !VISIBLE_SENSITIVITIES.has(normalized)
}

export function isObservationFieldProtected(
  observation: Pick<IntelligenceObservationView, 'sensitivity' | 'revealed'>,
  field: Pick<ObservationField, 'masked'>,
): boolean {
  if (field.masked) return true
  const sensitivity = String(observation.sensitivity || '')
    .trim()
    .toLowerCase()
  if (VISIBLE_SENSITIVITIES.has(sensitivity)) return false
  return !(observation.revealed === true && REVEALABLE_SENSITIVITIES.has(sensitivity))
}

export function safeObservationFieldDisplay(
  observation: Pick<IntelligenceObservationView, 'sensitivity' | 'revealed'>,
  field: Pick<ObservationField, 'displayValue' | 'masked'>,
): string {
  if (isObservationFieldProtected(observation, field)) {
    return PROTECTED_OBSERVATION_VALUE
  }
  return String(field.displayValue || '').trim() || '—'
}

export function safeObservationProvenance(
  observation: Pick<IntelligenceObservationView, 'sensitivity' | 'provenanceRef' | 'revealed'>,
  fallback = 'Proveniencia interna registrada',
): string {
  const sensitivity = String(observation.sensitivity || '')
    .trim()
    .toLowerCase()
  const protectedWithoutReveal =
    !VISIBLE_SENSITIVITIES.has(sensitivity) &&
    !(observation.revealed === true && REVEALABLE_SENSITIVITIES.has(sensitivity))
  if (protectedWithoutReveal) {
    return PROTECTED_OBSERVATION_REFERENCE
  }
  const provenanceRef = String(observation.provenanceRef || '').trim()
  if (!provenanceRef) return fallback
  return OPAQUE_PROVENANCE_PATTERN.test(provenanceRef)
    ? provenanceRef
    : PROTECTED_OBSERVATION_REFERENCE
}
