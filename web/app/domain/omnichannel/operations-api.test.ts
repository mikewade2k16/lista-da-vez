import { describe, expect, it, vi } from 'vitest'
import type { ApiRequest } from './config-api'
import { fetchOperationalHealth, fetchRolloutConfig, putRolloutConfig } from './operations-api'

describe('omnichannel operations API', () => {
  it('uses protected tenant-scoped health and rollout routes', async () => {
    const request = vi.fn().mockResolvedValue({})
    const api = request as unknown as ApiRequest

    await fetchOperationalHealth(api)
    await fetchRolloutConfig(api)
    await putRolloutConfig(api, {
      mode: 'paused',
      allowedInstanceIds: [],
      allowedInstagramAccountIds: [],
      allowedQueueIds: [],
      autoReplyPercent: 0,
      allowedHours: { timezone: 'America/Sao_Paulo', windows: [] },
      excludedTags: [],
      maxDailyAutoReplies: 0,
      killSwitchReason: 'incidente controlado',
      expectedRevision: 2,
      reason: 'pausa para investigação',
    })

    expect(request.mock.calls).toEqual([
      ['/v1/omnichannel/operations/health', { dedupe: false }],
      ['/v1/omnichannel/settings/rollout', { dedupe: false }],
      [
        '/v1/omnichannel/settings/rollout',
        {
          method: 'PUT',
          body: expect.objectContaining({ mode: 'paused', expectedRevision: 2 }),
        },
      ],
    ])
  })
})
