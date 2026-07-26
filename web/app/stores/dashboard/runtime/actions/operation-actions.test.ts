import { describe, expect, it } from 'vitest'

import { createAppStore } from '~/stores/dashboard/runtime/create-dashboard-runtime'
import { createEmptyState } from '~/stores/dashboard/runtime/state'

describe('operation actions', () => {
  it('opens the finish modal for a pending validation supplied by the integrated overview', () => {
    const store = createAppStore(createEmptyState())
    const pending = {
      serviceId: 'service-pending-overview',
      storeId: 'store-from-overview',
      storeName: 'Loja integrada',
      personId: 'consultant-1',
      personName: 'Consultora',
      startedAt: 1000,
      finishedAt: 2000,
    }

    expect(store.openFinishModal(pending.serviceId, pending)).toBe(true)
    expect(store.getState().finishModalServiceId).toBe(pending.serviceId)
    expect(store.getState().finishModalPendingValidation).toEqual(pending)

    store.hydrate(store.getState())

    expect(store.getState().finishModalServiceId).toBe(pending.serviceId)
    expect(store.getState().finishModalPendingValidation).toEqual(pending)
  })

  it('keeps the pending list open when the service cannot be resolved', () => {
    const store = createAppStore(createEmptyState())

    expect(store.openFinishModal('missing-service')).toBe(false)
    expect(store.getState().finishModalServiceId).toBeNull()
  })
})
