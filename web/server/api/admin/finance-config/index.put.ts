// MOCK BFF (temporario) — salva config financeira. financeMockStore.ts.
export default defineEventHandler(async (event) => {
  const body = await readBody<Record<string, unknown>>(event)
  return { status: 'success', data: saveConfig(body || {}) }
})
