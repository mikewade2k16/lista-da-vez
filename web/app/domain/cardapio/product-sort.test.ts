import { describe, expect, it } from 'vitest'

import { sortProductRows, type SortableProductRow } from './product-sort'

function row(overrides: Partial<SortableProductRow>): SortableProductRow {
  return {
    name: '',
    categoryId: '',
    priceReais: 0,
    compareAtPriceReais: 0,
    isAvailable: true,
    isFeatured: false,
    imageUrl: '',
    createdAt: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

const categories: Record<string, string> = { a: 'Sobremesas', b: 'Bebidas' }
const categoryName = (id: string) => categories[id] ?? ''

const rows = [
  row({ name: 'Água', categoryId: 'b', priceReais: 5, imageUrl: 'x.jpg' }),
  row({ name: 'pudim', categoryId: 'a', priceReais: 12, isAvailable: false }),
  row({ name: 'Brigadeiro 10', categoryId: 'a', priceReais: 3, createdAt: '2026-03-01' }),
  row({ name: 'Brigadeiro 2', categoryId: 'a', priceReais: 3, imageUrl: 'y.jpg' }),
]
const names = (list: SortableProductRow[]) => list.map((item) => item.name)

describe('sortProductRows', () => {
  it('sem ordenacao escolhida mostra o ultimo adicionado primeiro', () => {
    expect(names(sortProductRows(rows, null, categoryName))).toEqual([
      'Brigadeiro 10',
      'Água',
      'pudim',
      'Brigadeiro 2',
    ])
  })

  it('mantem a ordem original com chave desconhecida', () => {
    expect(sortProductRows(rows, { key: 'tags', dir: 'asc' }, categoryName)).toBe(rows)
  })

  it('ordena nome sem acento e com numeros naturais', () => {
    expect(names(sortProductRows(rows, { key: 'name', dir: 'asc' }, categoryName))).toEqual([
      'Água',
      'Brigadeiro 2',
      'Brigadeiro 10',
      'pudim',
    ])
  })

  it('ordena pelo nome da categoria e desempata pelo nome', () => {
    expect(names(sortProductRows(rows, { key: 'categoryId', dir: 'asc' }, categoryName))).toEqual([
      'Água',
      'Brigadeiro 2',
      'Brigadeiro 10',
      'pudim',
    ])
  })

  it('ordena preco de forma estavel nos empates', () => {
    expect(names(sortProductRows(rows, { key: 'priceReais', dir: 'desc' }, categoryName))).toEqual([
      'pudim',
      'Água',
      'Brigadeiro 10',
      'Brigadeiro 2',
    ])
  })

  it('coloca disponiveis e com foto primeiro no crescente', () => {
    expect(names(sortProductRows(rows, { key: 'isAvailable', dir: 'desc' }, categoryName))[0]).toBe(
      'pudim',
    )
    expect(names(sortProductRows(rows, { key: 'imageSrc', dir: 'asc' }, categoryName))).toEqual([
      'Água',
      'Brigadeiro 2',
      'pudim',
      'Brigadeiro 10',
    ])
  })

  it('ordena por data de cadastro', () => {
    expect(names(sortProductRows(rows, { key: 'createdAt', dir: 'desc' }, categoryName))[0]).toBe(
      'Brigadeiro 10',
    )
  })
})
