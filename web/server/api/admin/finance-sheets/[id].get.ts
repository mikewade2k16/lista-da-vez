// MOCK BFF (temporario) — detalhe da planilha. Ver server/utils/financeMockStore.ts.
export default defineEventHandler((event) => {
  const id = getRouterParam(event, 'id') || ''
  const data = getSheet(id)
  if (!data) throw createError({ statusCode: 404, statusMessage: 'Sheet not found' })
  return { status: 'success', data }
})
