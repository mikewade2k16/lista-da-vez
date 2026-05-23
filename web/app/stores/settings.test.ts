import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { useSettingsStore } from './settings'

describe('useSettingsStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('fails fast when trying to persist settings without an authenticated session', async () => {
    const store = useSettingsStore()

    await expect(store.updateSetting('queueLimit', 5)).resolves.toEqual({
      ok: false,
      message: 'Sessao indisponivel.',
    })
  })
})