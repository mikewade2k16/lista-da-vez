import { createApiRequest, getWebSocketBase } from '~/utils/api-client'

type RealtimeQueryValue = boolean | number | string | null | undefined
type RealtimeTicketResponse = {
  ticket?: string
}

function appendRealtimeQuery(url: URL, query: Record<string, RealtimeQueryValue>) {
  Object.entries(query).forEach(([key, value]) => {
    const normalizedValue = String(value ?? '').trim()
    if (!key || !normalizedValue) {
      return
    }

    url.searchParams.set(key, normalizedValue)
  })
}

export async function requestRealtimeTicket(
  runtimeConfig: ReturnType<typeof useRuntimeConfig>,
  accessToken: string,
) {
  const apiRequest = createApiRequest(runtimeConfig, () => accessToken)
  const response = (await apiRequest('/v1/ws/ticket', {
    method: 'POST',
    dedupe: false,
    skipLoadingIndicator: true,
  })) as RealtimeTicketResponse
  const ticket = String(response?.ticket || '').trim()

  if (!ticket) {
    throw new Error('Realtime ticket vazio.')
  }

  return ticket
}

export async function buildRealtimeSocketURL(
  runtimeConfig: ReturnType<typeof useRuntimeConfig>,
  path: string,
  query: Record<string, RealtimeQueryValue>,
  accessToken: string,
) {
  const ticket = await requestRealtimeTicket(runtimeConfig, accessToken)
  const url = new URL(path, getWebSocketBase(runtimeConfig))
  appendRealtimeQuery(url, query)
  url.searchParams.set('ticket', ticket)
  return url.toString()
}
