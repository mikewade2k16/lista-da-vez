import { reactive } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useScriptsManager } from './useScriptsManager'

const mocks = vi.hoisted(() => ({
  api: vi.fn(),
  account: { activeAccountId: 'account-a' },
  auth: { role: 'owner', accessToken: 'test', effectivePermissionKeys: ['tools.scripts.manage'] },
}))
vi.mock('../../layers/core/stores/account', () => ({ useCoreAccountStore: () => mocks.account }))
vi.mock('~/utils/api-client', () => ({
  createApiRequest: () => mocks.api,
  getApiErrorMessage: (_error: unknown, fallback: string) => fallback,
}))

const document = {
  id: 'script-1',
  title: 'Persistido',
  rows: [{ audio: 'áudio', video: 'vídeo' }],
  description: '',
  updatedAt: '2026-11-11T12:00:00.123456Z',
  clientAccountId: '',
  clientName: '',
  clientVisible: true,
  shared: false,
}

describe('documentos de roteiros por conta', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.account = reactive({ activeAccountId: 'account-a' })
    mocks.auth = reactive({
      role: 'owner',
      accessToken: 'test',
      effectivePermissionKeys: ['tools.scripts.manage'],
    })
    vi.stubGlobal('useAuthStore', () => mocks.auth)
  })
  it('não faz leitura nem gravação sem a permissão necessária', async () => {
    mocks.auth.effectivePermissionKeys = []
    const manager = useScriptsManager()
    await manager.load()
    expect(
      await manager.save('', {
        title: 'Roteiro',
        rows: [],
        description: '',
        clientAccountId: '',
        clientVisible: true,
      }),
    ).toBeNull()
    expect(mocks.api).not.toHaveBeenCalled()
  })
  it('ignora uma resposta de leitura da conta anterior', async () => {
    let resolve!: (value: unknown) => void
    mocks.api.mockReturnValueOnce(
      new Promise((done) => {
        resolve = done
      }),
    )
    const manager = useScriptsManager()
    const previous = manager.load()
    mocks.account.activeAccountId = 'account-b'
    mocks.api.mockResolvedValueOnce({ data: [], hasMore: false })
    await manager.load()
    resolve({ data: [document], hasMore: false })
    await previous
    expect(manager.documents.value).toEqual([])
  })
  it('usa a resposta autoritativa e envia o token de concorrência ao salvar', async () => {
    mocks.api.mockResolvedValueOnce({ data: document })
    const manager = useScriptsManager()
    expect(await manager.save(document.id, document)).toEqual(document)
    expect(manager.documents.value).toEqual([document])
    expect(mocks.api).toHaveBeenCalledWith(
      '/v1/tools/scripts/script-1',
      expect.objectContaining({
        method: 'PUT',
        body: document,
        headers: { 'X-Account-Id': 'account-a' },
      }),
    )
  })
})
