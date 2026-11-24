import { describe, expect, it } from 'vitest'

import {
  alertControlBadge,
  alertControlDetail,
  formatSilenceUntil,
  isAcknowledged,
  isSilenced,
  SILENCE_OPTIONS,
} from './alerts'

const NOW = new Date('2026-11-24T15:00:00').getTime()

describe('alerts utils', () => {
  it('oferece 1 h, 8 h, 24 h e 7 dias', () => {
    expect(SILENCE_OPTIONS.map((option) => option.hours)).toEqual([1, 8, 24, 168])
  })

  it('so considera silencio no futuro', () => {
    expect(isSilenced({ silencedUntil: '2026-11-24T16:00:00' }, NOW)).toBe(true)
    expect(isSilenced({ silencedUntil: '2026-11-24T14:00:00' }, NOW)).toBe(false)
    expect(isSilenced({ silencedUntil: null }, NOW)).toBe(false)
    expect(isSilenced({ silencedUntil: 'lixo' }, NOW)).toBe(false)
    expect(isSilenced({}, NOW)).toBe(false)
  })

  it('detecta reconhecimento', () => {
    expect(isAcknowledged({ acknowledgedAt: '2026-11-24T10:00:00Z' })).toBe(true)
    expect(isAcknowledged({ acknowledgedAt: null })).toBe(false)
  })

  it('formata o horario do silencio', () => {
    expect(formatSilenceUntil('2026-11-24T16:30:00', NOW)).toBe('16:30')
    expect(formatSilenceUntil('2026-11-25T09:05:00', NOW)).toBe('25/11 09:05')
    expect(formatSilenceUntil(null, NOW)).toBe('-')
    expect(formatSilenceUntil('x', NOW)).toBe('-')
  })

  it('escolhe o selo: silencio antes de reconhecido', () => {
    expect(alertControlBadge({}, NOW)).toBeNull()
    expect(alertControlBadge({ acknowledgedAt: '2026-11-24T10:00:00Z' }, NOW)).toEqual({
      kind: 'acknowledged',
      text: 'reconhecido',
    })
    expect(
      alertControlBadge(
        { acknowledgedAt: '2026-11-24T10:00:00Z', silencedUntil: '2026-11-24T16:30:00' },
        NOW,
      ),
    ).toEqual({ kind: 'silenced', text: 'silenciado até 16:30' })
    expect(alertControlBadge({ silencedUntil: '2026-11-24T14:00:00' }, NOW)).toBeNull()
  })

  it('descreve quem reconheceu e silenciou', () => {
    expect(alertControlDetail({}, NOW)).toBe('')
    expect(
      alertControlDetail(
        {
          acknowledgedAt: '2026-11-24T10:00:00Z',
          acknowledgedBy: 'Ana',
          silencedUntil: '2026-11-24T16:30:00',
          silencedBy: 'Beto',
        },
        NOW,
      ),
    ).toBe('Reconhecido por Ana · Silenciado por Beto até 16:30')
    expect(alertControlDetail({ acknowledgedAt: '2026-11-24T10:00:00Z' }, NOW)).toBe('Reconhecido')
  })
})
