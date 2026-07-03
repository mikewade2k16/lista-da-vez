import { describe, expect, it } from 'vitest'

import {
  ERP_RECORDS_COLUMNS_BY_TAB,
  ERP_RECORDS_DATA_TYPE_BY_TAB,
  ERP_RECORDS_LABEL_BY_TAB,
  formatCurrency,
  formatDateTime,
  formatNumber,
  formatPrice,
  formatSourceFileName,
  productRowKey,
  recordsRowKey,
} from './erp-display'

// Intl separa o simbolo da moeda do valor com NBSP (U+00A0). Normalizamos para um
// espaco comum para o assert nao depender do glifo exato.
const money = (value: string) => value.replace(/ /g, ' ')

describe('formatCurrency', () => {
  it('formats cents into pt-BR currency', () => {
    expect(money(formatCurrency(0))).toBe('R$ 0,00')
    expect(money(formatCurrency(null))).toBe('R$ 0,00')
    expect(money(formatCurrency(123456))).toBe('R$ 1.234,56')
  })
})

describe('formatNumber', () => {
  it('formats numbers with pt-BR thousands separators', () => {
    expect(formatNumber(null)).toBe('0')
    expect(formatNumber(1234567)).toBe('1.234.567')
  })
})

describe('formatDateTime', () => {
  it('returns a dash for empty input', () => {
    expect(formatDateTime('')).toBe('-')
    expect(formatDateTime(null)).toBe('-')
  })

  it('returns the raw string when it is not a valid date', () => {
    expect(formatDateTime('not-a-date')).toBe('not-a-date')
  })

  it('formats a local ISO string without asserting the full ICU output', () => {
    // Sem Z: hora local, independente do TZ da maquina/CI.
    const formatted = formatDateTime('2026-05-21T14:30:00')
    expect(formatted).toContain('2026')
    expect(formatted).toContain('14:30')
    expect(formatted).toContain(' as ')
  })
})

describe('formatSourceFileName', () => {
  it('returns a dash for empty input', () => {
    expect(formatSourceFileName(null)).toBe('-')
  })

  it('returns the raw name when there is no timestamp prefix', () => {
    expect(formatSourceFileName('produtos.csv')).toBe('produtos.csv')
  })

  it('parses the leading timestamp into a local date', () => {
    expect(formatSourceFileName('20260521143000_produtos.csv')).toContain('14:30')
  })
})

describe('formatPrice', () => {
  it('prefers finite cents and falls back to the raw value', () => {
    expect(money(formatPrice(undefined, 1990))).toBe('R$ 19,90')
    expect(money(formatPrice('1990', undefined))).toBe('R$ 19,90')
  })

  it('characterizes the null-cents and zero-cents branches as a dash', () => {
    // caracterizacao: Number(null) = 0 e' finito e vence o rawValue, entao cai no '-'.
    expect(formatPrice('1990', null)).toBe('-')
    // caracterizacao: cents zero (finito) tambem cai no '-'.
    expect(formatPrice(undefined, 0)).toBe('-')
  })
})

describe('row key helpers', () => {
  it('builds product row keys from sku and identifier', () => {
    expect(productRowKey({ sku: 'A', identifier: 'B' })).toBe('A-B')
  })

  it('falls back through the records id chain, ending on the index', () => {
    expect(recordsRowKey({}, 3)).toBe('3')
    expect(recordsRowKey({ order_id: 'o1' }, 0)).toBe('o1')
    expect(recordsRowKey({ id: 'i1', order_id: 'o2' }, 0)).toBe('i1')
  })
})

describe('ERP records declarative tables', () => {
  it('keeps the column/data-type/label maps in sync with unique column ids', () => {
    for (const tab of Object.keys(ERP_RECORDS_COLUMNS_BY_TAB)) {
      expect(ERP_RECORDS_DATA_TYPE_BY_TAB[tab]).toBeDefined()
      expect(ERP_RECORDS_LABEL_BY_TAB[tab]).toBeDefined()

      const ids = ERP_RECORDS_COLUMNS_BY_TAB[tab].map((column) => column.id)
      expect(new Set(ids).size).toBe(ids.length)
    }
  })
})
