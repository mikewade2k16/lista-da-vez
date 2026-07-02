// MOCK BFF (temporario) — cria planilha. Ver server/utils/financeMockStore.ts.
export default defineEventHandler(async (event) => {
  const body = await readBody<Record<string, unknown>>(event)
  return { status: 'success', data: createSheet(body || {}) }
})
