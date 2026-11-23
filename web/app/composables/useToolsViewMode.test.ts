import { nextTick } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useToolsViewMode } from './useToolsViewMode'

const mocks = vi.hoisted(() => ({
  mount: null as (() => Promise<void>) | null,
  ensureLoaded: vi.fn(),
  readStringArray: vi.fn(),
  writeStringArray: vi.fn(),
}))

vi.mock('vue', async (original) => ({
  ...(await original<typeof import('vue')>()),
  onMounted: (callback: () => Promise<void>) => {
    mocks.mount = callback
  },
}))
vi.mock('~/composables/useAdminPreferences', () => ({
  useAdminPreferences: () => mocks,
}))

describe('visualização de QR Codes e links curtos', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.mount = null
    mocks.ensureLoaded.mockResolvedValue(undefined)
    mocks.readStringArray.mockImplementation((_path, fallback) => fallback)
  })

  it('abre links em tabela e QR em cards quando não há escolha salva', async () => {
    const links = useToolsViewMode('tools.short-links', 'table')
    await mocks.mount?.()
    expect(links.value).toBe('table')
    const qr = useToolsViewMode('tools.qr-codes')
    await mocks.mount?.()
    expect(qr.value).toBe('cards')
  })

  it('restaura a escolha salva mesmo quando difere do padrão da tela', async () => {
    mocks.readStringArray.mockReturnValue(['cards'])
    const links = useToolsViewMode('tools.short-links', 'table')
    await mocks.mount?.()
    expect(links.value).toBe('cards')
  })

  it('salva mudanças com uma chave separada para cada tela', async () => {
    const qr = useToolsViewMode('tools.qr-codes')
    await mocks.mount?.()
    qr.value = 'table'
    await nextTick()
    expect(mocks.writeStringArray).toHaveBeenCalledWith(
      ['ui', 'view_mode', 'tools.qr-codes'],
      ['table'],
    )
    const links = useToolsViewMode('tools.short-links', 'table')
    await mocks.mount?.()
    links.value = 'cards'
    await nextTick()
    expect(mocks.writeStringArray).toHaveBeenCalledWith(
      ['ui', 'view_mode', 'tools.short-links'],
      ['cards'],
    )
  })
})
