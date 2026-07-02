// MOCK BFF (temporario) — config financeira. Ver server/utils/financeMockStore.ts.
export default defineEventHandler((event) => {
  const query = getQuery(event)
  return { status: 'success', data: getConfig(query.coreTenantId) }
})
