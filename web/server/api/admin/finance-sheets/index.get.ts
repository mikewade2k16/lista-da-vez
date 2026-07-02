// MOCK BFF (temporario) — lista planilhas. Ver server/utils/financeMockStore.ts.
export default defineEventHandler((event) => {
  const query = getQuery(event)
  const data = listSheets(query.coreTenantId, {
    q: String(query.q || ''),
    period: String(query.period || ''),
  })
  return {
    status: 'success',
    data,
    meta: { page: 1, limit: data.length, total: data.length, totalPages: 1, hasMore: false },
  }
})
