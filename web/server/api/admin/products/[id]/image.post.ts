import { createError, getHeader, getRouterParam, readMultipartFormData } from 'h3'
import { assertReferenceAdmin } from '~~/server/utils/reference-admin-access'
import { updateProductField } from '~~/server/utils/products-repository'

const allowedMimeTypes = new Set(['image/jpeg', 'image/png', 'image/webp', 'image/gif'])
const MAX_BYTES = 8 * 1024 * 1024

export default defineEventHandler(async (event) => {
  const access = assertReferenceAdmin(event)
  const contentType = String(getHeader(event, 'content-type') ?? '').toLowerCase()
  if (!contentType.includes('multipart/form-data')) {
    throw createError({
      statusCode: 415,
      statusMessage: 'Envie a imagem como multipart/form-data.',
    })
  }

  const rawContentLength = Number.parseInt(String(getHeader(event, 'content-length') ?? ''), 10)
  if (Number.isFinite(rawContentLength) && rawContentLength > MAX_BYTES + 128 * 1024) {
    throw createError({ statusCode: 413, statusMessage: 'Arquivo muito grande. Maximo de 8MB.' })
  }

  const id = Number.parseInt(String(getRouterParam(event, 'id') ?? ''), 10)
  if (!Number.isFinite(id) || id <= 0) {
    throw createError({ statusCode: 400, statusMessage: 'Product id invalido.' })
  }

  const parts = await readMultipartFormData(event)
  const filePart = (parts || []).find((part) => part.name === 'file' && part.filename && part.data)
  if (!filePart || !filePart.filename || !filePart.data) {
    throw createError({ statusCode: 400, statusMessage: 'Arquivo de imagem nao enviado.' })
  }

  const mimeType = String(filePart.type || '').toLowerCase()
  if (!allowedMimeTypes.has(mimeType)) {
    throw createError({ statusCode: 400, statusMessage: 'Formato de imagem nao suportado.' })
  }

  if (filePart.data.byteLength > MAX_BYTES) {
    throw createError({ statusCode: 400, statusMessage: 'Arquivo muito grande. Maximo de 8MB.' })
  }

  const dataUrl = `data:${mimeType};base64,${filePart.data.toString('base64')}`
  const updated = updateProductField(id, 'image', dataUrl, {
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
