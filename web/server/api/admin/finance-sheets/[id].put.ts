// MOCK BFF (temporario) — atualiza planilha (full-replace). financeMockStore.ts.
export default defineEventHandler(async (event) => {
  const id = getRouterParam(event, 'id') || ''
  const body = await readBody<Record<string, unknown>>(event)
  const data = updateSheet(id, body || {})
  if (!data) throw createError({ statusCode: 404, statusMessage: 'Sheet not found' })
  return { status: 'success', data }
})
