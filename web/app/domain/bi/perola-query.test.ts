import { describe, expect, it } from 'vitest'

import {
  buildPerolaQueryFilters,
  createInitialPerolaFilterDrafts,
  normalizePerolaDatasetCatalog,
  normalizePerolaDatasetQueryResponse,
  perolaQueryResultColumns,
} from './perola-query'

const catalogPayload = {
  datasets: [
    {
      id: 'nota',
      label: 'Nota',
      description: 'Notas fiscais',
      defaultLimit: 15,
      maxLimit: 25,
      defaultOrderBy: { field: 'dataEmissao', direction: 'DESC' },
      allowedOrderFields: ['id', 'dataEmissao'],
      filters: [
        { field: 'dataEmissao', valueType: 'date', operators: ['gte', 'lte'] },
        { field: 'id', valueType: 'integer', operators: ['eq'] },
      ],
      requiredFilterRule: 'Informe id ou período.',
      requiredFilterAlternatives: [
        [
          { field: 'dataEmissao', operator: 'gte' },
          { field: 'dataEmissao', operator: 'lte' },
        ],
        [{ field: 'id', operator: 'eq' }],
      ],
      dateRange: { field: 'dataEmissao', maxDays: 31 },
    },
  ],
}

describe('Pérola typed query helpers', () => {
  it('normalizes the public catalog and creates the required filter draft', () => {
    const [dataset] = normalizePerolaDatasetCatalog(catalogPayload)
    expect(dataset?.id).toBe('nota')
    expect(createInitialPerolaFilterDrafts(dataset!)).toMatchObject([
      { field: 'dataEmissao', operator: 'gte' },
      { field: 'dataEmissao', operator: 'lte' },
    ])
  })

  it('validates and converts a bounded server-side query', () => {
    const [dataset] = normalizePerolaDatasetCatalog(catalogPayload)
    const drafts = createInitialPerolaFilterDrafts(dataset!)
    drafts[0]!.value = '2026-07-01'
    drafts[1]!.value = '2026-07-31'

    expect(buildPerolaQueryFilters(dataset!, drafts)).toEqual({
      filters: [
        { field: 'dataEmissao', operator: 'gte', value: '2026-07-01' },
        { field: 'dataEmissao', operator: 'lte', value: '2026-07-31' },
      ],
      error: '',
    })

    drafts[1]!.value = '2026-08-01'
    expect(buildPerolaQueryFilters(dataset!, drafts).error).toContain('31 dias')
  })

  it('rejects zero identifiers before calling the API', () => {
    const [dataset] = normalizePerolaDatasetCatalog(catalogPayload)
    const result = buildPerolaQueryFilters(dataset!, [
      { id: 'id', field: 'id', operator: 'eq', value: '0' },
    ])
    expect(result.error).toContain('inteiro positivo')
  })

  it('normalizes a structured page and derives manageable dynamic columns', () => {
    const response = normalizePerolaDatasetQueryResponse({
      datasetId: 'nota',
      datasetLabel: 'Nota',
      pageNumber: 1,
      limit: 15,
      totalRecords: 20,
      totalPages: 2,
      returned: 1,
      hasMore: true,
      orderBy: { field: 'id', direction: 'DESC' },
      filterCount: 1,
      durationMs: 25,
      records: [{ id: 1, numDocumento: '123' }],
    })

    expect(response?.hasMore).toBe(true)
    expect(perolaQueryResultColumns(response?.records || []).map((column) => column.id)).toEqual([
      'id',
      'numDocumento',
    ])
  })
})
