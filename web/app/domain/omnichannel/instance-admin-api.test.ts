import { describe, expect, it, vi } from 'vitest'
import type { ApiRequest } from './config-api'
import { resetInstanceHistory } from './instance-admin-api'

describe('resetInstanceHistory', () => {
  it('posts the selected encoded instance with confirmation and optimistic revision', async () => {
    const request = vi.fn().mockResolvedValue({
      instanceId: 'instance/a',
      hiddenBefore: '2026-08-27T12:00:00.000Z',
      resetRevision: 8,
    })

    await resetInstanceHistory(request as unknown as ApiRequest, 'instance/a', {
      confirmation: '  crow-principal  ',
      reason: '  troca de operação  ',
      expectedRevision: 7,
    })

    expect(request).toHaveBeenCalledTimes(1)
    expect(request).toHaveBeenCalledWith(
      '/v1/omnichannel/tenant/whatsapp/instances/instance%2Fa/history/reset',
      {
        method: 'POST',
        body: {
          confirmation: 'crow-principal',
          reason: 'troca de operação',
          expectedRevision: 7,
        },
      },
    )
  })
})
