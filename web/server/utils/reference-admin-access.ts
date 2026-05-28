import { createError, getHeader, type H3Event } from 'h3'

export interface ReferenceAdminAccess {
  userType: 'admin' | 'client'
  clientId: number
}

function parseClientId(value: unknown) {
  const parsed = Number.parseInt(String(value ?? '').trim(), 10)
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return 0
  }

  return parsed
}

export function resolveReferenceAdminAccess(event: H3Event): ReferenceAdminAccess {
  const userType =
    String(getHeader(event, 'x-user-type') ?? '')
      .trim()
      .toLowerCase() === 'admin'
      ? 'admin'
      : 'client'

  return {
    userType,
    clientId: parseClientId(getHeader(event, 'x-client-id')),
  }
}

export function assertReferenceAdmin(event: H3Event) {
  const access = resolveReferenceAdminAccess(event)

  if (access.userType !== 'admin') {
    throw createError({ statusCode: 403, statusMessage: 'Acesso restrito ao admin da plataforma.' })
  }

  return access
}
