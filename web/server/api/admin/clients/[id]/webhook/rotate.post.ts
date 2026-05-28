import { createError, getRouterParam } from 'h3'
import { assertReferenceAdmin } from '~~/server/utils/reference-admin-access'
import { rotateClientWebhookKey } from '~~/server/utils/clients-repository'

export default defineEventHandler((event) => {
  const access = assertReferenceAdmin(event)
  const id = Number.parseInt(String(getRouterParam(event, 'id') ?? ''), 10)
  if (!Number.isFinite(id) || id <= 0) {
    throw createError({ statusCode: 400, statusMessage: 'Cliente invalido.' })
  }

  const updated = rotateClientWebhookKey(id, {
    viewerUserType: access.userType,
    viewerClientId: access.clientId,
  })
  if (!updated) {
    throw createError({ statusCode: 404, statusMessage: 'Cliente nao encontrado.' })
  }

  return {
    status: 'success',
    data: updated,
  }
})
