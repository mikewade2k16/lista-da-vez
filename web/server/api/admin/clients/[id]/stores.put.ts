import { createError, getRouterParam, readBody } from 'h3'
import { assertReferenceAdmin } from '~~/server/utils/reference-admin-access'
import { replaceClientStores } from '~~/server/utils/clients-repository'

interface StoresBody {
  stores?: Array<{ id?: string; name?: string; amount?: number | string }>
}

export default defineEventHandler(async (event) => {
  const access = assertReferenceAdmin(event)
  const id = Number.parseInt(String(getRouterParam(event, 'id') ?? ''), 10)
  if (!Number.isFinite(id) || id <= 0) {
    throw createError({ statusCode: 400, statusMessage: 'Cliente invalido.' })
  }

  const body = await readBody<StoresBody>(event)
  const stores = Array.isArray(body?.stores) ? body.stores : []

  const updated = replaceClientStores(id, stores, {
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
