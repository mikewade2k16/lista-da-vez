import { describe, expect, it } from 'vitest'

import { BI_API_ENTITIES, biApiEntityFieldCount } from './api-catalog'

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
})
