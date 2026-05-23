import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { useOperationsStore } from './operations'

describe('useOperationsStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('rejects operational commands when there is no authenticated active store', async () => {
    const store = useOperationsStore()

    await expect(store.addToQueue('person-1')).resolves.toEqual({
      ok: false,
      message: 'Sessao ou loja ativa indisponivel.',
    })
  })
})