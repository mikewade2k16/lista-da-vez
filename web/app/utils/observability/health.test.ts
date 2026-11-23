import { describe, expect, it } from 'vitest'

import type { ObsSecurityCheck } from '~/types/observability'
import {
  alertChannelText,
  checkStatusBand,
  formatAge,
  formatBytes,
  groupChecksByCategory,
  resolveLinkTab,
  securityCoverage,
  shouldSendHeartbeat,
  statusBand,
} from './health'

describe('health utils', () => {
  it('mapeia status para faixa', () => {
    expect(statusBand('ok')).toBe('good')
    expect(statusBand('critical')).toBe('bad')
    expect(statusBand('unknown')).toBe('none')
  })

  it('formata idade relativa', () => {
    const now = new Date('2026-01-01T12:00:00Z').getTime()
    expect(formatAge('2026-01-01T11:59:58Z', now)).toBe('agora')
    expect(formatAge('2026-01-01T11:59:30Z', now)).toBe('há 30 s')
    expect(formatAge('2026-01-01T11:50:00Z', now)).toBe('há 10 min')
    expect(formatAge('2026-01-01T09:00:00Z', now)).toBe('há 3 h')
    expect(formatAge('2025-12-30T12:00:00Z', now)).toBe('há 2 d')
    expect(formatAge(null, now)).toBe('-')
  })

  it('nao testado fica amarelo', () => {
    expect(checkStatusBand('not_tested')).toBe('warn')
    expect(checkStatusBand('pass')).toBe('good')
    expect(checkStatusBand('fail')).toBe('bad')
  })

  it('calcula cobertura de seguranca honesta', () => {
    const low = securityCoverage({
      checks: { pass: 2, fail: 1, partial: 0, not_tested: 27 },
      findings: { open: { critical: 1, high: 2, medium: 0, low: 0 }, fixed: 0 },
    })
    expect(low.low).toBe(true)
    expect(low.text).toContain('NÃO foi testado')
    expect(low.detail).toContain('3 de 30 ataques testados, 1 falha(s)')
    expect(low.detail).toContain('1 crítico(s), 2 alto(s)')
    expect(securityCoverage(null).low).toBe(true)
  })

  it('agrupa checks por categoria', () => {
    const base = { key: '', title: '', standard: '', status: 'pass', detail: '', checkedAt: null }
    const groups = groupChecksByCategory([
      { ...base, category: 'B', source: 'x' },
      { ...base, category: 'A', source: 'x' },
      { ...base, category: 'B', source: 'x' },
    ] as ObsSecurityCheck[])
    expect(groups.map((g) => [g.category, g.items.length])).toEqual([
      ['A', 1],
      ['B', 2],
    ])
  })

  it('resolve aba do link', () => {
    expect(resolveLinkTab({ tab: 'db' })).toBe('db')
    expect(resolveLinkTab({ tab: 'nada' })).toBeNull()
    expect(resolveLinkTab(null)).toBeNull()
  })

  it('texto do canal de alerta', () => {
    expect(alertChannelText({ configured: true, recipients: 2 })).toBe(
      'Avisos por e-mail: ativo para 2 destinatário(s)',
    )
    expect(alertChannelText({ configured: false, recipients: 0 })).toContain('OBS_ALERT_EMAILS')
  })

  it('decide envio do heartbeat', () => {
    const base = {
      hidden: false,
      loggedIn: true,
      enabled: true,
      lastSentAt: 1000,
      now: 2000,
      routeChanged: false,
    }
    expect(shouldSendHeartbeat(base)).toBe(false)
    expect(shouldSendHeartbeat({ ...base, now: 31_000 })).toBe(true)
    expect(shouldSendHeartbeat({ ...base, routeChanged: true })).toBe(true)
    expect(shouldSendHeartbeat({ ...base, routeChanged: true, hidden: true })).toBe(false)
    expect(shouldSendHeartbeat({ ...base, now: 99_000, loggedIn: false })).toBe(false)
    expect(shouldSendHeartbeat({ ...base, now: 99_000, enabled: false })).toBe(false)
  })

  it('formata bytes', () => {
    expect(formatBytes(512)).toBe('512 B')
    expect(formatBytes(1536)).toBe('1,5 KB')
    expect(formatBytes(undefined)).toBe('-')
  })
})
