import { describe, expect, it } from 'vitest'
import {
  customerVersionSummary,
  formatBytes,
  formatRecordDate,
  maskDocument,
  originalSummary,
  receiptLinkSummary,
  retentionDaysLabel,
} from './finance-records'
import type { FinanceReceiptItem } from '../types/records'

const item = (link?: FinanceReceiptItem['link']): FinanceReceiptItem => ({
  document: {
    id: 'd1',
    sourceName: 'uber.pdf',
    extractedText: '',
    updatedAt: 'v1',
    warnings: [],
    missingFields: [],
    fields: {
      amount: 32.5,
      paymentDate: '2026-11-05',
      payeeName: 'Uber',
    } as FinanceReceiptItem['document']['fields'],
  },
  link,
  sheetTitle: link ? 'Casa novembro' : undefined,
})

describe('finance records labels', () => {
  it('formats date-only values without timezone shift', () => {
    expect(formatRecordDate('2026-11-05')).toBe('05/11/2026')
    expect(formatRecordDate('')).toBe('')
    expect(formatRecordDate('invalid')).toBe('')
  })

  it('describes receipt link states', () => {
    expect(receiptLinkSummary(item()).tone).toBe('muted')
    const link = {
      documentId: 'd1',
      sheetId: 's',
      lineId: 'l',
      kind: 'saida',
      scope: 'pessoal',
      amount: '32.50',
      createdLine: true,
      status: 'linked',
    } as const
    expect(receiptLinkSummary(item(link)).text).toBe(
      'Lançado como saída pessoal em “Casa novembro”',
    )
    expect(receiptLinkSummary(item({ ...link, status: 'line_removed' })).tone).toBe('warning')
    expect(receiptLinkSummary(item({ ...link, status: 'amount_changed' })).text).toContain(
      'R$ 32.50',
    )
  })

  it('describes original retention states honestly', () => {
    expect(originalSummary(undefined).text).toContain('não guardado')
    const base = {
      documentId: 'd1',
      fileName: 'a.pdf',
      contentType: 'application/pdf',
      sizeBytes: 2048,
    }
    expect(
      originalSummary({ ...base, status: 'stored', expiresAt: '2027-02-03T10:00:00Z' }).text,
    ).toContain('2 KB')
    expect(
      originalSummary({ ...base, status: 'purged', purgedAt: '2027-02-03T10:00:00Z' }).text,
    ).toContain('retenção')
    expect(originalSummary({ ...base, status: 'not_stored' }).tone).toBe('warning')
  })

  it('formats sizes, presets, masks and versions', () => {
    expect(formatBytes(0)).toBe('0 KB')
    expect(formatBytes(3 * 1024 * 1024)).toBe('3,0 MB')
    expect(retentionDaysLabel(90)).toBe('3 meses (90 dias)')
    expect(retentionDaysLabel(45)).toBe('45 dias')
    expect(maskDocument('52998224725')).toBe('•••••••4725')
    expect(
      customerVersionSummary({
        version: 2,
        createdAt: '2026-11-07',
        data: { name: 'João', amount: 650, editedManually: true },
      }),
    ).toBe('v2 · 07/11/2026 · editado na tela · João · R$ 650,00')
  })
})
