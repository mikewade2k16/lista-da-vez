import { describe, expect, it } from 'vitest'

import type { ObsAlertSettings, ObsRuleCatalogItem } from '~/types/observability-settings'

import {
  alertsGeneralSummary,
  cloneJson,
  customizedRules,
  emailSummary,
  fieldErrorsFrom,
  isValidEmail,
  minutesLabel,
  parseRecipients,
  rulesPayload,
  sameJson,
  weeklySummary,
} from './settings'

const CATALOG: ObsRuleCatalogItem[] = [
  {
    rule: 'slow_route',
    label: 'Rota lenta',
    area: 'api',
    hasThreshold: true,
    defaultThreshold: 2000,
    unit: 'ms',
    min: 200,
    max: 60000,
  },
  {
    rule: 'pg_deadlock',
    label: 'Deadlock',
    area: 'database',
    hasThreshold: false,
    defaultThreshold: 0,
    unit: '',
    min: 0,
    max: 0,
  },
]

describe('settings utils', () => {
  it('valida e-mails', () => {
    expect(isValidEmail('ana@exemplo.com')).toBe(true)
    expect(isValidEmail(' ana@exemplo.com ')).toBe(true)
    expect(isValidEmail('ana@exemplo')).toBe(false)
    expect(isValidEmail('sem-arroba')).toBe(false)
    expect(isValidEmail('a b@x.com')).toBe(false)
  })

  it('separa e-mails validos, invalidos e repetidos', () => {
    const parsed = parseRecipients('Ana@x.com, bia@x.com; ana@x.com xx\nca@y.com', ['bia@x.com'])
    expect(parsed.valid).toEqual(['ana@x.com', 'ca@y.com'])
    expect(parsed.invalid).toEqual(['xx'])
    expect(parseRecipients('  ').valid).toEqual([])
  })

  it('le os erros por campo do 400', () => {
    const cause = {
      data: {
        error: { details: { fields: { renotifyMinutes: 'Informe de 15 a 1440 minutos.', x: 3 } } },
      },
    }
    expect(fieldErrorsFrom(cause)).toEqual({ renotifyMinutes: 'Informe de 15 a 1440 minutos.' })
    expect(fieldErrorsFrom(new Error('x'))).toEqual({})
    expect(fieldErrorsFrom(null)).toEqual({})
  })

  it('conta regras personalizadas', () => {
    const rules: ObsAlertSettings['rules'] = {
      slow_route: { enabled: true, severity: 'auto', threshold: 2000 },
      pg_deadlock: { enabled: false, severity: 'auto', threshold: null },
    }
    expect(customizedRules(rules, CATALOG)).toBe(1)
    rules.slow_route = { enabled: true, severity: 'high', threshold: 2000 }
    expect(customizedRules(rules, CATALOG)).toBe(2)
    rules.slow_route = { enabled: true, severity: 'auto', threshold: 3000 }
    rules.pg_deadlock = { enabled: true, severity: 'auto', threshold: null }
    expect(customizedRules(rules, CATALOG)).toBe(1)
  })

  it('monta o corpo das regras sem limite para regra sem limite', () => {
    const payload = rulesPayload(
      {
        slow_route: { enabled: true, severity: '', threshold: 2500 },
        pg_deadlock: { enabled: false, severity: 'high', threshold: 7 },
        inexistente: { enabled: true, severity: 'auto', threshold: 1 },
      },
      CATALOG,
    )
    expect(payload).toEqual({
      slow_route: { enabled: true, severity: 'auto', threshold: 2500 },
      pg_deadlock: { enabled: false, severity: 'high', threshold: null },
    })
  })

  it('clona e compara JSON', () => {
    const original = { a: [1, 2], b: { c: 'x' } }
    const copy = cloneJson(original)
    expect(copy).not.toBe(original)
    expect(sameJson(original, copy)).toBe(true)
    copy.a.push(3)
    expect(sameJson(original, copy)).toBe(false)
  })

  it('formata resumos', () => {
    expect(minutesLabel(60)).toBe('1 h')
    expect(minutesLabel(90)).toBe('90 min')
    const alerts: ObsAlertSettings = {
      recipients: [],
      renotifyMinutes: 120,
      quietHours: {
        enabled: true,
        start: '22:00',
        end: '07:00',
        timezone: 'America/Sao_Paulo',
        allowCritical: true,
      },
      rules: {},
    }
    expect(alertsGeneralSummary(alerts, 2)).toBe(
      'padrão do ambiente (2); reaviso a cada 2 h; silêncio 22:00–07:00',
    )
    expect(
      emailSummary({
        subjectPrefix: '[Omni]',
        group: false,
        sections: ['summary'],
        includeResolved: true,
        footer: '',
      }),
    ).toBe('Prefixo [Omni]; um e-mail por alerta; 1 seção(ões)')
    expect(
      weeklySummary({ enabled: false, weekday: 1, hour: 8, timezone: '', recipients: [] }),
    ).toBe('Desligado')
    expect(
      weeklySummary({ enabled: true, weekday: 1, hour: 8, timezone: '', recipients: [] }),
    ).toBe('Segunda-feira, 08:00')
  })
})
