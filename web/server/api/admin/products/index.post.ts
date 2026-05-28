import { readBody, setResponseStatus } from 'h3'
import { assertReferenceAdmin } from '~~/server/utils/reference-admin-access'
import { createProduct } from '~~/server/utils/products-repository'

export default defineEventHandler(async (event) => {
  const access = assertReferenceAdmin(event)
  const body = await readBody<{
    name?: string
    code?: string
    image?: string
    clientId?: string | number
    clientName?: string
  }>(event)

  const created = createProduct(
    {
      name: body?.name,
      code: body?.code,
      image: body?.image,
      clientId: body?.clientId,
      clientName: body?.clientName,
    },
    {
      viewerUserType: access.userType,
      viewerClientId: access.clientId,
    },
  )

  setResponseStatus(event, 201)

  return {
    status: 'success',
    data: created,
  }
})
