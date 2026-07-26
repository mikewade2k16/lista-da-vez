import type { createApiRequest } from '~/utils/api-client'
import type {
  CustomerDataCapabilityKey,
  CustomerDataCapabilityWriteInput,
  CustomerDataControlState,
  CustomerDataWriterKey,
  CustomerDataWriterWriteInput,
} from '~/domain/customer-intelligence/control-plane-types'

type ControlPlaneApi = ReturnType<typeof createApiRequest>

function clientQuery(clientAccountId: string): string {
  const query = new URLSearchParams({
    clientAccountId: clientAccountId.trim(),
  })
  return query.toString()
}

export function fetchCustomerDataControlState(
  api: ControlPlaneApi,
  clientAccountId: string,
  signal?: AbortSignal,
): Promise<CustomerDataControlState> {
  return api(`/v1/customer-data/control-state?${clientQuery(clientAccountId)}`, {
    signal,
    dedupe: false,
  }) as Promise<CustomerDataControlState>
}

export function updateCustomerDataCapability(
  api: ControlPlaneApi,
  clientAccountId: string,
  key: CustomerDataCapabilityKey,
  input: CustomerDataCapabilityWriteInput,
): Promise<unknown> {
  return api(
    `/v1/customer-data/capabilities/${encodeURIComponent(key)}?${clientQuery(clientAccountId)}`,
    { method: 'PUT', body: input },
  )
}

export function updateCustomerDataWriter(
  api: ControlPlaneApi,
  clientAccountId: string,
  key: CustomerDataWriterKey,
  input: CustomerDataWriterWriteInput,
): Promise<unknown> {
  return api(
    `/v1/customer-data/writer-states/${encodeURIComponent(key)}?${clientQuery(clientAccountId)}`,
    { method: 'PUT', body: input },
  )
}
