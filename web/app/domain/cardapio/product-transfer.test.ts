import { describe, expect, it } from 'vitest'

import {
  applyProductImportCategoryDecisions,
  detectProductTransferFormat,
  parseProductTransfer,
  serializeProductTransfer,
  type ProductTransferDocument,
} from '~/domain/cardapio/product-transfer'

function sampleDocument(): ProductTransferDocument {
  return {
    version: 1,
    exportedAt: '2026-07-27T12:00:00.000Z',
    products: [
      {
        categorySlug: 'entradas',
        categoryName: 'Entradas',
        slug: 'bolinho-de-bacalhau',
        name: 'Bolinho de Bacalhau',
        shortDesc: 'Crocante, leve',
        description: 'Receita da casa\ncom limao.',
        body: '',
        priceCents: 4200,
        compareAtPriceCents: 0,
        imageUrl: '',
        gallery: ['https://example.com/1.jpg'],
        weight: '180g',
        cookTime: '15 min',
        diet: [],
        allergens: ['peixe'],
        pairing: { name: 'Vinho branco', type: 'vinho', priceCents: 0 },
        tags: ['entrada'],
        isAvailable: true,
        isFeatured: false,
        sortOrder: 1,
        variations: [{ name: 'Grande', priceDeltaCents: 500, sortOrder: 0 }],
        addons: [{ name: 'Molho', priceCents: 200, sortOrder: 0 }],
      },
    ],
  }
}

describe('product transfer', () => {
  it('round-trips every portable field through CSV', () => {
    const source = sampleDocument()
    const csv = serializeProductTransfer(source, 'csv')
    const parsed = parseProductTransfer(csv, 'csv')

    expect(parsed.products).toEqual(source.products)
  })

  it('accepts both the versioned JSON document and a direct array', () => {
    const source = sampleDocument()

    expect(parseProductTransfer(serializeProductTransfer(source, 'json'), 'json').products).toEqual(
      source.products,
    )
    expect(parseProductTransfer(JSON.stringify(source.products), 'json').products).toEqual(
      source.products,
    )
  })

  it('rejects unsupported extensions and empty files', () => {
    expect(detectProductTransferFormat('catalogo.json')).toBe('json')
    expect(detectProductTransferFormat('catalogo.csv')).toBe('csv')
    expect(() => detectProductTransferFormat('catalogo.md')).toThrow(/nao suportado/i)
    expect(() => parseProductTransfer('[]', 'json')).toThrow(/nao contem produtos/i)
  })

  it('renames, merges or removes proposed categories before import', () => {
    const products = [
      sampleDocument().products[0],
      {
        ...sampleDocument().products[0],
        slug: 'suco',
        name: 'Suco',
        categorySlug: 'bebidas-novas',
        categoryName: 'Bebidas novas',
      },
    ]
    const result = applyProductImportCategoryDecisions(products, [
      {
        originalSlug: 'entradas',
        originalName: 'Entradas importadas',
        name: 'Entradas',
        removed: false,
      },
      {
        originalSlug: 'bebidas-novas',
        originalName: 'Bebidas novas',
        name: 'Bebidas novas',
        removed: true,
      },
    ])

    expect(result.acceptedCategorySlugs).toEqual(['entradas'])
    expect(result.products[0]).toMatchObject({
      categorySlug: 'entradas',
      categoryName: 'Entradas',
    })
    expect(result.products[1]).toMatchObject({ categorySlug: '', categoryName: '' })
  })
})
