import { createError, getRouterParam } from 'h3'
import { assertReferenceAdmin } from '~~/server/utils/reference-admin-access'
import { deleteLeadById } from '~~/server/utils/leads-repository'

export default defineEventHandler((event) => {
  const access = assertReferenceAdmin(event)

  const id = Number.parseInt(String(getRouterParam(event, 'id') ?? ''), 10)
  if (!Number.isFinite(id) || id <= 0) {
    throw createError({ statusCode: 400, statusMessage: 'Lead id invalido.' })
  }

  const deleted = deleteLeadById(id, {
    viewerUserType: access.userType,
    viewerClientId: access.clientId,
  })
  if (!deleted) {
    throw createError({ statusCode: 404, statusMessage: 'Lead nao encontrado.' })
  }

  return {
    status: 'success',
  }
})
