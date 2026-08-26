import { beforeEach, describe, expect, it, vi } from 'vitest'

const authStore = {
  role: '',
  effectivePermissionKeys: [] as string[],
}

vi.mock('~/stores/auth', () => ({
  useAuthStore: () => authStore,
}))

import { useCan } from './useCan'

describe('useCan', () => {
  beforeEach(() => {
    authStore.role = ''
    authStore.effectivePermissionKeys = []
  })

  it('allows platform administrators through the backend-equivalent bypass', () => {
    authStore.role = 'platform_admin'

    expect(useCan('tasks.boards.manage').value).toBe(true)
  })

  it('uses permissions resolved for the active account', () => {
    authStore.effectivePermissionKeys = ['tasks.boards.manage']

    expect(useCan('tasks.boards.manage').value).toBe(true)
    expect(useCan('tasks.tracking.view_all').value).toBe(false)
  })
})
