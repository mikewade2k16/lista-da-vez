import { parseCents } from './types'

// No grouping while editing: the cursor and selection remain under browser control.
export function moneyDraft(reais: number): string {
  if (!Number.isFinite(reais) || reais === 0) return ''
  return String(Number(reais.toFixed(2))).replace('.', ',')
}

export function moneyValue(draft: string, wholeAmounts: boolean): number {
  const cents = parseCents(draft)
  return (wholeAmounts ? Math.trunc(cents / 100) * 100 : cents) / 100
}
