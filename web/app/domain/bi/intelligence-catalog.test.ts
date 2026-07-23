import { describe, expect, it } from 'vitest'

import {
  BI_INTELLIGENCE_OPPORTUNITIES,
  BI_INTELLIGENCE_SOURCES,
  ERP_DATA_NOT_OBSERVED_IN_PEROLA,
} from './intelligence-catalog'

describe('BI intelligence catalog', () => {
  it('documents Pérola, ERP and Queue as complementary sources', () => {
    expect(BI_INTELLIGENCE_SOURCES.map((source) => source.id)).toEqual(['perola', 'erp', 'queue'])
  })

  it('keeps opportunities unique and attached to known sources', () => {
    const ids = BI_INTELLIGENCE_OPPORTUNITIES.map((opportunity) => opportunity.id)
    const knownSources = new Set(BI_INTELLIGENCE_SOURCES.map((source) => source.id))

    expect(new Set(ids).size).toBe(ids.length)
    for (const opportunity of BI_INTELLIGENCE_OPPORTUNITIES) {
      expect(opportunity.sources.length).toBeGreaterThan(0)
      expect(opportunity.sources.every((source) => knownSources.has(source))).toBe(true)
    }
  })

  it('does not mark unresolved product and fiscal joins as ready', () => {
    const unresolved = ['stock-movement', 'product-profitability', 'fiscal-reconciliation']

    for (const id of unresolved) {
      expect(BI_INTELLIGENCE_OPPORTUNITIES.find((item) => item.id === id)?.readiness).toBe(
        'mapping-gap',
      )
    }
  })

  it('records the ERP fields that were not observed in Pérola', () => {
    expect(ERP_DATA_NOT_OBSERVED_IN_PEROLA).toContain('Forma de pagamento do pedido')
    expect(
      ERP_DATA_NOT_OBSERVED_IN_PEROLA.some((item) => item.toLowerCase().includes('telefone')),
    ).toBe(true)
  })
})
