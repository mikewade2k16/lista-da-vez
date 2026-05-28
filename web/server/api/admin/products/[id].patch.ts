import { createError, getRouterParam, readBody } from 'h3'
import { assertReferenceAdmin } from '~~/server/utils/reference-admin-access'
import { updateProductField } from '~~/server/utils/products-repository'

const allowedFields = new Set([
  'name',
  'code',
  'image',
  'description',
  'categories',
  'categoriesText',
  'campaigns',
  'campaignsText',
  'price',
  'fator',
  'tipo',
  'stock',
  'status',
])

export default defineEventHandler(async (event) => {
  const access = assertReferenceAdmin(event)

  const id = Number.parseInt(String(getRouterParam(event, 'id') ?? ''), 10)
  if (!Number.isFinite(id) || id <= 0) {
    throw createError({ statusCode: 400, statusMessage: 'Product id invalido.' })
  }

  const body = await readBody<Record<string, unknown>>(event)
  const field = String(body?.field ?? '').trim()
  if (!field || !allowedFields.has(field)) {
    throw createError({ statusCode: 400, statusMessage: 'Campo invalido para atualizacao.' })
  }

  const updated = updateProductField(id, field, body?.value, {
    viewerUserType: access.userType,
    viewerClientId: access.clientId,
  })
  if (!updated) {
    throw createError({ statusCode: 404, statusMessage: 'Produto nao encontrado.' })
  }

  return {
    status: 'success',
    data: updated,
  }
})
