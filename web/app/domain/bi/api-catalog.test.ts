import { describe, expect, it } from 'vitest'

import {
  BI_API_ENTITIES,
  biApiEntityFieldCount,
  biApiSchemaRows,
  filterBiApiSchemaRows,
} from './api-catalog'

describe('Pérola BI API catalog', () => {
  it('documents the six confirmed entities and endpoints', () => {
    expect(BI_API_ENTITIES.map(({ id, endpoint }) => ({ id, endpoint }))).toEqual([
      { id: 'item', endpoint: '/item/find' },
      { id: 'image-item', endpoint: '/imagemItem/find' },
      { id: 'purchase-price', endpoint: '/itemSaldoPrecoCompra/find' },
      { id: 'invoice', endpoint: '/nota/find' },
      { id: 'invoice-item', endpoint: '/notaItem/find' },
      { id: 'inventory', endpoint: '/inventario/find' },
    ])
  })

  it('keeps every observed field unique inside its entity', () => {
    for (const entity of BI_API_ENTITIES) {
      const fields = entity.fieldGroups.flatMap((fieldGroup) =>
        fieldGroup.fields.map((field) => field.key),
      )

      expect(new Set(fields).size, entity.label).toBe(fields.length)
      expect(biApiEntityFieldCount(entity)).toBe(fields.length)
    }
  })

  it('marks inventory as requiring a selective filter', () => {
    const inventory = BI_API_ENTITIES.find((entity) => entity.id === 'inventory')

    expect(inventory?.tone).toBe('attention')
    expect(inventory?.queryRule).toContain('obrigatório')
    expect(inventory?.performance).toContain('35 s')
  })

  it('flattens every observed column into the six-entity schema table', () => {
    const rows = biApiSchemaRows()
    const expected = BI_API_ENTITIES.reduce(
      (total, entity) => total + biApiEntityFieldCount(entity),
      0,
    )

    expect(rows).toHaveLength(expected)
    expect(new Set(rows.map((row) => row.id)).size).toBe(expected)
    expect(new Set(rows.map((row) => row.entityId))).toEqual(
      new Set(BI_API_ENTITIES.map((entity) => entity.id)),
    )
  })

  it('filters the complete schema by entity, type and normalized text', () => {
    const rows = biApiSchemaRows()
    const inventoryNumbers = filterBiApiSchemaRows(rows, {
      entityId: 'inventory',
      type: 'number',
    })
    const customerDocument = filterBiApiSchemaRows(rows, {
      search: 'inscricao estadual',
    })

    expect(inventoryNumbers.every((row) => row.entityId === 'inventory')).toBe(true)
    expect(inventoryNumbers.every((row) => row.type === 'number')).toBe(true)
    expect(customerDocument.map((row) => row.field)).toContain('pessoaRgIe')
  })
})
