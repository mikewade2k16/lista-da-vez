import type { ApiRequest } from '~/domain/omnichannel/config-api'
import type { OmniCapabilities, OmniChannelLimit } from '~/domain/omnichannel/config-types'

const WA = '/v1/omnichannel/tenant/whatsapp'

export interface OmniInstanceHistoryResetInput {
  confirmation: string
  reason?: string
  expectedRevision: number
}

export interface OmniInstanceHistoryResetResult {
  instanceId: string
  hiddenBefore: string
  resetRevision: number
}

export function deleteInstance(api: ApiRequest, id: string): Promise<void> {
  return api(`${WA}/instances/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  }) as Promise<void>
}

export function updateChannelLimit(
  api: ApiRequest,
  maxChannels: number,
): Promise<OmniChannelLimit> {
  return api(`${WA}/limits`, {
    method: 'PUT',
    body: { maxChannels },
  }) as Promise<OmniChannelLimit>
}

export function resetInstanceHistory(
  api: ApiRequest,
  id: string,
  input: OmniInstanceHistoryResetInput,
): Promise<OmniInstanceHistoryResetResult> {
  const reason = input.reason?.trim()
  return api(`${WA}/instances/${encodeURIComponent(id)}/history/reset`, {
    method: 'POST',
    body: {
      confirmation: input.confirmation.trim(),
      expectedRevision: input.expectedRevision,
      ...(reason ? { reason } : {}),
    },
  }) as Promise<OmniInstanceHistoryResetResult>
}

export function fetchCapabilities(api: ApiRequest, id: string): Promise<OmniCapabilities | null> {
  return (
    api(`${WA}/instances/${encodeURIComponent(id)}/capabilities`, {
      dedupe: false,
    }) as Promise<OmniCapabilities>
  )
    .then((capabilities) => capabilities)
    .catch(() => null)
}
