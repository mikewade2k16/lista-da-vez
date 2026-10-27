import { describe, expect, it } from 'vitest'
import { moneyDraft, moneyValue } from './money-input'
import { normalizeProductSearch } from './product-search'

describe('edicao de valores do cardapio', () => {
  it('interpreta 22 como 22 reais em ambos os modos', () => {
    expect(moneyValue('22', true)).toBe(22)
    expect(moneyValue('22', false)).toBe(22)
  })
  it('permite apagar todo o valor', () => {
    expect(moneyDraft(0)).toBe('')
    expect(moneyValue('', true)).toBe(0)
    expect(moneyValue('', false)).toBe(0)
  })
  it('aceita centavos explicitos e colagem em formato brasileiro', () => {
    expect(moneyValue('22,50', false)).toBe(22.5)
    expect(moneyValue('22.50', false)).toBe(22.5)
    expect(moneyValue('R$ 1.234,56', false)).toBe(1234.56)
    expect(moneyValue('22,50', true)).toBe(22)
  })
  it('preserva valores existentes com centavos e variacoes negativas', () => {
    expect(moneyDraft(22.5)).toBe('22,5')
    expect(moneyValue(moneyDraft(22.5), false)).toBe(22.5)
    expect(moneyValue('-5,50', false)).toBe(-5.5)
  })
})

describe('busca de produtos', () => {
  it('ignora acentos, caixa e espacos nas pontas', () => {
    expect(normalizeProductSearch(' AÇAÍ ')).toBe('acai')
    expect(normalizeProductSearch('Açaí com banana').includes(normalizeProductSearch('ACAI'))).toBe(
      true,
    )
    expect(normalizeProductSearch(undefined)).toBe('')
  })
})
