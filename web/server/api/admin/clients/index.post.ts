import { readBody, setResponseStatus } from 'h3'
import { assertReferenceAdmin } from '~~/server/utils/reference-admin-access'
import { createClient } from '~~/server/utils/clients-repository'

export default defineEventHandler(async (event) => {
  assertReferenceAdmin(event)

  const body = await readBody<{
    name?: string
    status?: string
    adminName?: string
    adminEmail?: string
    adminPassword?: string
  }>(event)

  const created = createClient({
    name: body?.name,
    status: body?.status,
  })

  setResponseStatus(event, 201)

  return {
    status: 'success',
    data: created,
  }
})
