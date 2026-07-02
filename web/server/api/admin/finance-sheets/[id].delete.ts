// MOCK BFF (temporario) — remove planilha. Ver server/utils/financeMockStore.ts.
export default defineEventHandler((event) => {
  const id = getRouterParam(event, 'id') || ''
  const ok = deleteSheet(id)
  if (!ok) throw createError({ statusCode: 404, statusMessage: 'Sheet not found' })
  return { status: 'success' }
})
