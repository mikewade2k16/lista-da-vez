// MOCK BFF (temporario) — efetiva/data de uma linha. financeMockStore.ts.
export default defineEventHandler(async (event) => {
  const id = getRouterParam(event, 'id') || ''
  const lineId = getRouterParam(event, 'lineId') || ''
  const body = await readBody<Record<string, unknown>>(event)
  const data = patchLine(id, lineId, body || {})
  if (!data) throw createError({ statusCode: 404, statusMessage: 'Line not found' })
  return { status: 'success', data }
})
