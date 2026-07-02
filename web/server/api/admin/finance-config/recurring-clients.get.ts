// MOCK BFF (temporario) — recorrencias de clientes (vazio no mock).
// No back real vira read model (core.accounts + finance.recurring_entries).
export default defineEventHandler((event) => {
  const query = getQuery(event)
  return { status: 'success', data: listRecurringClients(query.coreTenantId) }
})
