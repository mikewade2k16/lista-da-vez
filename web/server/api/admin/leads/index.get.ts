import { getQuery } from 'h3'
import { assertReferenceAdmin } from '~~/server/utils/reference-admin-access'
import { listLeads } from '~~/server/utils/leads-repository'

export default defineEventHandler((event) => {
  const query = getQuery(event)
  const access = assertReferenceAdmin(event)

  const page = Number.parseInt(String(query.page ?? '1'), 10)
  const limit = Number.parseInt(String(query.limit ?? '60'), 10)

  const { items, meta, filters } = listLeads({
    page: Number.isFinite(page) ? page : 1,
    limit: Number.isFinite(limit) ? limit : 60,
    q: String(query.q ?? ''),
    clientId: String(query.clientId ?? ''),
    viewerUserType: access.userType,
    viewerClientId: access.clientId,
  })

  return {
    status: 'success',
    data: items,
    meta,
    filters,
  }
})
