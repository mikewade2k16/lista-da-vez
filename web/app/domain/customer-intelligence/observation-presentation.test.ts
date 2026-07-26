import { describe, expect, it } from 'vitest'
import type { IntelligenceObservationView } from './audit-types'
import {
  PROTECTED_OBSERVATION_REFERENCE,
  PROTECTED_OBSERVATION_VALUE,
  isObservationFieldProtected,
  safeObservationFieldDisplay,
  safeObservationProvenance,
} from './observation-presentation'

function observation(
  sensitivity: string,
  displayValue = 'Ana 11999999999',
  masked = false,
  revealed = false,
): IntelligenceObservationView {
  return {
    id: 'observation-1',
    sourceKey: 'erp',
    provenanceRef: `obsref:v1:${'a'.repeat(43)}`,
    sensitivity,
    purposeKey: 'customer_profile',
    retentionState: 'active',
    observedAt: '2026-07-23T10:00:00Z',
    revealed,
    snapshotFields: [{ label: 'preferred_name', displayValue, masked }],
  }
}

describe('observation presentation privacy', () => {
  it.each(['personal', 'sensitive', 'restricted'])(
    'oculta valor e proveniencia classificados como %s',
    (sensitivity) => {
      const item = observation(sensitivity)
      const field = item.snapshotFields[0]!

      expect(isObservationFieldProtected(item, field)).toBe(true)
      expect(safeObservationFieldDisplay(item, field)).toBe(PROTECTED_OBSERVATION_VALUE)
      expect(safeObservationProvenance(item)).toBe(PROTECTED_OBSERVATION_REFERENCE)
      expect(safeObservationFieldDisplay(item, field)).not.toContain('Ana')
      expect(safeObservationProvenance(item)).not.toContain('11999999999')
    },
  )

  it('mantem mascara do backend mesmo para classificacao interna', () => {
    const item = observation('internal', 'diagnostico privado', true)
    const field = item.snapshotFields[0]!

    expect(safeObservationFieldDisplay(item, field)).toBe(PROTECTED_OBSERVATION_VALUE)
  })

  it('falha fechado para classificacao ausente ou desconhecida', () => {
    for (const sensitivity of ['', 'future_classification']) {
      const item = observation(sensitivity)
      const field = item.snapshotFields[0]!

      expect(safeObservationFieldDisplay(item, field)).toBe(PROTECTED_OBSERVATION_VALUE)
      expect(safeObservationProvenance(item)).toBe(PROTECTED_OBSERVATION_REFERENCE)
    }
  })

  it.each(['personal', 'sensitive', 'restricted'])(
    'exibe somente resposta explicitamente revelada para %s',
    (sensitivity) => {
      const item = observation(sensitivity, 'Ana', false, true)
      const field = item.snapshotFields[0]!

      expect(isObservationFieldProtected(item, field)).toBe(false)
      expect(safeObservationFieldDisplay(item, field)).toBe('Ana')
      expect(safeObservationProvenance(item)).toBe(`obsref:v1:${'a'.repeat(43)}`)
    },
  )

  it('mantem fail-closed para classificacao desconhecida mesmo se o payload alegar reveal', () => {
    const item = observation('future_classification', 'segredo', false, true)
    const field = item.snapshotFields[0]!

    expect(safeObservationFieldDisplay(item, field)).toBe(PROTECTED_OBSERVATION_VALUE)
    expect(safeObservationProvenance(item)).toBe(PROTECTED_OBSERVATION_REFERENCE)
  })

  it('respeita mascara do backend mesmo em resposta marcada como revelada', () => {
    const item = observation('personal', 'segredo', true, true)
    const field = item.snapshotFields[0]!

    expect(safeObservationFieldDisplay(item, field)).toBe(PROTECTED_OBSERVATION_VALUE)
  })

  it('exibe apenas valor interno nao mascarado', () => {
    const item = observation('internal', 'cliente recorrente')
    const field = item.snapshotFields[0]!

    expect(isObservationFieldProtected(item, field)).toBe(false)
    expect(safeObservationFieldDisplay(item, field)).toBe('cliente recorrente')
    expect(safeObservationProvenance(item)).toBe(`obsref:v1:${'a'.repeat(43)}`)
  })

  it('nao mostra proveniencia legada que possa conter identificador pessoal', () => {
    const item = observation('internal')
    item.provenanceRef = 'erp:customer:11999999999'

    expect(safeObservationProvenance(item)).toBe(PROTECTED_OBSERVATION_REFERENCE)
  })
})
